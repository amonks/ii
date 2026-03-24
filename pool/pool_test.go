package pool

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"monks.co/ii/job"
	"monks.co/ii/todo"
	"monks.co/ww/ww"
)

type fakePool struct {
	acquired bool
	released bool
}

func (p *fakePool) Acquire(repoPath string, opts ww.AcquireOptions) (string, error) {
	p.acquired = true
	return "ws-1", nil
}

func (p *fakePool) Release(wsPath string) error {
	p.released = true
	return nil
}

func (p *fakePool) Close() error {
	return nil
}

type fakeStore struct {
	readyCalls    int
	readyFn       func(int) ([]todo.Todo, error)
	startIDs      []string
	queueIDs      []string
	queueJobID    string
	updateCalls   int
	updateStatus  *todo.Status
	releaseCalls  int
}

func (s *fakeStore) Ready(limit int) ([]todo.Todo, error) {
	s.readyCalls++
	if s.readyFn != nil {
		return s.readyFn(s.readyCalls)
	}
	return nil, nil
}

func (s *fakeStore) Start(ids []string) ([]todo.Todo, error) {
	s.startIDs = append(s.startIDs, ids...)
	return nil, nil
}

func (s *fakeStore) QueueForMerge(ids []string, jobID string) ([]todo.Todo, error) {
	s.queueIDs = append(s.queueIDs, ids...)
	s.queueJobID = jobID
	return nil, nil
}

func (s *fakeStore) Update(ids []string, opts todo.UpdateOptions) ([]todo.Todo, error) {
	if opts.Status != nil {
		s.updateCalls++
		s.updateStatus = opts.Status
	}
	return nil, nil
}

func (s *fakeStore) Release() error {
	s.releaseCalls++
	return nil
}

func TestNormalizeOptionsDefaults(t *testing.T) {
	opts := normalizeOptions(Options{})
	if opts.Workers != defaultWorkerCount {
		t.Fatalf("Workers = %d, want %d", opts.Workers, defaultWorkerCount)
	}
	if opts.PollInterval != defaultPollInterval {
		t.Fatalf("PollInterval = %s, want %s", opts.PollInterval, defaultPollInterval)
	}
	if opts.Now == nil {
		t.Fatalf("expected Now to be set")
	}
	if opts.LoadConfig == nil {
		t.Fatalf("expected LoadConfig to be set")
	}
	if opts.RunTests == nil {
		t.Fatalf("expected RunTests to be set")
	}
	if opts.UpdateStale == nil {
		t.Fatalf("expected UpdateStale to be set")
	}
	if opts.Snapshot == nil {
		t.Fatalf("expected Snapshot to be set")
	}
}

