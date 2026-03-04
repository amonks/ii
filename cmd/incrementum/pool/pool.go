package pool

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"monks.co/incrementum/agent"
	"monks.co/incrementum/internal/config"
	"monks.co/incrementum/internal/jj"
	"monks.co/incrementum/job"
	"monks.co/incrementum/todo"
	"monks.co/incrementum/workspace"
)

const (
	defaultPollInterval = time.Second
	defaultWorkerCount  = 4
	poolPurpose         = "pool worker"
	poolQueuePurpose    = "pool queue todo"
	poolStartPurpose    = "pool start todo"
	poolReopenPurpose   = "pool reopen todo"
)

type workspacePool interface {
	Acquire(repoPath string, opts workspace.AcquireOptions) (string, error)
	Release(wsPath string) error
	Close() error
}

type todoStore interface {
	Ready(limit int) ([]todo.Todo, error)
	Start(ids []string) ([]todo.Todo, error)
	QueueForMerge(ids []string, jobID string) ([]todo.Todo, error)
	Update(ids []string, opts todo.UpdateOptions) ([]todo.Todo, error)
	Release() error
}

type jobRunner interface {
	Run(string, string, job.RunOptions) (*job.RunResult, error)
}

var openWorkspacePool = func() (workspacePool, error) {
	return workspace.Open()
}

var openTodoStore = func(repoPath, purpose string) (todoStore, error) {
	return todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false, Purpose: purpose})
}

var updateWorkspaceStale = func(wsPath string) error {
	return jj.New().WorkspaceUpdateStale(wsPath)
}

var runJob jobRunner = jobRunnerFunc(job.Run)

var runWorkerFn = runWorker

type jobRunnerFunc func(string, string, job.RunOptions) (*job.RunResult, error)

func (runner jobRunnerFunc) Run(repoPath, todoID string, opts job.RunOptions) (*job.RunResult, error) {
	return runner(repoPath, todoID, opts)
}

// RunLLMFunc runs an LLM session for job execution.
// This matches job.RunOptions.RunLLM.
type RunLLMFunc func(job.AgentRunOptions) (job.AgentRunResult, error)

// TranscriptsFunc retrieves transcripts for job sessions.
type TranscriptsFunc func(string, []job.AgentSession) ([]job.AgentTranscript, error)

// Options configures pool execution.
type Options struct {
	Workers      int
	RepoPath     string
	RunLLM       RunLLMFunc
	Transcripts  TranscriptsFunc
	PollInterval time.Duration
	Now          func() time.Time
	LoadConfig   func(string) (*config.Config, error)
	RunTests     func(string, []string) ([]job.TestCommandResult, error)
	Model        string
	UpdateStale  func(string) error
	Snapshot     func(string) error
}

// Run starts a pool of job workers.
func Run(ctx context.Context, opts Options) error {
	opts = normalizeOptions(opts)
	if opts.RepoPath == "" {
		return fmt.Errorf("repo path is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.Workers < 1 {
		return fmt.Errorf("workers must be at least 1")
	}
	if opts.RunLLM == nil {
		return fmt.Errorf("RunLLM must be configured")
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pool, err := openWorkspacePool()
	if err != nil {
		return fmt.Errorf("open workspace pool: %w", err)
	}
	defer func() {
		_ = pool.Close()
	}()

	var wg sync.WaitGroup
	errCh := make(chan error, opts.Workers)
	for i := 0; i < opts.Workers; i++ {
		wg.Go(func() {
			if err := runWorkerFn(ctx, pool, opts); err != nil {
				errCh <- err
				if !errors.Is(err, context.Canceled) {
					cancel()
				}
				return
			}
		})
	}

	go func() {
		wg.Wait()
		close(errCh)
	}()

	var errs error
	for err := range errCh {
		normalized := normalizeWorkerExit(err)
		if normalized != nil {
			errs = errors.Join(errs, normalized)
		}
	}
	return errs
}

func normalizeWorkerExit(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func runWorker(ctx context.Context, pool workspacePool, opts Options) error {
	wsPath, err := pool.Acquire(opts.RepoPath, workspace.AcquireOptions{
		Purpose: poolPurpose,
		Rev:     "main",
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

		if err := updateWorkspaceStale(wsPath); err != nil {
			return err
		}

		store, err := openTodoStore(opts.RepoPath, poolPurpose)
		if err != nil {
			return err
		}
		ready, err := store.Ready(1)
		releaseErr := store.Release()
		if err != nil || releaseErr != nil {
			return errors.Join(err, releaseErr)
		}
		if len(ready) == 0 {
			if err := sleepOrDone(ctx, opts.PollInterval); err != nil {
				return err
			}
			continue
		}

		item := ready[0]
		startStore, err := openTodoStore(opts.RepoPath, poolStartPurpose)
		if err != nil {
			return err
		}
		_, err = startStore.Start([]string{item.ID})
		releaseErr = startStore.Release()
		if err != nil || releaseErr != nil {
			return errors.Join(err, releaseErr)
		}

		result, runErr := runJob.Run(opts.RepoPath, item.ID, job.RunOptions{
			SkipFinalize:  true,
			WorkspacePath: wsPath,
			RunLLM:        opts.RunLLM,
			Transcripts:   opts.Transcripts,
			LoadConfig:    opts.LoadConfig,
			RunTests:      opts.RunTests,
			Model:         opts.Model,
			Now:           opts.Now,
			UpdateStale:   opts.UpdateStale,
			Snapshot:      opts.Snapshot,
		})

		if runErr != nil {
			if err := reopenTodo(opts.RepoPath, item.ID); err != nil {
				return errors.Join(runErr, err)
			}
			if errors.Is(runErr, job.ErrJobInterrupted) || errors.Is(runErr, job.ErrJobAbandoned) || errors.Is(runErr, agent.ErrNoModelConfigured) {
				return runErr
			}
			continue
		}

		jobID := result.Job.ID
		queueStore, err := openTodoStore(opts.RepoPath, poolQueuePurpose)
		if err != nil {
			return err
		}
		_, err = queueStore.QueueForMerge([]string{item.ID}, jobID)
		releaseErr = queueStore.Release()
		if err != nil || releaseErr != nil {
			return errors.Join(err, releaseErr)
		}
	}
}

func reopenTodo(repoPath, todoID string) error {
	store, err := openTodoStore(repoPath, poolReopenPurpose)
	if err != nil {
		return err
	}
	status := todo.StatusOpen
	_, err = store.Update([]string{todoID}, todo.UpdateOptions{Status: &status})
	releaseErr := store.Release()
	return errors.Join(err, releaseErr)
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
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.LoadConfig == nil {
		opts.LoadConfig = config.Load
	}
	if opts.RunTests == nil {
		opts.RunTests = job.RunTestCommands
	}
	if opts.Transcripts == nil {
		opts.Transcripts = defaultTranscripts
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

func defaultTranscripts(_ string, sessions []job.AgentSession) ([]job.AgentTranscript, error) {
	if len(sessions) == 0 {
		return nil, nil
	}
	transcripts := make([]job.AgentTranscript, 0, len(sessions))
	for _, session := range sessions {
		transcripts = append(transcripts, job.AgentTranscript{Purpose: session.Purpose, Transcript: "-"})
	}
	return transcripts, nil
}
