// Package llm provides a unified abstraction for interacting with LLM providers.
// It wraps internal/llm to add completion history storage and model configuration
// loading from incrementum config files.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"monks.co/incrementum/internal/config"
	internalids "monks.co/incrementum/internal/ids"
	internalllm "monks.co/incrementum/internal/llm"
	"monks.co/incrementum/internal/paths"
)

// Re-export types from internal/llm for convenience
type (
	// Message types
	Message           = internalllm.Message
	UserMessage       = internalllm.UserMessage
	AssistantMessage  = internalllm.AssistantMessage
	ToolResultMessage = internalllm.ToolResultMessage

	// Content block types
	ContentBlock    = internalllm.ContentBlock
	TextContent     = internalllm.TextContent
	ThinkingContent = internalllm.ThinkingContent
	ImageContent    = internalllm.ImageContent
	ToolCall        = internalllm.ToolCall

	// Usage types
	Usage     = internalllm.Usage
	UsageCost = internalllm.UsageCost

	// Request/Response types
	Request       = internalllm.Request
	SystemBlock   = internalllm.SystemBlock
	Tool          = internalllm.Tool
	StreamOptions = internalllm.StreamOptions
	StreamHandle  = internalllm.StreamHandle
	StreamEvent   = internalllm.StreamEvent

	// Stream event types
	StartEvent         = internalllm.StartEvent
	TextDeltaEvent     = internalllm.TextDeltaEvent
	ThinkingDeltaEvent = internalllm.ThinkingDeltaEvent
	ToolCallDeltaEvent = internalllm.ToolCallDeltaEvent
	ToolCallEndEvent   = internalllm.ToolCallEndEvent
	DoneEvent          = internalllm.DoneEvent
	ErrorEvent         = internalllm.ErrorEvent

	// Other types
	StopReason     = internalllm.StopReason
	CacheRetention = internalllm.CacheRetention
	ThinkingLevel  = internalllm.ThinkingLevel
)

// Re-export constants
const (
	StopReasonEnd       = internalllm.StopReasonEnd
	StopReasonToolUse   = internalllm.StopReasonToolUse
	StopReasonMaxTokens = internalllm.StopReasonMaxTokens
	StopReasonError     = internalllm.StopReasonError
	StopReasonAborted   = internalllm.StopReasonAborted

	CacheNone  = internalllm.CacheNone
	CacheShort = internalllm.CacheShort
	CacheLong  = internalllm.CacheLong

	ThinkingOff     = internalllm.ThinkingOff
	ThinkingMinimal = internalllm.ThinkingMinimal
	ThinkingLow     = internalllm.ThinkingLow
	ThinkingMedium  = internalllm.ThinkingMedium
	ThinkingHigh    = internalllm.ThinkingHigh
	ThinkingXHigh   = internalllm.ThinkingXHigh
)

// Store provides access to LLM functionality with model configuration
// loaded from incrementum config files and completion history storage.
type Store struct {
	stateDir   string
	historyDir string
	models     []Model
	modelIndex map[string]int // model ID -> index in models slice

	// Default model from config (llm.model)
	defaultModel string

	// Provider config cache (model ID -> provider config)
	providerConfigs map[string]config.LLMProvider

	// API key cache
	keyCache   map[string]string
	keyCacheMu sync.RWMutex
}

// Options configures how the store is opened.
type Options struct {
	// StateDir is the directory for state files.
	// Default: ~/.local/state/incrementum
	StateDir string

	// HistoryDir is the directory for completion history.
	// Default: ~/.local/share/incrementum/llm/history
	HistoryDir string

	// RepoPath is the repository path for loading project-specific config.
	// If empty, only global config is loaded.
	RepoPath string
}

// Open opens the LLM store with default options.
// Only loads global configuration (no project-specific config).
func Open() (*Store, error) {
	return OpenWithOptions(Options{})
}

// OpenWithOptions opens the LLM store with the given options.
func OpenWithOptions(opts Options) (*Store, error) {
	stateDir, err := paths.ResolveWithDefault(opts.StateDir, paths.DefaultStateDir)
	if err != nil {
		return nil, fmt.Errorf("resolve state dir: %w", err)
	}

	historyDir, err := paths.ResolveWithDefault(opts.HistoryDir, defaultHistoryDir)
	if err != nil {
		return nil, fmt.Errorf("resolve history dir: %w", err)
	}

	// Load config
	var cfg *config.Config
	if opts.RepoPath != "" {
		cfg, err = config.Load(opts.RepoPath)
	} else {
		cfg, err = config.LoadGlobal()
	}
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Build models from config
	models, modelIndex, providerConfigs, err := buildModelsFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &Store{
		stateDir:        stateDir,
		historyDir:      historyDir,
		models:          models,
		modelIndex:      modelIndex,
		defaultModel:    cfg.LLM.Model,
		providerConfigs: providerConfigs,
		keyCache:        make(map[string]string),
	}, nil
}

