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
