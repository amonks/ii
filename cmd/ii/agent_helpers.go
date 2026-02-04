package main

import (
	"context"
	"fmt"
	"os"

	"github.com/amonks/incrementum/agent"
	"github.com/amonks/incrementum/agents"
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
func makeRunLLMFunc(repoPath string, runner agents.Runner) (func(jobpkg.AgentRunOptions) (jobpkg.AgentRunResult, error), error) {
	return func(opts jobpkg.AgentRunOptions) (jobpkg.AgentRunResult, error) {
		ctx := context.Background()

		handle, err := runner.Run(ctx, agents.RunOptions{
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

		// Record events to job event log
		eventErrCh := jobpkg.RecordAgentEvents(opts.EventLog, handle.Events())
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

		store, err := agents.OpenTranscriptStore()
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

func makeAgentRunner(repoPath string, kind jobAgentKind) (agents.Runner, error) {
	switch kind {
	case jobAgentInternal:
		return agents.NewInternalRunner(repoPath)
	case jobAgentClaude:
		return agents.NewClaudeRunner(), nil
	case jobAgentCodex:
		return agents.NewCodexRunner(), nil
	default:
		return nil, fmt.Errorf("unknown agent %q", kind)
	}
}
