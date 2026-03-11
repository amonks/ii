package agents

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"monks.co/incrementum/internal/config"
	"monks.co/pkg/llm"
)

// ModelRegistry holds configured models and resolves them by ID.
// It replaces the model-resolution portion of the former llm.Store.
type ModelRegistry struct {
	models       []llm.Model
	modelIndex   map[string]int // model ID → index in models slice
	defaultModel string         // from config llm.model

	providerConfigs map[string]config.LLMProvider
	keyCache        map[string]string
	keyCacheMu      sync.RWMutex
}

// NewModelRegistry builds a registry from config.
func NewModelRegistry(cfg *config.Config) (*ModelRegistry, error) {
	models, modelIndex, providerConfigs, err := buildModelsFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &ModelRegistry{
		models:          models,
		modelIndex:      modelIndex,
		defaultModel:    cfg.LLM.Model,
		providerConfigs: providerConfigs,
		keyCache:        make(map[string]string),
	}, nil
}

// buildModelsFromConfig builds the model list from configuration.
func buildModelsFromConfig(cfg *config.Config) ([]llm.Model, map[string]int, map[string]config.LLMProvider, error) {
	var models []llm.Model
	modelIndex := make(map[string]int)
	providerConfigs := make(map[string]config.LLMProvider)

	for _, provider := range cfg.LLM.Providers {
		api, ok := parseAPI(provider.API)
		if !ok {
			continue
		}

		for _, modelID := range provider.Models {
			if _, exists := modelIndex[modelID]; exists {
				continue
			}

			model := llm.Model{
				ID:       modelID,
				API:      api,
				Provider: provider.Name,
				BaseURL:  provider.BaseURL,
			}

			if err := applyWellKnownInfo(&model); err != nil {
				return nil, nil, nil, err
			}

			modelIndex[modelID] = len(models)
			models = append(models, model)
			providerConfigs[modelID] = provider
		}
	}

	return models, modelIndex, providerConfigs, nil
}

// parseAPI parses an API string into the API type.
func parseAPI(s string) (llm.API, bool) {
	switch s {
	case "anthropic-messages":
		return llm.APIAnthropicMessages, true
	case "openai-completions":
		return llm.APIOpenAICompletions, true
	case "openai-responses":
		return llm.APIOpenAIResponses, true
	default:
		return "", false
	}
}

// ListModels returns all configured models.
func (r *ModelRegistry) ListModels() ([]llm.Model, error) {
	result := make([]llm.Model, len(r.models))
	copy(result, r.models)
	return result, nil
}

// DefaultModel returns the default model ID from config (llm.model).
func (r *ModelRegistry) DefaultModel() string {
	return r.defaultModel
}

// GetModel returns the model with the given ID.
// The ID can be a prefix if it uniquely identifies a model.
func (r *ModelRegistry) GetModel(id string) (llm.Model, error) {
	// Try exact match first
	if idx, ok := r.modelIndex[id]; ok {
		model := r.models[idx]
		return r.resolveModelAPIKey(model)
	}

	// Try prefix match
	var matches []int
	idLower := strings.ToLower(id)
	for modelID, idx := range r.modelIndex {
		if strings.HasPrefix(strings.ToLower(modelID), idLower) {
			matches = append(matches, idx)
		}
	}

	if len(matches) == 0 {
		return llm.Model{}, ErrModelNotFound
	}
	if len(matches) > 1 {
		return llm.Model{}, ErrAmbiguousModel
	}

	model := r.models[matches[0]]
	return r.resolveModelAPIKey(model)
}

// GetDefaultModel returns the default model from config.
func (r *ModelRegistry) GetDefaultModel() (llm.Model, error) {
	if r.defaultModel == "" {
		return llm.Model{}, fmt.Errorf("no default model configured: set llm.model in config")
	}
	return r.GetModel(r.defaultModel)
}

// resolveModelAPIKey resolves the API key for a model.
func (r *ModelRegistry) resolveModelAPIKey(model llm.Model) (llm.Model, error) {
	cfg, ok := r.providerConfigs[model.ID]
	if !ok {
		return model, nil
	}

	if cfg.APIKeyCommand == "" {
		return model, nil
	}

	apiKey, err := r.resolveAPIKey(cfg.APIKeyCommand)
	if err != nil {
		return llm.Model{}, fmt.Errorf("resolve API key for %s: %w", model.Provider, err)
	}

	model.APIKey = apiKey
	return model, nil
}

// resolveAPIKey executes the api-key-command and caches the result.
func (r *ModelRegistry) resolveAPIKey(command string) (string, error) {
	r.keyCacheMu.RLock()
	if key, ok := r.keyCache[command]; ok {
		r.keyCacheMu.RUnlock()
		return key, nil
	}
	r.keyCacheMu.RUnlock()

	r.keyCacheMu.Lock()
	defer r.keyCacheMu.Unlock()

	if key, ok := r.keyCache[command]; ok {
		return key, nil
	}

	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("execute api-key-command: %w", err)
	}

	key := strings.TrimSpace(string(output))
	r.keyCache[command] = key
	return key, nil
}

// Error sentinels
var (
	ErrModelNotFound = fmt.Errorf("model not found")
	ErrAmbiguousModel = fmt.Errorf("ambiguous model ID")
	ErrUnknownModel  = fmt.Errorf("unknown model: not in well-known models list (add it to internal/agents/wellknown.go)")
)
