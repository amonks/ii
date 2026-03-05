package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/sftp"
)

func TestRootedFS_Filewrite_and_Fileread(t *testing.T) {
	root := t.TempDir()
	fs := &rootedFS{root: root}

	// Write a file.
	wr, err := fs.Filewrite(&sftp.Request{Method: "Put", Filepath: "/subdir/test.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wr.WriteAt([]byte("hello"), 0); err != nil {
		t.Fatal(err)
	}
	if closer, ok := wr.(io.Closer); ok {
		closer.Close()
	}

	// Verify the file landed at root/subdir/test.txt.
	data, err := os.ReadFile(filepath.Join(root, "subdir", "test.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("got %q, want %q", data, "hello")
	}

	// Read it back through the handler.
	rd, err := fs.Fileread(&sftp.Request{Method: "Get", Filepath: "/subdir/test.txt"})
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 5)
	n, err := rd.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if string(buf[:n]) != "hello" {
		t.Fatalf("got %q, want %q", buf[:n], "hello")
	}
	if closer, ok := rd.(io.Closer); ok {
		closer.Close()
	}
}

func TestRootedFS_Filecmd_Mkdir_Remove(t *testing.T) {
	root := t.TempDir()
	fs := &rootedFS{root: root}

	// Mkdir
	if err := fs.Filecmd(&sftp.Request{Method: "Mkdir", Filepath: "/a/b/c"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(root, "a", "b", "c"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}

	// Create a file in it to test Remove.
	os.WriteFile(filepath.Join(root, "a", "b", "c", "file"), []byte("x"), 0644)
	if err := fs.Filecmd(&sftp.Request{Method: "Remove", Filepath: "/a/b/c/file"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "a", "b", "c", "file")); !os.IsNotExist(err) {
		t.Fatal("expected file to be removed")
	}
}

func TestRootedFS_Filecmd_Setstat_Noop(t *testing.T) {
	root := t.TempDir()
	fs := &rootedFS{root: root}

	p := filepath.Join(root, "ts.txt")
	os.WriteFile(p, []byte("data"), 0644)

	// Without the acmodtime flag, Setstat is a no-op.
	req := &sftp.Request{Method: "Setstat", Filepath: "/ts.txt"}
	if err := fs.Filecmd(req); err != nil {
		t.Fatal(err)
	}
}

func TestRootedFS_Filelist(t *testing.T) {
	root := t.TempDir()
	fs := &rootedFS{root: root}

	os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(root, "b.txt"), []byte("bb"), 0644)

	// List
	la, err := fs.Filelist(&sftp.Request{Method: "List", Filepath: "/"})
	if err != nil {
		t.Fatal(err)
	}
	infos := make([]os.FileInfo, 10)
	n, _ := la.ListAt(infos, 0)
	if n != 2 {
		t.Fatalf("got %d entries, want 2", n)
	}

	// Stat
	la, err = fs.Filelist(&sftp.Request{Method: "Stat", Filepath: "/a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	statInfos := make([]os.FileInfo, 1)
	n, _ = la.ListAt(statInfos, 0)
	if n != 1 || statInfos[0].Name() != "a.txt" {
		t.Fatalf("got %v", statInfos[:n])
	}
}

func TestRootedFS_resolve_prevents_traversal(t *testing.T) {
	root := t.TempDir()
	fs := &rootedFS{root: root}

	got := fs.resolve("../../etc/passwd")
	want := filepath.Join(root, "etc", "passwd")
	if got != want {
		t.Fatalf("resolve(../../etc/passwd) = %q, want %q", got, want)
	}
}
