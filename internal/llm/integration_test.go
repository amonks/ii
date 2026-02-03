package llm_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/internal/llm"
)

// These tests are integration-style (exercise real LLM calls) but they live under
// internal/* and are not guaranteed to run from the repo root. To keep them
// hermetic and aligned with ./incrementum.toml, we copy the repo config into a
// temp HOME and then resolve models from that config.

func requireModelFromRepoConfig(t *testing.T, modelID string) llm.Model {
	t.Helper()

	homeDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(homeDir, ".config", "incrementum"), 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	data, err := os.ReadFile(filepath.Join("..", "incrementum.toml"))
	if err != nil {
		t.Fatalf("read ../incrementum.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".config", "incrementum", "config.toml"), data, 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".local", "state", "incrementum"), 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".local", "share", "incrementum"), 0o755); err != nil {
		t.Fatalf("create share dir: %v", err)
	}

	t.Setenv("HOME", homeDir)

	store, err := llm.OpenWithOptions(llm.Options{})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	model, err := store.GetModel(modelID)
	if err != nil {
		t.Fatalf("test requires model %q to be configured in ../incrementum.toml: %v", modelID, err)
	}
	return model
}

func TestAnthropicStream_Simple(t *testing.T) {
	model := requireModelFromRepoConfig(t, "claude-sonnet-4-5")
	model.ID = "claude-sonnet-4-20250514"

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
	var events []llm.StreamEvent
	for event := range handle.Events {
		events = append(events, event)
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
	model.ID = "claude-sonnet-4-20250514"

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
	model := requireModelFromRepoConfig(t, "o4-mini")
	model.UseMaxCompletionTokens = true // reasoning models use max_completion_tokens

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
	// Define all frontier models to test
	testCases := []struct {
		id                     string
		api                    llm.API
		useMaxCompletionTokens bool
		reasoning              bool // Reasoning models need more tokens for internal thinking
	}{
		// Anthropic latest models
		{"claude-sonnet-4-5-20250929", llm.APIAnthropicMessages, false, true},
		{"claude-haiku-4-5-20251001", llm.APIAnthropicMessages, false, true},
		{"claude-opus-4-5-20251101", llm.APIAnthropicMessages, false, true},
		// OpenAI frontier models (GPT-5 series uses max_completion_tokens)
		{"gpt-5.2", llm.APIOpenAICompletions, true, true},
		{"gpt-5-mini", llm.APIOpenAICompletions, true, true},
		{"gpt-5-nano", llm.APIOpenAICompletions, true, true},
		{"gpt-5.2-pro", llm.APIOpenAIResponses, false, true}, // Uses Responses API
		{"gpt-5", llm.APIOpenAICompletions, true, true},
		{"gpt-4.1", llm.APIOpenAICompletions, false, false}, // non-reasoning, uses max_tokens
	}

	store, err := publicllm.OpenWithOptions(publicllm.Options{RepoPath: "."})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	anyConfigured := false
	for _, tc := range testCases {
		if _, err := store.GetModel(tc.id); err == nil {
			anyConfigured = true
			break
		}
	}
	if !anyConfigured {
		t.Fatalf("TestWellKnownModels_Integration requires at least one of the frontier models to be configured in ./incrementum.toml")
	}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			m, err := store.GetModel(tc.id)
			if err != nil {
				t.Fatalf("model %q is not configured in ./incrementum.toml: %v", tc.id, err)
			}
			model := llm.Model(m)
			model.API = tc.api
			model.UseMaxCompletionTokens = tc.useMaxCompletionTokens

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
			if tc.reasoning {
				maxTokens = 500
			}
			opts := llm.StreamOptions{
				MaxTokens: &maxTokens,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			handle, err := llm.Stream(ctx, model, req, opts)
			if err != nil {
				t.Fatalf("Stream failed for %s: %v", tc.id, err)
			}

			// Drain events
			for range handle.Events {
			}

			msg, err := handle.Wait()
			if err != nil {
				t.Fatalf("Wait failed for %s: %v", tc.id, err)
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

			t.Logf("%s: response=%q, input=%d, output=%d", tc.id, responseText, msg.Usage.Input, msg.Usage.Output)
		})
	}
}
