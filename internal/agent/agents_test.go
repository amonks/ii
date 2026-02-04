package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAgentsPrelude_WhenMissing_ReturnsEmpty(t *testing.T) {
	prelude, err := agentsPrelude(t.TempDir(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prelude != "" {
		t.Fatalf("expected empty prelude, got %q", prelude)
	}
}

func TestAgentsPrelude_WhenPresent_ReturnsTrimmedWithBlankLine(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("  hello\nworld\n\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	prelude, err := agentsPrelude(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "hello\nworld\n\n"
	if prelude != want {
		t.Fatalf("unexpected prelude.\nwant: %q\n got: %q", want, prelude)
	}
}

func TestAgentsPrelude_PrefersAGENTSoverCLAUDE(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("from agents"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("from claude"), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}

	prelude, err := agentsPrelude(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "from agents\n\n"
	if prelude != want {
		t.Fatalf("expected AGENTS.md to win, got %q", prelude)
	}
}

func TestAgentsPrelude_FallsBackToCLAUDE(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("from claude"), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}

	prelude, err := agentsPrelude(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "from claude\n\n"
	if prelude != want {
		t.Fatalf("expected CLAUDE.md content, got %q", prelude)
	}
}

func TestAgentsPrelude_AncestorDirectories(t *testing.T) {
	// Create a directory structure:
	// root/
	//   AGENTS.md (content: "root instructions")
	//   sub/
	//     AGENTS.md (content: "sub instructions")
	//     deep/
	//       (no AGENTS.md)
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	deep := filepath.Join(sub, "deep")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("root instructions"), 0o644); err != nil {
		t.Fatalf("write root AGENTS.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "AGENTS.md"), []byte("sub instructions"), 0o644); err != nil {
		t.Fatalf("write sub AGENTS.md: %v", err)
	}

	prelude, err := agentsPrelude(deep, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain both, root first then sub
	want := "root instructions\n\nsub instructions\n\n"
	if prelude != want {
		t.Fatalf("unexpected prelude.\nwant: %q\n got: %q", want, prelude)
	}
}

func TestAgentsPrelude_GlobalConfigDir(t *testing.T) {
	// Create:
	// globalDir/
	//   AGENTS.md (content: "global instructions")
	// workDir/
	//   AGENTS.md (content: "project instructions")
	globalDir := t.TempDir()
	workDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(globalDir, "AGENTS.md"), []byte("global instructions"), 0o644); err != nil {
		t.Fatalf("write global AGENTS.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "AGENTS.md"), []byte("project instructions"), 0o644); err != nil {
		t.Fatalf("write project AGENTS.md: %v", err)
	}

	prelude, err := agentsPrelude(workDir, globalDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Global first, then project
	want := "global instructions\n\nproject instructions\n\n"
	if prelude != want {
		t.Fatalf("unexpected prelude.\nwant: %q\n got: %q", want, prelude)
	}
}

func TestAgentsPrelude_MixedFilenames(t *testing.T) {
	// Create:
	// root/
	//   CLAUDE.md (content: "root via claude")
	//   sub/
	//     AGENTS.md (content: "sub via agents")
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("root via claude"), 0o644); err != nil {
		t.Fatalf("write root CLAUDE.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "AGENTS.md"), []byte("sub via agents"), 0o644); err != nil {
		t.Fatalf("write sub AGENTS.md: %v", err)
	}

	prelude, err := agentsPrelude(sub, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Root (via CLAUDE.md) first, then sub (via AGENTS.md)
	want := "root via claude\n\nsub via agents\n\n"
	if prelude != want {
		t.Fatalf("unexpected prelude.\nwant: %q\n got: %q", want, prelude)
	}
}

func TestLoadContextFiles_DeduplicatesPaths(t *testing.T) {
	// If globalConfigDir is a parent of workDir, the file should only appear once
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("root instructions"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	// Use root as globalConfigDir, but it's also an ancestor of sub
	files, err := LoadContextFiles(LoadContextFilesOptions{
		WorkDir:         sub,
		GlobalConfigDir: root,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only have one entry (deduplicated)
	if len(files) != 1 {
		t.Fatalf("expected 1 file (deduplicated), got %d", len(files))
	}
	if files[0].Content != "root instructions" {
		t.Fatalf("unexpected content: %q", files[0].Content)
	}
}

func TestLoadContextFiles_EmptyFilesSkipped(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("   \n\n  "), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	prelude, err := agentsPrelude(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prelude != "" {
		t.Fatalf("expected empty prelude for whitespace-only file, got %q", prelude)
	}
}

func TestAgentsPrelude_RelativeWorkDir(t *testing.T) {
	// Create a directory structure within temp:
	// root/
	//   AGENTS.md (content: "root instructions")
	//   sub/
	//     deep/
	//       (working directory)
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	deep := filepath.Join(sub, "deep")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("root instructions"), 0o644); err != nil {
		t.Fatalf("write root AGENTS.md: %v", err)
	}

	// Save current directory to restore later
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer os.Chdir(origDir)

	// Change into deep directory and call with relative path "."
	if err := os.Chdir(deep); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	prelude, err := agentsPrelude(".", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still find root AGENTS.md via ancestor traversal
	// even though we passed a relative path
	if prelude == "" {
		t.Fatal("expected to find ancestor AGENTS.md with relative workDir, got empty prelude")
	}
	want := "root instructions\n\n"
	if prelude != want {
		t.Fatalf("unexpected prelude.\nwant: %q\n got: %q", want, prelude)
	}
}

func TestLoadContextFiles_DeduplicatesWithDifferentPathForms(t *testing.T) {
	// Test that paths are canonicalized for deduplication
	// Create root/AGENTS.md and use root as globalConfigDir while root is also an ancestor
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("root instructions"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	// Use root with a trailing slash and ".." in the path to create a different textual representation
	globalConfigDir := filepath.Join(root, "sub", "..")

	files, err := LoadContextFiles(LoadContextFilesOptions{
		WorkDir:         sub,
		GlobalConfigDir: globalConfigDir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only have one entry (deduplicated despite different path forms)
	if len(files) != 1 {
		t.Fatalf("expected 1 file (deduplicated), got %d: %+v", len(files), files)
	}
	if files[0].Content != "root instructions" {
		t.Fatalf("unexpected content: %q", files[0].Content)
	}
}

func TestLoadContextFiles_ReturnsCanonicalizedPaths(t *testing.T) {
	// Test that returned paths are absolute and cleaned
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("root instructions"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "AGENTS.md"), []byte("sub instructions"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	// Use a non-canonical path for workDir
	nonCanonicalWorkDir := filepath.Join(root, "sub", "..", "sub")

	files, err := LoadContextFiles(LoadContextFilesOptions{
		WorkDir: nonCanonicalWorkDir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both files should be found
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	// Both paths should be absolute and cleaned (no ".." components)
	for _, f := range files {
		if !filepath.IsAbs(f.Path) {
			t.Errorf("expected absolute path, got %q", f.Path)
		}
		if filepath.Clean(f.Path) != f.Path {
			t.Errorf("expected cleaned path, got %q (cleaned: %q)", f.Path, filepath.Clean(f.Path))
		}
	}

	// Verify exact paths
	wantRoot := filepath.Join(root, "AGENTS.md")
	wantSub := filepath.Join(sub, "AGENTS.md")
	if files[0].Path != wantRoot {
		t.Errorf("expected first file path %q, got %q", wantRoot, files[0].Path)
	}
	if files[1].Path != wantSub {
		t.Errorf("expected second file path %q, got %q", wantSub, files[1].Path)
	}
}
