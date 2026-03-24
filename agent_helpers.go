package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"

	"monks.co/ii/agent"
	"monks.co/ii/internal/db"
	"monks.co/ii/internal/paths"
	jobpkg "monks.co/ii/job"
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

func openAgentStoreForRepo(repoPath string) (*agent.Store, func() error, error) {
	sqlDB, closeFn, stateDir, err := openAgentDB()
	if err != nil {
		return nil, nil, err
	}

	store, err := agent.OpenWithDB(sqlDB, agent.Options{
		StateDir: stateDir,
		RepoPath: repoPath,
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

		result, err := handle.Wait()
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

func makeAgentRunnerFunc(repoPath string) (*agent.Store, error) {
	store, closeFn, err := openAgentStoreForRepo(repoPath)
	if err != nil {
		return nil, err
	}
	store.SetCloseFunc(closeFn)
	return store, nil
}
