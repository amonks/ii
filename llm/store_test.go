package llm_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/amonks/incrementum/internal/testsupport"
	"github.com/amonks/incrementum/llm"
)

func TestOpen_NoConfig(t *testing.T) {
	testsupport.SetupTestHome(t)

	store, err := llm.Open()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	models, err := store.ListModels()
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 0 {
		t.Errorf("expected 0 models, got %d", len(models))
	}
}

func TestOpen_WithProviders(t *testing.T) {
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
models = ["claude-sonnet-4-20250514", "claude-haiku-4-20250514"]

[[llm.providers]]
name = "openai"
api = "openai-completions"
base-url = "https://api.openai.com/v1"
api-key-command = "echo openai-key"
models = ["gpt-4o", "gpt-4o-mini"]
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store, err := llm.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	models, err := store.ListModels()
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 4 {
		t.Fatalf("expected 4 models, got %d", len(models))
	}

	// Check that models have well-known info applied
	for _, m := range models {
		if m.Name == "" {
			t.Errorf("model %s has empty name", m.ID)
		}
		if m.ContextWindow == 0 {
			t.Errorf("model %s has zero context window", m.ID)
		}
		if m.MaxTokens == 0 {
			t.Errorf("model %s has zero max tokens", m.ID)
		}
	}
}

func TestGetModel_ExactMatch(t *testing.T) {
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
models = ["claude-sonnet-4-20250514"]
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store, err := llm.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	model, err := store.GetModel("claude-sonnet-4-20250514")
	if err != nil {
		t.Fatalf("GetModel failed: %v", err)
	}

	if model.ID != "claude-sonnet-4-20250514" {
		t.Errorf("expected ID 'claude-sonnet-4-20250514', got %q", model.ID)
	}
	if model.Name != "Claude Sonnet 4" {
		t.Errorf("expected Name 'Claude Sonnet 4', got %q", model.Name)
	}
	if model.Provider != "anthropic" {
		t.Errorf("expected Provider 'anthropic', got %q", model.Provider)
	}
	if model.API != llm.APIAnthropicMessages {
		t.Errorf("expected API 'anthropic-messages', got %q", model.API)
	}
	if model.APIKey != "test-key" {
		t.Errorf("expected APIKey 'test-key', got %q", model.APIKey)
	}
}

func TestGetModel_PrefixMatch(t *testing.T) {
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
models = ["claude-sonnet-4-20250514"]
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store, err := llm.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	model, err := store.GetModel("claude-sonnet")
	if err != nil {
		t.Fatalf("GetModel failed: %v", err)
	}

	if model.ID != "claude-sonnet-4-20250514" {
		t.Errorf("expected ID 'claude-sonnet-4-20250514', got %q", model.ID)
	}
}

func TestGetModel_NotFound(t *testing.T) {
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
models = ["claude-sonnet-4-20250514"]
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store, err := llm.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	_, err = store.GetModel("nonexistent")
	if err != llm.ErrModelNotFound {
		t.Errorf("expected ErrModelNotFound, got %v", err)
	}
}

func TestGetModel_Ambiguous(t *testing.T) {
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
models = ["claude-sonnet-4-20250514", "claude-haiku-4-20250514"]
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store, err := llm.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	_, err = store.GetModel("claude")
	if err != llm.ErrAmbiguousModel {
		t.Errorf("expected ErrAmbiguousModel, got %v", err)
	}
}

func TestWellKnownModels(t *testing.T) {
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
models = ["claude-sonnet-4-20250514"]

[[llm.providers]]
name = "openai"
api = "openai-completions"
base-url = "https://api.openai.com/v1"
models = ["gpt-4o"]
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store, err := llm.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Test Claude Sonnet 4
	sonnet, err := store.GetModel("claude-sonnet-4-20250514")
	if err != nil {
		t.Fatalf("GetModel(claude-sonnet-4) failed: %v", err)
	}
	if sonnet.Name != "Claude Sonnet 4" {
		t.Errorf("expected Name 'Claude Sonnet 4', got %q", sonnet.Name)
	}
	if sonnet.ContextWindow != 200000 {
		t.Errorf("expected ContextWindow 200000, got %d", sonnet.ContextWindow)
	}
	if sonnet.MaxTokens != 64000 {
		t.Errorf("expected MaxTokens 64000, got %d", sonnet.MaxTokens)
	}
	if !sonnet.Reasoning {
		t.Error("expected Reasoning to be true for Claude Sonnet 4")
	}
	if sonnet.Cost.Input != 3.0 {
		t.Errorf("expected Cost.Input 3.0, got %f", sonnet.Cost.Input)
	}

	// Test GPT-4o
	gpt4o, err := store.GetModel("gpt-4o")
	if err != nil {
		t.Fatalf("GetModel(gpt-4o) failed: %v", err)
	}
	if gpt4o.Name != "GPT-4o" {
		t.Errorf("expected Name 'GPT-4o', got %q", gpt4o.Name)
	}
	if gpt4o.ContextWindow != 128000 {
		t.Errorf("expected ContextWindow 128000, got %d", gpt4o.ContextWindow)
	}
	if gpt4o.Reasoning {
		t.Error("expected Reasoning to be false for GPT-4o")
	}
}

