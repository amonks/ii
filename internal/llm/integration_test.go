package llm_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	publicllm "github.com/amonks/incrementum/llm"
	"github.com/amonks/incrementum/internal/llm"
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

func requireOpenAIReasoningModelFromRepoConfig(t *testing.T) llm.Model {
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

	// Copy the real global config into the test HOME.
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
	models, err := publicstore.ListModels()
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	for _, model := range models {
		if !model.UseMaxCompletionTokens {
			continue
		}
		if model.API != publicllm.APIOpenAICompletions && model.API != publicllm.APIOpenAIResponses {
			continue
		}
		return llm.Model(model)
	}

	t.Fatalf("test requires at least one OpenAI reasoning model (UseMaxCompletionTokens=true) to be configured")
	return llm.Model{}
}

func TestAnthropicStream_Simple(t *testing.T) {
	model := requireModelFromRepoConfig(t, "claude-sonnet-4-5")

	req := llm.Request{
		SystemPrompt: "You are a helpful assistant. Be concise.",
		Messages: []llm.Message{
			llm.UserMessage{
				Role:      "user",
				Content:   []llm.ContentBlock{llm.TextContent{Type: "text", Text: "What is 2+2? Just say the number."}},
				Timestamp: time.Now(),
			},
		},
	}

	maxTokens := 100
	opts := llm.StreamOptions{
		MaxTokens: &maxTokens,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	handle, err := llm.Stream(ctx, model, req, opts)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	// Collect events
	for range handle.Events {
	}

	// Wait for completion
	msg, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Verify response
	if msg.Role != "assistant" {
		t.Errorf("Role = %q, want %q", msg.Role, "assistant")
	}

	if msg.Model != model.ID {
		t.Errorf("Model = %q, want %q", msg.Model, model.ID)
	}

	if len(msg.Content) == 0 {
		t.Fatal("No content in response")
	}

	// Should have text content with "4"
	var text string
	for _, block := range msg.Content {
		if tc, ok := block.(llm.TextContent); ok {
			text += tc.Text
		}
	}

	if !strings.Contains(text, "4") {
		t.Errorf("Response does not contain '4': %q", text)
	}

	// Should have usage info
	if msg.Usage.Input == 0 {
		t.Error("Input tokens should be > 0")
	}
	if msg.Usage.Output == 0 {
		t.Error("Output tokens should be > 0")
	}

	t.Logf("Response: %q", text)
	t.Logf("Usage: input=%d, output=%d, cost=$%.6f", msg.Usage.Input, msg.Usage.Output, msg.Usage.Cost.Total)
}

func TestAnthropicStream_WithTool(t *testing.T) {
	model := requireModelFromRepoConfig(t, "claude-sonnet-4-5")

	type CalculatorParams struct {
		A         float64 `json:"a" jsonschema:"description=First number"`
		B         float64 `json:"b" jsonschema:"description=Second number"`
		Operation string  `json:"operation" jsonschema:"description=The operation to perform,enum=add,enum=subtract,enum=multiply,enum=divide"`
	}

	req := llm.Request{
		SystemPrompt: "You are a helpful assistant with access to a calculator. Use the calculator tool for math.",
		Messages: []llm.Message{
			llm.UserMessage{
				Role:      "user",
				Content:   []llm.ContentBlock{llm.TextContent{Type: "text", Text: "What is 15 * 7?"}},
				Timestamp: time.Now(),
			},
		},
		Tools: []llm.Tool{
			{
				Name:        "calculator",
				Description: "Perform basic arithmetic operations",
				Parameters:  CalculatorParams{},
			},
		},
	}

	maxTokens := 200
	opts := llm.StreamOptions{
		MaxTokens: &maxTokens,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	handle, err := llm.Stream(ctx, model, req, opts)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	// Drain events
	for range handle.Events {
	}

	msg, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Should have tool use stop reason
	if msg.StopReason != llm.StopReasonToolUse {
		t.Errorf("StopReason = %q, want %q", msg.StopReason, llm.StopReasonToolUse)
	}

	// Should have a tool call in content
	var toolCall llm.ToolCall
	for _, block := range msg.Content {
		if tc, ok := block.(llm.ToolCall); ok {
			toolCall = tc
			break
		}
	}

	if toolCall.Name != "calculator" {
		t.Errorf("Tool name = %q, want %q", toolCall.Name, "calculator")
	}

	t.Logf("Tool call: %s(%v)", toolCall.Name, toolCall.Arguments)
}

func TestOpenAIStream_Simple(t *testing.T) {
	model := requireModelFromRepoConfig(t, "gpt-5.2")

	req := llm.Request{
		SystemPrompt: "You are a helpful assistant. Be concise.",
		Messages: []llm.Message{
			llm.UserMessage{
				Role:      "user",
				Content:   []llm.ContentBlock{llm.TextContent{Type: "text", Text: "What is 2+2? Just say the number."}},
				Timestamp: time.Now(),
			},
		},
	}

	maxTokens := 100
	opts := llm.StreamOptions{
		MaxTokens: &maxTokens,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	handle, err := llm.Stream(ctx, model, req, opts)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	// Collect events
	for range handle.Events {
	}

	msg, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Verify response
	if msg.Role != "assistant" {
		t.Errorf("Role = %q, want %q", msg.Role, "assistant")
	}

	var text string
	for _, block := range msg.Content {
		if tc, ok := block.(llm.TextContent); ok {
			text += tc.Text
		}
	}

	if !strings.Contains(text, "4") {
		t.Errorf("Response does not contain '4': %q", text)
	}

	t.Logf("Response: %q", text)
	t.Logf("Usage: input=%d, output=%d", msg.Usage.Input, msg.Usage.Output)
}

func TestOpenAIStream_WithTool(t *testing.T) {
	model := requireModelFromRepoConfig(t, "gpt-5.2")

	type CalculatorParams struct {
		A         float64 `json:"a" jsonschema:"description=First number"`
		B         float64 `json:"b" jsonschema:"description=Second number"`
		Operation string  `json:"operation" jsonschema:"description=The operation to perform,enum=add,enum=subtract,enum=multiply,enum=divide"`
	}

	req := llm.Request{
		SystemPrompt: "You are a helpful assistant with access to a calculator. Use the calculator tool for math.",
		Messages: []llm.Message{
			llm.UserMessage{
				Role:      "user",
				Content:   []llm.ContentBlock{llm.TextContent{Type: "text", Text: "What is 15 * 7?"}},
				Timestamp: time.Now(),
			},
		},
		Tools: []llm.Tool{
			{
				Name:        "calculator",
				Description: "Perform basic arithmetic operations",
				Parameters:  CalculatorParams{},
			},
		},
	}

	maxTokens := 200
	opts := llm.StreamOptions{
		MaxTokens: &maxTokens,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	handle, err := llm.Stream(ctx, model, req, opts)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	for range handle.Events {
	}

	msg, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Should have tool use stop reason
	if msg.StopReason != llm.StopReasonToolUse {
		t.Errorf("StopReason = %q, want %q", msg.StopReason, llm.StopReasonToolUse)
	}

	// Should have a tool call in content
	var toolCall llm.ToolCall
	for _, block := range msg.Content {
		if tc, ok := block.(llm.ToolCall); ok {
			toolCall = tc
			break
		}
	}

	if toolCall.Name != "calculator" {
		t.Errorf("Tool name = %q, want %q", toolCall.Name, "calculator")
	}

	t.Logf("Tool call: %s(%v)", toolCall.Name, toolCall.Arguments)
}

// TestOpenAIStream_ReasoningModel tests that o1/o3/o4 reasoning models work correctly.
// These models require max_completion_tokens instead of max_tokens.
func TestOpenAIStream_ReasoningModel(t *testing.T) {
	model := requireOpenAIReasoningModelFromRepoConfig(t)

	req := llm.Request{
		// Note: o-series models don't support system prompts in the same way
		Messages: []llm.Message{
			llm.UserMessage{
				Role:      "user",
				Content:   []llm.ContentBlock{llm.TextContent{Type: "text", Text: "What is 2+2? Just say the number."}},
				Timestamp: time.Now(),
			},
		},
	}

	maxTokens := 100
	opts := llm.StreamOptions{
		MaxTokens: &maxTokens,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // Reasoning models can be slower
	defer cancel()

	handle, err := llm.Stream(ctx, model, req, opts)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	// Collect events
	for range handle.Events {
	}

	msg, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Verify response
	if msg.Role != "assistant" {
		t.Errorf("Role = %q, want %q", msg.Role, "assistant")
	}

	var text string
	for _, block := range msg.Content {
		if tc, ok := block.(llm.TextContent); ok {
			text += tc.Text
		}
	}

	if !strings.Contains(text, "4") {
		t.Errorf("Response does not contain '4': %q", text)
	}

	t.Logf("Response: %q", text)
	t.Logf("Usage: input=%d, output=%d", msg.Usage.Input, msg.Usage.Output)
}

// TestWellKnownModels_Integration tests that all well-known frontier models can successfully
// complete a simple request. This verifies that our model configuration (including
// UseMaxCompletionTokens) is correct for each model.
//
// Models tested:
// - Anthropic latest: claude-sonnet-4-5-20250929, claude-haiku-4-5-20251001, claude-opus-4-5-20251101
// - OpenAI frontier: gpt-5.2, gpt-5-mini, gpt-5-nano, gpt-5.2-pro, gpt-5, gpt-4.1
func TestWellKnownModels_Integration(t *testing.T) {
	// Define all frontier models to test; only configured models are exercised.
	frontierModels := map[string]struct{}{
		// Anthropic latest models
		"claude-sonnet-4-5-20250929": {},
		"claude-haiku-4-5-20251001":  {},
		"claude-opus-4-5-20251101":   {},
		// OpenAI frontier models
		"gpt-5.2":     {},
		"gpt-5-mini":  {},
		"gpt-5-nano":  {},
		"gpt-5.2-pro": {},
		"gpt-5":       {},
		"gpt-4.1":     {},
	}

	repoRoot := findRepoRoot(t)
	store, err := publicllm.OpenWithOptions(publicllm.Options{RepoPath: repoRoot})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	models, err := store.ListModels()
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	var configuredFrontier []string
	for _, model := range models {
		if _, ok := frontierModels[model.ID]; ok {
			configuredFrontier = append(configuredFrontier, model.ID)
		}
	}
	if len(configuredFrontier) == 0 {
		t.Fatalf("TestWellKnownModels_Integration requires at least one configured frontier model in %s/incrementum.toml", repoRoot)
	}

	for _, modelID := range configuredFrontier {
		t.Run(modelID, func(t *testing.T) {
			m, err := store.GetModel(modelID)
			if err != nil {
				t.Fatalf("model %q is not configured in %s/incrementum.toml: %v", modelID, repoRoot, err)
			}
			model := llm.Model(m)

			req := llm.Request{
				SystemPrompt: "You are a helpful assistant. Be extremely concise.",
				Messages: []llm.Message{
					llm.UserMessage{
						Role:      "user",
						Content:   []llm.ContentBlock{llm.TextContent{Type: "text", Text: "What is 2+2? Reply with just the number."}},
						Timestamp: time.Now(),
					},
				},
			}

			// Reasoning models need more tokens because they use many tokens for internal thinking
			// before producing visible output. 50 tokens is not enough for reasoning models.
			maxTokens := 50
			if model.Reasoning {
				maxTokens = 500
			}
			opts := llm.StreamOptions{
				MaxTokens: &maxTokens,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			handle, err := llm.Stream(ctx, model, req, opts)
			if err != nil {
				t.Fatalf("Stream failed for %s: %v", modelID, err)
			}

			// Drain events
			for range handle.Events {
			}

			msg, err := handle.Wait()
			if err != nil {
				t.Fatalf("Wait failed for %s: %v", modelID, err)
			}

			// Verify we got a response
			if msg.Role != "assistant" {
				t.Errorf("Role = %q, want %q", msg.Role, "assistant")
			}

			var responseText string
			for _, block := range msg.Content {
				if textContent, ok := block.(llm.TextContent); ok {
					responseText += textContent.Text
				}
			}

			if responseText == "" {
				t.Error("Expected non-empty response")
			}

			// The response should contain "4"
			if !strings.Contains(responseText, "4") {
				t.Errorf("Response does not contain '4': %q", responseText)
			}

			t.Logf("%s: response=%q, input=%d, output=%d", modelID, responseText, msg.Usage.Input, msg.Usage.Output)
		})
	}
}
