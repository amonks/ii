package job

import (
	"context"
	"errors"
	"time"

	"monks.co/ii/internal/agents"
	internalagent "monks.co/pkg/agent"
	"monks.co/ii/internal/todoenv"
)

// AgentRunOptions configures an LLM run for job execution.
type AgentRunOptions struct {
	RepoPath      string
	WorkspacePath string
	Prompt        internalagent.PromptContent
	Model         string
	StartedAt     time.Time
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
// This is an alias to JobAgentSession for compatibility with Job.AgentSessions.
type AgentSession = JobAgentSession

// AgentTranscript contains the transcript for an LLM session.
type AgentTranscript struct {
	Purpose    string
	Transcript string
}

// runLLM runs an LLM session with snapshot support.
// It uses the adapter if available, falling back to the RunLLM callback.
func runLLM(opts RunOptions, runOpts AgentRunOptions) (AgentRunResult, error) {
	snapshotWorkspace(opts.Snapshot, runOpts.WorkspacePath)

	// Use adapter if available
	if opts.Adapter != nil {
		return runWithAdapter(opts.Adapter, runOpts)
	}

	// Fall back to legacy callback
	result, err := opts.RunLLM(runOpts)
	if err != nil {
		return AgentRunResult{}, err
	}
	return result, nil
}

// runWithAdapter invokes an adapter with flattened prompt content.
func runWithAdapter(a agents.Adapter, runOpts AgentRunOptions) (AgentRunResult, error) {
	flatPrompt := agents.FlattenPrompt(runOpts.Prompt)

	result, err := a.Run(context.Background(), agents.RunOptions{
		WorkDir: runOpts.WorkspacePath,
		Prompt:  flatPrompt,
		Model:   runOpts.Model,
		Env:     runOpts.Env,
	})

	agentResult := AgentRunResult{
		ExitCode:     result.ExitCode,
		InputTokens:  result.InputTokens,
		OutputTokens: result.OutputTokens,
		TotalTokens:  result.InputTokens + result.OutputTokens,
	}

	if err != nil {
		agentResult.Error = err.Error()
		return agentResult, err
	}
	if result.ExitCode != 0 {
		agentResult.Error = result.Stderr
	}

	return agentResult, nil
}

// defaultRunLLM is the default implementation for RunOptions.RunLLM.
func defaultRunLLM(opts AgentRunOptions) (AgentRunResult, error) {
	return AgentRunResult{}, errors.New("RunLLM not configured; set RunOptions.Adapter or RunOptions.RunLLM")
}
