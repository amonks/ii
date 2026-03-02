package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverModules(t *testing.T) {
	root := t.TempDir()

	// Create some package dirs with .go files.
	for _, dir := range []string{"apps/foo", "apps/bar", "pkg/baz", "cmd/tool"} {
		p := filepath.Join(root, dir)
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "main.go"), []byte("package main\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create a dir without .go files -- should be ignored.
	if err := os.MkdirAll(filepath.Join(root, "apps/empty"), 0755); err != nil {
		t.Fatal(err)
	}

	mods, err := discoverModules(root)
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]bool{}
	for _, m := range mods {
		got[m.dir] = true
	}

	for _, want := range []string{"apps/foo", "apps/bar", "pkg/baz", "cmd/tool"} {
		if !got[want] {
			t.Errorf("expected module for %s, not found", want)
		}
	}
	if got["apps/empty"] {
		t.Error("should not discover apps/empty (no .go files)")
	}
}

func TestDiscoverModulesDefaultModulePath(t *testing.T) {
	root := t.TempDir()

	p := filepath.Join(root, "pkg/serve")
	if err := os.MkdirAll(p, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "serve.go"), []byte("package serve\n"), 0644); err != nil {
		t.Fatal(err)
	}

	mods, err := discoverModules(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(mods) != 1 {
		t.Fatalf("expected 1 module, got %d", len(mods))
	}
	if mods[0].modulePath != "monks.co/pkg/serve" {
		t.Errorf("expected module path monks.co/pkg/serve, got %s", mods[0].modulePath)
	}
}

func TestGenerateGoWork(t *testing.T) {
	mods := []module{
		{dir: "apps/foo", modulePath: "monks.co/apps/foo"},
		{dir: "pkg/bar", modulePath: "monks.co/pkg/bar"},
		{dir: "cmd/baz", modulePath: "monks.co/cmd/baz"},
	}

	content := generateGoWork(mods)

	if !strings.Contains(content, "go 1.26.0") {
		t.Error("expected go version directive")
	}
	if !strings.Contains(content, "\t.") {
		t.Error("expected root module in use block")
	}
	for _, m := range mods {
		if !strings.Contains(content, "\t./"+m.dir) {
			t.Errorf("expected %s in use block", m.dir)
		}
	}
}

func TestGenerateGoMod(t *testing.T) {
	content := generateGoMod("monks.co/pkg/serve")

	if !strings.Contains(content, "module monks.co/pkg/serve") {
		t.Error("expected module directive")
	}
	if !strings.Contains(content, "go 1.26.0") {
		t.Error("expected go version directive")
	}
}

func TestApplyModulePathOverrides(t *testing.T) {
	mods := []module{
		{dir: "cmd/run", modulePath: "monks.co/cmd/run"},
		{dir: "pkg/serve", modulePath: "monks.co/pkg/serve"},
	}
	cfg := &publishConfig{
		Package: []publishPackage{
			{Dir: "cmd/run", ModulePath: "github.com/amonks/run"},
		},
	}

	applyModulePathOverrides(mods, cfg)

	if mods[0].modulePath != "github.com/amonks/run" {
		t.Errorf("expected github.com/amonks/run, got %s", mods[0].modulePath)
	}
	if mods[1].modulePath != "monks.co/pkg/serve" {
		t.Errorf("expected monks.co/pkg/serve unchanged, got %s", mods[1].modulePath)
	}
}

func TestLoadPublishConfigMissing(t *testing.T) {
	root := t.TempDir()
	cfg, err := loadPublishConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Package) != 0 {
		t.Error("expected empty config when file doesn't exist")
	}
}

func TestGenerateGoModDoesNotOverwriteExisting(t *testing.T) {
	root := t.TempDir()

	p := filepath.Join(root, "pkg/serve")
	if err := os.MkdirAll(p, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "serve.go"), []byte("package serve\n"), 0644); err != nil {
		t.Fatal(err)
	}

	existing := "module monks.co/pkg/serve\n\ngo 1.26.0\n\nrequire example.com/foo v1.0.0\n"
	if err := os.WriteFile(filepath.Join(p, "go.mod"), []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	mods, err := discoverModules(root)
	if err != nil {
		t.Fatal(err)
	}

	if err := writeModuleFiles(root, mods); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(p, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != existing {
		t.Error("existing go.mod should not be overwritten")
	}
}
