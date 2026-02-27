package job

import (
	"errors"
	"time"

	"github.com/amonks/incrementum/agent"
	internalagent "github.com/amonks/incrementum/internal/agent"
	statestore "github.com/amonks/incrementum/internal/state"
	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/internal/todoenv"
)

// AgentRunOptions configures an LLM run for job execution.
type AgentRunOptions struct {
	RepoPath      string
	WorkspacePath string
	Prompt        internalagent.PromptContent
	Model         string
	StartedAt     time.Time
	EventLog      *EventLog
	Env           []string
}

// AgentRunResult captures output from running an LLM session.
type AgentRunResult struct {
	SessionID string
	ExitCode  int
	// Error contains the error message when ExitCode is non-zero.
	// This field is optional and may be empty even when ExitCode != 0;
	// not all failure conditions produce a detailed error message.
	Error string
	// InputTokens is the number of input tokens consumed.
	InputTokens int
	// OutputTokens is the number of output tokens generated.
	OutputTokens int
	// TotalTokens is the total number of tokens consumed.
	TotalTokens int
	// ContextWindow is the model's context window size, for diagnostics.
	ContextWindow int
}

func agentRunEnv() []string {
	return []string{todoenv.ProposerEnvVar + "=true"}
}

// AgentSession identifies an LLM session within a job.
// This is an alias to state.JobAgentSession for compatibility with Job.AgentSessions.
type AgentSession = statestore.JobAgentSession

// AgentTranscript contains the transcript for an LLM session.
type AgentTranscript struct {
	Purpose    string
	Transcript string
}

// runLLMWithEvents runs an LLM session, recording events and emitting
// job-level events for the run.
func runLLMWithEvents(opts RunOptions, runOpts AgentRunOptions, purpose string) (AgentRunResult, error) {
	snapshotWorkspace(opts.Snapshot, runOpts.WorkspacePath)
	if err := appendJobEvent(opts.EventLog, jobEventAgentStart, agentStartEventData{Purpose: purpose, Model: runOpts.Model}); err != nil {
		return AgentRunResult{}, err
	}
	result, err := opts.RunLLM(runOpts)
	if err != nil {
		logErr := appendJobEvent(opts.EventLog, jobEventAgentError, agentErrorEventData{Purpose: purpose, Error: err.Error()})
		if logErr != nil {
			return AgentRunResult{}, errors.Join(err, logErr)
		}
		return AgentRunResult{}, err
	}
	if err := appendJobEvent(opts.EventLog, jobEventAgentEnd, agentEndEventData{
		Purpose:       purpose,
		SessionID:     result.SessionID,
		ExitCode:      result.ExitCode,
		Error:         result.Error,
		InputTokens:   result.InputTokens,
		OutputTokens:  result.OutputTokens,
		TotalTokens:   result.TotalTokens,
		ContextWindow: result.ContextWindow,
	}); err != nil {
		return AgentRunResult{}, err
	}
	return result, nil
}

// loadTranscript loads a transcript for the given session.
func loadTranscript(opts RunOptions, session AgentSession) string {
	if opts.Transcripts == nil || internalstrings.IsBlank(session.ID) {
		return ""
	}
	// Transcripts don't need repoPath since session ID is globally unique
	transcripts, err := opts.Transcripts("", []AgentSession{session})
	if err != nil || len(transcripts) == 0 {
		return ""
	}
	return transcripts[0].Transcript
}

// Agent event data types for job event logging

type agentStartEventData struct {
	Purpose string `json:"purpose"`
	Model   string `json:"model,omitempty"`
}

type agentEndEventData struct {
	Purpose       string `json:"purpose"`
	SessionID     string `json:"session_id,omitempty"`
	ExitCode      int    `json:"exit_code"`
	Error         string `json:"error,omitempty"`
	InputTokens   int    `json:"input_tokens,omitempty"`
	OutputTokens  int    `json:"output_tokens,omitempty"`
	TotalTokens   int    `json:"total_tokens,omitempty"`
	ContextWindow int    `json:"context_window,omitempty"`
}

type agentErrorEventData struct {
	Purpose string `json:"purpose"`
	Error   string `json:"error"`
}

// RecordAgentEvents forwards agent events to the job event log.
// Returns a channel that receives any error encountered during recording.
func RecordAgentEvents(log *EventLog, events <-chan agent.Event) <-chan error {
	done := make(chan error, 1)
	if events == nil {
		done <- nil
		return done
	}
	go func() {
		var recordErr error
		for event := range events {
			if log == nil || recordErr != nil {
				continue
			}
			sse := agent.EventToSSE(event)
			recordErr = log.Append(Event{ID: sse.ID, Name: sse.Name, Data: sse.Data})
		}
		done <- recordErr
	}()
	return done
}

// defaultRunLLM is the default implementation for RunOptions.RunLLM.
// It uses the agent package to run LLM sessions.
func defaultRunLLM(opts AgentRunOptions) (AgentRunResult, error) {
	// Note: The actual agent run is handled by the caller who sets up
	// the agent runner. This function is called by normalizeRunOptions
	// to set up a default.
	return AgentRunResult{}, errors.New("RunLLM not configured; set RunOptions.RunLLM or use CLI helpers")
}
