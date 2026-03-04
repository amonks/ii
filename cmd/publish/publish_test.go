package main

import (
	"os"
	"path/filepath"
	"testing"

	"monks.co/pkg/ci/publish"
	"monks.co/pkg/depgraph"
	"monks.co/pkg/env"
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
		order, err := publish.TopoSort(public, graph)
		if err != nil {
			t.Fatal(err)
		}
		if len(order) != 2 {
			t.Fatalf("expected 2 items, got %d", len(order))
		}
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
		graph := map[string][]string{
			"pkg/a": {},
			"pkg/b": {"pkg/a"},
			"pkg/c": {"pkg/b"},
		}
		order, err := publish.TopoSort(public, graph)
		if err != nil {
			t.Fatal(err)
		}
		if len(order) != 3 {
			t.Fatalf("expected 3 items, got %d", len(order))
		}
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
		graph := map[string][]string{
			"pkg/a": {},
			"pkg/b": {"pkg/a"},
			"pkg/c": {"pkg/b"},
		}
		order, err := publish.TopoSort(public, graph)
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
		_, err := publish.TopoSort(public, graph)
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
		got := publish.MirrorTag(tt.monorepoTag, tt.dir)
		if got != tt.want {
			t.Errorf("MirrorTag(%q, %q) = %q, want %q", tt.monorepoTag, tt.dir, got, tt.want)
		}
	}
}

func TestGitEnv(t *testing.T) {
	t.Run("no jj dir", func(t *testing.T) {
		root := t.TempDir()
		env := publish.GitEnv(root)
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

		env := publish.GitEnv(root)
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
		src := publish.CloneSource(root)
		if src != root {
			t.Errorf("expected %s, got %s", root, src)
		}
	})

	t.Run("with jj dir", func(t *testing.T) {
		root := t.TempDir()
		gitDir := filepath.Join(root, ".jj", "repo", "store", "git")
		os.MkdirAll(gitDir, 0755)

		src := publish.CloneSource(root)
		if src != gitDir {
			t.Errorf("expected %s, got %s", gitDir, src)
		}
	})
}