func TestRunRequiresRunLLM(t *testing.T) {
	err := Run(context.Background(), Options{RepoPath: "repo"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "RunLLM") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWorkerQueuesForMerge(t *testing.T) {
	errStop := errors.New("stop")
	readyStore := &fakeStore{readyFn: func(call int) ([]todo.Todo, error) {
		if call == 1 {
			return []todo.Todo{{ID: "todo-1"}}, nil
		}
		return nil, errStop
	}}
	startStore := &fakeStore{}
	queueStore := &fakeStore{}

	oldOpenTodoStore := openTodoStore
	oldRunJob := runJob
	oldUpdateWorkspaceStale := updateWorkspaceStale
	t.Cleanup(func() {
		openTodoStore = oldOpenTodoStore
		runJob = oldRunJob
		updateWorkspaceStale = oldUpdateWorkspaceStale
	})

	openTodoStore = func(repoPath, purpose string) (todoStore, error) {
		switch purpose {
		case poolPurpose:
			return readyStore, nil
		case poolStartPurpose:
			return startStore, nil
		case poolQueuePurpose:
			return queueStore, nil
		default:
			return &fakeStore{}, nil
		}
	}

	stalePath := ""
	updateWorkspaceStale = func(wsPath string) error {
		stalePath = wsPath
		return nil
	}

	runJob = jobRunnerFunc(func(repoPath, todoID string, opts job.RunOptions) (*job.RunResult, error) {
		return &job.RunResult{Job: job.Job{ID: "job-1"}}, nil
	})

	pool := &fakePool{}
	opts := normalizeOptions(Options{
		RepoPath: "repo",
		RunLLM:   func(job.AgentRunOptions) (job.AgentRunResult, error) { return job.AgentRunResult{}, nil },
		Now:      func() time.Time { return time.Time{} },
	})

	err := runWorker(context.Background(), pool, opts)
	if !errors.Is(err, errStop) {
		t.Fatalf("expected stop error, got %v", err)
	}
	if len(startStore.startIDs) != 1 || startStore.startIDs[0] != "todo-1" {
		t.Fatalf("expected start to be called for todo-1")
	}
	if queueStore.queueJobID != "job-1" {
		t.Fatalf("expected queue job id to be job-1, got %q", queueStore.queueJobID)
	}
	if stalePath != "ws-1" {
		t.Fatalf("expected stale update for ws-1, got %q", stalePath)
	}
}

func TestRunWorkerReopensOnFailure(t *testing.T) {
	errJob := errors.New("job failed")
	errStop := errors.New("stop")
	readyStore := &fakeStore{readyFn: func(call int) ([]todo.Todo, error) {
		if call == 1 {
			return []todo.Todo{{ID: "todo-2"}}, nil
		}
		return nil, errStop
	}}
	reopenStore := &fakeStore{}
	startStore := &fakeStore{}

	oldOpenTodoStore := openTodoStore
	oldRunJob := runJob
	oldUpdateWorkspaceStale := updateWorkspaceStale
	t.Cleanup(func() {
		openTodoStore = oldOpenTodoStore
		runJob = oldRunJob
		updateWorkspaceStale = oldUpdateWorkspaceStale
	})

	openTodoStore = func(repoPath, purpose string) (todoStore, error) {
		switch purpose {
		case poolPurpose:
			return readyStore, nil
		case poolStartPurpose:
			return startStore, nil
		case poolReopenPurpose:
			return reopenStore, nil
		default:
			return &fakeStore{}, nil
		}
	}

	stalePath := ""
	updateWorkspaceStale = func(wsPath string) error {
		stalePath = wsPath
		return nil
	}

	runJob = jobRunnerFunc(func(repoPath, todoID string, opts job.RunOptions) (*job.RunResult, error) {
		return &job.RunResult{}, errJob
	})

	pool := &fakePool{}
	opts := normalizeOptions(Options{
		RepoPath: "repo",
		RunLLM:   func(job.AgentRunOptions) (job.AgentRunResult, error) { return job.AgentRunResult{}, nil },
		Now:      func() time.Time { return time.Time{} },
	})

	err := runWorker(context.Background(), pool, opts)
	if !errors.Is(err, errStop) {
		t.Fatalf("expected stop error, got %v", err)
	}
	if reopenStore.updateCalls != 1 {
		t.Fatalf("expected reopen to be called once, got %d", reopenStore.updateCalls)
	}
	if reopenStore.updateStatus == nil || *reopenStore.updateStatus != todo.StatusOpen {
		t.Fatalf("expected reopen to set status open")
	}
	if stalePath != "ws-1" {
		t.Fatalf("expected stale update for ws-1, got %q", stalePath)
	}
}

func TestRunCancelsOnWorkerError(t *testing.T) {
	var calls int32
	stopErr := errors.New("stop")

	oldRunWorker := runWorkerFn
	t.Cleanup(func() {
		runWorkerFn = oldRunWorker
	})

	runWorkerFn = func(ctx context.Context, pool workspacePool, opts Options) error {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			return stopErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			return nil
		}
	}

	err := Run(context.Background(), Options{
		RepoPath: "repo",
		Workers:  2,
		RunLLM:   func(job.AgentRunOptions) (job.AgentRunResult, error) { return job.AgentRunResult{}, nil },
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, stopErr) {
		t.Fatalf("expected stop error, got %v", err)
	}
}
