package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"monks.co/pkg/env"
)

func TestValidatePublicDeps(t *testing.T) {
	graph := map[string][]string{
		"pkg/serve":    {"pkg/middleware"},
		"pkg/middleware": {"pkg/reqlog"},
		"pkg/reqlog":   {},
		"pkg/set":      {},
	}

	t.Run("all deps public", func(t *testing.T) {
		public := map[string]bool{
			"pkg/serve":      true,
			"pkg/middleware": true,
			"pkg/reqlog":     true,
		}
		errs := ValidatePublicDeps(graph, public)
		if len(errs) != 0 {
			t.Errorf("expected no errors, got: %v", errs)
		}
	})

	t.Run("transitive private dep", func(t *testing.T) {
		public := map[string]bool{
			"pkg/serve":      true,
			"pkg/middleware": true,
			// pkg/reqlog is private
		}
		errs := ValidatePublicDeps(graph, public)
		if len(errs) == 0 {
			t.Fatal("expected errors")
		}
		found := false
		for _, e := range errs {
			if strings.Contains(e, "pkg/reqlog") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected error about pkg/reqlog, got: %v", errs)
		}
	})

	t.Run("no deps is fine", func(t *testing.T) {
		public := map[string]bool{"pkg/set": true}
		errs := ValidatePublicDeps(graph, public)
		if len(errs) != 0 {
			t.Errorf("expected no errors, got: %v", errs)
		}
	})
}

func TestValidateLicenses(t *testing.T) {
	root := t.TempDir()

	// Create two public dirs, one with LICENSE, one without.
	os.MkdirAll(filepath.Join(root, "pkg/good"), 0755)
	os.MkdirAll(filepath.Join(root, "pkg/bad"), 0755)
	os.WriteFile(filepath.Join(root, "pkg/good/LICENSE"), []byte("MIT"), 0644)

	public := map[string]bool{
		"pkg/good": true,
		"pkg/bad":  true,
	}

	errs := ValidateLicenses(root, public)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0], "pkg/bad") {
		t.Errorf("expected error about pkg/bad, got: %s", errs[0])
	}
}

func TestValidateGoModPaths(t *testing.T) {
	root := t.TempDir()

	os.MkdirAll(filepath.Join(root, "pkg/serve"), 0755)
	os.MkdirAll(filepath.Join(root, "cmd/run"), 0755)

	os.WriteFile(filepath.Join(root, "pkg/serve/go.mod"),
		[]byte("module monks.co/pkg/serve\n\ngo 1.26.0\n"), 0644)
	os.WriteFile(filepath.Join(root, "cmd/run/go.mod"),
		[]byte("module github.com/amonks/run\n\ngo 1.26.0\n"), 0644)

	cfg := &PublishConfig{
		Package: []PublishPackage{
			{Dir: "pkg/serve"},
			{Dir: "cmd/run", ModulePath: "github.com/amonks/run"},
		},
	}

	errs := ValidateGoModPaths(root, cfg)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}

	// Now test with wrong module path.
	os.WriteFile(filepath.Join(root, "cmd/run/go.mod"),
		[]byte("module monks.co/cmd/run\n\ngo 1.26.0\n"), 0644)

	errs = ValidateGoModPaths(root, cfg)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0], "github.com/amonks/run") {
		t.Errorf("expected error about module path, got: %s", errs[0])
	}
}

func TestTransitiveDeps(t *testing.T) {
	graph := map[string][]string{
		"pkg/a": {"pkg/b"},
		"pkg/b": {"pkg/c"},
		"pkg/c": {},
	}

	deps := TransitiveDeps(graph, "pkg/a")
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
			dir, ok := resolveImportDir(tt.importPath, modPathToDir)
			if ok != tt.wantOK {
				t.Fatalf("resolveImportDir(%q) ok = %v, want %v", tt.importPath, ok, tt.wantOK)
			}
			if dir != tt.wantDir {
				t.Errorf("resolveImportDir(%q) = %q, want %q", tt.importPath, dir, tt.wantDir)
			}
		})
	}
}

func TestBuildDepGraphNonStandardModulePath(t *testing.T) {
	// monks.co/beetman lives at cmd/beetman. Intra-module imports like
	// monks.co/beetman/internal/foo should resolve to cmd/beetman (self),
	// not produce a bogus "beetman/internal" dep.
	root := t.TempDir()

	// cmd/beetman: module monks.co/beetman, imports its own internal pkg
	// and also imports monks.co/pkg/util.
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

	// pkg/util: standard module path.
	utilDir := filepath.Join(root, "pkg", "util")
	os.MkdirAll(utilDir, 0755)
	os.WriteFile(filepath.Join(utilDir, "go.mod"),
		[]byte("module monks.co/pkg/util\n\ngo 1.26.0\n"), 0644)
	os.WriteFile(filepath.Join(utilDir, "util.go"),
		[]byte("package util\n"), 0644)

	graph, err := BuildDepGraph(root)
	if err != nil {
		t.Fatal(err)
	}

	deps := graph["cmd/beetman"]

	// Should depend on pkg/util.
	hasUtil := false
	for _, dep := range deps {
		if dep == "pkg/util" {
			hasUtil = true
		}
		if dep == "beetman/internal" {
			t.Errorf("dep graph should not contain bogus dir %q", dep)
		}
	}
	if !hasUtil {
		t.Errorf("expected cmd/beetman to depend on pkg/util, got %v", deps)
	}
}

func TestBuildDepGraphReal(t *testing.T) {
	root := env.InMonksRoot()
	graph, err := BuildDepGraph(root)
	if err != nil {
		t.Fatal(err)
	}

	// pkg/set should have no internal deps.
	if deps := graph["pkg/set"]; len(deps) != 0 {
		t.Errorf("expected pkg/set to have no deps, got %v", deps)
	}

	// apps/proxy should depend on several pkg/* packages.
	proxyDeps := graph["apps/proxy"]
	if len(proxyDeps) == 0 {
		t.Error("expected apps/proxy to have dependencies")
	}
}

// TestPublishInvariants is the real validation test that runs against
// the live repo. It validates that config/publish.toml is consistent.
func TestPublishInvariants(t *testing.T) {
	root := env.InMonksRoot()

	cfg, err := LoadPublishConfig(root)
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

	graph, err := BuildDepGraph(root)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("public packages do not depend on private packages", func(t *testing.T) {
		errs := ValidatePublicDeps(graph, publicDirs)
		for _, e := range errs {
			t.Error(e)
		}
	})

	t.Run("public packages have LICENSE files", func(t *testing.T) {
		errs := ValidateLicenses(root, publicDirs)
		for _, e := range errs {
			t.Error(e)
		}
	})

	t.Run("public packages have correct go.mod module paths", func(t *testing.T) {
		errs := ValidateGoModPaths(root, cfg)
		for _, e := range errs {
			t.Error(e)
		}
	})
}
