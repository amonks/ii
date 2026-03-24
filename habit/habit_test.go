package habit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Run("loads habit without frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		habitsDir := filepath.Join(dir, HabitsDir)
		if err := os.MkdirAll(habitsDir, 0755); err != nil {
			t.Fatal(err)
		}

		content := `# Clean Up

Look for code cleanup opportunities.

## Guidelines

- Remove dead code
- Simplify logic`
		if err := os.WriteFile(filepath.Join(habitsDir, "cleanup.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		habit, err := Load(dir, "cleanup")
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if habit.Name != "cleanup" {
			t.Errorf("Name = %q, want %q", habit.Name, "cleanup")
		}
		if habit.Instructions != content {
			t.Errorf("Instructions mismatch:\ngot:\n%s\nwant:\n%s", habit.Instructions, content)
		}
		if habit.ImplementationModel != "" {
			t.Errorf("ImplementationModel = %q, want empty", habit.ImplementationModel)
		}
		if habit.ReviewModel != "" {
			t.Errorf("ReviewModel = %q, want empty", habit.ReviewModel)
		}
	})

	t.Run("loads habit with frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		habitsDir := filepath.Join(dir, HabitsDir)
		if err := os.MkdirAll(habitsDir, 0755); err != nil {
			t.Fatal(err)
		}

		content := `---
models:
  implementation: claude-haiku-4-5-20251001
  review: claude-haiku-4-5
---

# Performance

Look for performance improvements.`
		if err := os.WriteFile(filepath.Join(habitsDir, "perf.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		habit, err := Load(dir, "perf")
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if habit.Name != "perf" {
			t.Errorf("Name = %q, want %q", habit.Name, "perf")
		}
		expectedInstructions := "# Performance\n\nLook for performance improvements."
		if habit.Instructions != expectedInstructions {
			t.Errorf("Instructions mismatch:\ngot:\n%s\nwant:\n%s", habit.Instructions, expectedInstructions)
		}
		if habit.ImplementationModel != "claude-haiku-4-5-20251001" {
			t.Errorf("ImplementationModel = %q, want %q", habit.ImplementationModel, "claude-haiku-4-5-20251001")
		}
		if habit.ReviewModel != "claude-haiku-4-5" {
			t.Errorf("ReviewModel = %q, want %q", habit.ReviewModel, "claude-haiku-4-5")
		}
	})

	t.Run("loads habit with partial frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		habitsDir := filepath.Join(dir, HabitsDir)
		if err := os.MkdirAll(habitsDir, 0755); err != nil {
			t.Fatal(err)
		}

		content := `---
models:
  implementation: claude-haiku-4-5
---

# Docs

Update documentation.`
		if err := os.WriteFile(filepath.Join(habitsDir, "docs.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		habit, err := Load(dir, "docs")
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if habit.ImplementationModel != "claude-haiku-4-5" {
			t.Errorf("ImplementationModel = %q, want %q", habit.ImplementationModel, "claude-haiku-4-5")
		}
		if habit.ReviewModel != "" {
			t.Errorf("ReviewModel = %q, want empty", habit.ReviewModel)
		}
	})

	t.Run("returns error for missing habit", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Load(dir, "nonexistent")
		if err == nil {
			t.Error("expected error for missing habit")
		}
	})

	t.Run("returns error for empty name", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Load(dir, "")
		if err == nil {
			t.Error("expected error for empty name")
		}
	})
}

