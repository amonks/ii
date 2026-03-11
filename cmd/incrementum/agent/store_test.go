package agent_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"monks.co/incrementum/agent"
	"monks.co/incrementum/internal/db"
	"monks.co/incrementum/internal/testsupport"
)

func TestOpen_NoConfig(t *testing.T) {
	testsupport.SetupTestHome(t)

	store, err := agent.Open()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer store.Close()

	// Should be able to open without config
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestOpenWithOptions_CustomDirs(t *testing.T) {
	testsupport.SetupTestHome(t)
	tmpDir := t.TempDir()

	stateDir := filepath.Join(tmpDir, "state")

	store, err := agent.OpenWithOptions(agent.Options{
		StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("OpenWithOptions failed: %v", err)
	}
	defer store.Close()

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
	defer store.Close()

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
	defer store.Close()

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
	defer store.Close()

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
	defer store.Close()

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
	defer store.Close()

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
	defer store.Close()

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
	defer store.Close()

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
	defer store.Close()

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
	defer store.Close()

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
	defer store.Close()

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

	sqlDB := openAgentTestDB(t, stateDir)
	repoName, err := db.GetOrCreateRepoName(sqlDB, "/path/to/repo")
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}

	now := time.Now()
	err = insertAgentSession(sqlDB, repoName, "12345678", agent.SessionActive, "claude-haiku-4-5-20251001", now, now, now, time.Time{}, nil, 0, 0, 0)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
	err = insertAgentSession(sqlDB, repoName, "87654321", agent.SessionCompleted, "claude-haiku-4-5-20251001", now.Add(-time.Hour), now.Add(-time.Hour), now.Add(-time.Hour), now.Add(-30*time.Minute), nil, 0, 0, 0)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	store, err := agent.OpenWithOptions(agent.Options{
		StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer store.Close()

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

	sqlDB := openAgentTestDB(t, stateDir)
	repoName, err := db.GetOrCreateRepoName(sqlDB, "/path/to/repo")
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}

	now := time.Now()
	err = insertAgentSession(sqlDB, repoName, "12345678", agent.SessionActive, "claude-haiku-4-5-20251001", now, now, now, time.Time{}, nil, 0, 0, 0)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	store, err := agent.OpenWithOptions(agent.Options{
		StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer store.Close()

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

	sqlDB := openAgentTestDB(t, stateDir)
	repoName, err := db.GetOrCreateRepoName(sqlDB, "/path/to/repo")
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}

	now := time.Now()
	err = insertAgentSession(sqlDB, repoName, "12345678", agent.SessionActive, "claude-haiku-4-5-20251001", now, now, now, time.Time{}, nil, 0, 0, 0)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	store, err := agent.OpenWithOptions(agent.Options{
		StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer store.Close()

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

	sqlDB := openAgentTestDB(t, stateDir)
	_, err := db.GetOrCreateRepoName(sqlDB, "/path/to/repo")
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}

	store, err := agent.OpenWithOptions(agent.Options{
		StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer store.Close()

	_, err = store.FindSession("/path/to/repo", "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent session")
	}
}

func TestFindSession_Ambiguous(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	stateDir := filepath.Join(homeDir, ".local", "state", "incrementum")

	sqlDB := openAgentTestDB(t, stateDir)
	repoName, err := db.GetOrCreateRepoName(sqlDB, "/path/to/repo")
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}

	now := time.Now()
	err = insertAgentSession(sqlDB, repoName, "12345678", agent.SessionActive, "claude-haiku-4-5-20251001", now, now, now, time.Time{}, nil, 0, 0, 0)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
	err = insertAgentSession(sqlDB, repoName, "12367890", agent.SessionActive, "claude-haiku-4-5-20251001", now, now, now, time.Time{}, nil, 0, 0, 0)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	store, err := agent.OpenWithOptions(agent.Options{
		StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer store.Close()

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


