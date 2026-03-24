package merge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"monks.co/pkg/jj"
	jobpkg "monks.co/ii/job"
)

func TestNormalizeOptionsDefaults(t *testing.T) {
	now := time.Now()
	opts := normalizeOptions(Options{RepoPath: "/repo", Now: func() time.Time { return now }})
	if opts.Target != "main" {
		t.Fatalf("Target = %q, want %q", opts.Target, "main")
	}
	if opts.WorkspacePath != opts.RepoPath {
		t.Fatalf("WorkspacePath = %q, want %q", opts.WorkspacePath, opts.RepoPath)
	}
	if opts.Now == nil {
		t.Fatalf("Now should be set")
	}
}

func TestMergeValidatesInputs(t *testing.T) {
	err := Merge(context.Background(), Options{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "repo path is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMergeRequiresRunLLMForConflicts(t *testing.T) {
	repoPath, branchChange := createConflictRepo(t)

	err := Merge(context.Background(), Options{
		RepoPath:      repoPath,
		WorkspacePath: repoPath,
		ChangeID:      branchChange,
		Target:        "main",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "RunLLM is not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMergeResolvesConflicts(t *testing.T) {
	repoPath, branchChange := createConflictRepo(t)

	calls := 0
	runLLM := func(opts jobpkg.AgentRunOptions) (jobpkg.AgentRunResult, error) {
		calls++
		if err := writeFile(opts.WorkspacePath, "base.txt", "resolved"); err != nil {
			return jobpkg.AgentRunResult{}, err
		}
		return jobpkg.AgentRunResult{ExitCode: 0}, nil
	}

	if err := Merge(context.Background(), Options{
		RepoPath:      repoPath,
		WorkspacePath: repoPath,
		ChangeID:      branchChange,
		Target:        "main",
		RunLLM:        runLLM,
	}); err != nil {
		t.Fatalf("merge: %v", err)
	}

	if calls == 0 {
		t.Fatalf("expected RunLLM to be called")
	}

	client := jj.New()
	updated, err := client.ChangeIDAt(repoPath, "main")
	if err != nil {
		t.Fatalf("read main: %v", err)
	}
	if updated != branchChange {
		t.Fatalf("main not advanced to %s (got %s)", branchChange, updated)
	}
}

func TestMergeFailsWhenResolutionFails(t *testing.T) {
	repoPath, branchChange := createConflictRepo(t)

	runLLM := func(opts jobpkg.AgentRunOptions) (jobpkg.AgentRunResult, error) {
		return jobpkg.AgentRunResult{ExitCode: 1, Error: "cannot resolve"}, nil
	}

	err := Merge(context.Background(), Options{
		RepoPath:      repoPath,
		WorkspacePath: repoPath,
		ChangeID:      branchChange,
		Target:        "main",
		RunLLM:        runLLM,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "conflict resolution failed") {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify bookmark was not advanced.
	client := jj.New()
	mainChange, err := client.ChangeIDAt(repoPath, "main")
	if err != nil {
		t.Fatalf("read main: %v", err)
	}
	if mainChange == branchChange {
		t.Fatalf("main should not have advanced to branch change")
	}
}

func createConflictRepo(t *testing.T) (string, string) {
	t.Helper()

	repoPath := t.TempDir()
	client := jj.New()
	if err := client.Init(repoPath); err != nil {
		t.Fatalf("init repo: %v", err)
	}
	if err := client.BookmarkCreate(repoPath, "main", "@"); err != nil {
		t.Fatalf("create main: %v", err)
	}

	if err := writeFile(repoPath, "base.txt", "base"); err != nil {
		t.Fatalf("write base: %v", err)
	}
	if err := client.Snapshot(repoPath); err != nil {
		t.Fatalf("snapshot base: %v", err)
	}
	if err := client.Commit(repoPath, "base"); err != nil {
		t.Fatalf("commit base: %v", err)
	}
	baseChange, err := client.ChangeIDAt(repoPath, "@-")
	if err != nil {
		t.Fatalf("base change: %v", err)
	}
	if err := client.BookmarkSet(repoPath, "main", baseChange); err != nil {
		t.Fatalf("advance main: %v", err)
	}

	if _, err := client.NewChange(repoPath, "main"); err != nil {
		t.Fatalf("new change: %v", err)
	}
	if err := writeFile(repoPath, "base.txt", "branch"); err != nil {
		t.Fatalf("write branch: %v", err)
	}
	if err := client.Snapshot(repoPath); err != nil {
		t.Fatalf("snapshot branch: %v", err)
	}
	if err := client.Commit(repoPath, "branch"); err != nil {
		t.Fatalf("commit branch: %v", err)
	}
	branchChange, err := client.ChangeIDAt(repoPath, "@-")
	if err != nil {
		t.Fatalf("branch change: %v", err)
	}

	if err := client.Edit(repoPath, "main"); err != nil {
		t.Fatalf("edit main: %v", err)
	}
	if err := writeFile(repoPath, "base.txt", "main"); err != nil {
		t.Fatalf("write main: %v", err)
	}
	if err := client.Snapshot(repoPath); err != nil {
		t.Fatalf("snapshot main: %v", err)
	}
	if err := client.Commit(repoPath, "main update"); err != nil {
		t.Fatalf("commit main update: %v", err)
	}
	mainTip, err := client.ChangeIDAt(repoPath, "@-")
	if err != nil {
		t.Fatalf("main change: %v", err)
	}
	if err := client.BookmarkSet(repoPath, "main", mainTip); err != nil {
		t.Fatalf("advance main tip: %v", err)
	}

	if err := client.Edit(repoPath, branchChange); err != nil {
		t.Fatalf("edit branch change: %v", err)
	}

	return repoPath, branchChange
}

func writeFile(dir, name, content string) error {
	path := filepath.Join(dir, name)
	return os.WriteFile(path, []byte(content), 0o644)
}
