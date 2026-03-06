package agent_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"monks.co/incrementum/internal/agent"
	"monks.co/incrementum/internal/llm"
	publicllm "monks.co/incrementum/llm"
)

// These tests are integration-style (exercise real LLM calls) but they live under
// internal/* and are not guaranteed to run from the repo root. To keep them
// hermetic and aligned with the repo's incrementum.toml, we set a temp HOME
// (so global config is empty) and load providers from the repo config.

func requireModelFromRepoConfig(t *testing.T, modelID string) llm.Model {
	t.Helper()

	// Save the real home before overriding HOME.
	realHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("get home dir: %v", err)
	}

	homeDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(homeDir, ".local", "state", "incrementum"), 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".local", "share", "incrementum"), 0o755); err != nil {
		t.Fatalf("create share dir: %v", err)
	}

	// Copy the real global config (which has LLM provider definitions) into
	// the test HOME so the store can resolve models.
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(realHome, ".config", "incrementum", "config.toml"))
	if err != nil {
		t.Fatalf("read global config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("HOME", homeDir)

	repoRoot := findRepoRoot(t)
	publicstore, err := publicllm.OpenWithOptions(publicllm.Options{RepoPath: repoRoot})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	model, err := publicstore.GetModel(modelID)
	if err != nil {
		t.Fatalf("test requires model %q to be configured: %v", modelID, err)
	}
	return llm.Model(model)
}

func findRepoRoot(t *testing.T) string {
	t.Helper()

	_, filePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to determine caller path")
	}
	dir := filepath.Dir(filePath)
	for {
		if _, err := os.Stat(filepath.Join(dir, "incrementum.toml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find incrementum.toml in any parent directory")
		}
		dir = parent
	}
}