func TestUnknownModel(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configContent := `
[[llm.providers]]
name = "custom"
api = "openai-completions"
base-url = "https://custom.example.com"
models = ["custom-model-v1"]
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Unknown models should cause an error at store open time
	_, err := llm.Open()
	if err == nil {
		t.Fatal("expected error for unknown model, got nil")
	}
	if !errors.Is(err, llm.ErrUnknownModel) {
		t.Errorf("expected ErrUnknownModel, got: %v", err)
	}
}

func TestOpenWithOptions_CustomDirs(t *testing.T) {
	testsupport.SetupTestHome(t)
	tmpDir := t.TempDir()

	stateDir := filepath.Join(tmpDir, "state")
	historyDir := filepath.Join(tmpDir, "history")

	store, err := llm.OpenWithOptions(llm.Options{
		StateDir:   stateDir,
		HistoryDir: historyDir,
	})
	if err != nil {
		t.Fatalf("OpenWithOptions failed: %v", err)
	}

	// Verify we can list models (empty list is fine)
	models, err := store.ListModels()
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if models == nil {
		t.Error("expected non-nil models slice")
	}

	// Verify we can list completions (empty list is fine)
	completions, err := store.ListCompletions()
	if err != nil {
		t.Fatalf("ListCompletions failed: %v", err)
	}
	if len(completions) > 0 {
		t.Error("expected empty completions list")
	}
}

func TestListCompletions_Empty(t *testing.T) {
	testsupport.SetupTestHome(t)
	tmpDir := t.TempDir()

	store, err := llm.OpenWithOptions(llm.Options{
		HistoryDir: filepath.Join(tmpDir, "history"),
	})
	if err != nil {
		t.Fatalf("OpenWithOptions failed: %v", err)
	}

	completions, err := store.ListCompletions()
	if err != nil {
		t.Fatalf("ListCompletions failed: %v", err)
	}

	if len(completions) > 0 {
		t.Errorf("expected empty completions, got %d", len(completions))
	}
}

func TestAPIKeyResolution(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configContent := `
[[llm.providers]]
name = "test-provider"
api = "anthropic-messages"
base-url = "https://test.example.com"
api-key-command = "echo secret-api-key"
models = ["claude-sonnet-4-5-20250929"]
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store, err := llm.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	model, err := store.GetModel("claude-sonnet-4-5-20250929")
	if err != nil {
		t.Fatalf("GetModel failed: %v", err)
	}

	if model.APIKey != "secret-api-key" {
		t.Errorf("expected APIKey 'secret-api-key', got %q", model.APIKey)
	}
}

func TestClaude45Models(t *testing.T) {
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
models = ["claude-sonnet-4-5-20250929", "claude-haiku-4-5-20251001", "claude-opus-4-5-20251101"]
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store, err := llm.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	tests := []struct {
		id            string
		name          string
		inputCost     float64
		outputCost    float64
		cacheRead     float64
		contextWindow int
	}{
		{"claude-sonnet-4-5-20250929", "Claude Sonnet 4.5", 3.0, 15.0, 0.30, 200000},
		{"claude-haiku-4-5-20251001", "Claude Haiku 4.5", 1.0, 5.0, 0.10, 200000},
		{"claude-opus-4-5-20251101", "Claude Opus 4.5", 5.0, 25.0, 0.50, 200000},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			model, err := store.GetModel(tt.id)
			if err != nil {
				t.Fatalf("GetModel(%s) failed: %v", tt.id, err)
			}
			if model.Name != tt.name {
				t.Errorf("expected Name %q, got %q", tt.name, model.Name)
			}
			if model.Cost.Input != tt.inputCost {
				t.Errorf("expected Input cost %f, got %f", tt.inputCost, model.Cost.Input)
			}
			if model.Cost.Output != tt.outputCost {
				t.Errorf("expected Output cost %f, got %f", tt.outputCost, model.Cost.Output)
			}
			if model.Cost.CacheRead != tt.cacheRead {
				t.Errorf("expected CacheRead cost %f, got %f", tt.cacheRead, model.Cost.CacheRead)
			}
			if model.ContextWindow != tt.contextWindow {
				t.Errorf("expected ContextWindow %d, got %d", tt.contextWindow, model.ContextWindow)
			}
			if !model.Reasoning {
				t.Error("expected Reasoning to be true")
			}
		})
	}
}

