// Package config handles loading project and global configuration files.
package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	internalstrings "github.com/amonks/incrementum/internal/strings"
)

// Config represents the configuration file schema.
type Config struct {
	Workspace Workspace `toml:"workspace"`
	Job       Job       `toml:"job"`
	LLM       LLM       `toml:"llm"`
	Agent     Agent     `toml:"agent"`
}

// LLM contains LLM-related configuration.
type LLM struct {
	// Providers defines the LLM providers available for use.
	Providers []LLMProvider `toml:"providers"`
	// Model is the default model ID for LLM completions when no other model is specified.
	Model string `toml:"model"`
}

// Agent contains agent-related configuration.
type Agent struct {
	// Model is the default model ID for agent runs when no task-specific model is set.
	Model string `toml:"model"`
}

// LLMProvider configures a single LLM provider.
type LLMProvider struct {
	// Name is a unique identifier for this provider configuration.
	Name string `toml:"name"`
	// API specifies which API style to use (anthropic-messages, openai-completions, openai-responses).
	API string `toml:"api"`
	// BaseURL is the API endpoint.
	BaseURL string `toml:"base-url"`
	// APIKeyCommand is a command to run to get the API key. If empty, no auth is used.
	APIKeyCommand string `toml:"api-key-command"`
	// Models lists the model IDs available through this provider.
	Models []string `toml:"models"`
}

// Workspace contains workspace-related configuration.
type Workspace struct {
	// OnCreate is a script to run when a workspace is first created.
	// Can include a shebang line; defaults to bash if not specified.
	OnCreate string `toml:"on-create"`

	// OnAcquire is a script to run every time a workspace is acquired.
	// Can include a shebang line; defaults to bash if not specified.
	OnAcquire string `toml:"on-acquire"`
}

// Job contains job-related configuration.
type Job struct {
	// TestCommands defines commands to run during job testing.
	TestCommands []string `toml:"test-commands"`
	// Model is the default model for job runs when no stage-specific model is set.
	Model string `toml:"model"`
	// ImplementationModel selects the model for implementing.
	ImplementationModel string `toml:"implementation-model"`
	// CodeReviewModel selects the model for step review.
	CodeReviewModel string `toml:"code-review-model"`
	// ProjectReviewModel selects the model for final project review.
	ProjectReviewModel string `toml:"project-review-model"`
}

// Load loads configuration from the repo root and the global config file.
// Returns an empty config if no config files exist.
func Load(repoPath string) (*Config, error) {
	globalPath, err := globalConfigPath()
	if err != nil {
		return nil, err
	}

	globalCfg, globalMeta, err := loadConfigFile(globalPath)
	if err != nil {
		return nil, err
	}

	projectCfg, projectMeta, err := loadProjectConfig(repoPath)
	if err != nil {
		return nil, err
	}

	merged := mergeConfigs(globalCfg, projectCfg, globalMeta, projectMeta)
	return merged, nil
}

// LoadGlobal loads only the global configuration file.
// Returns an empty config if the file doesn't exist.
func LoadGlobal() (*Config, error) {
	globalPath, err := globalConfigPath()
	if err != nil {
		return nil, err
	}

	cfg, _, err := loadConfigFile(globalPath)
	return cfg, err
}

func loadProjectConfig(repoPath string) (*Config, toml.MetaData, error) {
	rootPath := filepath.Join(repoPath, "incrementum.toml")
	altPath := filepath.Join(repoPath, ".incrementum", "config.toml")

	rootExists, err := fileExists(rootPath)
	if err != nil {
		return nil, toml.MetaData{}, fmt.Errorf("check project config %s: %w", rootPath, err)
	}

	altExists, err := fileExists(altPath)
	if err != nil {
		return nil, toml.MetaData{}, fmt.Errorf("check project config %s: %w", altPath, err)
	}

	if rootExists && altExists {
		return nil, toml.MetaData{}, fmt.Errorf("project config files both exist: %s and %s", rootPath, altPath)
	}

	if rootExists {
		return loadConfigFile(rootPath)
	}
	if altExists {
		return loadConfigFile(altPath)
	}

	return &Config{}, toml.MetaData{}, nil
}

func fileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return !info.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func globalConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "incrementum", "config.toml"), nil
}