func TestPackageByDir(t *testing.T) {
	cfg := &publish.Config{
		DefaultMirror: "github.com/amonks/go",
		Package: []publish.Package{
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

func TestTransitiveDeps(t *testing.T) {
	graph := map[string][]string{
		"pkg/a": {"pkg/b"},
		"pkg/b": {"pkg/c"},
		"pkg/c": {},
	}

	deps := depgraph.TransitiveDeps(graph, "pkg/a")
	if !deps["pkg/b"] || !deps["pkg/c"] {
		t.Errorf("expected pkg/b and pkg/c, got %v", deps)
	}
	if deps["pkg/a"] {
		t.Error("should not include self")
	}
}

func TestResolveImportDir(t *testing.T) {
	modPathToDir := map[string]string{
		"monks.co/pkg/serve": "pkg/serve",
		"monks.co/beetman":   "cmd/beetman",
	}

	tests := []struct {
		importPath string
		wantDir    string
		wantOK     bool
	}{
		{"monks.co/pkg/serve", "pkg/serve", true},
		{"monks.co/pkg/serve/something", "pkg/serve", true},
		{"monks.co/beetman", "cmd/beetman", true},
		{"monks.co/beetman/internal/foo", "cmd/beetman", true},
		{"monks.co/unknown/pkg", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.importPath, func(t *testing.T) {
			dir, ok := depgraph.ResolveImportDir(tt.importPath, modPathToDir)
			if ok != tt.wantOK {
				t.Fatalf("ResolveImportDir(%q) ok = %v, want %v", tt.importPath, ok, tt.wantOK)
			}
			if dir != tt.wantDir {
				t.Errorf("ResolveImportDir(%q) = %q, want %q", tt.importPath, dir, tt.wantDir)
			}
		})
	}
}

func TestBuildDepGraphNonStandardModulePath(t *testing.T) {
	root := t.TempDir()

	// Create workspace so go/packages can resolve cross-module imports.
	os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.26.0\n\nuse (\n\t./cmd/beetman\n\t./pkg/util\n)\n"), 0644)

	beetDir := filepath.Join(root, "cmd", "beetman")
	os.MkdirAll(beetDir, 0755)
	os.WriteFile(filepath.Join(beetDir, "go.mod"),
		[]byte("module monks.co/beetman\n\ngo 1.26.0\n"), 0644)
	os.WriteFile(filepath.Join(beetDir, "main.go"), []byte(`package beetman

import (
	"monks.co/beetman/internal/foo"
	"monks.co/pkg/util"
)

var _ = foo.X
var _ = util.Y
`), 0644)

	utilDir := filepath.Join(root, "pkg", "util")
	os.MkdirAll(utilDir, 0755)
	os.WriteFile(filepath.Join(utilDir, "go.mod"),
		[]byte("module monks.co/pkg/util\n\ngo 1.26.0\n"), 0644)
	os.WriteFile(filepath.Join(utilDir, "util.go"),
		[]byte("package util\n"), 0644)

	graph, err := depgraph.BuildDepGraph(root)
	if err != nil {
		t.Fatal(err)
	}

	deps := graph["cmd/beetman"]

	hasUtil := false
	for _, dep := range deps {
		if dep == "pkg/util" {
			hasUtil = true
		}
		if dep == "beetman/internal" {
			t.Errorf("dep graph should not contain bogus dir %q", dep)
		}
		if dep == "cmd/beetman" {
			t.Error("dep graph should not contain self-reference")
		}
	}
	if !hasUtil {
		t.Errorf("expected cmd/beetman to depend on pkg/util, got %v", deps)
	}
}

func TestBuildDepGraphSelfImport(t *testing.T) {
	root := t.TempDir()

	beetDir := filepath.Join(root, "cmd", "mylib")
	os.MkdirAll(beetDir, 0755)
	os.WriteFile(filepath.Join(beetDir, "go.mod"),
		[]byte("module monks.co/mylib\n\ngo 1.26.0\n"), 0644)
	os.WriteFile(filepath.Join(beetDir, "lib.go"),
		[]byte("package mylib\n"), 0644)

	cliDir := filepath.Join(beetDir, "mylib")
	os.MkdirAll(cliDir, 0755)
	os.WriteFile(filepath.Join(cliDir, "main.go"), []byte(`package main

import "monks.co/mylib"

var _ = mylib.X
`), 0644)

	graph, err := depgraph.BuildDepGraph(root)
	if err != nil {
		t.Fatal(err)
	}

	for _, dep := range graph["cmd/mylib"] {
		if dep == "cmd/mylib" {
			t.Error("dep graph should not contain self-reference")
		}
	}
}

func TestBuildDepGraphReal(t *testing.T) {
	root := env.InMonksRoot()
	graph, err := depgraph.BuildDepGraph(root)
	if err != nil {
		t.Fatal(err)
	}

	if deps := graph["pkg/set"]; len(deps) != 0 {
		t.Errorf("expected pkg/set to have no deps, got %v", deps)
	}

	proxyDeps := graph["apps/proxy"]
	if len(proxyDeps) == 0 {
		t.Error("expected apps/proxy to have dependencies")
	}

	allDirs := map[string]bool{}
	for dir := range graph {
		allDirs[dir] = true
	}
	if _, err := publish.TopoSort(allDirs, graph); err != nil {
		t.Errorf("topoSort of all packages: %v", err)
	}
}

func TestPublishInvariants(t *testing.T) {
	root := env.InMonksRoot()

	cfg, err := publish.LoadConfig(root)
	if err != nil {
		if _, statErr := os.Stat(filepath.Join(root, "config", "publish.toml")); os.IsNotExist(statErr) {
			t.Skip("config/publish.toml not found, skipping")
		}
		t.Fatal(err)
	}

	if len(cfg.Package) == 0 {
		t.Skip("no public packages configured")
	}

	publicDirs := cfg.PublicDirs()

	graph, err := depgraph.BuildDepGraph(root)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("public packages do not depend on private packages", func(t *testing.T) {
		errs := publish.ValidatePublicDeps(graph, publicDirs)
		for _, e := range errs {
			t.Error(e)
		}
	})

	t.Run("public packages have LICENSE files", func(t *testing.T) {
		errs := publish.ValidateLicenses(root, publicDirs)
		for _, e := range errs {
			t.Error(e)
		}
	})

	t.Run("public packages have correct go.mod module paths", func(t *testing.T) {
		errs := publish.ValidateGoModPaths(root, cfg)
		for _, e := range errs {
			t.Error(e)
		}
	})
}
