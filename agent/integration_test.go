package agent_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"monks.co/ii/agent"
	internalagent "monks.co/pkg/agent"
	"monks.co/ii/internal/agents"
	"monks.co/ii/internal/config"
	"monks.co/ii/internal/testsupport"
	"monks.co/pkg/llm"
)

// These integration tests require a real provider configuration in ./incrementum.toml.
// They fail loudly if required model/provider configuration is missing.

func setupTestConfigFromRepoConfig(t *testing.T, requiredModels ...string) string {
	t.Helper()

	// Read the real global config before SetupTestHome overrides HOME.
	realHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}
	globalConfig, err := os.ReadFile(filepath.Join(realHome, ".config", "incrementum", "config.toml"))
	if err != nil {
		t.Fatalf("failed to read global config: %v", err)
	}

	homeDir := testsupport.SetupTestHome(t)

	// Copy the real global config (which has LLM provider definitions) into
	// the test HOME so the store can resolve models.
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), globalConfig, 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Validate required models against the copied config.
	cfg, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	registry, err := agents.NewModelRegistry(cfg)
	if err != nil {
		t.Fatalf("failed to build model registry: %v", err)
	}
	for _, modelID := range requiredModels {
		if _, err := registry.GetModel(modelID); err != nil {
			t.Fatalf("test requires model %q to be configured: %v", modelID, err)
		}
	}

	return homeDir
}

func TestAgentStoreRun_SimpleCompletion_Anthropic(t *testing.T) {
	homeDir := setupTestConfigFromRepoConfig(t, "claude-sonnet-4-5")

	tmpDir := t.TempDir()
	stateDir := filepath.Join(homeDir, ".local", "state", "incrementum")

	store, err := agent.OpenWithOptions(agent.Options{
		StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("OpenWithOptions failed: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	opts := agent.RunOptions{
		RepoPath:  "/test/repo",
		WorkDir:   tmpDir,
		Prompt:    internalagent.PromptContent{UserContent: "What is 2+2? Just say the number, nothing else."},
		Model:     "claude-sonnet-4-5",
		StartedAt: time.Now(),
	}

	handle, err := store.Run(ctx, opts)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	result, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Verify session ID was generated
	if result.SessionID == "" {
		t.Error("Expected session ID")
	}

	// Verify we got messages
	if len(result.Messages) < 2 {
		t.Errorf("Expected at least 2 messages, got %d", len(result.Messages))
	}

	// Verify the assistant message contains "4"
	var responseText strings.Builder
	for _, msg := range result.Messages {
		if am, ok := msg.(llm.AssistantMessage); ok {
			for _, block := range am.Content {
				if tc, ok := block.(llm.TextContent); ok {
					responseText.WriteString(tc.Text)
				}
			}
		}
	}

	if !strings.Contains(responseText.String(), "4") {
		t.Errorf("Expected response to contain '4', got %q", responseText.String())
	}

	// Verify usage was tracked
	if result.Usage.Input == 0 {
		t.Error("Expected input tokens > 0")
	}
	if result.Usage.Output == 0 {
		t.Error("Expected output tokens > 0")
	}

	// Verify session was recorded
	sessions, err := store.ListSessions("/test/repo")
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(sessions))
	}

	if len(sessions) > 0 {
		session := sessions[0]
		if session.ID != result.SessionID {
			t.Errorf("Session ID mismatch: %s != %s", session.ID, result.SessionID)
		}
		if session.Status != agent.SessionCompleted {
			t.Errorf("Expected status 'completed', got %q", session.Status)
		}
		if session.TokensUsed == 0 {
			t.Error("Expected TokensUsed > 0")
		}
	}

	t.Logf("Response: %s", responseText.String())
	t.Logf("Session ID: %s", result.SessionID)
	t.Logf("Usage: input=%d, output=%d, cost=$%.6f", result.Usage.Input, result.Usage.Output, result.Usage.Cost.Total)
}