func loadConfigFile(path string) (*Config, toml.MetaData, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, toml.MetaData{}, nil
	}
	if err != nil {
		return nil, toml.MetaData{}, fmt.Errorf("read config file %s: %w", path, err)
	}

	var cfg Config
	meta, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return nil, toml.MetaData{}, fmt.Errorf("parse config file %s: %w", path, err)
	}

	return &cfg, meta, nil
}

func mergeConfigs(globalCfg, projectCfg *Config, globalMeta, projectMeta toml.MetaData) *Config {
	if globalCfg == nil {
		globalCfg = &Config{}
	}
	if projectCfg == nil {
		projectCfg = &Config{}
	}

	merged := Config{}
	merged.Workspace.OnCreate = mergeString(projectMeta.IsDefined("workspace", "on-create"), projectCfg.Workspace.OnCreate, globalCfg.Workspace.OnCreate)
	merged.Workspace.OnAcquire = mergeString(projectMeta.IsDefined("workspace", "on-acquire"), projectCfg.Workspace.OnAcquire, globalCfg.Workspace.OnAcquire)
	merged.Job.Model = mergeString(projectMeta.IsDefined("job", "model"), projectCfg.Job.Model, globalCfg.Job.Model)
	merged.Job.ImplementationModel = mergeString(projectMeta.IsDefined("job", "implementation-model"), projectCfg.Job.ImplementationModel, globalCfg.Job.ImplementationModel)
	merged.Job.CodeReviewModel = mergeString(projectMeta.IsDefined("job", "code-review-model"), projectCfg.Job.CodeReviewModel, globalCfg.Job.CodeReviewModel)
	merged.Job.ProjectReviewModel = mergeString(projectMeta.IsDefined("job", "project-review-model"), projectCfg.Job.ProjectReviewModel, globalCfg.Job.ProjectReviewModel)
	if projectMeta.IsDefined("job", "test-commands") {
		merged.Job.TestCommands = append([]string(nil), projectCfg.Job.TestCommands...)
	} else if globalMeta.IsDefined("job", "test-commands") {
		merged.Job.TestCommands = append([]string(nil), globalCfg.Job.TestCommands...)
	}

	// Merge LLM config
	merged.LLM.Providers = mergeLLMProviders(globalCfg.LLM.Providers, projectCfg.LLM.Providers)
	merged.LLM.Model = mergeString(projectMeta.IsDefined("llm", "model"), projectCfg.LLM.Model, globalCfg.LLM.Model)

	// Merge Agent config
	merged.Agent.Model = mergeString(projectMeta.IsDefined("agent", "model"), projectCfg.Agent.Model, globalCfg.Agent.Model)

	return &merged
}

// mergeLLMProviders merges global and project LLM providers.
// Project providers with the same name as global providers override them.
// Providers are returned in order: project providers first, then remaining global providers.
func mergeLLMProviders(global, project []LLMProvider) []LLMProvider {
	if len(project) == 0 && len(global) == 0 {
		return nil
	}

	// Build a set of project provider names for quick lookup
	projectNames := make(map[string]bool, len(project))
	for _, p := range project {
		projectNames[p.Name] = true
	}

	// Start with project providers
	result := make([]LLMProvider, 0, len(project)+len(global))
	result = append(result, project...)

	// Add global providers that aren't overridden by project
	for _, p := range global {
		if !projectNames[p.Name] {
			result = append(result, p)
		}
	}

	return result
}

func mergeString(projectDefined bool, projectValue, globalValue string) string {
	value := globalValue
	if projectDefined {
		value = projectValue
	}
	return internalstrings.TrimSpace(value)
}

// RunScript executes a script in the given directory.
// If the script starts with a shebang (#!), that interpreter is used.
// Otherwise, the script is run with /bin/bash.
func RunScript(dir, script string) error {
	script = internalstrings.TrimSpace(script)
	if script == "" {
		return nil
	}

	var interpreter string
	scriptBody := ""

	if strings.HasPrefix(script, "#!") {
		// Extract shebang line
		lines := strings.SplitN(script, "\n", 2)
		interpreter = internalstrings.TrimSpace(strings.TrimPrefix(lines[0], "#!"))
		if len(lines) > 1 {
			scriptBody = lines[1]
		}
	} else {
		interpreter = "/bin/bash"
		scriptBody = script
	}

	// Parse interpreter and args (e.g., "/usr/bin/env python3" or "/bin/bash -e")
	parts := strings.Fields(interpreter)
	if len(parts) == 0 {
		return fmt.Errorf("empty interpreter in shebang")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(scriptBody)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
