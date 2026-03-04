package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"

	"monks.co/incrementum/agent"
	"monks.co/incrementum/internal/db"
	"monks.co/incrementum/internal/paths"
	jobpkg "monks.co/incrementum/job"
)

func openAgentStoreAndRepoPath() (*agent.Store, func() error, string, error) {
	repoPath, err := getRepoPath()
	if err != nil {
		return nil, nil, "", err
	}

	store, closeFn, err := openAgentStoreForRepo(repoPath)
	if err != nil {
		return nil, nil, "", err
	}

	return store, closeFn, repoPath, nil
}

func openAgentStore() (*agent.Store, func() error, error) {
	sqlDB, closeFn, stateDir, err := openAgentDB()
	if err != nil {
		return nil, nil, err
	}

	store, err := agent.OpenWithDB(sqlDB, agent.Options{
		StateDir: stateDir,
		EventsDir: os.Getenv("INCREMENTUM_AGENT_EVENTS_DIR"),
	})
	if err != nil {
		_ = closeFn()
		return nil, nil, err
	}

	return store, closeFn, nil
}

func openAgentStoreForRepo(repoPath string) (*agent.Store, func() error, error) {
	sqlDB, closeFn, stateDir, err := openAgentDB()
	if err != nil {
		return nil, nil, err
	}

	store, err := agent.OpenWithDB(sqlDB, agent.Options{
		StateDir: stateDir,
		RepoPath:  repoPath,
		EventsDir: os.Getenv("INCREMENTUM_AGENT_EVENTS_DIR"),
	})
	if err != nil {
		_ = closeFn()
		return nil, nil, err
	}

	return store, closeFn, nil
}

func openAgentDB() (*sql.DB, func() error, string, error) {
	stateDir, err := paths.ResolveWithDefault(os.Getenv("INCREMENTUM_STATE_DIR"), paths.DefaultStateDir)
	if err != nil {
		return nil, nil, "", err
	}

	path := filepath.Join(stateDir, "state.db")
	store, err := db.Open(path, db.OpenOptions{LegacyJSONPath: filepath.Join(stateDir, "state.json")})
	if err != nil {
		return nil, nil, "", err
	}

	return store.SqlDB(), store.Close, stateDir, nil
}

// makeRunLLMFunc creates an LLM run function for use with job.RunOptions.RunLLM.
func makeRunLLMFunc(repoPath string, store *agent.Store) (func(jobpkg.AgentRunOptions) (jobpkg.AgentRunResult, error), error) {
	return func(opts jobpkg.AgentRunOptions) (jobpkg.AgentRunResult, error) {
		ctx := context.Background()

		handle, err := store.Run(ctx, agent.RunOptions{
			RepoPath:  opts.RepoPath,
			WorkDir:   opts.WorkspacePath,
			Prompt:    opts.Prompt,
			Model:     opts.Model,
			StartedAt: opts.StartedAt,
			Version:   buildVersion(),
			Env:       opts.Env,
		})
		if err != nil {
			return jobpkg.AgentRunResult{}, err
		}

		// Record events to job event log.
		// Wait for recording to complete before calling Wait() to avoid a race
		// condition where both RecordAgentEvents and Wait() consume from the
		// same events channel.
		eventErrCh := jobpkg.RecordAgentEvents(opts.EventLog, handle.Events)
		eventErr := <-eventErrCh
		result, err := handle.Wait()
		if eventErr != nil {
			return jobpkg.AgentRunResult{}, eventErr
		}
		if err != nil {
			return jobpkg.AgentRunResult{}, err
		}

		return jobpkg.AgentRunResult{
			SessionID:     result.SessionID,
			ExitCode:      result.ExitCode,
			Error:         result.Error,
			InputTokens:   result.Usage.Input,
			OutputTokens:  result.Usage.Output,
			TotalTokens:   result.Usage.Total,
			ContextWindow: result.ContextWindow,
		}, nil
	}, nil
}

// makeTranscriptsFunc creates a transcripts function for use with job.RunOptions.Transcripts.
func makeTranscriptsFunc() func(string, []jobpkg.AgentSession) ([]jobpkg.AgentTranscript, error) {
	return func(repoPath string, sessions []jobpkg.AgentSession) ([]jobpkg.AgentTranscript, error) {
		if len(sessions) == 0 {
			return nil, nil
		}

		store, closeFn, err := openAgentStore()
		if err != nil {
			return nil, err
		}
		defer closeFn()

		transcripts := make([]jobpkg.AgentTranscript, 0, len(sessions))
		for _, session := range sessions {
			transcript, err := store.TranscriptSnapshot(session.ID)
			if err != nil {
				// If we can't get a transcript, just use an empty one
				transcript = "-"
			}
			if transcript == "" {
				transcript = "-"
			}
			transcripts = append(transcripts, jobpkg.AgentTranscript{
				Purpose:    session.Purpose,
				Transcript: transcript,
			})
		}
		return transcripts, nil
	}
}

func makeAgentRunner(repoPath string) (*agent.Store, error) {
	store, closeFn, err := openAgentStoreForRepo(repoPath)
	if err != nil {
		return nil, err
	}
	store.SetCloseFunc(closeFn)
	return store, nil
}

func makeAgentRunnerFunc(repoPath string) (*agent.Store, error) {
	return makeAgentRunner(repoPath)
}