func TestList(t *testing.T) {
	t.Run("returns empty for no habits directory", func(t *testing.T) {
		dir := t.TempDir()
		names, err := List(dir)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(names) != 0 {
			t.Errorf("got %d names, want 0", len(names))
		}
	})

	t.Run("returns empty for empty habits directory", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, HabitsDir), 0755); err != nil {
			t.Fatal(err)
		}

		names, err := List(dir)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(names) != 0 {
			t.Errorf("got %d names, want 0", len(names))
		}
	})

	t.Run("returns sorted habit names", func(t *testing.T) {
		dir := t.TempDir()
		habitsDir := filepath.Join(dir, HabitsDir)
		if err := os.MkdirAll(habitsDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create habits in non-alphabetical order
		for _, name := range []string{"zebra", "alpha", "mid"} {
			if err := os.WriteFile(filepath.Join(habitsDir, name+".md"), []byte("# "+name), 0644); err != nil {
				t.Fatal(err)
			}
		}

		names, err := List(dir)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		expected := []string{"alpha", "mid", "zebra"}
		if len(names) != len(expected) {
			t.Fatalf("got %d names, want %d", len(names), len(expected))
		}
		for i, name := range names {
			if name != expected[i] {
				t.Errorf("names[%d] = %q, want %q", i, name, expected[i])
			}
		}
	})

	t.Run("ignores non-md files", func(t *testing.T) {
		dir := t.TempDir()
		habitsDir := filepath.Join(dir, HabitsDir)
		if err := os.MkdirAll(habitsDir, 0755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(habitsDir, "valid.md"), []byte("# Valid"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(habitsDir, "ignored.txt"), []byte("ignored"), 0644); err != nil {
			t.Fatal(err)
		}

		names, err := List(dir)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(names) != 1 || names[0] != "valid" {
			t.Errorf("got names %v, want [valid]", names)
		}
	})

	t.Run("ignores directories", func(t *testing.T) {
		dir := t.TempDir()
		habitsDir := filepath.Join(dir, HabitsDir)
		if err := os.MkdirAll(habitsDir, 0755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(habitsDir, "valid.md"), []byte("# Valid"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(habitsDir, "subdir.md"), 0755); err != nil {
			t.Fatal(err)
		}

		names, err := List(dir)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(names) != 1 || names[0] != "valid" {
			t.Errorf("got names %v, want [valid]", names)
		}
	})
}

func TestPath(t *testing.T) {
	t.Run("returns path for valid name", func(t *testing.T) {
		dir := t.TempDir()
		path, err := Path(dir, "cleanup")
		if err != nil {
			t.Fatalf("Path failed: %v", err)
		}
		expected := filepath.Join(dir, HabitsDir, "cleanup.md")
		if path != expected {
			t.Errorf("path = %q, want %q", path, expected)
		}
	})

	t.Run("trims whitespace from name", func(t *testing.T) {
		dir := t.TempDir()
		path, err := Path(dir, "  cleanup  ")
		if err != nil {
			t.Fatalf("Path failed: %v", err)
		}
		expected := filepath.Join(dir, HabitsDir, "cleanup.md")
		if path != expected {
			t.Errorf("path = %q, want %q", path, expected)
		}
	})

	t.Run("returns error for empty name", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Path(dir, "")
		if err == nil {
			t.Error("expected error for empty name")
		}
	})

	t.Run("returns error for whitespace-only name", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Path(dir, "   ")
		if err == nil {
			t.Error("expected error for whitespace-only name")
		}
	})
}

func TestExists(t *testing.T) {
	t.Run("returns true for existing habit", func(t *testing.T) {
		dir := t.TempDir()
		habitsDir := filepath.Join(dir, HabitsDir)
		if err := os.MkdirAll(habitsDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(habitsDir, "cleanup.md"), []byte("# Cleanup"), 0644); err != nil {
			t.Fatal(err)
		}

		exists, err := Exists(dir, "cleanup")
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if !exists {
			t.Error("expected exists to be true")
		}
	})

	t.Run("returns false for non-existing habit", func(t *testing.T) {
		dir := t.TempDir()
		exists, err := Exists(dir, "nonexistent")
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Error("expected exists to be false")
		}
	})

	t.Run("returns error for empty name", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Exists(dir, "")
		if err == nil {
			t.Error("expected error for empty name")
		}
	})
}

