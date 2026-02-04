package agent_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/agent"
	"github.com/amonks/incrementum/internal/state"
	"github.com/amonks/incrementum/internal/testsupport"
)

func TestOpen_NoConfig(t *testing.T) {
	testsupport.SetupTestHome(t)

	store, err := agent.Open()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be able to open without config
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestOpenWithOptions_CustomDirs(t *testing.T) {
	testsupport.SetupTestHome(t)
	tmpDir := t.TempDir()

	stateDir := filepath.Join(tmpDir, "state")
	eventsDir := filepath.Join(tmpDir, "events")

	store, err := agent.OpenWithOptions(agent.Options{
		StateDir:  stateDir,
		EventsDir: eventsDir,
	})
	if err != nil {
		t.Fatalf("OpenWithOptions failed: %v", err)
	}

	// Verify store was created
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestResolveModel_Explicit(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configContent := `
[[llm.providers]]
name = "anthropic"
api = "anthropic-messages"
base-url = "https://api.anthropic.com"
api-key-command = "echo test-key"
models = ["claude-haiku-4-5-20251001"]
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store, err := agent.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	model, err := store.ResolveModel("claude-haiku-4-5-20251001", "")
	if err != nil {
		t.Fatalf("ResolveModel failed: %v", err)
	}

	if model.ID != "claude-haiku-4-5-20251001" {
		t.Errorf("expected ID 'claude-haiku-4-5-20251001', got %q", model.ID)
	}
}

func TestResolveModel_EnvVar(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configContent := `
[[llm.providers]]
name = "anthropic"
api = "anthropic-messages"
base-url = "https://api.anthropic.com"
api-key-command = "echo test-key"
models = ["claude-haiku-4-5-20251001", "claude-haiku-4-5"]
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	t.Setenv("INCREMENTUM_AGENT_MODEL", "claude-haiku-4-5")

	store, err := agent.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// With no explicit model, should use env var
	model, err := store.ResolveModel("", "")
	if err != nil {
		t.Fatalf("ResolveModel failed: %v", err)
	}

	if model.ID != "claude-haiku-4-5" {
		t.Errorf("expected ID 'claude-haiku-4-5', got %q", model.ID)
	}
}

func TestResolveModel_TaskModel(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configContent := `
[[llm.providers]]
name = "anthropic"
api = "anthropic-messages"
base-url = "https://api.anthropic.com"
api-key-command = "echo test-key"
models = ["claude-haiku-4-5-20251001", "claude-haiku-4-5"]
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store, err := agent.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// With task model specified, should use that
	model, err := store.ResolveModel("", "claude-haiku-4-5")
	if err != nil {
		t.Fatalf("ResolveModel failed: %v", err)
	}

	if model.ID != "claude-haiku-4-5" {
		t.Errorf("expected ID 'claude-haiku-4-5', got %q", model.ID)
	}
}

func TestResolveModel_ConfigFallback(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configContent := `
[[llm.providers]]
name = "anthropic"
api = "anthropic-messages"
base-url = "https://api.anthropic.com"
api-key-command = "echo test-key"
models = ["claude-haiku-4-5-20251001"]

[agent]
model = "claude-haiku-4-5-20251001"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store, err := agent.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// With no explicit model or env var, should use agent.model from config
	model, err := store.ResolveModel("", "")
	if err != nil {
		t.Fatalf("ResolveModel failed: %v", err)
	}

	if model.ID != "claude-haiku-4-5-20251001" {
		t.Errorf("expected ID 'claude-haiku-4-5-20251001', got %q", model.ID)
	}
}

func TestResolveModel_LLMModelFallback(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configContent := `
[[llm.providers]]
name = "anthropic"
api = "anthropic-messages"
base-url = "https://api.anthropic.com"
api-key-command = "echo test-key"
models = ["claude-haiku-4-5-20251001"]

[llm]
model = "claude-haiku-4-5-20251001"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store, err := agent.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// With no explicit model, env var, or agent.model, should fallback to llm.model
	model, err := store.ResolveModel("", "")
	if err != nil {
		t.Fatalf("ResolveModel failed: %v", err)
	}

	if model.ID != "claude-haiku-4-5-20251001" {
		t.Errorf("expected ID 'claude-haiku-4-5-20251001', got %q", model.ID)
	}
}

func TestResolveImplementationModel(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configContent := `
[[llm.providers]]
name = "anthropic"
api = "anthropic-messages"
base-url = "https://api.anthropic.com"
api-key-command = "echo test-key"
models = ["claude-haiku-4-5-20251001", "claude-haiku-4-5"]

[job]
implementation-model = "claude-haiku-4-5"

[agent]
model = "claude-haiku-4-5-20251001"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store, err := agent.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// ResolveImplementationModel should use job.implementation-model over agent.model
	model, err := store.ResolveImplementationModel("")
	if err != nil {
		t.Fatalf("ResolveImplementationModel failed: %v", err)
	}

	if model.ID != "claude-haiku-4-5" {
		t.Errorf("expected ID 'claude-haiku-4-5', got %q", model.ID)
	}
}

func TestResolveCodeReviewModel(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configContent := `
[[llm.providers]]
name = "anthropic"
api = "anthropic-messages"
base-url = "https://api.anthropic.com"
api-key-command = "echo test-key"
models = ["claude-haiku-4-5-20251001", "claude-haiku-4-5"]

[job]
code-review-model = "claude-haiku-4-5"

[agent]
model = "claude-haiku-4-5-20251001"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store, err := agent.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// ResolveCodeReviewModel should use job.code-review-model over agent.model
	model, err := store.ResolveCodeReviewModel("")
	if err != nil {
		t.Fatalf("ResolveCodeReviewModel failed: %v", err)
	}

	if model.ID != "claude-haiku-4-5" {
		t.Errorf("expected ID 'claude-haiku-4-5', got %q", model.ID)
	}
}

func TestResolveProjectReviewModel(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configContent := `
[[llm.providers]]
name = "anthropic"
api = "anthropic-messages"
base-url = "https://api.anthropic.com"
api-key-command = "echo test-key"
models = ["claude-haiku-4-5-20251001", "claude-haiku-4-5"]

[job]
project-review-model = "claude-haiku-4-5"

[agent]
model = "claude-haiku-4-5-20251001"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store, err := agent.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// ResolveProjectReviewModel should use job.project-review-model over agent.model
	model, err := store.ResolveProjectReviewModel("")
	if err != nil {
		t.Fatalf("ResolveProjectReviewModel failed: %v", err)
	}

	if model.ID != "claude-haiku-4-5" {
		t.Errorf("expected ID 'claude-haiku-4-5', got %q", model.ID)
	}
}

func TestResolveModel_NoModelConfigured(t *testing.T) {
	testsupport.SetupTestHome(t)

	store, err := agent.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// With no model anywhere, should error
	_, err = store.ResolveModel("", "")
	if err == nil {
		t.Error("expected error when no model configured")
	}
}

func TestListSessions_Empty(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	stateDir := filepath.Join(homeDir, ".local", "state", "incrementum")

	store, err := agent.OpenWithOptions(agent.Options{
		StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	sessions, err := store.ListSessions("/some/repo/path")
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestListSessions_WithSessions(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	stateDir := filepath.Join(homeDir, ".local", "state", "incrementum")

	// Create state with sessions
	stateStore := state.NewStore(stateDir)
	err := stateStore.Update(func(st *state.State) error {
		st.Repos["test-repo"] = state.RepoInfo{SourcePath: "/path/to/repo"}
		st.AgentSessions["test-repo/12345678"] = state.AgentSession{
			ID:        "12345678",
			Repo:      "test-repo",
			Status:    state.AgentSessionActive,
			Model:     "claude-haiku-4-5-20251001",
			CreatedAt: time.Now(),
			StartedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		st.AgentSessions["test-repo/87654321"] = state.AgentSession{
			ID:        "87654321",
			Repo:        "test-repo",
			Status:      state.AgentSessionCompleted,
			Model:       "claude-haiku-4-5-20251001",
			CreatedAt:   time.Now().Add(-time.Hour),
			StartedAt:   time.Now().Add(-time.Hour),
			UpdatedAt:   time.Now().Add(-time.Hour),
			CompletedAt: time.Now().Add(-30 * time.Minute),
		}
		return nil
	})
	if err != nil {
		t.Fatalf("setup state failed: %v", err)
	}

	store, err := agent.OpenWithOptions(agent.Options{
		StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	sessions, err := store.ListSessions("/path/to/repo")
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	// Verify sessions are sorted by creation time (most recent first)
	if sessions[0].CreatedAt.Before(sessions[1].CreatedAt) {
		t.Error("expected sessions to be sorted most recent first")
	}
}

func TestFindSession_ExactMatch(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	stateDir := filepath.Join(homeDir, ".local", "state", "incrementum")

	// Create state with session
	stateStore := state.NewStore(stateDir)
	err := stateStore.Update(func(st *state.State) error {
		st.Repos["test-repo"] = state.RepoInfo{SourcePath: "/path/to/repo"}
		st.AgentSessions["test-repo/12345678"] = state.AgentSession{
			ID:        "12345678",
			Repo:      "test-repo",
			Status:    state.AgentSessionActive,
			Model:     "claude-haiku-4-5-20251001",
			CreatedAt: time.Now(),
			StartedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		return nil
	})
	if err != nil {
		t.Fatalf("setup state failed: %v", err)
	}

	store, err := agent.OpenWithOptions(agent.Options{
		StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	session, err := store.FindSession("/path/to/repo", "12345678")
	if err != nil {
		t.Fatalf("FindSession failed: %v", err)
	}

	if session.ID != "12345678" {
		t.Errorf("expected ID '12345678', got %q", session.ID)
	}
}

func TestFindSession_PrefixMatch(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	stateDir := filepath.Join(homeDir, ".local", "state", "incrementum")

	// Create state with session
	stateStore := state.NewStore(stateDir)
	err := stateStore.Update(func(st *state.State) error {
		st.Repos["test-repo"] = state.RepoInfo{SourcePath: "/path/to/repo"}
		st.AgentSessions["test-repo/12345678"] = state.AgentSession{
			ID:        "12345678",
			Repo:      "test-repo",
			Status:    state.AgentSessionActive,
			Model:     "claude-haiku-4-5-20251001",
			CreatedAt: time.Now(),
			StartedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		return nil
	})
	if err != nil {
		t.Fatalf("setup state failed: %v", err)
	}

	store, err := agent.OpenWithOptions(agent.Options{
		StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Should find session by prefix
	session, err := store.FindSession("/path/to/repo", "123")
	if err != nil {
		t.Fatalf("FindSession failed: %v", err)
	}

	if session.ID != "12345678" {
		t.Errorf("expected ID '12345678', got %q", session.ID)
	}
}

func TestFindSession_NotFound(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	stateDir := filepath.Join(homeDir, ".local", "state", "incrementum")

	// Create state with repo but no sessions
	stateStore := state.NewStore(stateDir)
	err := stateStore.Update(func(st *state.State) error {
		st.Repos["test-repo"] = state.RepoInfo{SourcePath: "/path/to/repo"}
		return nil
	})
	if err != nil {
		t.Fatalf("setup state failed: %v", err)
	}

	store, err := agent.OpenWithOptions(agent.Options{
		StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	_, err = store.FindSession("/path/to/repo", "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent session")
	}
}

func TestFindSession_Ambiguous(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	stateDir := filepath.Join(homeDir, ".local", "state", "incrementum")

	// Create state with sessions that have the same prefix
	stateStore := state.NewStore(stateDir)
	err := stateStore.Update(func(st *state.State) error {
		st.Repos["test-repo"] = state.RepoInfo{SourcePath: "/path/to/repo"}
		st.AgentSessions["test-repo/12345678"] = state.AgentSession{
			ID:        "12345678",
			Repo:      "test-repo",
			Status:    state.AgentSessionActive,
			Model:     "claude-haiku-4-5-20251001",
			CreatedAt: time.Now(),
			StartedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		st.AgentSessions["test-repo/12367890"] = state.AgentSession{
			ID:        "12367890",
			Repo:      "test-repo",
			Status:    state.AgentSessionActive,
			Model:     "claude-haiku-4-5-20251001",
			CreatedAt: time.Now(),
			StartedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		return nil
	})
	if err != nil {
		t.Fatalf("setup state failed: %v", err)
	}

	store, err := agent.OpenWithOptions(agent.Options{
		StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Should fail with ambiguous prefix
	_, err = store.FindSession("/path/to/repo", "123")
	if err == nil {
		t.Error("expected error for ambiguous session ID")
	}
}

func TestSessionStatus_IsValid(t *testing.T) {
	tests := []struct {
		status agent.SessionStatus
		valid  bool
	}{
		{agent.SessionActive, true},
		{agent.SessionCompleted, true},
		{agent.SessionFailed, true},
		{agent.SessionStatus("unknown"), false},
		{agent.SessionStatus(""), false},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			if got := tc.status.IsValid(); got != tc.valid {
				t.Errorf("IsValid() = %v, want %v", got, tc.valid)
			}
		})
	}
}

func TestTranscriptSnapshot(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	eventsDir := filepath.Join(homeDir, ".local", "share", "incrementum", "agent", "events")

	// Create events directory
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatalf("failed to create events dir: %v", err)
	}

	// Write a sample event log
	sessionID := "test12345"
	eventLog := `{"ID":"0","Name":"agent.start","Data":"{}"}
{"ID":"1","Name":"message.end","Data":"{\"Message\":{\"Role\":\"assistant\",\"Content\":[{\"Type\":\"text\",\"Text\":\"Hello from the assistant!\"}]}}"}
{"ID":"2","Name":"agent.end","Data":"{}"}
`
	logPath := filepath.Join(eventsDir, sessionID+".jsonl")
	if err := os.WriteFile(logPath, []byte(eventLog), 0o644); err != nil {
		t.Fatalf("failed to write event log: %v", err)
	}

	store, err := agent.OpenWithOptions(agent.Options{
		EventsDir: eventsDir,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	transcript, err := store.TranscriptSnapshot(sessionID)
	if err != nil {
		t.Fatalf("TranscriptSnapshot failed: %v", err)
	}

	// Verify transcript contains expected content
	if transcript == "" {
		t.Error("expected non-empty transcript")
	}
	if !strings.Contains(transcript, "# Agent Session") {
		t.Error("expected transcript to contain '# Agent Session'")
	}
	if !strings.Contains(transcript, "Hello from the assistant!") {
		t.Error("expected transcript to contain assistant message")
	}
}

func TestTranscriptSnapshot_NotFound(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	eventsDir := filepath.Join(homeDir, ".local", "share", "incrementum", "agent", "events")

	store, err := agent.OpenWithOptions(agent.Options{
		EventsDir: eventsDir,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	_, err = store.TranscriptSnapshot("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent session")
	}
}

func TestTranscriptSnapshot_IncludesToolOutput(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	eventsDir := filepath.Join(homeDir, ".local", "share", "incrementum", "agent", "events")

	// Create events directory
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatalf("failed to create events dir: %v", err)
	}

	// Write event log with tool output
	sessionID := "tooltest"
	eventLog := `{"ID":"0","Name":"agent.start","Data":"{}"}
{"ID":"1","Name":"message.end","Data":"{\"Message\":{\"Role\":\"assistant\",\"Content\":[{\"Type\":\"text\",\"Text\":\"Running a command...\"}]}}"}
{"ID":"2","Name":"tool.end","Data":"{\"Result\":{\"Content\":[{\"Type\":\"text\",\"Text\":\"command output here\"}]}}"}
{"ID":"3","Name":"message.end","Data":"{\"Message\":{\"Role\":\"assistant\",\"Content\":[{\"Type\":\"text\",\"Text\":\"Command completed.\"}]}}"}
{"ID":"4","Name":"agent.end","Data":"{}"}
`
	logPath := filepath.Join(eventsDir, sessionID+".jsonl")
	if err := os.WriteFile(logPath, []byte(eventLog), 0o644); err != nil {
		t.Fatalf("failed to write event log: %v", err)
	}

	store, err := agent.OpenWithOptions(agent.Options{
		EventsDir: eventsDir,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	transcript, err := store.TranscriptSnapshot(sessionID)
	if err != nil {
		t.Fatalf("TranscriptSnapshot failed: %v", err)
	}

	// TranscriptSnapshot should include tool output
	if !strings.Contains(transcript, "command output here") {
		t.Error("expected TranscriptSnapshot to include tool output")
	}
	if !strings.Contains(transcript, "Running a command...") {
		t.Error("expected transcript to contain first assistant message")
	}
	if !strings.Contains(transcript, "Command completed.") {
		t.Error("expected transcript to contain second assistant message")
	}
}

func TestTranscript_ExcludesToolOutput(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	eventsDir := filepath.Join(homeDir, ".local", "share", "incrementum", "agent", "events")
	stateDir := filepath.Join(homeDir, ".local", "state", "incrementum")
	repoPath := "/path/to/test-repo"
	sessionID := "toolexclude"

	// Create events directory
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatalf("failed to create events dir: %v", err)
	}

	// Create state with a session for this repo
	stateStore := state.NewStore(stateDir)
	err := stateStore.Update(func(st *state.State) error {
		st.Repos["test-repo"] = state.RepoInfo{SourcePath: repoPath}
		st.AgentSessions["test-repo/"+sessionID] = state.AgentSession{
			ID:        sessionID,
			Repo:      "test-repo",
			Status:    state.AgentSessionActive,
			Model:     "claude-haiku-4-5-20251001",
			CreatedAt: time.Now(),
			StartedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		return nil
	})
	if err != nil {
		t.Fatalf("setup state failed: %v", err)
	}

	// Write event log with tool output to that session's event log
	eventLog := `{"ID":"0","Name":"agent.start","Data":"{}"}
{"ID":"1","Name":"message.end","Data":"{\"Message\":{\"Role\":\"assistant\",\"Content\":[{\"Type\":\"text\",\"Text\":\"Running a command...\"}]}}"}
{"ID":"2","Name":"tool.end","Data":"{\"Result\":{\"Content\":[{\"Type\":\"text\",\"Text\":\"secret tool output\"}]}}"}
{"ID":"3","Name":"message.end","Data":"{\"Message\":{\"Role\":\"assistant\",\"Content\":[{\"Type\":\"text\",\"Text\":\"Command completed.\"}]}}"}
{"ID":"4","Name":"agent.end","Data":"{}"}
`
	logPath := filepath.Join(eventsDir, sessionID+".jsonl")
	if err := os.WriteFile(logPath, []byte(eventLog), 0o644); err != nil {
		t.Fatalf("failed to write event log: %v", err)
	}

	store, err := agent.OpenWithOptions(agent.Options{
		EventsDir: eventsDir,
		StateDir:  stateDir,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Call Transcript (not TranscriptSnapshot) - should exclude tool output
	transcript, err := store.Transcript(repoPath, sessionID)
	if err != nil {
		t.Fatalf("Transcript failed: %v", err)
	}

	// Verify tool output is NOT present
	if strings.Contains(transcript, "secret tool output") {
		t.Error("expected Transcript to EXCLUDE tool output, but found 'secret tool output'")
	}

	// Verify assistant messages ARE present (sanity check)
	if !strings.Contains(transcript, "Running a command...") {
		t.Error("expected transcript to contain first assistant message")
	}
	if !strings.Contains(transcript, "Command completed.") {
		t.Error("expected transcript to contain second assistant message")
	}
}
