package llm

import (
	"fmt"

	internalllm "monks.co/incrementum/internal/llm"
)

// Model represents an LLM model with its configuration and capabilities.
// This type is identical to internal/llm.Model but is exported from the public package.
type Model = internalllm.Model

// Cost represents pricing per million tokens.
type Cost = internalllm.Cost

// API represents the API style used by a provider.
type API = internalllm.API

const (
	// APIAnthropicMessages is the Anthropic Messages API.
	APIAnthropicMessages = internalllm.APIAnthropicMessages
	// APIOpenAICompletions is the OpenAI Chat Completions API.
	APIOpenAICompletions = internalllm.APIOpenAICompletions
	// APIOpenAIResponses is the OpenAI Responses API.
	APIOpenAIResponses = internalllm.APIOpenAIResponses
)

// wellKnownModels contains built-in knowledge of well-known models.
// This information is used to populate model capabilities and pricing
// when a model ID matches a known model.
var wellKnownModels = map[string]modelInfo{
	// Claude 4.5 models
	"claude-sonnet-4-5-20250929": {
		Name:          "Claude Sonnet 4.5",
		ContextWindow: 200000,
		MaxTokens:     64000,
		Reasoning:     true,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:      3.0,
			Output:     15.0,
			CacheRead:  0.30,
			CacheWrite: 3.75,
		},
	},
	"claude-haiku-4-5-20251001": {
		Name:          "Claude Haiku 4.5",
		ContextWindow: 200000,
		MaxTokens:     64000,
		Reasoning:     true,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:      1.0,
			Output:     5.0,
			CacheRead:  0.10,
			CacheWrite: 1.25,
		},
	},
	"claude-opus-4-5-20251101": {
		Name:          "Claude Opus 4.5",
		ContextWindow: 200000,
		MaxTokens:     64000,
		Reasoning:     true,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:      5.0,
			Output:     25.0,
			CacheRead:  0.50,
			CacheWrite: 6.25,
		},
	},
	// Claude 4.5 undated aliases (point to the same model as dated versions)
	"claude-sonnet-4-5": {
		Name:          "Claude Sonnet 4.5",
		ContextWindow: 200000,
		MaxTokens:     64000,
		Reasoning:     true,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:      3.0,
			Output:     15.0,
			CacheRead:  0.30,
			CacheWrite: 3.75,
		},
	},
	"claude-haiku-4-5": {
		Name:          "Claude Haiku 4.5",
		ContextWindow: 200000,
		MaxTokens:     64000,
		Reasoning:     true,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:      1.0,
			Output:     5.0,
			CacheRead:  0.10,
			CacheWrite: 1.25,
		},
	},
	"claude-opus-4-5": {
		Name:          "Claude Opus 4.5",
		ContextWindow: 200000,
		MaxTokens:     64000,
		Reasoning:     true,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:      5.0,
			Output:     25.0,
			CacheRead:  0.50,
			CacheWrite: 6.25,
		},
	},
	// Claude 4 models
	"claude-sonnet-4-20250514": {
		Name:          "Claude Sonnet 4",
		ContextWindow: 200000,
		MaxTokens:     64000,
		Reasoning:     true,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:      3.0,
			Output:     15.0,
			CacheRead:  0.30,
			CacheWrite: 3.75,
		},
	},
	"claude-haiku-4-20250514": {
		Name:          "Claude Haiku 4",
		ContextWindow: 200000,
		MaxTokens:     64000,
		Reasoning:     false,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:      0.80,
			Output:     4.0,
			CacheRead:  0.08,
			CacheWrite: 1.0,
		},
	},
	// Claude 3.5 models
	"claude-3-5-sonnet-20241022": {
		Name:          "Claude 3.5 Sonnet",
		ContextWindow: 200000,
		MaxTokens:     8192,
		Reasoning:     false,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:      3.0,
			Output:     15.0,
			CacheRead:  0.30,
			CacheWrite: 3.75,
		},
	},
	"claude-3-5-haiku-20241022": {
		Name:          "Claude 3.5 Haiku",
		ContextWindow: 200000,
		MaxTokens:     8192,
		Reasoning:     false,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:      0.80,
			Output:     4.0,
			CacheRead:  0.08,
			CacheWrite: 1.0,
		},
	},
	// Claude 3 Opus
	"claude-3-opus-20240229": {
		Name:          "Claude 3 Opus",
		ContextWindow: 200000,
		MaxTokens:     4096,
		Reasoning:     false,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:      15.0,
			Output:     75.0,
			CacheRead:  1.50,
			CacheWrite: 18.75,
		},
	},
	// OpenAI GPT-5 series (frontier models)
	// GPT-5 models use max_completion_tokens instead of max_tokens
	"gpt-5.2": {
		Name:                   "GPT-5.2",
		ContextWindow:          1047576,
		MaxTokens:              32768,
		Reasoning:              true,
		UseMaxCompletionTokens: true,
		InputTypes:             []string{"text", "image"},
		Cost: Cost{
			Input:      1.75,
			Output:     14.0,
			CacheRead:  0.175,
			CacheWrite: 1.75,
		},
	},
	"gpt-5.2-codex": {
		Name:                   "GPT-5.2 Codex",
		ContextWindow:          400000,
		MaxTokens:              128000,
		Reasoning:              true,
		UseMaxCompletionTokens: true,
		InputTypes:             []string{"text", "image"},
		Cost: Cost{
			Input:      1.75,
			Output:     14.0,
			CacheRead:  0.175,
			CacheWrite: 1.75,
		},
	},
	"gpt-5.2-pro": {
		Name:                   "GPT-5.2 Pro",
		ContextWindow:          1047576,
		MaxTokens:              32768,
		Reasoning:              true,
		UseMaxCompletionTokens: true,
		RequiresResponsesAPI:   true, // gpt-5.2-pro requires Responses API, not Chat Completions
		InputTypes:             []string{"text", "image"},
		Cost: Cost{
			Input:  21.0,
			Output: 168.0,
		},
	},
	"gpt-5.1": {
		Name:                   "GPT-5.1",
		ContextWindow:          400000,
		MaxTokens:              128000,
		Reasoning:              true,
		UseMaxCompletionTokens: true,
		InputTypes:             []string{"text", "image"},
		Cost: Cost{
			Input:      1.25,
			Output:     10.0,
			CacheRead:  0.125,
			CacheWrite: 1.25,
		},
	},
	"gpt-5": {
		Name:                   "GPT-5",
		ContextWindow:          1047576,
		MaxTokens:              32768,
		Reasoning:              true,
		UseMaxCompletionTokens: true,
		InputTypes:             []string{"text", "image"},
		Cost: Cost{
			Input:      1.25,
			Output:     10.0,
			CacheRead:  0.125,
			CacheWrite: 1.25,
		},
	},
	"gpt-5-mini": {
		Name:                   "GPT-5 Mini",
		ContextWindow:          1047576,
		MaxTokens:              32768,
		Reasoning:              true,
		UseMaxCompletionTokens: true,
		InputTypes:             []string{"text", "image"},
		Cost: Cost{
			Input:      0.25,
			Output:     2.0,
			CacheRead:  0.025,
			CacheWrite: 0.25,
		},
	},
	"gpt-5-nano": {
		Name:                   "GPT-5 Nano",
		ContextWindow:          1047576,
		MaxTokens:              32768,
		Reasoning:              true,
		UseMaxCompletionTokens: true,
		InputTypes:             []string{"text", "image"},
		Cost: Cost{
			Input:      0.05,
			Output:     0.40,
			CacheRead:  0.005,
			CacheWrite: 0.05,
		},
	},
	// OpenAI GPT-4.1 models
	"gpt-4.1": {
		Name:          "GPT-4.1",
		ContextWindow: 1047576,
		MaxTokens:     32768,
		Reasoning:     false,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:      2.0,
			Output:     8.0,
			CacheRead:  0.50,
			CacheWrite: 2.0,
		},
	},
	"gpt-4.1-2025-04-14": {
		Name:          "GPT-4.1 (2025-04-14)",
		ContextWindow: 1047576,
		MaxTokens:     32768,
		Reasoning:     false,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:      2.0,
			Output:     8.0,
			CacheRead:  0.50,
			CacheWrite: 2.0,
		},
	},
	"gpt-4.1-mini": {
		Name:          "GPT-4.1 Mini",
		ContextWindow: 1047576,
		MaxTokens:     32768,
		Reasoning:     false,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:      0.40,
			Output:     1.60,
			CacheRead:  0.10,
			CacheWrite: 0.40,
		},
	},
	"gpt-4.1-mini-2025-04-14": {
		Name:          "GPT-4.1 Mini (2025-04-14)",
		ContextWindow: 1047576,
		MaxTokens:     32768,
		Reasoning:     false,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:      0.40,
			Output:     1.60,
			CacheRead:  0.10,
			CacheWrite: 0.40,
		},
	},
	"gpt-4.1-nano": {
		Name:          "GPT-4.1 Nano",
		ContextWindow: 1047576,
		MaxTokens:     32768,
		Reasoning:     false,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:      0.10,
			Output:     0.40,
			CacheRead:  0.025,
			CacheWrite: 0.10,
		},
	},
	"gpt-4.1-nano-2025-04-14": {
		Name:          "GPT-4.1 Nano (2025-04-14)",
		ContextWindow: 1047576,
		MaxTokens:     32768,
		Reasoning:     false,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:      0.10,
			Output:     0.40,
			CacheRead:  0.025,
			CacheWrite: 0.10,
		},
	},
	// OpenAI GPT-4o models
	"gpt-4o": {
		Name:          "GPT-4o",
		ContextWindow: 128000,
		MaxTokens:     16384,
		Reasoning:     false,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:  2.50,
			Output: 10.0,
		},
	},
	"gpt-4o-2024-11-20": {
		Name:          "GPT-4o (2024-11-20)",
		ContextWindow: 128000,
		MaxTokens:     16384,
		Reasoning:     false,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:  2.50,
			Output: 10.0,
		},
	},
	"gpt-4o-mini": {
		Name:          "GPT-4o Mini",
		ContextWindow: 128000,
		MaxTokens:     16384,
		Reasoning:     false,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:  0.15,
			Output: 0.60,
		},
	},
	"gpt-4o-mini-2024-07-18": {
		Name:          "GPT-4o Mini (2024-07-18)",
		ContextWindow: 128000,
		MaxTokens:     16384,
		Reasoning:     false,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:  0.15,
			Output: 0.60,
		},
	},
	// OpenAI o1 reasoning models
	"o1": {
		Name:                   "o1",
		ContextWindow:          200000,
		MaxTokens:              100000,
		Reasoning:              true,
		UseMaxCompletionTokens: true,
		InputTypes:             []string{"text", "image"},
		Cost: Cost{
			Input:  15.0,
			Output: 60.0,
		},
	},
	"o1-2024-12-17": {
		Name:                   "o1 (2024-12-17)",
		ContextWindow:          200000,
		MaxTokens:              100000,
		Reasoning:              true,
		UseMaxCompletionTokens: true,
		InputTypes:             []string{"text", "image"},
		Cost: Cost{
			Input:  15.0,
			Output: 60.0,
		},
	},
	"o1-mini": {
		Name:                   "o1 Mini",
		ContextWindow:          128000,
		MaxTokens:              65536,
		Reasoning:              true,
		UseMaxCompletionTokens: true,
		InputTypes:             []string{"text"},
		Cost: Cost{
			Input:  3.0,
			Output: 12.0,
		},
	},
	"o1-mini-2024-09-12": {
		Name:                   "o1 Mini (2024-09-12)",
		ContextWindow:          128000,
		MaxTokens:              65536,
		Reasoning:              true,
		UseMaxCompletionTokens: true,
		InputTypes:             []string{"text"},
		Cost: Cost{
			Input:  3.0,
			Output: 12.0,
		},
	},
	"o3": {
		Name:                   "o3",
		ContextWindow:          200000,
		MaxTokens:              100000,
		Reasoning:              true,
		UseMaxCompletionTokens: true,
		InputTypes:             []string{"text", "image"},
		Cost: Cost{
			Input:      2.0,
			Output:     8.0,
			CacheRead:  0.50,
			CacheWrite: 2.0,
		},
	},
	"o3-2025-04-16": {
		Name:                   "o3 (2025-04-16)",
		ContextWindow:          200000,
		MaxTokens:              100000,
		Reasoning:              true,
		UseMaxCompletionTokens: true,
		InputTypes:             []string{"text", "image"},
		Cost: Cost{
			Input:      2.0,
			Output:     8.0,
			CacheRead:  0.50,
			CacheWrite: 2.0,
		},
	},
	"o3-mini": {
		Name:                   "o3 Mini",
		ContextWindow:          200000,
		MaxTokens:              100000,
		Reasoning:              true,
		UseMaxCompletionTokens: true,
		InputTypes:             []string{"text"},
		Cost: Cost{
			Input:      1.10,
			Output:     4.40,
			CacheRead:  0.55,
			CacheWrite: 1.10,
		},
	},
	"o3-mini-2025-01-31": {
		Name:                   "o3 Mini (2025-01-31)",
		ContextWindow:          200000,
		MaxTokens:              100000,
		Reasoning:              true,
		UseMaxCompletionTokens: true,
		InputTypes:             []string{"text"},
		Cost: Cost{
			Input:      1.10,
			Output:     4.40,
			CacheRead:  0.55,
			CacheWrite: 1.10,
		},
	},
	"o4-mini": {
		Name:                   "o4 Mini",
		ContextWindow:          200000,
		MaxTokens:              100000,
		Reasoning:              true,
		UseMaxCompletionTokens: true,
		InputTypes:             []string{"text", "image"},
		Cost: Cost{
			Input:      1.10,
			Output:     4.40,
			CacheRead:  0.275,
			CacheWrite: 1.10,
		},
	},
	"o4-mini-2025-04-16": {
		Name:                   "o4 Mini (2025-04-16)",
		ContextWindow:          200000,
		MaxTokens:              100000,
		Reasoning:              true,
		UseMaxCompletionTokens: true,
		InputTypes:             []string{"text", "image"},
		Cost: Cost{
			Input:      1.10,
			Output:     4.40,
			CacheRead:  0.275,
			CacheWrite: 1.10,
		},
	},
	// GPT-4 Turbo
	"gpt-4-turbo": {
		Name:          "GPT-4 Turbo",
		ContextWindow: 128000,
		MaxTokens:     4096,
		Reasoning:     false,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:  10.0,
			Output: 30.0,
		},
	},
	"gpt-4-turbo-2024-04-09": {
		Name:          "GPT-4 Turbo (2024-04-09)",
		ContextWindow: 128000,
		MaxTokens:     4096,
		Reasoning:     false,
		InputTypes:    []string{"text", "image"},
		Cost: Cost{
			Input:  10.0,
			Output: 30.0,
		},
	},
}

