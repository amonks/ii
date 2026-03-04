package job

import (
	"errors"
	"testing"
	"time"

	"monks.co/incrementum/internal/agent"
	"monks.co/incrementum/internal/llm"
)

func TestRecordAgentEvents(t *testing.T) {
	tmpDir := t.TempDir()
	log, err := OpenEventLog("test-job", EventLogOptions{EventsDir: tmpDir})
	if err != nil {
		t.Fatalf("OpenEventLog error: %v", err)
	}
	defer log.Close()

	eventCh := make(chan agent.Event, 10)

	// Simulate agent events including tool start/end pairs
	eventCh <- agent.AgentStartEvent{Config: agent.AgentConfig{
		Model: llm.Model{ID: "test-model"},
	}}
	eventCh <- agent.TurnStartEvent{TurnIndex: 0}
	eventCh <- agent.ToolExecutionStartEvent{
		TurnIndex:  0,
		ToolCallID: "tool-1",
		ToolName:   "bash",
		Arguments:  map[string]any{"command": "echo hello"},
	}
	eventCh <- agent.ToolExecutionEndEvent{
		TurnIndex:  0,
		ToolCallID: "tool-1",
		ToolName:   "bash",
		Result:     llm.ToolResultMessage{ToolCallID: "tool-1"},
	}
	eventCh <- agent.AgentEndEvent{
		Messages: nil,
		Usage:    llm.Usage{Input: 100, Output: 50},
	}
	close(eventCh)

	errCh := RecordAgentEvents(log, eventCh)
	err = <-errCh
	if err != nil {
		t.Fatalf("RecordAgentEvents error: %v", err)
	}

	// Verify all events were recorded
	events, err := EventSnapshot("test-job", EventLogOptions{EventsDir: tmpDir})
	if err != nil {
		t.Fatalf("EventSnapshot error: %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(events))
	}

	// Verify tool start and end events are both present
	var toolStarts, toolEnds int
	for _, e := range events {
		switch e.Name {
		case "tool.start":
			toolStarts++
		case "tool.end":
			toolEnds++
		}
	}
	if toolStarts != 1 {
		t.Errorf("expected 1 tool.start event, got %d", toolStarts)
	}
	if toolEnds != 1 {
		t.Errorf("expected 1 tool.end event, got %d", toolEnds)
	}
}

func TestRecordAgentEvents_NilEvents(t *testing.T) {
	errCh := RecordAgentEvents(nil, nil)
	err := <-errCh
	if err != nil {
		t.Fatalf("expected nil error for nil events, got: %v", err)
	}
}

func TestRecordAgentEvents_NilLog(t *testing.T) {
	eventCh := make(chan agent.Event, 1)
	eventCh <- agent.TurnStartEvent{TurnIndex: 0}
	close(eventCh)

	errCh := RecordAgentEvents(nil, eventCh)
	err := <-errCh
	if err != nil {
		t.Fatalf("expected nil error for nil log, got: %v", err)
	}
}

func TestRunLLMWithEvents_Success(t *testing.T) {
	tmpDir := t.TempDir()
	log, err := OpenEventLog("test-job", EventLogOptions{EventsDir: tmpDir})
	if err != nil {
		t.Fatalf("OpenEventLog error: %v", err)
	}
	defer log.Close()

	opts := RunOptions{
		EventLog: log,
		RunLLM: func(runOpts AgentRunOptions) (AgentRunResult, error) {
			if runOpts.Prompt.UserContent != "test prompt" {
				t.Errorf("unexpected prompt: %q", runOpts.Prompt.UserContent)
			}
			if runOpts.Model != "test-model" {
				t.Errorf("unexpected model: %q", runOpts.Model)
			}
			return AgentRunResult{
				SessionID: "test123",
				ExitCode:  0,
			}, nil
		},
	}

	runOpts := AgentRunOptions{
		RepoPath:      "/test/repo",
		WorkspacePath: tmpDir,
		Prompt:        agent.PromptContent{UserContent: "test prompt"},
		Model:         "test-model",
		StartedAt:     time.Now(),
	}

	result, err := runLLMWithEvents(opts, runOpts, "implement")
	if err != nil {
		t.Fatalf("runLLMWithEvents error: %v", err)
	}
	if result.SessionID != "test123" {
		t.Errorf("unexpected session ID: %q", result.SessionID)
	}
	if result.ExitCode != 0 {
		t.Errorf("unexpected exit code: %d", result.ExitCode)
	}

	// Verify events were logged
	events, err := EventSnapshot("test-job", EventLogOptions{EventsDir: tmpDir})
	if err != nil {
		t.Fatalf("EventSnapshot error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Name != jobEventAgentStart {
		t.Errorf("expected first event to be %q, got %q", jobEventAgentStart, events[0].Name)
	}
	if events[1].Name != jobEventAgentEnd {
		t.Errorf("expected second event to be %q, got %q", jobEventAgentEnd, events[1].Name)
	}
}

func TestRunLLMWithEvents_Error(t *testing.T) {
	tmpDir := t.TempDir()
	log, err := OpenEventLog("test-job", EventLogOptions{EventsDir: tmpDir})
	if err != nil {
		t.Fatalf("OpenEventLog error: %v", err)
	}
	defer log.Close()

	testErr := errors.New("agent failed")
	opts := RunOptions{
		EventLog: log,
		RunLLM: func(runOpts AgentRunOptions) (AgentRunResult, error) {
			return AgentRunResult{}, testErr
		},
	}

	runOpts := AgentRunOptions{
		RepoPath:      "/test/repo",
		WorkspacePath: tmpDir,
		Prompt:        agent.PromptContent{UserContent: "test prompt"},
		Model:         "test-model",
		StartedAt:     time.Now(),
	}

	_, err = runLLMWithEvents(opts, runOpts, "implement")
	if err == nil {
		t.Fatal("expected error from runLLMWithEvents")
	}
	if !errors.Is(err, testErr) {
		t.Errorf("expected error to wrap testErr, got: %v", err)
	}

	// Verify error event was logged
	events, err := EventSnapshot("test-job", EventLogOptions{EventsDir: tmpDir})
	if err != nil {
		t.Fatalf("EventSnapshot error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Name != jobEventAgentStart {
		t.Errorf("expected first event to be %q, got %q", jobEventAgentStart, events[0].Name)
	}
	if events[1].Name != jobEventAgentError {
		t.Errorf("expected second event to be %q, got %q", jobEventAgentError, events[1].Name)
	}
}

func TestRunLLMWithEvents_CallsRunLLM(t *testing.T) {
	tmpDir := t.TempDir()
	log, err := OpenEventLog("test-job", EventLogOptions{EventsDir: tmpDir})
	if err != nil {
		t.Fatalf("OpenEventLog error: %v", err)
	}
	defer log.Close()

	llmCalled := false

	opts := RunOptions{
		EventLog: log,
		RunLLM: func(runOpts AgentRunOptions) (AgentRunResult, error) {
			llmCalled = true
			if runOpts.Prompt.UserContent != "test prompt" {
				t.Errorf("unexpected prompt: %q", runOpts.Prompt.UserContent)
			}
			if runOpts.Model != "test-model" {
				t.Errorf("unexpected model: %q", runOpts.Model)
			}
			return AgentRunResult{
				SessionID: "dispatch123",
				ExitCode:  0,
			}, nil
		},
	}

	runOpts := AgentRunOptions{
		RepoPath:      "/test/repo",
		WorkspacePath: tmpDir,
		Prompt:        agent.PromptContent{UserContent: "test prompt"},
		Model:         "test-model",
		StartedAt:     time.Now(),
	}

	result, err := runLLMWithEvents(opts, runOpts, "implement")
	if err != nil {
		t.Fatalf("runLLMWithEvents error: %v", err)
	}
	if !llmCalled {
		t.Error("expected RunLLM to be called")
	}
	if result.SessionID != "dispatch123" {
		t.Errorf("unexpected session ID: %q", result.SessionID)
	}
	if result.ExitCode != 0 {
		t.Errorf("unexpected exit code: %d", result.ExitCode)
	}
}

func TestLoadTranscript(t *testing.T) {
	t.Run("returns empty string when Transcripts is nil", func(t *testing.T) {
		opts := RunOptions{}
		result := loadTranscript(opts, AgentSession{Purpose: "implement", ID: "123"})
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("returns empty string when session ID is blank", func(t *testing.T) {
		opts := RunOptions{
			Transcripts: func(_ string, _ []AgentSession) ([]AgentTranscript, error) {
				return []AgentTranscript{{Purpose: "implement", Transcript: "test"}}, nil
			},
		}
		result := loadTranscript(opts, AgentSession{Purpose: "implement", ID: ""})
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("returns transcript from Transcripts function", func(t *testing.T) {
		opts := RunOptions{
			Transcripts: func(_ string, sessions []AgentSession) ([]AgentTranscript, error) {
				if len(sessions) != 1 || sessions[0].ID != "123" {
					t.Errorf("unexpected sessions: %v", sessions)
				}
				return []AgentTranscript{{Purpose: "implement", Transcript: "test transcript"}}, nil
			},
		}
		result := loadTranscript(opts, AgentSession{Purpose: "implement", ID: "123"})
		if result != "test transcript" {
			t.Errorf("expected 'test transcript', got %q", result)
		}
	})

	t.Run("returns empty string on fetch error", func(t *testing.T) {
		opts := RunOptions{
			Transcripts: func(_ string, _ []AgentSession) ([]AgentTranscript, error) {
				return nil, errors.New("fetch error")
			},
		}
		result := loadTranscript(opts, AgentSession{Purpose: "implement", ID: "123"})
		if result != "" {
			t.Errorf("expected empty string on error, got %q", result)
		}
	})

	t.Run("returns empty string when no transcripts returned", func(t *testing.T) {
		opts := RunOptions{
			Transcripts: func(_ string, _ []AgentSession) ([]AgentTranscript, error) {
				return []AgentTranscript{}, nil
			},
		}
		result := loadTranscript(opts, AgentSession{Purpose: "implement", ID: "123"})
		if result != "" {
			t.Errorf("expected empty string for empty result, got %q", result)
		}
	})
}
