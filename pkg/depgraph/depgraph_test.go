package depgraph

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildDepGraph(t *testing.T) {
	root := t.TempDir()

	// Create pkg/a that depends on pkg/b.
	mkModule(t, root, "pkg/a", "monks.co/pkg/a", `package a

import "monks.co/pkg/b"

var _ = b.X
`)
	// Create pkg/b with no deps.
	mkModule(t, root, "pkg/b", "monks.co/pkg/b", `package b

var X = 1
`)
	// Create apps/myapp that depends on pkg/a.
	mkModule(t, root, "apps/myapp", "monks.co/apps/myapp", `package myapp

import "monks.co/pkg/a"

var _ = a.X
`)

	graph, err := BuildDepGraph(root)
	if err != nil {
		t.Fatal(err)
	}

	// apps/myapp should depend on pkg/a.
	assertDeps(t, graph, "apps/myapp", []string{"pkg/a"})

	// pkg/a should depend on pkg/b.
	assertDeps(t, graph, "pkg/a", []string{"pkg/b"})

	// pkg/b should have no deps.
	assertDeps(t, graph, "pkg/b", nil)
}

func TestBuildDepGraph_SelfImportFiltered(t *testing.T) {
	root := t.TempDir()

	// cmd/tool has module monks.co/tool and imports itself.
	dir := filepath.Join(root, "cmd", "tool")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module monks.co/tool\n\ngo 1.26.0\n"), 0644)
	os.WriteFile(filepath.Join(dir, "lib.go"),
		[]byte("package tool\n"), 0644)

	subDir := filepath.Join(dir, "tool")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "main.go"), []byte(`package main

import "monks.co/tool"

var _ = tool.X
`), 0644)

	graph, err := BuildDepGraph(root)
	if err != nil {
		t.Fatal(err)
	}

	for _, dep := range graph["cmd/tool"] {
		if dep == "cmd/tool" {
			t.Error("dep graph should not contain self-reference")
		}
	}
}

func TestTransitiveDeps(t *testing.T) {
	graph := map[string][]string{
		"pkg/a": {"pkg/b"},
		"pkg/b": {"pkg/c"},
		"pkg/c": {},
		"pkg/d": {},
	}

	t.Run("full chain", func(t *testing.T) {
		deps := TransitiveDeps(graph, "pkg/a")
		if !deps["pkg/b"] || !deps["pkg/c"] {
			t.Errorf("expected pkg/b and pkg/c, got %v", deps)
		}
		if deps["pkg/a"] {
			t.Error("should not include self")
		}
		if deps["pkg/d"] {
			t.Error("should not include unrelated pkg/d")
		}
	})

	t.Run("leaf node", func(t *testing.T) {
		deps := TransitiveDeps(graph, "pkg/c")
		if len(deps) != 0 {
			t.Errorf("expected no deps for leaf, got %v", deps)
		}
	})

	t.Run("unknown node", func(t *testing.T) {
		deps := TransitiveDeps(graph, "pkg/unknown")
		if len(deps) != 0 {
			t.Errorf("expected no deps for unknown, got %v", deps)
		}
	})
}

func TestBuildModuleMap(t *testing.T) {
	root := t.TempDir()

	mkModule(t, root, "pkg/serve", "monks.co/pkg/serve", "package serve\n")
	mkModule(t, root, "cmd/beetman", "monks.co/beetman", "package beetman\n")

	// Create a non-monks.co module — should be excluded.
	mkModule(t, root, "pkg/external", "github.com/other/external", "package external\n")

	modMap, err := BuildModuleMap(root)
	if err != nil {
		t.Fatal(err)
	}

	if modMap["monks.co/pkg/serve"] != "pkg/serve" {
		t.Errorf("expected monks.co/pkg/serve -> pkg/serve, got %v", modMap)
	}
	if modMap["monks.co/beetman"] != "cmd/beetman" {
		t.Errorf("expected monks.co/beetman -> cmd/beetman, got %v", modMap)
	}
	if _, ok := modMap["github.com/other/external"]; ok {
		t.Error("should not include non-monks.co modules")
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
			dir, ok := ResolveImportDir(tt.importPath, modPathToDir)
			if ok != tt.wantOK {
				t.Fatalf("ResolveImportDir(%q) ok = %v, want %v", tt.importPath, ok, tt.wantOK)
			}
			if dir != tt.wantDir {
				t.Errorf("ResolveImportDir(%q) = %q, want %q", tt.importPath, dir, tt.wantDir)
			}
		})
	}
}

func TestReadModulePath(t *testing.T) {
	dir := t.TempDir()

	goMod := filepath.Join(dir, "go.mod")
	os.WriteFile(goMod, []byte("module monks.co/pkg/serve\n\ngo 1.26.0\n"), 0644)

	got := ReadModulePath(goMod)
	if got != "monks.co/pkg/serve" {
		t.Errorf("expected monks.co/pkg/serve, got %q", got)
	}

	// Non-existent file returns empty.
	got = ReadModulePath(filepath.Join(dir, "nope"))
	if got != "" {
		t.Errorf("expected empty for missing file, got %q", got)
	}
}

// mkModule creates a module directory with go.mod and a .go file.
func mkModule(t *testing.T, root, dir, modulePath, goSource string) {
	t.Helper()
	absDir := filepath.Join(root, dir)
	if err := os.MkdirAll(absDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(absDir, "go.mod"),
		[]byte("module "+modulePath+"\n\ngo 1.26.0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(absDir, "main.go"),
		[]byte(goSource), 0644); err != nil {
		t.Fatal(err)
	}
}

func assertDeps(t *testing.T, graph map[string][]string, dir string, want []string) {
	t.Helper()
	got := graph[dir]
	if len(got) != len(want) {
		t.Errorf("%s: expected %d deps %v, got %d deps %v", dir, len(want), want, len(got), got)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%s: dep[%d] = %q, want %q", dir, i, got[i], want[i])
		}
	}
}