func TestCreate(t *testing.T) {
	t.Run("creates habit file with template", func(t *testing.T) {
		dir := t.TempDir()
		path, err := Create(dir, "cleanup")
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		expectedPath := filepath.Join(dir, HabitsDir, "cleanup.md")
		if path != expectedPath {
			t.Errorf("path = %q, want %q", path, expectedPath)
		}

		// Verify file exists and contains template
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read created file: %v", err)
		}
		if !strings.Contains(string(content), "# cleanup") {
			t.Errorf("content does not contain expected heading:\n%s", content)
		}
		if !strings.Contains(string(content), "Guidelines") {
			t.Errorf("content does not contain Guidelines section:\n%s", content)
		}
	})

	t.Run("converts dashes and underscores to spaces in title", func(t *testing.T) {
		dir := t.TempDir()
		path, err := Create(dir, "code-cleanup_task")
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read created file: %v", err)
		}
		if !strings.Contains(string(content), "# code cleanup task") {
			t.Errorf("content does not contain expected heading with spaces:\n%s", content)
		}
	})

	t.Run("creates habits directory if not exists", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Create(dir, "newhabit")
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Verify directory was created
		info, err := os.Stat(filepath.Join(dir, HabitsDir))
		if err != nil {
			t.Fatalf("habits directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("habits path is not a directory")
		}
	})

	t.Run("returns error if habit already exists", func(t *testing.T) {
		dir := t.TempDir()
		habitsDir := filepath.Join(dir, HabitsDir)
		if err := os.MkdirAll(habitsDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(habitsDir, "existing.md"), []byte("# Existing"), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := Create(dir, "existing")
		if err == nil {
			t.Error("expected error for existing habit")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("error should mention 'already exists': %v", err)
		}
	})

	t.Run("returns error for empty name", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Create(dir, "")
		if err == nil {
			t.Error("expected error for empty name")
		}
	})

	t.Run("trims whitespace from name", func(t *testing.T) {
		dir := t.TempDir()
		path, err := Create(dir, "  trimmed  ")
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		expectedPath := filepath.Join(dir, HabitsDir, "trimmed.md")
		if path != expectedPath {
			t.Errorf("path = %q, want %q", path, expectedPath)
		}
	})
}

func TestFirst(t *testing.T) {
	t.Run("returns nil for no habits", func(t *testing.T) {
		dir := t.TempDir()
		habit, err := First(dir)
		if err != nil {
			t.Fatalf("First failed: %v", err)
		}
		if habit != nil {
			t.Error("expected nil habit for empty directory")
		}
	})

	t.Run("returns first habit alphabetically", func(t *testing.T) {
		dir := t.TempDir()
		habitsDir := filepath.Join(dir, HabitsDir)
		if err := os.MkdirAll(habitsDir, 0755); err != nil {
			t.Fatal(err)
		}

		for _, name := range []string{"zebra", "alpha", "mid"} {
			if err := os.WriteFile(filepath.Join(habitsDir, name+".md"), []byte("# "+name), 0644); err != nil {
				t.Fatal(err)
			}
		}

		habit, err := First(dir)
		if err != nil {
			t.Fatalf("First failed: %v", err)
		}

		if habit.Name != "alpha" {
			t.Errorf("Name = %q, want %q", habit.Name, "alpha")
		}
	})
}

func TestFind(t *testing.T) {
	t.Run("finds habit by exact name", func(t *testing.T) {
		dir := t.TempDir()
		habitsDir := filepath.Join(dir, HabitsDir)
		if err := os.MkdirAll(habitsDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(habitsDir, "cleanup.md"), []byte("# Cleanup"), 0644); err != nil {
			t.Fatal(err)
		}

		habit, err := Find(dir, "cleanup")
		if err != nil {
			t.Fatalf("Find failed: %v", err)
		}
		if habit.Name != "cleanup" {
			t.Errorf("Name = %q, want %q", habit.Name, "cleanup")
		}
	})

	t.Run("finds habit by unique prefix", func(t *testing.T) {
		dir := t.TempDir()
		habitsDir := filepath.Join(dir, HabitsDir)
		if err := os.MkdirAll(habitsDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(habitsDir, "cleanup.md"), []byte("# Cleanup"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(habitsDir, "docs.md"), []byte("# Docs"), 0644); err != nil {
			t.Fatal(err)
		}

		habit, err := Find(dir, "c")
		if err != nil {
			t.Fatalf("Find failed: %v", err)
		}
		if habit.Name != "cleanup" {
			t.Errorf("Name = %q, want %q", habit.Name, "cleanup")
		}
	})

	t.Run("returns error for ambiguous prefix", func(t *testing.T) {
		dir := t.TempDir()
		habitsDir := filepath.Join(dir, HabitsDir)
		if err := os.MkdirAll(habitsDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(habitsDir, "cleanup.md"), []byte("# Cleanup"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(habitsDir, "clean-code.md"), []byte("# Clean Code"), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := Find(dir, "clea")
		if err == nil {
			t.Error("expected error for ambiguous prefix")
		}
		if !strings.Contains(err.Error(), "ambiguous") {
			t.Errorf("error should mention 'ambiguous': %v", err)
		}
	})

	t.Run("returns error for not found", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Find(dir, "nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent habit")
		}
		if err != ErrHabitNotFound {
			t.Errorf("expected ErrHabitNotFound, got: %v", err)
		}
	})

	t.Run("returns error for empty name", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Find(dir, "")
		if err == nil {
			t.Error("expected error for empty name")
		}
	})
}