func TestGPT5Models(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configContent := `
[[llm.providers]]
name = "openai"
api = "openai-completions"
base-url = "https://api.openai.com/v1"
models = ["gpt-5.2", "gpt-5.2-codex", "gpt-5.2-pro", "gpt-5.1", "gpt-5", "gpt-5-mini", "gpt-5-nano"]
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store, err := llm.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	tests := []struct {
		id            string
		name          string
		inputCost     float64
		outputCost    float64
		cacheRead     float64
		contextWindow int
	}{
		{"gpt-5.2", "GPT-5.2", 1.75, 14.0, 0.175, 1047576},
		{"gpt-5.2-codex", "GPT-5.2 Codex", 1.75, 14.0, 0.175, 400000},
		{"gpt-5.2-pro", "GPT-5.2 Pro", 21.0, 168.0, 0.0, 1047576},
		{"gpt-5.1", "GPT-5.1", 1.25, 10.0, 0.125, 400000},
		{"gpt-5", "GPT-5", 1.25, 10.0, 0.125, 1047576},
		{"gpt-5-mini", "GPT-5 Mini", 0.25, 2.0, 0.025, 1047576},
		{"gpt-5-nano", "GPT-5 Nano", 0.05, 0.40, 0.005, 1047576},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			model, err := store.GetModel(tt.id)
			if err != nil {
				t.Fatalf("GetModel(%s) failed: %v", tt.id, err)
			}
			if model.Name != tt.name {
				t.Errorf("expected Name %q, got %q", tt.name, model.Name)
			}
			if model.Cost.Input != tt.inputCost {
				t.Errorf("expected Input cost %f, got %f", tt.inputCost, model.Cost.Input)
			}
			if model.Cost.Output != tt.outputCost {
				t.Errorf("expected Output cost %f, got %f", tt.outputCost, model.Cost.Output)
			}
			if model.Cost.CacheRead != tt.cacheRead {
				t.Errorf("expected CacheRead cost %f, got %f", tt.cacheRead, model.Cost.CacheRead)
			}
			if model.ContextWindow != tt.contextWindow {
				t.Errorf("expected ContextWindow %d, got %d", tt.contextWindow, model.ContextWindow)
			}
			if !model.Reasoning {
				t.Error("expected Reasoning to be true for GPT-5 series")
			}
		})
	}
}

func TestAPIKeyResolution_NoCommand(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configContent := `
[[llm.providers]]
name = "no-auth-provider"
api = "openai-completions"
base-url = "https://no-auth.example.com"
models = ["gpt-4o"]
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store, err := llm.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	model, err := store.GetModel("gpt-4o")
	if err != nil {
		t.Fatalf("GetModel failed: %v", err)
	}

	if model.APIKey != "" {
		t.Errorf("expected empty APIKey, got %q", model.APIKey)
	}
}

func TestOpenAIReasoningModels_UseMaxCompletionTokens(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Configure all OpenAI reasoning models that should use max_completion_tokens
	configContent := `
[[llm.providers]]
name = "openai"
api = "openai-completions"
base-url = "https://api.openai.com/v1"
models = ["o1", "o1-2024-12-17", "o1-mini", "o1-mini-2024-09-12", "o3", "o3-2025-04-16", "o3-mini", "o3-mini-2025-01-31", "o4-mini", "o4-mini-2025-04-16", "gpt-4o", "gpt-4o-mini", "gpt-5.2", "gpt-5.2-codex", "gpt-5.2-pro", "gpt-5.1", "gpt-5", "gpt-5-mini", "gpt-5-nano", "gpt-4.1"]
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store, err := llm.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// These models SHOULD use max_completion_tokens
	reasoningModels := []string{
		// o-series reasoning models
		"o1", "o1-2024-12-17",
		"o1-mini", "o1-mini-2024-09-12",
		"o3", "o3-2025-04-16",
		"o3-mini", "o3-mini-2025-01-31",
		"o4-mini", "o4-mini-2025-04-16",
		// GPT-5 series (frontier models that use max_completion_tokens)
		"gpt-5.2", "gpt-5.2-codex", "gpt-5.2-pro", "gpt-5.1", "gpt-5", "gpt-5-mini", "gpt-5-nano",
	}

	for _, modelID := range reasoningModels {
		t.Run(modelID, func(t *testing.T) {
			model, err := store.GetModel(modelID)
			if err != nil {
				t.Fatalf("GetModel(%s) failed: %v", modelID, err)
			}
			if !model.UseMaxCompletionTokens {
				t.Errorf("expected UseMaxCompletionTokens=true for %s", modelID)
			}
			if !model.Reasoning {
				t.Errorf("expected Reasoning=true for %s", modelID)
			}
		})
	}

	// These models should NOT use max_completion_tokens (they use max_tokens)
	standardModels := []string{
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-4.1", // GPT-4.1 is non-reasoning, uses max_tokens
	}

	for _, modelID := range standardModels {
		t.Run(modelID, func(t *testing.T) {
			model, err := store.GetModel(modelID)
			if err != nil {
				t.Fatalf("GetModel(%s) failed: %v", modelID, err)
			}
			if model.UseMaxCompletionTokens {
				t.Errorf("expected UseMaxCompletionTokens=false for %s", modelID)
			}
		})
	}
}