func defaultHistoryDir() (string, error) {
	home, err := paths.HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "incrementum", "llm", "history"), nil
}

// buildModelsFromConfig builds the model list from configuration.
func buildModelsFromConfig(cfg *config.Config) ([]Model, map[string]int, map[string]config.LLMProvider, error) {
	var models []Model
	modelIndex := make(map[string]int)
	providerConfigs := make(map[string]config.LLMProvider)

	for _, provider := range cfg.LLM.Providers {
		api, ok := parseAPI(provider.API)
		if !ok {
			// Skip invalid API types
			continue
		}

		for _, modelID := range provider.Models {
			// Skip if we already have this model ID (first definition wins)
			if _, exists := modelIndex[modelID]; exists {
				continue
			}

			model := Model{
				ID:       modelID,
				API:      api,
				Provider: provider.Name,
				BaseURL:  provider.BaseURL,
				// APIKey will be resolved lazily when needed
			}

			// Apply well-known model information
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
func parseAPI(s string) (API, bool) {
	switch s {
	case "anthropic-messages":
		return APIAnthropicMessages, true
	case "openai-completions":
		return APIOpenAICompletions, true
	case "openai-responses":
		return APIOpenAIResponses, true
	default:
		return "", false
	}
}

// ListModels returns all configured models.
func (s *Store) ListModels() ([]Model, error) {
	// Return a copy to prevent modification
	result := make([]Model, len(s.models))
	copy(result, s.models)
	return result, nil
}

// DefaultModel returns the default model ID from config (llm.model).
// Returns empty string if no default is configured.
func (s *Store) DefaultModel() string {
	return s.defaultModel
}

// GetDefaultModel returns the default model from config (llm.model).
// Returns an error if no default is configured or the model is not found.
func (s *Store) GetDefaultModel() (Model, error) {
	if s.defaultModel == "" {
		return Model{}, fmt.Errorf("no default model configured: set llm.model in config")
	}
	return s.GetModel(s.defaultModel)
}

// GetModel returns the model with the given ID.
// The ID can be a prefix if it uniquely identifies a model.
func (s *Store) GetModel(id string) (Model, error) {
	// Try exact match first
	if idx, ok := s.modelIndex[id]; ok {
		model := s.models[idx]
		return s.resolveModelAPIKey(model)
	}

	// Try prefix match
	var matches []int
	idLower := strings.ToLower(id)
	for modelID, idx := range s.modelIndex {
		if strings.HasPrefix(strings.ToLower(modelID), idLower) {
			matches = append(matches, idx)
		}
	}

	if len(matches) == 0 {
		return Model{}, ErrModelNotFound
	}
	if len(matches) > 1 {
		return Model{}, ErrAmbiguousModel
	}

	model := s.models[matches[0]]
	return s.resolveModelAPIKey(model)
}

// resolveModelAPIKey resolves the API key for a model.
func (s *Store) resolveModelAPIKey(model Model) (Model, error) {
	// Find the provider config to get the api-key-command
	cfg, ok := s.providerConfigs[model.ID]
	if !ok {
		return model, nil // No provider config means no API key needed
	}

	if cfg.APIKeyCommand == "" {
		return model, nil // No API key needed
	}

	apiKey, err := s.resolveAPIKey(cfg.APIKeyCommand)
	if err != nil {
		return Model{}, fmt.Errorf("resolve API key for %s: %w", model.Provider, err)
	}

	model.APIKey = apiKey
	return model, nil
}

// resolveAPIKey executes the api-key-command and caches the result.
func (s *Store) resolveAPIKey(command string) (string, error) {
	s.keyCacheMu.RLock()
	if key, ok := s.keyCache[command]; ok {
		s.keyCacheMu.RUnlock()
		return key, nil
	}
	s.keyCacheMu.RUnlock()

	s.keyCacheMu.Lock()
	defer s.keyCacheMu.Unlock()

	// Double-check after acquiring write lock
	if key, ok := s.keyCache[command]; ok {
		return key, nil
	}

	// Execute the command
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("execute api-key-command: %w", err)
	}

	key := strings.TrimSpace(string(output))
	s.keyCache[command] = key
	return key, nil
}

// Completion represents a recorded LLM completion.
type Completion struct {
	ID        string           `json:"id"`
	Model     string           `json:"model"`
	Request   Request          `json:"request"`
	Response  AssistantMessage `json:"response"`
	CreatedAt time.Time        `json:"created_at"`
}

// Stream starts a streaming completion and records it to history.
func (s *Store) Stream(ctx context.Context, model Model, req Request, opts StreamOptions) (*StreamHandle, error) {
	// Ensure model has API key resolved
	if model.APIKey == "" {
		resolvedModel, err := s.resolveModelAPIKey(model)
		if err != nil {
			return nil, err
		}
		model = resolvedModel
	}

	// Start the stream
	handle, err := internalllm.Stream(ctx, model, req, opts)
	if err != nil {
		return nil, err
	}

	// Wrap the handle to record completion when done
	return s.wrapStreamHandle(handle, model, req)
}

// wrapStreamHandle wraps a stream handle to record completions.
func (s *Store) wrapStreamHandle(handle *StreamHandle, model Model, req Request) (*StreamHandle, error) {
	// Create channels for the wrapped handle
	wrappedEvents := make(chan StreamEvent, 100)
	wrappedDone := make(chan AssistantMessage, 1)
	wrappedErr := make(chan error, 1)

	go func() {
		defer close(wrappedEvents)

		var finalMessage AssistantMessage
		var finalErr error

		// Forward all events
		for event := range handle.Events {
			wrappedEvents <- event

			// Capture final message from Done or Error events
			switch e := event.(type) {
			case DoneEvent:
				finalMessage = e.Message
			case ErrorEvent:
				finalMessage = e.Message
			}
		}

		// Wait for the original handle to complete
		msg, err := handle.Wait()
		if err != nil {
			finalErr = err
		} else {
			finalMessage = msg
		}

		// Record the completion
		if finalErr == nil {
			now := time.Now()
			completion := Completion{
				ID:        internalids.GenerateWithTimestamp(model.ID, now, internalids.DefaultLength),
				Model:     model.ID,
				Request:   req,
				Response:  finalMessage,
				CreatedAt: now,
			}
			if recordErr := s.recordCompletion(completion); recordErr != nil {
				// Log but don't fail the stream
				fmt.Fprintf(os.Stderr, "warning: failed to record completion: %v\n", recordErr)
			}
		}

		if finalErr != nil {
			wrappedErr <- finalErr
		} else {
			wrappedDone <- finalMessage
		}
	}()

	return internalllm.NewStreamHandle(wrappedEvents, wrappedDone, wrappedErr), nil
}

// recordCompletion saves a completion to the history directory.
func (s *Store) recordCompletion(completion Completion) error {
	if err := os.MkdirAll(s.historyDir, 0755); err != nil {
		return fmt.Errorf("create history dir: %w", err)
	}

	// Use timestamp-based filename for sorting
	filename := fmt.Sprintf("%s_%s.json", completion.CreatedAt.Format("20060102-150405"), completion.ID)
	path := filepath.Join(s.historyDir, filename)

	data, err := json.MarshalIndent(completion, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal completion: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write completion: %w", err)
	}

	return nil
}

// ListCompletions returns all completions from history.
func (s *Store) ListCompletions() ([]Completion, error) {
	entries, err := os.ReadDir(s.historyDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read history dir: %w", err)
	}

	var completions []Completion
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(s.historyDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip unreadable files
		}

		var completion Completion
		if err := json.Unmarshal(data, &completion); err != nil {
			continue // Skip invalid files
		}

		completions = append(completions, completion)
	}

	return completions, nil
}

// GetCompletion returns the completion with the given ID.
// The ID can be a prefix if it uniquely identifies a completion.
func (s *Store) GetCompletion(id string) (Completion, error) {
	completions, err := s.ListCompletions()
	if err != nil {
		return Completion{}, err
	}

	var matches []Completion
	idLower := strings.ToLower(id)

	for _, c := range completions {
		if c.ID == id {
			return c, nil // Exact match
		}
		if strings.HasPrefix(strings.ToLower(c.ID), idLower) {
			matches = append(matches, c)
		}
	}

	if len(matches) == 0 {
		return Completion{}, fmt.Errorf("completion not found: %s", id)
	}
	if len(matches) > 1 {
		return Completion{}, fmt.Errorf("ambiguous completion ID: %s", id)
	}

	return matches[0], nil
}
