package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/pkg/sftp"

	"monks.co/pkg/env"
	"monks.co/pkg/meta"
	"monks.co/pkg/reqlog"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
)

func main() {
	if err := run(); err != nil {
		if !errors.Is(err, context.Canceled) {
			slog.Error("fatal", "error", err.Error(), "app.name", meta.AppName())
		}
		reqlog.Shutdown()
		os.Exit(1)
	}
}

func run() error {
	reqlog.SetupLogging()

	root := os.Getenv("VAULT_ROOT")
	if root == "" {
		return fmt.Errorf("VAULT_ROOT not set")
	}

	ctx := sigctx.New()
	if err := tailnet.WaitReady(ctx); err != nil {
		return fmt.Errorf("tailnet: %w", err)
	}

	ln, err := tailnet.Listen("tcp", ":22")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()

	srv, err := wish.NewServer(
		wish.WithHostKeyPath(env.InMonksData("vault_host_key")),
		wish.WithSubsystem("sftp", func(s ssh.Session) {
			handleSFTP(s, root)
		}),
	)
	if err != nil {
		return fmt.Errorf("wish server: %w", err)
	}
	// No auth handlers configured → charmbracelet/ssh sets NoClientAuth = true.

	slog.Info("vault listening")

	errs := make(chan error, 1)
	go func() {
		errs <- srv.Serve(ln)
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}

func handleSFTP(s ssh.Session, root string) {
	who, err := tailnet.WhoIs(s.Context(), s.RemoteAddr().String())
	if err != nil {
		slog.Error("whois failed", "error", err, "remote", s.RemoteAddr())
		return
	}

	node := who.Node
	hasTag := slices.Contains(node.Tags, "tag:monks-co")
	if !hasTag {
		slog.Error("unauthorized peer", "node", node.ComputedName, "tags", node.Tags)
		return
	}

	// Strip the tailnet domain suffix to get the short machine name.
	machineName := node.ComputedName
	if machineName == "" {
		machineName = strings.TrimSuffix(node.Name, ".")
		if i := strings.IndexByte(machineName, '.'); i > 0 {
			machineName = machineName[:i]
		}
	}

	peerRoot := filepath.Join(root, machineName)
	if err := os.MkdirAll(peerRoot, 0755); err != nil {
		slog.Error("mkdir failed", "error", err, "path", peerRoot)
		return
	}

	slog.Info("sftp session", "peer", machineName)

	handler := &rootedFS{root: peerRoot}
	srv := sftp.NewRequestServer(s, sftp.Handlers{
		FileGet:  handler,
		FilePut:  handler,
		FileCmd:  handler,
		FileList: handler,
	})
	if err := srv.Serve(); err != nil && err != io.EOF {
		slog.Error("sftp serve error", "error", err, "peer", machineName)
	}
}

// rootedFS implements sftp.Handlers interfaces, restricting all file
// operations to a root directory.
type rootedFS struct {
	root string
}

func (fs *rootedFS) resolve(p string) string {
	return filepath.Join(fs.root, filepath.Clean("/"+p))
}

func (fs *rootedFS) Fileread(r *sftp.Request) (io.ReaderAt, error) {
	f, err := os.Open(fs.resolve(r.Filepath))
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (fs *rootedFS) Filewrite(r *sftp.Request) (io.WriterAt, error) {
	p := fs.resolve(r.Filepath)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (fs *rootedFS) Filecmd(r *sftp.Request) error {
	switch r.Method {
	case "Setstat":
		if r.AttrFlags().Acmodtime {
			attrs := r.Attributes()
			atime := time.Unix(int64(attrs.Atime), 0)
			mtime := time.Unix(int64(attrs.Mtime), 0)
			return os.Chtimes(fs.resolve(r.Filepath), atime, mtime)
		}
		return nil
	case "Rename":
		return os.Rename(fs.resolve(r.Filepath), fs.resolve(r.Target))
	case "Rmdir":
		return os.Remove(fs.resolve(r.Filepath))
	case "Remove":
		return os.Remove(fs.resolve(r.Filepath))
	case "Mkdir":
		return os.MkdirAll(fs.resolve(r.Filepath), 0755)
	}
	return sftp.ErrSSHFxOpUnsupported
}

func (fs *rootedFS) Filelist(r *sftp.Request) (sftp.ListerAt, error) {
	p := fs.resolve(r.Filepath)
	switch r.Method {
	case "List":
		entries, err := os.ReadDir(p)
		if err != nil {
			return nil, err
		}
		infos := make([]os.FileInfo, len(entries))
		for i, e := range entries {
			info, err := e.Info()
			if err != nil {
				return nil, err
			}
			infos[i] = info
		}
		return listerAt(infos), nil
	case "Stat":
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		return listerAt([]os.FileInfo{info}), nil
	}
	return nil, sftp.ErrSSHFxOpUnsupported
}

type listerAt []os.FileInfo

func (l listerAt) ListAt(ls []os.FileInfo, offset int64) (int, error) {
	if offset >= int64(len(l)) {
		return 0, io.EOF
	}
	n := copy(ls, l[offset:])
	if n+int(offset) >= len(l) {
		return n, io.EOF
	}
	return n, nil
}
