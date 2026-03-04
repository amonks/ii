package job

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"monks.co/incrementum/internal/jj"
	"monks.co/incrementum/internal/testsupport"
	"monks.co/incrementum/todo"
)

func TestRunInterruptMarksJobFailed(t *testing.T) {
	repoPath := setupJobRepo(t)

	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: true, PromptToCreate: false})
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	created, err := store.Create("Interrupt job", todo.CreateOptions{Priority: new(todo.PriorityMedium)})
	if err != nil {
		store.Release()
		t.Fatalf("create todo: %v", err)
	}
	store.Release()

	started := make(chan struct{})
	block := make(chan struct{})
	done := make(chan struct{})

	var result *RunResult
	var runErr error
	go func() {
		result, runErr = Run(repoPath, created.ID, RunOptions{
			Now: func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) },
			RunLLM: func(opts AgentRunOptions) (AgentRunResult, error) {
				close(started)
				<-block
				return AgentRunResult{SessionID: "session-1", ExitCode: 0}, nil
			},
		})
		close(done)
	}()

	<-started

	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("find process: %v", err)
	}
	if err := proc.Signal(os.Interrupt); err != nil {
		t.Fatalf("send interrupt: %v", err)
	}

	close(block)
	<-done

	if runErr == nil {
		t.Fatal("expected interrupt error")
	}
	if !errors.Is(runErr, ErrJobInterrupted) {
		t.Fatalf("expected interrupt error, got %v", runErr)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Job.Status != StatusFailed {
		t.Fatalf("expected failed status, got %q", result.Job.Status)
	}

	store, err = todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false})
	if err != nil {
		t.Fatalf("reopen todo store: %v", err)
	}
	items, err := store.Show([]string{created.ID})
	if err != nil {
		store.Release()
		t.Fatalf("show todo: %v", err)
	}
	if items[0].Status != todo.StatusOpen {
		store.Release()
		t.Fatalf("expected todo status open, got %q", items[0].Status)
	}
	store.Release()

}

func setupJobRepo(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	tmpDir, _ = filepath.EvalSymlinks(tmpDir)

	testsupport.SetupTestHome(t)

	client := jj.New()
	if err := client.Init(tmpDir); err != nil {
		t.Fatalf("init repo: %v", err)
	}

	configPath := filepath.Join(tmpDir, "incrementum.toml")
	config := "[job]\n" +
		"test-commands = [\"true\"]\n"
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return tmpDir
}
