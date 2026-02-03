package llm_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amonks/incrementum/internal/testsupport"
	"github.com/amonks/incrementum/llm"
)

func setupIntegrationConfigFromRepoConfig(t *testing.T, requiredModels ...string) string {
	t.Helper()

	homeDir := testsupport.SetupTestHome(t)

	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	data, err := os.ReadFile(filepath.Join("..", "incrementum.toml"))
	if err != nil {
		t.Fatalf("failed to read ./incrementum.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), data, 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store, err := llm.OpenWithOptions(llm.Options{StateDir: filepath.Join(homeDir, ".local", "state", "incrementum")})
	if err != nil {
		t.Fatalf("failed to open llm store: %v", err)
	}
	for _, modelID := range requiredModels {
		if _, err := store.GetModel(modelID); err != nil {
			t.Fatalf("test requires model %q to be configured in ./incrementum.toml: %v", modelID, err)
		}
	}

	return homeDir
}

func TestIntegration_StreamAnthropic(t *testing.T) {
	homeDir := setupIntegrationConfigFromRepoConfig(t, "claude-sonnet-4-5")
	historyDir := filepath.Join(homeDir, "history")

	store, err := llm.OpenWithOptions(llm.Options{
		HistoryDir: historyDir,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	model, err := store.GetModel("claude-sonnet-4-5")
	if err != nil {
		t.Fatalf("GetModel failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := llm.Request{
		SystemPrompt: "You are a helpful assistant. Keep responses very brief.",
		Messages: []llm.Message{
			llm.UserMessage{
				Role:    "user",
				Content: []llm.ContentBlock{llm.TextContent{Type: "text", Text: "What is 2+2? Answer with just the number."}},
			},
		},
	}

	handle, err := store.Stream(ctx, model, req, llm.StreamOptions{})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	var receivedText string
	var receivedEvents int
	for event := range handle.Events {
		receivedEvents++
		switch e := event.(type) {
		case llm.TextDeltaEvent:
			receivedText += e.Delta
		}
	}

	msg, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	if receivedEvents == 0 {
		t.Error("expected to receive events")
	}

	if msg.StopReason != llm.StopReasonEnd {
		t.Errorf("expected StopReasonEnd, got %v", msg.StopReason)
	}

	// Check that completion was recorded
	// Give it a moment to write
	time.Sleep(200 * time.Millisecond)

	completions, err := store.ListCompletions()
	if err != nil {
		t.Fatalf("ListCompletions failed: %v", err)
	}

	if len(completions) != 1 {
		t.Errorf("expected 1 completion, got %d", len(completions))
	}

	if len(completions) > 0 {
		c := completions[0]
		if c.Model != model.ID {
			t.Errorf("expected model %q, got %q", model.ID, c.Model)
		}
	}

	t.Logf("Received response: %s", receivedText)
	t.Logf("Received %d events", receivedEvents)
}

func TestIntegration_StreamOpenAI(t *testing.T) {
	homeDir := setupIntegrationConfigFromRepoConfig(t, "gpt-5.2")
	historyDir := filepath.Join(homeDir, "history")

	store, err := llm.OpenWithOptions(llm.Options{
		HistoryDir: historyDir,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	model, err := store.GetModel("gpt-5.2")
	if err != nil {
		t.Fatalf("GetModel failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := llm.Request{
		SystemPrompt: "You are a helpful assistant. Keep responses very brief.",
		Messages: []llm.Message{
			llm.UserMessage{
				Role:    "user",
				Content: []llm.ContentBlock{llm.TextContent{Type: "text", Text: "What is 3+3? Answer with just the number."}},
			},
		},
	}

	handle, err := store.Stream(ctx, model, req, llm.StreamOptions{})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	var receivedText string
	var receivedEvents int
	for event := range handle.Events {
		receivedEvents++
		switch e := event.(type) {
		case llm.TextDeltaEvent:
			receivedText += e.Delta
		}
	}

	msg, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	if receivedEvents == 0 {
		t.Error("expected to receive events")
	}

	if msg.StopReason != llm.StopReasonEnd {
		t.Errorf("expected StopReasonEnd, got %v", msg.StopReason)
	}

	t.Logf("Received response: %s", receivedText)
	t.Logf("Received %d events", receivedEvents)
}

func TestIntegration_CompletionHistory(t *testing.T) {
	homeDir := setupIntegrationConfigFromRepoConfig(t, "claude-sonnet-4-5")
	historyDir := filepath.Join(homeDir, "history")

	store, err := llm.OpenWithOptions(llm.Options{
		HistoryDir: historyDir,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	model, err := store.GetModel("claude-sonnet-4-5")
	if err != nil {
		t.Fatalf("GetModel failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := llm.Request{
		Messages: []llm.Message{
			llm.UserMessage{
				Role:    "user",
				Content: []llm.ContentBlock{llm.TextContent{Type: "text", Text: "Say hello"}},
			},
		},
	}

	handle, err := store.Stream(ctx, model, req, llm.StreamOptions{})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	// Drain events
	for range handle.Events {
	}

	_, err = handle.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Give time for completion to be written
	time.Sleep(100 * time.Millisecond)

	// List completions
	completions, err := store.ListCompletions()
	if err != nil {
		t.Fatalf("ListCompletions failed: %v", err)
	}

	if len(completions) != 1 {
		t.Fatalf("expected 1 completion, got %d", len(completions))
	}

	c := completions[0]

	// Get completion by ID
	retrieved, err := store.GetCompletion(c.ID)
	if err != nil {
		t.Fatalf("GetCompletion failed: %v", err)
	}

	if retrieved.ID != c.ID {
		t.Errorf("ID mismatch: %q vs %q", retrieved.ID, c.ID)
	}

	// Get completion by prefix
	prefix := c.ID[:4]
	retrieved, err = store.GetCompletion(prefix)
	if err != nil {
		t.Fatalf("GetCompletion by prefix failed: %v", err)
	}

	if retrieved.ID != c.ID {
		t.Errorf("ID mismatch after prefix lookup: %q vs %q", retrieved.ID, c.ID)
	}
}
