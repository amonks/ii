package serve

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"monks.co/ii/internal/config"
	"monks.co/pkg/jj"
	internalstrings "monks.co/ii/internal/strings"
	"monks.co/ii/job"
	"monks.co/ii/merge"
	"monks.co/ii/pool"
	"monks.co/ii/todo"
	"monks.co/ww/ww"
	"golang.org/x/sync/errgroup"
)

const (
	defaultPollInterval   = time.Second
	defaultWorkerCount    = 4
	mergeWorkspacePurpose = "serve merge"
	mergeListPurpose      = "serve list queued"
	mergeStartPurpose     = "serve start merge"
	mergeFailPurpose      = "serve merge failed"
	mergeFinishPurpose    = "serve finish merge"
)

type workspacePool interface {
	Acquire(repoPath string, opts ww.AcquireOptions) (string, error)
	Release(wsPath string) error
	Close() error
}

type todoStore interface {
	List(filter todo.ListFilter) ([]todo.Todo, error)
	Merge(ids []string) ([]todo.Todo, error)
	MergeFailed(ids []string) ([]todo.Todo, error)
	Finish(ids []string) ([]todo.Todo, error)
	Release() error
}

type jobManager interface {
	Find(jobID string) (job.Job, error)
	Close() error
}

var openWorkspacePool = func() (workspacePool, error) {
	return ww.Open()
}

var openTodoStore = func(repoPath, purpose string) (todoStore, error) {
	return todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false, Purpose: purpose})
}

var openJobManager = func(repoPath string) (jobManager, error) {
	return job.Open(repoPath, job.OpenOptions{})
}

var runMerge = merge.Merge

var poolRun = pool.Run

var runMergeLoopFn = runMergeLoop

// RunLLMFunc runs an LLM session for job execution or merge conflict resolution.
// This matches job.RunOptions.RunLLM.
type RunLLMFunc func(job.AgentRunOptions) (job.AgentRunResult, error)

// Options configures serve execution.
type Options struct {
	Workers      int
	RepoPath     string
	Target       string
	RunLLM       RunLLMFunc
	PollInterval time.Duration
	Now          func() time.Time
	LoadConfig   func(string) (*config.Config, error)
	RunTests     func(string, []string) ([]job.TestCommandResult, error)
	Model        string
	UpdateStale  func(string) error
	Snapshot     func(string) error
}

// Run starts pooled workers and a merge loop.
func Run(ctx context.Context, opts Options) error {
	opts = normalizeOptions(opts)
	if opts.RepoPath == "" {
		return fmt.Errorf("repo path is required")
	}
	if opts.RunLLM == nil {
		return fmt.Errorf("RunLLM must be configured")
	}
	if opts.Workers < 1 {
		return fmt.Errorf("workers must be at least 1")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return poolRun(groupCtx, pool.Options{
			Workers:      opts.Workers,
			RepoPath:     opts.RepoPath,
			RunLLM:       pool.RunLLMFunc(opts.RunLLM),
			PollInterval: opts.PollInterval,
			Now:          opts.Now,
			LoadConfig:   opts.LoadConfig,
			RunTests:     opts.RunTests,
			Model:        opts.Model,
			UpdateStale:  opts.UpdateStale,
			Snapshot:     opts.Snapshot,
		})
	})
	group.Go(func() error {
		return runMergeLoopFn(groupCtx, opts)
	})

	if err := group.Wait(); err != nil {
		return normalizeServeExit(err)
	}
	return nil
}

func normalizeServeExit(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func runMergeLoop(ctx context.Context, opts Options) error {
	pool, err := openWorkspacePool()
	if err != nil {
		return fmt.Errorf("open workspace pool: %w", err)
	}
	defer func() {
		_ = pool.Close()
	}()

	wsPath, err := pool.Acquire(opts.RepoPath, ww.AcquireOptions{
		Purpose: mergeWorkspacePurpose,
		Rev:     opts.Target,
	})
	if err != nil {
		return err
	}
	defer func() {
		_ = pool.Release(wsPath)
	}()

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		queued, err := listQueuedForMerge(opts.RepoPath)
		if err != nil {
			return err
		}
		if len(queued) == 0 {
			if err := sleepOrDone(ctx, opts.PollInterval); err != nil {
				return err
			}
			continue
		}

		item := queued[0]
		if err := markMerging(opts.RepoPath, item.ID); err != nil {
			return err
		}
		if err := processMerge(ctx, opts, wsPath, item); err != nil {
			return err
		}
	}
}

