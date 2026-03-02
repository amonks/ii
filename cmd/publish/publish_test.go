package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTopoSort(t *testing.T) {
	t.Run("no deps", func(t *testing.T) {
		public := map[string]bool{
			"pkg/a": true,
			"pkg/b": true,
		}
		graph := map[string][]string{
			"pkg/a": {},
			"pkg/b": {},
		}
		order, err := topoSort(public, graph)
		if err != nil {
			t.Fatal(err)
		}
		if len(order) != 2 {
			t.Fatalf("expected 2 items, got %d", len(order))
		}
		// Alphabetical when no deps.
		if order[0] != "pkg/a" || order[1] != "pkg/b" {
			t.Errorf("expected [pkg/a pkg/b], got %v", order)
		}
	})

	t.Run("linear chain", func(t *testing.T) {
		public := map[string]bool{
			"pkg/a": true,
			"pkg/b": true,
			"pkg/c": true,
		}
		// c depends on b, b depends on a.
		graph := map[string][]string{
			"pkg/a": {},
			"pkg/b": {"pkg/a"},
			"pkg/c": {"pkg/b"},
		}
		order, err := topoSort(public, graph)
		if err != nil {
			t.Fatal(err)
		}
		if len(order) != 3 {
			t.Fatalf("expected 3 items, got %d", len(order))
		}
		// a before b, b before c.
		indexOf := map[string]int{}
		for i, d := range order {
			indexOf[d] = i
		}
		if indexOf["pkg/a"] >= indexOf["pkg/b"] {
			t.Errorf("pkg/a should come before pkg/b: %v", order)
		}
		if indexOf["pkg/b"] >= indexOf["pkg/c"] {
			t.Errorf("pkg/b should come before pkg/c: %v", order)
		}
	})

	t.Run("ignores private deps", func(t *testing.T) {
		public := map[string]bool{
			"pkg/a": true,
			"pkg/c": true,
		}
		// c depends on b (private), b depends on a.
		graph := map[string][]string{
			"pkg/a": {},
			"pkg/b": {"pkg/a"},
			"pkg/c": {"pkg/b"},
		}
		order, err := topoSort(public, graph)
		if err != nil {
			t.Fatal(err)
		}
		if len(order) != 2 {
			t.Fatalf("expected 2 items, got %d", len(order))
		}
	})

	t.Run("cycle detection", func(t *testing.T) {
		public := map[string]bool{
			"pkg/a": true,
			"pkg/b": true,
		}
		graph := map[string][]string{
			"pkg/a": {"pkg/b"},
			"pkg/b": {"pkg/a"},
		}
		_, err := topoSort(public, graph)
		if err == nil {
			t.Fatal("expected cycle error")
		}
	})
}

func TestMirrorTag(t *testing.T) {
	tests := []struct {
		monorepoTag string
		dir         string
		want        string
	}{
		{"pkg/serve/v1.0.0", "pkg/serve", "v1.0.0"},
		{"pkg/serve/v0.2.3", "pkg/serve", "v0.2.3"},
		{"cmd/run/v2.0.0-beta.1", "cmd/run", "v2.0.0-beta.1"},
	}
	for _, tt := range tests {
		got := mirrorTag(tt.monorepoTag, tt.dir)
		if got != tt.want {
			t.Errorf("mirrorTag(%q, %q) = %q, want %q", tt.monorepoTag, tt.dir, got, tt.want)
		}
	}
}

func TestGitEnv(t *testing.T) {
	t.Run("no jj dir", func(t *testing.T) {
		root := t.TempDir()
		env := gitEnv(root)
		// Should just return os.Environ() — no GIT_DIR added.
		for _, e := range env {
			if len(e) >= 8 && e[:8] == "GIT_DIR=" {
				t.Errorf("should not set GIT_DIR when no .jj dir: %s", e)
			}
		}
	})

	t.Run("with jj dir", func(t *testing.T) {
		root := t.TempDir()
		gitDir := filepath.Join(root, ".jj", "repo", "store", "git")
		os.MkdirAll(gitDir, 0755)

		env := gitEnv(root)
		var foundGitDir, foundWorkTree bool
		for _, e := range env {
			if e == "GIT_DIR="+gitDir {
				foundGitDir = true
			}
			if e == "GIT_WORK_TREE="+root {
				foundWorkTree = true
			}
		}
		if !foundGitDir {
			t.Error("expected GIT_DIR to be set")
		}
		if !foundWorkTree {
			t.Error("expected GIT_WORK_TREE to be set")
		}
	})
}

func TestCloneSource(t *testing.T) {
	t.Run("no jj dir", func(t *testing.T) {
		root := t.TempDir()
		src := cloneSource(root)
		if src != root {
			t.Errorf("expected %s, got %s", root, src)
		}
	})

	t.Run("with jj dir", func(t *testing.T) {
		root := t.TempDir()
		gitDir := filepath.Join(root, ".jj", "repo", "store", "git")
		os.MkdirAll(gitDir, 0755)

		src := cloneSource(root)
		if src != gitDir {
			t.Errorf("expected %s, got %s", gitDir, src)
		}
	})
}

func TestPackageByDir(t *testing.T) {
	cfg := &PublishConfig{
		DefaultMirror: "github.com/amonks/go",
		Package: []PublishPackage{
			{Dir: "pkg/set"},
			{Dir: "cmd/run", Mirror: "github.com/amonks/run"},
		},
	}

	t.Run("found without mirror", func(t *testing.T) {
		pkg := cfg.PackageByDir("pkg/set")
		if pkg == nil {
			t.Fatal("expected to find pkg/set")
		}
		if pkg.Mirror != "" {
			t.Errorf("expected no explicit mirror, got %s", pkg.Mirror)
		}
	})

	t.Run("found with mirror", func(t *testing.T) {
		pkg := cfg.PackageByDir("cmd/run")
		if pkg == nil {
			t.Fatal("expected to find cmd/run")
		}
		if pkg.Mirror != "github.com/amonks/run" {
			t.Errorf("expected github.com/amonks/run, got %s", pkg.Mirror)
		}
	})

	t.Run("not found", func(t *testing.T) {
		pkg := cfg.PackageByDir("pkg/nope")
		if pkg != nil {
			t.Error("expected nil for unknown dir")
		}
	})
}