func TestLoadAll(t *testing.T) {
	t.Run("returns empty slice for no habits", func(t *testing.T) {
		dir := t.TempDir()
		habits, err := LoadAll(dir)
		if err != nil {
			t.Fatalf("LoadAll failed: %v", err)
		}
		if len(habits) != 0 {
			t.Errorf("got %d habits, want 0", len(habits))
		}
	})

	t.Run("loads all habits sorted", func(t *testing.T) {
		dir := t.TempDir()
		habitsDir := filepath.Join(dir, HabitsDir)
		if err := os.MkdirAll(habitsDir, 0755); err != nil {
			t.Fatal(err)
		}

		for _, name := range []string{"zebra", "alpha", "mid"} {
			if err := os.WriteFile(filepath.Join(habitsDir, name+".md"), []byte("# "+name), 0644); err != nil {
				t.Fatal(err)
			}
		}

		habits, err := LoadAll(dir)
		if err != nil {
			t.Fatalf("LoadAll failed: %v", err)
		}

		if len(habits) != 3 {
			t.Fatalf("got %d habits, want 3", len(habits))
		}

		// Habits should be sorted alphabetically
		expected := []string{"alpha", "mid", "zebra"}
		for i, h := range habits {
			if h.Name != expected[i] {
				t.Errorf("habits[%d].Name = %q, want %q", i, h.Name, expected[i])
			}
		}
	})
}

func TestPrefixLengths(t *testing.T) {
	t.Run("returns unique prefix lengths", func(t *testing.T) {
		habits := []*Habit{
			{Name: "cleanup"},
			{Name: "clean-code"},
			{Name: "docs"},
		}

		lengths := PrefixLengths(habits)

		// cleanup and clean-code share "clean", so need longer prefix
		if lengths["cleanup"] < 6 {
			t.Errorf("cleanup prefix length = %d, expected >= 6", lengths["cleanup"])
		}
		if lengths["clean-code"] < 6 {
			t.Errorf("clean-code prefix length = %d, expected >= 6", lengths["clean-code"])
		}
		// docs is unique
		if lengths["docs"] != 1 {
			t.Errorf("docs prefix length = %d, expected 1", lengths["docs"])
		}
	})

	t.Run("handles empty slice", func(t *testing.T) {
		habits := []*Habit{}
		lengths := PrefixLengths(habits)
		if len(lengths) != 0 {
			t.Errorf("got %d lengths, want 0", len(lengths))
		}
	})
}

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name       string
		data       string
		wantImpl   string
		wantReview string
	}{
		{
			name:       "empty",
			data:       "",
			wantImpl:   "",
			wantReview: "",
		},
		{
			name: "full models section",
			data: `models:
  implementation: claude-haiku-4-5-20251001
  review: claude-haiku-4-5`,
			wantImpl:   "claude-haiku-4-5-20251001",
			wantReview: "claude-haiku-4-5",
		},
		{
			name: "only implementation",
			data: `models:
  implementation: claude-haiku-4-5-20251001`,
			wantImpl:   "claude-haiku-4-5-20251001",
			wantReview: "",
		},
		{
			name: "only review",
			data: `models:
  review: claude-haiku-4-5`,
			wantImpl:   "",
			wantReview: "claude-haiku-4-5",
		},
		{
			name: "other fields ignored",
			data: `title: Test
models:
  implementation: claude-haiku-4-5-20251001
  review: claude-haiku-4-5
other: value`,
			wantImpl:   "claude-haiku-4-5-20251001",
			wantReview: "claude-haiku-4-5",
		},
		{
			name: "no models section",
			data: `title: Test
description: Something`,
			wantImpl:   "",
			wantReview: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotImpl, gotReview := parseFrontmatter(tt.data)
			if gotImpl != tt.wantImpl {
				t.Errorf("implementation = %q, want %q", gotImpl, tt.wantImpl)
			}
			if gotReview != tt.wantReview {
				t.Errorf("review = %q, want %q", gotReview, tt.wantReview)
			}
		})
	}
}