func listQueuedForMerge(repoPath string) ([]todo.Todo, error) {
	store, err := openTodoStore(repoPath, mergeListPurpose)
	if err != nil {
		return nil, err
	}
	status := todo.StatusQueuedForMerge
	items, err := store.List(todo.ListFilter{Status: &status})
	releaseErr := store.Release()
	if err != nil || releaseErr != nil {
		return nil, errors.Join(err, releaseErr)
	}
	return items, nil
}

func markMerging(repoPath, todoID string) error {
	store, err := openTodoStore(repoPath, mergeStartPurpose)
	if err != nil {
		return err
	}
	_, err = store.Merge([]string{todoID})
	releaseErr := store.Release()
	return errors.Join(err, releaseErr)
}

func markMergeFailed(repoPath, todoID string) error {
	store, err := openTodoStore(repoPath, mergeFailPurpose)
	if err != nil {
		return err
	}
	_, err = store.MergeFailed([]string{todoID})
	releaseErr := store.Release()
	return errors.Join(err, releaseErr)
}

func markMerged(repoPath, todoID string) error {
	store, err := openTodoStore(repoPath, mergeFinishPurpose)
	if err != nil {
		return err
	}
	_, err = store.Finish([]string{todoID})
	releaseErr := store.Release()
	return errors.Join(err, releaseErr)
}

func processMerge(ctx context.Context, opts Options, wsPath string, item todo.Todo) error {
	mergeErr := mergeTodo(ctx, opts, wsPath, item)
	if mergeErr != nil {
		if errors.Is(mergeErr, context.Canceled) {
			return mergeErr
		}
		if err := markMergeFailed(opts.RepoPath, item.ID); err != nil {
			return errors.Join(mergeErr, err)
		}
		return nil
	}
	return markMerged(opts.RepoPath, item.ID)
}

func mergeTodo(ctx context.Context, opts Options, wsPath string, item todo.Todo) error {
	if internalstrings.IsBlank(item.JobID) {
		return fmt.Errorf("todo %s is missing job id", item.ID)
	}
	manager, err := openJobManager(opts.RepoPath)
	if err != nil {
		return err
	}
	jobItem, err := manager.Find(item.JobID)
	closeErr := manager.Close()
	if err != nil || closeErr != nil {
		return errors.Join(err, closeErr)
	}
	changeID, err := latestChangeID(jobItem)
	if err != nil {
		return err
	}
	return runMerge(ctx, merge.Options{
		RepoPath:      opts.RepoPath,
		WorkspacePath: wsPath,
		ChangeID:      changeID,
		Target:        opts.Target,
		RunLLM:        merge.RunLLMFunc(opts.RunLLM),
		Now:           opts.Now,
	})
}

func latestChangeID(jobItem job.Job) (string, error) {
	if len(jobItem.Changes) == 0 {
		return "", fmt.Errorf("job %s has no changes", jobItem.ID)
	}
	changeID := internalstrings.TrimSpace(jobItem.Changes[len(jobItem.Changes)-1].ChangeID)
	if changeID == "" {
		return "", fmt.Errorf("job %s has empty change id", jobItem.ID)
	}
	return changeID, nil
}

func sleepOrDone(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func normalizeOptions(opts Options) Options {
	if opts.Workers == 0 {
		opts.Workers = defaultWorkerCount
	}
	if opts.PollInterval == 0 {
		opts.PollInterval = defaultPollInterval
	}
	if opts.Target == "" {
		opts.Target = "main"
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.LoadConfig == nil {
		opts.LoadConfig = config.Load
	}
	if opts.RunTests == nil {
		opts.RunTests = job.RunTestCommands
	}
	if opts.UpdateStale == nil || opts.Snapshot == nil {
		client := jj.New()
		if opts.UpdateStale == nil {
			opts.UpdateStale = client.WorkspaceUpdateStale
		}
		if opts.Snapshot == nil {
			opts.Snapshot = client.Snapshot
		}
	}
	return opts
}