// modelInfo contains the built-in knowledge for a well-known model.
type modelInfo struct {
	Name                   string
	ContextWindow          int
	MaxTokens              int
	Reasoning              bool
	UseMaxCompletionTokens bool // Use max_completion_tokens instead of max_tokens (for o1/o3/o4 models)
	RequiresResponsesAPI   bool // Model requires OpenAI Responses API instead of Chat Completions
	InputTypes             []string
	Cost                   Cost
}

// ErrUnknownModel is returned when a model ID is not in the well-known models list.
var ErrUnknownModel = fmt.Errorf("unknown model: not in well-known models list (add it to llm/models.go)")

// applyWellKnownInfo applies well-known model information to a model.
// Returns an error if the model ID is not recognized, since unknown models
// receive conservative defaults that may cause subtle failures (e.g., a
// 4096 max_tokens default on a model that supports 128k).
func applyWellKnownInfo(model *Model) error {
	info, ok := wellKnownModels[model.ID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownModel, model.ID)
	}

	// Apply well-known information
	if model.Name == "" {
		model.Name = info.Name
	}
	if model.ContextWindow == 0 {
		model.ContextWindow = info.ContextWindow
	}
	if model.MaxTokens == 0 {
		model.MaxTokens = info.MaxTokens
	}
	if len(model.InputTypes) == 0 {
		model.InputTypes = info.InputTypes
	}
	model.Reasoning = info.Reasoning
	model.UseMaxCompletionTokens = info.UseMaxCompletionTokens
	model.Cost = info.Cost

	// Override API if model requires Responses API
	if info.RequiresResponsesAPI && model.API == APIOpenAICompletions {
		model.API = APIOpenAIResponses
	}
	return nil
}

// ErrModelNotFound is returned when a model ID cannot be resolved.
var ErrModelNotFound = fmt.Errorf("model not found")

// ErrAmbiguousModel is returned when a model ID prefix matches multiple models.
var ErrAmbiguousModel = fmt.Errorf("ambiguous model ID")