func TestAgentRun_SimpleCompletion_Anthropic(t *testing.T) {
	model := requireModelFromRepoConfig(t, "claude-sonnet-4-5-20250929")

	config := agent.AgentConfig{
		Model: model,
		Permissions: agent.BashPermissions{
			Rules: []agent.BashRule{
				{Pattern: "*", Allow: true},
			},
		},
		WorkDir: t.TempDir(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	handle, err := agent.Run(ctx, agent.PromptContent{UserContent: "What is 2+2? Just say the number, nothing else."}, config)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Collect events
	var events []agent.Event
	for event := range handle.Events {
		events = append(events, event)
	}

	result, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Verify we got events
	if len(events) == 0 {
		t.Error("Expected events, got none")
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

	t.Logf("Response: %s", responseText.String())
	t.Logf("Usage: input=%d, output=%d, cost=$%.6f", result.Usage.Input, result.Usage.Output, result.Usage.Cost.Total)
}

func TestAgentRun_SimpleCompletion_OpenAI(t *testing.T) {
	model := requireModel(t, "gpt-5.2")

	config := agent.AgentConfig{
		Model: model,
		Permissions: agent.BashPermissions{
			Rules: []agent.BashRule{
				{Pattern: "*", Allow: true},
			},
		},
		WorkDir: t.TempDir(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	handle, err := agent.Run(ctx, agent.PromptContent{UserContent: "What is 2+2? Just say the number, nothing else."}, config)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Drain events
	for range handle.Events {
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
	t.Logf("Usage: input=%d, output=%d", result.Usage.Input, result.Usage.Output)
}

func TestAgentRun_ToolCall_Anthropic(t *testing.T) {
	model := requireModelFromRepoConfig(t, "claude-sonnet-4-5-20250929")
	tmpDir := t.TempDir()

	config := agent.AgentConfig{
		Model: model,
		Permissions: agent.BashPermissions{
			Rules: []agent.BashRule{
				{Pattern: "echo *", Allow: true},
				{Pattern: "*", Allow: false},
			},
		},
		WorkDir: tmpDir,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Ask the agent to use the bash tool
	handle, err := agent.Run(ctx, agent.PromptContent{UserContent: "Please run 'echo hello world' using the bash tool and tell me the output."}, config)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Collect events
	var toolExecutions []agent.ToolExecutionStartEvent
	for event := range handle.Events {
		if te, ok := event.(agent.ToolExecutionStartEvent); ok {
			toolExecutions = append(toolExecutions, te)
		}
	}

	result, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Verify tool was called
	if len(toolExecutions) == 0 {
		t.Error("Expected at least one tool execution")
	} else {
		t.Logf("Tool executions: %d", len(toolExecutions))
		for _, te := range toolExecutions {
			t.Logf("  Tool: %s, Args: %v", te.ToolName, te.Arguments)
		}
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
	t.Logf("Usage: input=%d, output=%d", result.Usage.Input, result.Usage.Output)
}

func TestAgentRun_FileOperations_Anthropic(t *testing.T) {
	model := requireModelFromRepoConfig(t, "claude-sonnet-4-5-20250929")
	tmpDir := t.TempDir()

	config := agent.AgentConfig{
		Model: model,
		Permissions: agent.BashPermissions{
			Rules: []agent.BashRule{
				{Pattern: "*", Allow: true},
			},
		},
		WorkDir: tmpDir,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Ask the agent to create a file
	testFile := filepath.Join(tmpDir, "test.txt")
	handle, err := agent.Run(ctx, agent.PromptContent{UserContent: "Please create a file at " + testFile + " with the content 'Hello from the agent!'"}, config)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Drain events
	for range handle.Events {
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
	t.Logf("Total messages: %d", len(result.Messages))
	t.Logf("Usage: input=%d, output=%d, cost=$%.6f", result.Usage.Input, result.Usage.Output, result.Usage.Cost.Total)
}

func TestAgentRun_PermissionDenied_Anthropic(t *testing.T) {
	model := requireModelFromRepoConfig(t, "claude-sonnet-4-5-20250929")
	tmpDir := t.TempDir()

	config := agent.AgentConfig{
		Model: model,
		Permissions: agent.BashPermissions{
			Rules: []agent.BashRule{
				// Only allow echo commands
				{Pattern: "echo *", Allow: true},
				{Pattern: "*", Allow: false},
			},
		},
		WorkDir: tmpDir,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Ask the agent to run a command that's not allowed
	handle, err := agent.Run(ctx, agent.PromptContent{UserContent: "Please run 'ls -la' using the bash tool."}, config)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Collect tool results
	var toolResults []agent.ToolExecutionEndEvent
	for event := range handle.Events {
		if te, ok := event.(agent.ToolExecutionEndEvent); ok {
			toolResults = append(toolResults, te)
		}
	}

	result, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Check if there was a permission denied result
	permissionDenied := false
	for _, tr := range toolResults {
		if tr.Result.IsError {
			for _, block := range tr.Result.Content {
				if tc, ok := block.(llm.TextContent); ok {
					if strings.Contains(tc.Text, "Permission denied") {
						permissionDenied = true
						break
					}
				}
			}
		}
	}

	if !permissionDenied {
		t.Error("Expected permission denied error for ls command")
	}

	t.Logf("Total messages: %d", len(result.Messages))
	t.Logf("Usage: input=%d, output=%d", result.Usage.Input, result.Usage.Output)
}

func TestAgentRun_ContextCancellation(t *testing.T) {
	model := requireModelFromRepoConfig(t, "claude-sonnet-4-5-20250929")
	tmpDir := t.TempDir()

	config := agent.AgentConfig{
		Model: model,
		Permissions: agent.BashPermissions{
			Rules: []agent.BashRule{
				{Pattern: "*", Allow: true},
			},
		},
		WorkDir: tmpDir,
	}

	// Create a context that we'll cancel shortly
	ctx, cancel := context.WithCancel(context.Background())

	handle, err := agent.Run(ctx, agent.PromptContent{UserContent: "Tell me a very long story about a dragon."}, config)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Cancel after receiving some events
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	// Drain events
	for range handle.Events {
	}

	result, err := handle.Wait()
	if err == nil {
		t.Error("Expected error from context cancellation")
	}

	t.Logf("Error (expected): %v", err)
	t.Logf("Messages before cancellation: %d", len(result.Messages))
}

func TestAgentRun_Events_Anthropic(t *testing.T) {
	model := requireModelFromRepoConfig(t, "claude-sonnet-4-5-20250929")
	tmpDir := t.TempDir()

	config := agent.AgentConfig{
		Model: model,
		Permissions: agent.BashPermissions{
			Rules: []agent.BashRule{
				{Pattern: "*", Allow: true},
			},
		},
		WorkDir: tmpDir,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	handle, err := agent.Run(ctx, agent.PromptContent{UserContent: "What is 2+2?"}, config)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify event sequence
	var hasAgentStart, hasAgentEnd bool
	var hasTurnStart, hasTurnEnd bool
	var hasMessageStart, hasMessageEnd bool

	for event := range handle.Events {
		switch event.(type) {
		case agent.AgentStartEvent:
			hasAgentStart = true
		case agent.AgentEndEvent:
			hasAgentEnd = true
		case agent.TurnStartEvent:
			hasTurnStart = true
		case agent.TurnEndEvent:
			hasTurnEnd = true
		case agent.MessageStartEvent:
			hasMessageStart = true
		case agent.MessageEndEvent:
			hasMessageEnd = true
		}
	}

	result, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	if !hasAgentStart {
		t.Error("Missing AgentStartEvent")
	}
	if !hasAgentEnd {
		t.Error("Missing AgentEndEvent")
	}
	if !hasTurnStart {
		t.Error("Missing TurnStartEvent")
	}
	if !hasTurnEnd {
		t.Error("Missing TurnEndEvent")
	}
	if !hasMessageStart {
		t.Error("Missing MessageStartEvent")
	}
	if !hasMessageEnd {
		t.Error("Missing MessageEndEvent")
	}

	t.Logf("Event verification passed")
	t.Logf("Total messages: %d", len(result.Messages))
}
