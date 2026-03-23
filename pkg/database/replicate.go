package database

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/benbjohnson/litestream"
	"github.com/pkg/sftp"
	"github.com/superfly/ltx"
	"golang.org/x/crypto/ssh"

	"monks.co/pkg/tailnet"
)

// Replication manages a litestream replication session. Close it when the
// database is being shut down.
type Replication struct {
	store *litestream.Store
}

// Close stops the litestream replication session.
func (r *Replication) Close() error {
	return r.store.Close(context.Background())
}

// StartReplication sets up litestream WAL replication for the given database
// path over SFTP to monks-vault-thor. The caller must call Close on the
// returned Replication when the database is shut down. The tailnet must be
// ready before calling this.
func StartReplication(ctx context.Context, dbPath string) (*Replication, error) {
	lsDB := litestream.NewDB(dbPath)

	client := newVaultClient(filepath.Base(dbPath))
	replica := litestream.NewReplicaWithClient(lsDB, client)
	lsDB.Replica = replica
	client.replica = replica

	levels := litestream.CompactionLevels{
		{Level: 0},
		{Level: 1, Interval: 10 * time.Second},
	}

	store := litestream.NewStore([]*litestream.DB{lsDB}, levels)
	if err := store.Open(ctx); err != nil {
		return nil, fmt.Errorf("opening litestream store: %w", err)
	}
	return &Replication{store: store}, nil
}

// vaultClient implements litestream.ReplicaClient, replicating over SFTP
// through the tailnet to monks-vault-thor.
type vaultClient struct {
	mu         sync.Mutex
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	dbName     string
	replica    *litestream.Replica
}

func newVaultClient(dbName string) *vaultClient {
	return &vaultClient{dbName: dbName}
}

func (c *vaultClient) Type() string { return "vault" }

func (c *vaultClient) Init(ctx context.Context) error {
	_, err := c.connect(ctx)
	return err
}

func (c *vaultClient) connect(ctx context.Context) (*sftp.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sftpClient != nil {
		return c.sftpClient, nil
	}

	conn, err := tailnet.Dial(ctx, "tcp", "monks-vault-thor:22")
	if err != nil {
		return nil, fmt.Errorf("vault dial: %w", err)
	}

	config := &ssh.ClientConfig{
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, "monks-vault-thor:22", config)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("vault ssh: %w", err)
	}
	c.sshClient = ssh.NewClient(sshConn, chans, reqs)

	c.sftpClient, err = sftp.NewClient(c.sshClient, sftp.UseConcurrentWrites(true))
	if err != nil {
		c.sshClient.Close()
		c.sshClient = nil
		return nil, fmt.Errorf("vault sftp: %w", err)
	}

	return c.sftpClient, nil
}

func (c *vaultClient) resetOnConnError(err error) {
	if !errors.Is(err, sftp.ErrSSHFxConnectionLost) {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sftpClient != nil {
		c.sftpClient.Close()
		c.sftpClient = nil
	}
	if c.sshClient != nil {
		c.sshClient.Close()
		c.sshClient = nil
	}
}

func (c *vaultClient) LTXFiles(ctx context.Context, level int, seek ltx.TXID, _ bool) (_ ltx.FileIterator, err error) {
	defer func() { c.resetOnConnError(err) }()

	sftpClient, err := c.connect(ctx)
	if err != nil {
		return nil, err
	}

	dir := litestream.LTXLevelDir(c.dbName, level)
	fis, err := sftpClient.ReadDir(dir)
	if os.IsNotExist(err) {
		return ltx.NewFileInfoSliceIterator(nil), nil
	} else if err != nil {
		return nil, err
	}

	infos := make([]*ltx.FileInfo, 0, len(fis))
	for _, fi := range fis {
		minTXID, maxTXID, err := ltx.ParseFilename(path.Base(fi.Name()))
		if err != nil {
			continue
		} else if minTXID < seek {
			continue
		}
		infos = append(infos, &ltx.FileInfo{
			Level:     level,
			MinTXID:   minTXID,
			MaxTXID:   maxTXID,
			Size:      fi.Size(),
			CreatedAt: fi.ModTime().UTC(),
		})
	}

	return ltx.NewFileInfoSliceIterator(infos), nil
}

func (c *vaultClient) OpenLTXFile(ctx context.Context, level int, minTXID, maxTXID ltx.TXID, offset, size int64) (_ io.ReadCloser, err error) {
	defer func() { c.resetOnConnError(err) }()

	sftpClient, err := c.connect(ctx)
	if err != nil {
		return nil, err
	}

	filename := litestream.LTXFilePath(c.dbName, level, minTXID, maxTXID)
	f, err := sftpClient.Open(filename)
	if err != nil {
		return nil, err
	}

	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			f.Close()
			return nil, err
		}
	}

	if size > 0 {
		return limitReadCloser(f, size), nil
	}
	return f, nil
}

