package agents

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/amonks/incrementum/agent"
	internalids "github.com/amonks/incrementum/internal/ids"
)

// Re-export types from agent for convenience.
type (
	// AgentConfig configures an agent run.
	AgentConfig = agent.AgentConfig

	// Event is an interface implemented by all agent event types.
	Event = agent.Event

	// AgentStartEvent indicates the agent run has started.
	AgentStartEvent = agent.AgentStartEvent

	// AgentEndEvent indicates the agent run has completed.
	AgentEndEvent = agent.AgentEndEvent

	// TurnStartEvent indicates a new turn has started.
	TurnStartEvent = agent.TurnStartEvent

	// TurnEndEvent indicates a turn has completed.
	TurnEndEvent = agent.TurnEndEvent

	// MessageStartEvent indicates the LLM has started streaming.
	MessageStartEvent = agent.MessageStartEvent

	// MessageUpdateEvent contains a streaming delta from the LLM.
	MessageUpdateEvent = agent.MessageUpdateEvent

	// MessageEndEvent indicates the LLM has finished streaming.
	MessageEndEvent = agent.MessageEndEvent

	// ToolExecutionStartEvent indicates a tool execution has started.
	ToolExecutionStartEvent = agent.ToolExecutionStartEvent

	// ToolExecutionEndEvent indicates a tool execution has completed.
	ToolExecutionEndEvent = agent.ToolExecutionEndEvent

	// SSEEvent represents an event in SSE format.
	SSEEvent = agent.SSEEvent
)

// EventToSSE converts an agent event to an SSE payload.
var EventToSSE = agent.EventToSSE

// RunOptions configures an agent run.
type RunOptions struct {
	RepoPath  string
	WorkDir   string
	Prompt    string
	Model     string
	StartedAt time.Time
	Version   string
	Env       []string
}

// RunResult contains the result of an agent run.
type RunResult struct {
	SessionID string
	ExitCode  int
	// Error contains the error message when ExitCode is non-zero.
	// This field is optional and may be empty even when ExitCode != 0;
	// external agent backends may not provide error details beyond the exit code.
	Error string
}

// RunHandle provides access to a running agent session.
type RunHandle interface {
	Events() <-chan Event
	Wait() (RunResult, error)
}

// Runner is implemented by agent backends.
type Runner interface {
	Run(ctx context.Context, opts RunOptions) (RunHandle, error)
}

// TranscriptStore provides access to stored agent transcripts.
type TranscriptStore interface {
	TranscriptSnapshot(sessionID string) (string, error)
}

// OpenTranscriptStore opens the internal transcript store.
func OpenTranscriptStore() (TranscriptStore, error) {
	return agent.Open()
}

// NewInternalRunner returns a runner that uses the built-in agent package.
func NewInternalRunner(repoPath string) (Runner, error) {
	store, err := agent.OpenWithOptions(agent.Options{RepoPath: repoPath})
	if err != nil {
		return nil, err
	}
	return internalRunner{store: store}, nil
}

type internalRunner struct {
	store *agent.Store
}

func (r internalRunner) Run(ctx context.Context, opts RunOptions) (RunHandle, error) {
	handle, err := r.store.Run(ctx, agent.RunOptions{
		RepoPath:  opts.RepoPath,
		WorkDir:   opts.WorkDir,
		Prompt:    opts.Prompt,
		Model:     opts.Model,
		StartedAt: opts.StartedAt,
		Version:   opts.Version,
		Env:       opts.Env,
	})
	if err != nil {
		return nil, err
	}
	return internalHandle{handle: handle}, nil
}

type internalHandle struct {
	handle *agent.RunHandle
}

func (h internalHandle) Events() <-chan Event {
	return h.handle.Events
}

func (h internalHandle) Wait() (RunResult, error) {
	result, err := h.handle.Wait()
	if err != nil {
		return RunResult{}, err
	}
	return RunResult{SessionID: result.SessionID, ExitCode: result.ExitCode, Error: result.Error}, nil
}

// NewClaudeRunner returns a runner that shells out to the Claude CLI.
func NewClaudeRunner() Runner {
	return shellRunner{name: "claude", command: []string{"claude", "-p", "--dangerously-skip-permissions"}}
}

// NewCodexRunner returns a runner that shells out to the Codex CLI.
func NewCodexRunner() Runner {
	return shellRunner{name: "codex", command: []string{"codex", "exec", "--skip-git-repo-check"}}
}

type shellRunner struct {
	name    string
	command []string
}

func (r shellRunner) Run(ctx context.Context, opts RunOptions) (RunHandle, error) {
	resultCh := make(chan runResult, 1)
	go func() {
		result, err := r.runOnce(ctx, opts)
		resultCh <- runResult{result: result, err: err}
	}()
	return shellHandle{resultCh: resultCh}, nil
}

type runResult struct {
	result RunResult
	err    error
}

type shellHandle struct {
	resultCh <-chan runResult
}

func (h shellHandle) Events() <-chan Event {
	return nil
}

func (h shellHandle) Wait() (RunResult, error) {
	outcome := <-h.resultCh
	return outcome.result, outcome.err
}

func (r shellRunner) runOnce(ctx context.Context, opts RunOptions) (RunResult, error) {
	if len(r.command) == 0 {
		return RunResult{}, errors.New("agent command is required")
	}
	cmd := exec.CommandContext(ctx, r.command[0], r.command[1:]...)
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}
	cmd.Stdin = strings.NewReader(opts.Prompt)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
			err = nil
		} else {
			return RunResult{}, err
		}
	}
	startedAt := opts.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	sessionID := "external-" + r.name + "-" + internalids.GenerateWithTimestamp(opts.Prompt, startedAt, internalids.DefaultLength)
	return RunResult{SessionID: sessionID, ExitCode: exitCode}, nil
}
