package serve

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"monks.co/incrementum/job"
	"monks.co/incrementum/merge"
	"monks.co/incrementum/todo"
)

type fakeStore struct {
	listed       []todo.Todo
	mergedIDs    []string
	failedIDs    []string
	finishedIDs  []string
	releaseCalls int
}

func (s *fakeStore) List(filter todo.ListFilter) ([]todo.Todo, error) {
	return s.listed, nil
}

func (s *fakeStore) Merge(ids []string) ([]todo.Todo, error) {
	s.mergedIDs = append(s.mergedIDs, ids...)
	return nil, nil
}

func (s *fakeStore) MergeFailed(ids []string) ([]todo.Todo, error) {
	s.failedIDs = append(s.failedIDs, ids...)
	return nil, nil
}

func (s *fakeStore) Finish(ids []string) ([]todo.Todo, error) {
	s.finishedIDs = append(s.finishedIDs, ids...)
	return nil, nil
}

func (s *fakeStore) Release() error {
	s.releaseCalls++
	return nil
}

type fakeJobManager struct {
	job job.Job
}

func (m *fakeJobManager) Find(jobID string) (job.Job, error) {
	return m.job, nil
}

func (m *fakeJobManager) Close() error {
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
	if opts.Target != "main" {
		t.Fatalf("Target = %q, want %q", opts.Target, "main")
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
	if opts.Transcripts == nil {
		t.Fatalf("expected Transcripts to be set")
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

func TestProcessMergeSkipsFailureOnCancel(t *testing.T) {
	item := todo.Todo{ID: "todo-1", JobID: "job-1"}
	store := &fakeStore{}

	oldOpenStore := openTodoStore
	oldOpenJobManager := openJobManager
	oldRunMerge := runMerge
	t.Cleanup(func() {
		openTodoStore = oldOpenStore
		openJobManager = oldOpenJobManager
		runMerge = oldRunMerge
	})

	openTodoStore = func(repoPath, purpose string) (todoStore, error) {
		return store, nil
	}
	openJobManager = func(repoPath string) (jobManager, error) {
		return &fakeJobManager{job: job.Job{ID: "job-1", Changes: []job.JobChange{{ChangeID: "chg-1"}}}}, nil
	}
	cancelErr := context.Canceled
	runMerge = func(ctx context.Context, opts merge.Options) error {
		return cancelErr
	}

	opts := normalizeOptions(Options{
		RepoPath: "repo",
		RunLLM:   func(job.AgentRunOptions) (job.AgentRunResult, error) { return job.AgentRunResult{}, nil },
		Now:      func() time.Time { return time.Time{} },
	})

	err := processMerge(context.Background(), opts, "ws", item)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancel error, got %v", err)
	}
	if len(store.failedIDs) != 0 {
		t.Fatalf("did not expect merge failed to be recorded")
	}
	if len(store.finishedIDs) != 0 {
		t.Fatalf("did not expect finish to be called")
	}
}

func TestProcessMergeMarksFailure(t *testing.T) {
	item := todo.Todo{ID: "todo-1", JobID: "job-1"}
	store := &fakeStore{}

	oldOpenStore := openTodoStore
	oldOpenJobManager := openJobManager
	oldRunMerge := runMerge
	t.Cleanup(func() {
		openTodoStore = oldOpenStore
		openJobManager = oldOpenJobManager
		runMerge = oldRunMerge
	})

	openTodoStore = func(repoPath, purpose string) (todoStore, error) {
		return store, nil
	}
	openJobManager = func(repoPath string) (jobManager, error) {
		return &fakeJobManager{job: job.Job{ID: "job-1", Changes: []job.JobChange{{ChangeID: "chg-1"}}}}, nil
	}
	runMerge = func(ctx context.Context, opts merge.Options) error {
		return errors.New("merge failed")
	}

	opts := normalizeOptions(Options{
		RepoPath: "repo",
		RunLLM:   func(job.AgentRunOptions) (job.AgentRunResult, error) { return job.AgentRunResult{}, nil },
		Now:      func() time.Time { return time.Time{} },
	})

	err := processMerge(context.Background(), opts, "ws", item)
	if err != nil {
		t.Fatalf("process merge: %v", err)
	}
	if len(store.failedIDs) != 1 || store.failedIDs[0] != "todo-1" {
		t.Fatalf("expected merge failed to be recorded")
	}
	if len(store.finishedIDs) != 0 {
		t.Fatalf("did not expect finish to be called")
	}
}

func TestProcessMergeMarksDone(t *testing.T) {
	item := todo.Todo{ID: "todo-2", JobID: "job-2"}
	store := &fakeStore{}

	oldOpenStore := openTodoStore
	oldOpenJobManager := openJobManager
	oldRunMerge := runMerge
	t.Cleanup(func() {
		openTodoStore = oldOpenStore
		openJobManager = oldOpenJobManager
		runMerge = oldRunMerge
	})

	openTodoStore = func(repoPath, purpose string) (todoStore, error) {
		return store, nil
	}
	openJobManager = func(repoPath string) (jobManager, error) {
		return &fakeJobManager{job: job.Job{ID: "job-2", Changes: []job.JobChange{{ChangeID: "chg-2"}}}}, nil
	}
	runMerge = func(ctx context.Context, opts merge.Options) error {
		return nil
	}

	opts := normalizeOptions(Options{
		RepoPath: "repo",
		RunLLM:   func(job.AgentRunOptions) (job.AgentRunResult, error) { return job.AgentRunResult{}, nil },
		Now:      func() time.Time { return time.Time{} },
	})

	err := processMerge(context.Background(), opts, "ws", item)
	if err != nil {
		t.Fatalf("process merge: %v", err)
	}
	if len(store.finishedIDs) != 1 || store.finishedIDs[0] != "todo-2" {
		t.Fatalf("expected finish to be recorded")
	}
	if len(store.failedIDs) != 0 {
		t.Fatalf("did not expect merge failed to be called")
	}
}