func (c *vaultClient) WriteLTXFile(ctx context.Context, level int, minTXID, maxTXID ltx.TXID, rd io.Reader) (info *ltx.FileInfo, err error) {
	defer func() { c.resetOnConnError(err) }()

	sftpClient, err := c.connect(ctx)
	if err != nil {
		return nil, err
	}

	filename := litestream.LTXFilePath(c.dbName, level, minTXID, maxTXID)

	// Peek at LTX header to extract timestamp.
	var buf [ltx.HeaderSize]byte
	if _, err := io.ReadFull(rd, buf[:]); err != nil {
		return nil, fmt.Errorf("reading LTX header: %w", err)
	}
	var hdr ltx.Header
	if err := hdr.UnmarshalBinary(buf[:]); err != nil {
		return nil, fmt.Errorf("parsing LTX header: %w", err)
	}
	timestamp := time.UnixMilli(hdr.Timestamp).UTC()

	fullReader := io.MultiReader(io.NopCloser(readerFromBytes(buf[:])), rd)

	if err := sftpClient.MkdirAll(path.Dir(filename)); err != nil {
		return nil, fmt.Errorf("sftp mkdir: %w", err)
	}

	f, err := sftpClient.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return nil, fmt.Errorf("sftp open: %w", err)
	}
	defer f.Close()

	n, err := io.Copy(f, fullReader)
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}

	if err := sftpClient.Chtimes(filename, timestamp, timestamp); err != nil {
		return nil, fmt.Errorf("sftp chtimes: %w", err)
	}

	return &ltx.FileInfo{
		Level:     level,
		MinTXID:   minTXID,
		MaxTXID:   maxTXID,
		Size:      n,
		CreatedAt: timestamp,
	}, nil
}

func (c *vaultClient) DeleteLTXFiles(ctx context.Context, a []*ltx.FileInfo) (err error) {
	defer func() { c.resetOnConnError(err) }()

	sftpClient, err := c.connect(ctx)
	if err != nil {
		return err
	}

	for _, info := range a {
		filename := litestream.LTXFilePath(c.dbName, info.Level, info.MinTXID, info.MaxTXID)
		if err := sftpClient.Remove(filename); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("sftp remove %q: %w", filename, err)
		}
	}
	return nil
}

func (c *vaultClient) DeleteAll(ctx context.Context) (err error) {
	defer func() { c.resetOnConnError(err) }()

	sftpClient, err := c.connect(ctx)
	if err != nil {
		return err
	}

	var dirs []string
	walker := sftpClient.Walk(c.dbName)
	for walker.Step() {
		if err := walker.Err(); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return fmt.Errorf("sftp walk %q: %w", walker.Path(), err)
		}
		if walker.Stat().IsDir() {
			dirs = append(dirs, walker.Path())
			continue
		}
		if err := sftpClient.Remove(walker.Path()); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("sftp remove %q: %w", walker.Path(), err)
		}
	}

	for i := len(dirs) - 1; i >= 0; i-- {
		if err := sftpClient.RemoveDirectory(dirs[i]); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("sftp rmdir %q: %w", dirs[i], err)
		}
	}

	return nil
}

// limitReadCloser wraps a ReadCloser with a size limit.
type limitedReadCloser struct {
	io.Reader
	io.Closer
}

func limitReadCloser(rc io.ReadCloser, n int64) io.ReadCloser {
	return &limitedReadCloser{
		Reader: io.LimitReader(rc, n),
		Closer: rc,
	}
}

// readerFromBytes returns a reader over a byte slice.
func readerFromBytes(b []byte) io.Reader {
	return &bytesReader{data: b}
}

type bytesReader struct {
	data []byte
	off  int
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.off >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.off:])
	r.off += n
	return n, nil
}
