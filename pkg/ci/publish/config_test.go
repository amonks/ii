package publish

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePublicDeps(t *testing.T) {
	graph := map[string][]string{
		"pkg/serve":      {"pkg/middleware"},
		"pkg/middleware":  {"pkg/reqlog"},
		"pkg/reqlog":     {},
		"pkg/set":        {},
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

	cfg := &Config{
		Package: []Package{
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

func TestPackageByDir(t *testing.T) {
	cfg := &Config{
		DefaultMirror: "github.com/amonks/go",
		Package: []Package{
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