func TestAgentStoreRun_SimpleCompletion_OpenAI(t *testing.T) {
	homeDir := setupTestConfigFromRepoConfig(t, "gpt-5.2")

	tmpDir := t.TempDir()
	stateDir := filepath.Join(homeDir, ".local", "state", "incrementum")

	store, err := agent.OpenWithOptions(agent.Options{
		StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("OpenWithOptions failed: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	opts := agent.RunOptions{
		RepoPath:  "/test/repo",
		WorkDir:   tmpDir,
		Prompt:    internalagent.PromptContent{UserContent: "What is 2+2? Just say the number, nothing else."},
		Model:     "gpt-5.2",
		StartedAt: time.Now(),
	}

	handle, err := store.Run(ctx, opts)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	result, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Verify we got messages
	if len(result.Messages) < 2 {
		t.Errorf("Expected at least 2 messages, got %d", len(result.Messages))
	}

	// Verify the response contains "4"
	var responseText strings.Builder
	for _, msg := range result.Messages {
		if am, ok := msg.(llm.AssistantMessage); ok {
			for _, block := range am.Content {
				if tc, ok := block.(llm.TextContent); ok {
					responseText.WriteString(tc.Text)
				}
			}
		}
	}

	if !strings.Contains(responseText.String(), "4") {
		t.Errorf("Expected response to contain '4', got %q", responseText.String())
	}

	t.Logf("Response: %s", responseText.String())
	t.Logf("Session ID: %s", result.SessionID)
	t.Logf("Usage: input=%d, output=%d", result.Usage.Input, result.Usage.Output)
}

func TestAgentStoreRun_ToolCall_Anthropic(t *testing.T) {
	homeDir := setupTestConfigFromRepoConfig(t, "claude-sonnet-4-5")

	tmpDir := t.TempDir()
	stateDir := filepath.Join(homeDir, ".local", "state", "incrementum")

	store, err := agent.OpenWithOptions(agent.Options{
		StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("OpenWithOptions failed: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Ask the agent to use the bash tool
	opts := agent.RunOptions{
		RepoPath:  "/test/repo",
		WorkDir:   tmpDir,
		Prompt:    internalagent.PromptContent{UserContent: "Please run 'echo hello world' using the bash tool and tell me the output."},
		Model:     "claude-sonnet-4-5",
		StartedAt: time.Now(),
	}

	handle, err := store.Run(ctx, opts)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	result, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Verify conversation has tool results
	hasToolResult := false
	for _, msg := range result.Messages {
		if _, ok := msg.(llm.ToolResultMessage); ok {
			hasToolResult = true
			break
		}
	}

	if !hasToolResult {
		t.Error("Expected tool result in conversation")
	}

	t.Logf("Total messages: %d", len(result.Messages))
	t.Logf("Session ID: %s", result.SessionID)
	t.Logf("Usage: input=%d, output=%d", result.Usage.Input, result.Usage.Output)
}

func TestAgentStoreRun_FileWrite_Anthropic(t *testing.T) {
	homeDir := setupTestConfigFromRepoConfig(t, "claude-sonnet-4-5")

	tmpDir := t.TempDir()
	stateDir := filepath.Join(homeDir, ".local", "state", "incrementum")

	store, err := agent.OpenWithOptions(agent.Options{
		StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("OpenWithOptions failed: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Ask the agent to create a file
	testFile := filepath.Join(tmpDir, "test.txt")
	opts := agent.RunOptions{
		RepoPath:  "/test/repo",
		WorkDir:   tmpDir,
		Prompt:    internalagent.PromptContent{UserContent: "Please create a file at " + testFile + " with the content 'Hello from the agent!'"},
		Model:     "claude-sonnet-4-5",
		StartedAt: time.Now(),
	}

	handle, err := store.Run(ctx, opts)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	result, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Verify file was created
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("File was not created: %v", err)
	}

	if !strings.Contains(string(content), "Hello from the agent!") {
		t.Errorf("File content = %q, want to contain 'Hello from the agent!'", string(content))
	}

	t.Logf("File content: %s", string(content))
	t.Logf("Session ID: %s", result.SessionID)
	t.Logf("Total messages: %d", len(result.Messages))
	t.Logf("Usage: input=%d, output=%d, cost=$%.6f", result.Usage.Input, result.Usage.Output, result.Usage.Cost.Total)
}
