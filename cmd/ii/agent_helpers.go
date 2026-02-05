package main

import (
	"context"
	"os"

	"github.com/amonks/incrementum/agent"
	jobpkg "github.com/amonks/incrementum/job"
)

func openAgentStoreAndRepoPath() (*agent.Store, string, error) {
	repoPath, err := getRepoPath()
	if err != nil {
		return nil, "", err
	}

	store, err := agent.OpenWithOptions(agent.Options{
		RepoPath:  repoPath,
		StateDir:  os.Getenv("INCREMENTUM_STATE_DIR"),
		EventsDir: os.Getenv("INCREMENTUM_AGENT_EVENTS_DIR"),
	})
	if err != nil {
		return nil, "", err
	}

	return store, repoPath, nil
}

var makeAgentRunnerFunc = makeAgentRunner

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
			Version:   buildCommitID,
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
			SessionID: result.SessionID,
			ExitCode:  result.ExitCode,
			Error:     result.Error,
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

func makeAgentRunner(repoPath string) (*agent.Store, error) {
	return agent.OpenWithOptions(agent.Options{
		RepoPath:  repoPath,
		StateDir:  os.Getenv("INCREMENTUM_STATE_DIR"),
		EventsDir: os.Getenv("INCREMENTUM_AGENT_EVENTS_DIR"),
	})
}
