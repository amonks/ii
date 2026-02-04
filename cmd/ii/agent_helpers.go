package main

import (
	"context"

	"github.com/amonks/incrementum/agent"
	jobpkg "github.com/amonks/incrementum/job"
)

func openAgentStoreAndRepoPath() (*agent.Store, string, error) {
	repoPath, err := getRepoPath()
	if err != nil {
		return nil, "", err
	}

	store, err := agent.OpenWithOptions(agent.Options{
		RepoPath: repoPath,
	})
	if err != nil {
		return nil, "", err
	}

	return store, repoPath, nil
}

// makeRunLLMFunc creates an LLM run function for use with job.RunOptions.RunLLM.
func makeRunLLMFunc(repoPath string) (func(jobpkg.AgentRunOptions) (jobpkg.AgentRunResult, error), error) {
	store, err := agent.OpenWithOptions(agent.Options{
		RepoPath: repoPath,
	})
	if err != nil {
		return nil, err
	}

	return func(opts jobpkg.AgentRunOptions) (jobpkg.AgentRunResult, error) {
		ctx := context.Background()

		handle, err := store.Run(ctx, agent.RunOptions{
			RepoPath:  opts.RepoPath,
			WorkDir:   opts.WorkspacePath,
			Prompt:    opts.Prompt,
			Model:     opts.Model,
			StartedAt: opts.StartedAt,
			Version:   buildCommitID,
		})
		if err != nil {
			return jobpkg.AgentRunResult{}, err
		}

		// Record events to job event log
		eventErrCh := jobpkg.RecordAgentEvents(opts.EventLog, handle.Events)
		result, err := handle.Wait()
		eventErr := <-eventErrCh
		if err != nil {
			return jobpkg.AgentRunResult{}, err
		}
		if eventErr != nil {
			return jobpkg.AgentRunResult{}, eventErr
		}

		return jobpkg.AgentRunResult{
			SessionID: result.SessionID,
			ExitCode:  result.ExitCode,
		}, nil
	}, nil
}

// makeTranscriptsFunc creates a transcripts function for use with job.RunOptions.Transcripts.
func makeTranscriptsFunc() func(string, []jobpkg.AgentSession) ([]jobpkg.AgentTranscript, error) {
	return func(repoPath string, sessions []jobpkg.AgentSession) ([]jobpkg.AgentTranscript, error) {
		if len(sessions) == 0 {
			return nil, nil
		}

		store, err := agent.Open()
		if err != nil {
			return nil, err
		}

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
