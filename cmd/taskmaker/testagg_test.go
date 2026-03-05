package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRedundantCommands(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "redundant-commands"), []byte("go test ./...\ngo tool staticcheck ./...\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmds, err := loadRedundantCommands(root)
	if err != nil {
		t.Fatal(err)
	}

	if !cmds["go test ./..."] {
		t.Error("expected 'go test ./...' to be redundant")
	}
	if !cmds["go tool staticcheck ./..."] {
		t.Error("expected 'go tool staticcheck ./...' to be redundant")
	}
	if cmds["go run . snapshot-cli"] {
		t.Error("unexpected redundant command")
	}
}

func TestLoadRedundantCommandsMissing(t *testing.T) {
	root := t.TempDir()
	cmds, err := loadRedundantCommands(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 0 {
		t.Error("expected empty map when file doesn't exist")
	}
}

func TestDiscoverModuleTasks(t *testing.T) {
	root := t.TempDir()

	// App with build, generate, and dev tasks.
	mkTaskfile(t, root, "apps/foo", `
[[task]]
id = "build"
type = "short"
cmd = "go build"

[[task]]
id = "generate"
type = "short"
cmd = "go generate"

[[task]]
id = "dev"
type = "long"
cmd = "go run ."
`)

	// App with only a start task (no build or generate).
	mkTaskfile(t, root, "apps/bar", `
[[task]]
id = "start"
type = "long"
cmd = "go run ."
`)

	// Pkg with a build task.
	mkTaskfile(t, root, "pkg/lib", `
[[task]]
id = "build"
type = "short"
cmd = "go build"
`)

	// Dir without tasks.toml — should be ignored.
	if err := os.MkdirAll(filepath.Join(root, "apps/empty"), 0755); err != nil {
		t.Fatal(err)
	}

	got, err := discoverModuleTasks(root)
	if err != nil {
		t.Fatal(err)
	}

	// apps/foo should have build, generate, and dev
	if ids, ok := got["apps/foo"]; !ok {
		t.Error("expected apps/foo")
	} else {
		idSet := toSet(ids)
		if !idSet["build"] {
			t.Error("expected apps/foo to have build")
		}
		if !idSet["generate"] {
			t.Error("expected apps/foo to have generate")
		}
		if !idSet["dev"] {
			t.Error("expected apps/foo to have dev")
		}
	}

	// apps/bar should have start but no build
	if ids, ok := got["apps/bar"]; !ok {
		t.Error("expected apps/bar")
	} else {
		idSet := toSet(ids)
		if !idSet["start"] {
			t.Error("expected apps/bar to have start")
		}
		if idSet["build"] {
			t.Error("apps/bar should not have build")
		}
	}

	// pkg/lib should have build
	if ids, ok := got["pkg/lib"]; !ok {
		t.Error("expected pkg/lib")
	} else if !toSet(ids)["build"] {
		t.Error("expected pkg/lib to have build")
	}

	// apps/empty should not appear
	if _, ok := got["apps/empty"]; ok {
		t.Error("should not discover apps/empty (no tasks.toml)")
	}
}

func mkTaskfile(t *testing.T, root, dir, content string) {
	t.Helper()
	p := filepath.Join(root, dir)
	if err := os.MkdirAll(p, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "tasks.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func toSet(ss []string) map[string]bool {
	m := map[string]bool{}
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func TestFindAppTestTasks(t *testing.T) {
	root := t.TempDir()

	// Create an app with tasks.toml containing test tasks.
	appDir := filepath.Join(root, "apps", "dungeon")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatal(err)
	}
	tasksToml := `
[[task]]
id = "test"
type = "short"
dependencies = ["test-go", "test-js"]

[[task]]
id = "test-go"
type = "short"
cmd = "go test ./..."

[[task]]
id = "test-js"
type = "short"
cmd = "npx vitest run"

[[task]]
id = "build"
type = "short"
cmd = "go build"
`
	if err := os.WriteFile(filepath.Join(appDir, "tasks.toml"), []byte(tasksToml), 0644); err != nil {
		t.Fatal(err)
	}

	redundant := map[string]bool{"go test ./...": true}

	taskIDs, err := findAppTestTasks(root, redundant)
	if err != nil {
		t.Fatal(err)
	}

	// Should include test-js (non-redundant) but not test-go (redundant).
	// Should not include "build" (not a test task).
	found := map[string]bool{}
	for _, id := range taskIDs {
		found[id] = true
	}

	if !found["apps/dungeon/test-js"] {
		t.Error("expected apps/dungeon/test-js to be discovered")
	}
	if found["apps/dungeon/test-go"] {
		t.Error("should not include redundant apps/dungeon/test-go")
	}
	if found["apps/dungeon/build"] {
		t.Error("should not include non-test task apps/dungeon/build")
	}
	if found["apps/dungeon/test"] {
		t.Error("should not include umbrella test task (it has no cmd, only deps)")
	}
}
