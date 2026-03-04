package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"monks.co/incrementum/internal/config"
	"monks.co/incrementum/internal/testsupport"
)

func TestLoad_NotFound(t *testing.T) {
	testsupport.SetupTestHome(t)
	tmpDir := t.TempDir()

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	if cfg.Workspace.OnCreate != "" {
		t.Error("expected empty OnCreate")
	}

	if cfg.Workspace.OnAcquire != "" {
		t.Error("expected empty OnAcquire")
	}
}

func TestLoad_Full(t *testing.T) {
	testsupport.SetupTestHome(t)
	tmpDir := t.TempDir()

	configContent := `
[workspace]
on-create = """
npm install
go mod download
"""
on-acquire = "npm install"
`

	if err := os.WriteFile(filepath.Join(tmpDir, "incrementum.toml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Workspace.OnCreate == "" {
		t.Error("expected non-empty OnCreate")
	}

	if cfg.Workspace.OnAcquire != "npm install" {
		t.Errorf("OnAcquire = %q, expected %q", cfg.Workspace.OnAcquire, "npm install")
	}
}

func TestLoad_Full_DotIncrementum(t *testing.T) {
	testsupport.SetupTestHome(t)
	tmpDir := t.TempDir()

	configContent := `
[workspace]
on-create = "dot config"
on-acquire = "dot acquire"
`

	configDir := filepath.Join(tmpDir, ".incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Workspace.OnCreate != "dot config" {
		t.Errorf("OnCreate = %q, expected %q", cfg.Workspace.OnCreate, "dot config")
	}
	if cfg.Workspace.OnAcquire != "dot acquire" {
		t.Errorf("OnAcquire = %q, expected %q", cfg.Workspace.OnAcquire, "dot acquire")
	}
}

func TestLoad_WithShebang(t *testing.T) {
	testsupport.SetupTestHome(t)
	tmpDir := t.TempDir()

	configContent := `
[workspace]
on-create = """
#!/usr/bin/env python3
print("hello from python")
"""
`

	if err := os.WriteFile(filepath.Join(tmpDir, "incrementum.toml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Workspace.OnCreate == "" {
		t.Error("expected non-empty OnCreate")
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	testsupport.SetupTestHome(t)
	tmpDir := t.TempDir()

	configContent := `this is not valid toml [`

	if err := os.WriteFile(filepath.Join(tmpDir, "incrementum.toml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err := config.Load(tmpDir)
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}

func TestLoad_ProjectConfigConflict(t *testing.T) {
	testsupport.SetupTestHome(t)
	tmpDir := t.TempDir()

	configDir := filepath.Join(tmpDir, ".incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "incrementum.toml"), []byte("[workspace]\non-create=\"root\"\n"), 0o644); err != nil {
		t.Fatalf("failed to write root config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("[workspace]\non-create=\"dot\"\n"), 0o644); err != nil {
		t.Fatalf("failed to write dot config: %v", err)
	}

	_, err := config.Load(tmpDir)
	if err == nil {
		t.Fatal("expected error for conflicting project configs")
	}
}

func TestLoad_JobConfig(t *testing.T) {
	testsupport.SetupTestHome(t)
	tmpDir := t.TempDir()

	configContent := `
[job]
test-commands = ["go test ./...", "golangci-lint run"]
model = "claude-haiku-4-5"
implementation-model = "claude-haiku-4-5-20251001"
code-review-model = "claude-haiku-4-5"
project-review-model = "claude-haiku-4-5-20251001"
`

	if err := os.WriteFile(filepath.Join(tmpDir, "incrementum.toml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.Job.TestCommands) != 2 {
		t.Fatalf("expected 2 test commands, got %d", len(cfg.Job.TestCommands))
	}

	if cfg.Job.TestCommands[0] != "go test ./..." {
		t.Fatalf("expected first test command %q, got %q", "go test ./...", cfg.Job.TestCommands[0])
	}

	if cfg.Job.Model != "claude-haiku-4-5" {
		t.Fatalf("expected model %q, got %q", "claude-haiku-4-5", cfg.Job.Model)
	}
	if cfg.Job.ImplementationModel != "claude-haiku-4-5-20251001" {
		t.Fatalf("expected implementation model %q, got %q", "claude-haiku-4-5-20251001", cfg.Job.ImplementationModel)
	}
	if cfg.Job.CodeReviewModel != "claude-haiku-4-5" {
		t.Fatalf("expected code review model %q, got %q", "claude-haiku-4-5", cfg.Job.CodeReviewModel)
	}
	if cfg.Job.ProjectReviewModel != "claude-haiku-4-5-20251001" {
		t.Fatalf("expected project review model %q, got %q", "claude-haiku-4-5-20251001", cfg.Job.ProjectReviewModel)
	}
}

func TestRunScript_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	// Empty script should be a no-op
	if err := config.RunScript(tmpDir, ""); err != nil {
		t.Errorf("unexpected error for empty script: %v", err)
	}

	if err := config.RunScript(tmpDir, "   "); err != nil {
		t.Errorf("unexpected error for whitespace script: %v", err)
	}
}

func TestRunScript_SimpleBash(t *testing.T) {
	tmpDir := t.TempDir()

	script := `touch created.txt`

	if err := config.RunScript(tmpDir, script); err != nil {
		t.Fatalf("script failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "created.txt")); os.IsNotExist(err) {
		t.Error("script did not create file")
	}
}

func TestRunScript_MultipleBashCommands(t *testing.T) {
	tmpDir := t.TempDir()

	script := `
touch file1.txt
touch file2.txt
echo "done"
`

	if err := config.RunScript(tmpDir, script); err != nil {
		t.Fatalf("script failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "file1.txt")); os.IsNotExist(err) {
		t.Error("script did not create file1.txt")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "file2.txt")); os.IsNotExist(err) {
		t.Error("script did not create file2.txt")
	}
}

func TestRunScript_ExplicitBashShebang(t *testing.T) {
	tmpDir := t.TempDir()

	script := `#!/bin/bash
touch from_bash.txt
`

	if err := config.RunScript(tmpDir, script); err != nil {
		t.Fatalf("script failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "from_bash.txt")); os.IsNotExist(err) {
		t.Error("script did not create file")
	}
}

func TestRunScript_ShebangWithArgs(t *testing.T) {
	tmpDir := t.TempDir()

	// Use bash -e to exit on first error
	script := `#!/bin/bash -e
touch success.txt
`

	if err := config.RunScript(tmpDir, script); err != nil {
		t.Fatalf("script failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "success.txt")); os.IsNotExist(err) {
		t.Error("script did not create file")
	}
}

func TestRunScript_FailingScript(t *testing.T) {
	tmpDir := t.TempDir()

	script := `exit 1`

	if err := config.RunScript(tmpDir, script); err == nil {
		t.Error("expected error for failing script")
	}
}

func TestLoad_UsesGlobalWhenProjectMissing(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configContent := `
[agent]
cache-retention = "long"

[workspace]
on-create = "global create"

[job]
model = "global-model"
implementation-model = "global-implement"
code-review-model = "global-review"
project-review-model = "global-project"
test-commands = ["go test ./..."]

[merge]
target = "main"

[pool]
workers = 5
`

	globalPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(globalPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write global config: %v", err)
	}

	repoDir := t.TempDir()
	cfg, err := config.Load(repoDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Workspace.OnCreate != "global create" {
		t.Errorf("OnCreate = %q, expected %q", cfg.Workspace.OnCreate, "global create")
	}
	if cfg.Job.Model != "global-model" {
		t.Errorf("Model = %q, expected %q", cfg.Job.Model, "global-model")
	}
	if cfg.Job.ImplementationModel != "global-implement" {
		t.Errorf("ImplementationModel = %q, expected %q", cfg.Job.ImplementationModel, "global-implement")
	}
	if cfg.Job.CodeReviewModel != "global-review" {
		t.Errorf("CodeReviewModel = %q, expected %q", cfg.Job.CodeReviewModel, "global-review")
	}
	if cfg.Job.ProjectReviewModel != "global-project" {
		t.Errorf("ProjectReviewModel = %q, expected %q", cfg.Job.ProjectReviewModel, "global-project")
	}
	if cfg.Agent.CacheRetention != "long" {
		t.Errorf("CacheRetention = %q, expected %q", cfg.Agent.CacheRetention, "long")
	}
	if cfg.Merge.Target != "main" {
		t.Fatalf("Merge.Target = %q, expected %q", cfg.Merge.Target, "main")
	}
	if cfg.Pool.Workers != 5 {
		t.Fatalf("Pool.Workers = %d, expected %d", cfg.Pool.Workers, 5)
	}
	if len(cfg.Job.TestCommands) != 1 || cfg.Job.TestCommands[0] != "go test ./..." {
		t.Fatalf("expected global test commands to load")
	}
}

func TestLoad_ProjectOverridesGlobal(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	globalContent := `
[agent]
cache-retention = "short"

[workspace]
on-create = "global create"

[job]
model = "global-model"
implementation-model = "global-implement"
code-review-model = "global-review"
project-review-model = "global-project"
test-commands = ["global command"]

[merge]
target = "main"

[pool]
workers = 2
`
	globalPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(globalPath, []byte(globalContent), 0o644); err != nil {
		t.Fatalf("failed to write global config: %v", err)
	}

	projectContent := `
[agent]
cache-retention = "long"

[workspace]
on-acquire = "project acquire"

[job]
model = "project-model"
implementation-model = "project-implement"
code-review-model = "project-review"
project-review-model = "project-project"
test-commands = ["project command"]

[merge]
target = "release"

[pool]
workers = 4
`

	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, "incrementum.toml"), []byte(projectContent), 0o644); err != nil {
		t.Fatalf("failed to write project config: %v", err)
	}

	cfg, err := config.Load(repoDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Workspace.OnCreate != "global create" {
		t.Errorf("OnCreate = %q, expected %q", cfg.Workspace.OnCreate, "global create")
	}
	if cfg.Workspace.OnAcquire != "project acquire" {
		t.Errorf("OnAcquire = %q, expected %q", cfg.Workspace.OnAcquire, "project acquire")
	}
	if cfg.Job.Model != "project-model" {
		t.Errorf("Model = %q, expected %q", cfg.Job.Model, "project-model")
	}
	if cfg.Job.ImplementationModel != "project-implement" {
		t.Errorf("ImplementationModel = %q, expected %q", cfg.Job.ImplementationModel, "project-implement")
	}
	if cfg.Job.CodeReviewModel != "project-review" {
		t.Errorf("CodeReviewModel = %q, expected %q", cfg.Job.CodeReviewModel, "project-review")
	}
	if cfg.Job.ProjectReviewModel != "project-project" {
		t.Errorf("ProjectReviewModel = %q, expected %q", cfg.Job.ProjectReviewModel, "project-project")
	}
	if len(cfg.Job.TestCommands) != 1 || cfg.Job.TestCommands[0] != "project command" {
		t.Fatalf("expected project test commands to override global")
	}
	if cfg.Agent.CacheRetention != "long" {
		t.Fatalf("CacheRetention = %q, expected %q", cfg.Agent.CacheRetention, "long")
	}
	if cfg.Merge.Target != "release" {
		t.Fatalf("Merge.Target = %q, expected %q", cfg.Merge.Target, "release")
	}
	if cfg.Pool.Workers != 4 {
		t.Fatalf("Pool.Workers = %d, expected %d", cfg.Pool.Workers, 4)
	}
}

func TestLoad_ProjectEmptyOverridesGlobal(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	globalContent := `
[agent]
cache-retention = "long"

[workspace]
on-create = "global create"
on-acquire = "global acquire"

[job]
model = "global-model"
implementation-model = "global-implement"
code-review-model = "global-review"
project-review-model = "global-project"
test-commands = ["global command"]

[merge]
target = "main"

[pool]
workers = 6
`
	globalPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(globalPath, []byte(globalContent), 0o644); err != nil {
		t.Fatalf("failed to write global config: %v", err)
	}

	projectContent := `
[agent]
cache-retention = ""

[workspace]
on-create = ""
on-acquire = ""

[job]
model = ""
implementation-model = ""
code-review-model = ""
project-review-model = ""
test-commands = []

[merge]
target = ""

[pool]
workers = 0
`

	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, "incrementum.toml"), []byte(projectContent), 0o644); err != nil {
		t.Fatalf("failed to write project config: %v", err)
	}

	cfg, err := config.Load(repoDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Workspace.OnCreate != "" {
		t.Errorf("OnCreate = %q, expected empty string", cfg.Workspace.OnCreate)
	}
	if cfg.Workspace.OnAcquire != "" {
		t.Errorf("OnAcquire = %q, expected empty string", cfg.Workspace.OnAcquire)
	}
	if cfg.Job.Model != "" {
		t.Errorf("Model = %q, expected empty string", cfg.Job.Model)
	}
	if cfg.Job.ImplementationModel != "" {
		t.Errorf("ImplementationModel = %q, expected empty string", cfg.Job.ImplementationModel)
	}
	if cfg.Job.CodeReviewModel != "" {
		t.Errorf("CodeReviewModel = %q, expected empty string", cfg.Job.CodeReviewModel)
	}
	if cfg.Job.ProjectReviewModel != "" {
		t.Errorf("ProjectReviewModel = %q, expected empty string", cfg.Job.ProjectReviewModel)
	}
	if len(cfg.Job.TestCommands) != 0 {
		t.Fatalf("expected empty test commands, got %d", len(cfg.Job.TestCommands))
	}
	if cfg.Merge.Target != "" {
		t.Fatalf("Merge.Target = %q, expected empty string", cfg.Merge.Target)
	}
	if cfg.Pool.Workers != 0 {
		t.Fatalf("Pool.Workers = %d, expected %d", cfg.Pool.Workers, 0)
	}
	if cfg.Agent.CacheRetention != "short" {
		t.Fatalf("CacheRetention = %q, expected %q", cfg.Agent.CacheRetention, "short")
	}
}

func TestLoad_LLMProviders(t *testing.T) {
	testsupport.SetupTestHome(t)
	tmpDir := t.TempDir()

	configContent := `
[[llm.providers]]
name = "anthropic"
api = "anthropic-messages"
base-url = "https://api.anthropic.com"
api-key-command = "op read op://Private/Anthropic/credential"
models = ["claude-haiku-4-5-20251001", "claude-haiku-4-5"]

[[llm.providers]]
name = "openai"
api = "openai-completions"
base-url = "https://api.openai.com/v1"
api-key-command = "echo $OPENAI_API_KEY"
models = ["gpt-4o", "gpt-4o-mini"]
`

	if err := os.WriteFile(filepath.Join(tmpDir, "incrementum.toml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.LLM.Providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(cfg.LLM.Providers))
	}

	anthropic := cfg.LLM.Providers[0]
	if anthropic.Name != "anthropic" {
		t.Errorf("expected provider name 'anthropic', got %q", anthropic.Name)
	}
	if anthropic.API != "anthropic-messages" {
		t.Errorf("expected API 'anthropic-messages', got %q", anthropic.API)
	}
	if anthropic.BaseURL != "https://api.anthropic.com" {
		t.Errorf("expected base URL 'https://api.anthropic.com', got %q", anthropic.BaseURL)
	}
	if anthropic.APIKeyCommand != "op read op://Private/Anthropic/credential" {
		t.Errorf("expected api-key-command, got %q", anthropic.APIKeyCommand)
	}
	if len(anthropic.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(anthropic.Models))
	}

	openai := cfg.LLM.Providers[1]
	if openai.Name != "openai" {
		t.Errorf("expected provider name 'openai', got %q", openai.Name)
	}
	if len(openai.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(openai.Models))
	}
}

func TestLoad_LLMProviderNoAPIKey(t *testing.T) {
	testsupport.SetupTestHome(t)
	tmpDir := t.TempDir()

	configContent := `
[[llm.providers]]
name = "internal-claude"
api = "anthropic-messages"
base-url = "https://internal-claude.example.com"
models = ["claude-haiku-4-5-20251001"]
`

	if err := os.WriteFile(filepath.Join(tmpDir, "incrementum.toml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.LLM.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(cfg.LLM.Providers))
	}

	provider := cfg.LLM.Providers[0]
	if provider.APIKeyCommand != "" {
		t.Errorf("expected empty api-key-command, got %q", provider.APIKeyCommand)
	}
}

func TestLoad_LLMProvidersGlobalOnly(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	globalContent := `
[[llm.providers]]
name = "anthropic"
api = "anthropic-messages"
base-url = "https://api.anthropic.com"
api-key-command = "op read op://Private/Anthropic/credential"
models = ["claude-haiku-4-5-20251001"]
`
	globalPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(globalPath, []byte(globalContent), 0o644); err != nil {
		t.Fatalf("failed to write global config: %v", err)
	}

	repoDir := t.TempDir()
	cfg, err := config.Load(repoDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.LLM.Providers) != 1 {
		t.Fatalf("expected 1 provider from global config, got %d", len(cfg.LLM.Providers))
	}

	if cfg.LLM.Providers[0].Name != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", cfg.LLM.Providers[0].Name)
	}
}

func TestLoad_LLMProvidersProjectOverridesGlobal(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	globalContent := `
[[llm.providers]]
name = "anthropic"
api = "anthropic-messages"
base-url = "https://api.anthropic.com"
api-key-command = "global-key-cmd"
models = ["global-model"]

[[llm.providers]]
name = "openai"
api = "openai-completions"
base-url = "https://api.openai.com/v1"
models = ["gpt-4o"]
`
	globalPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(globalPath, []byte(globalContent), 0o644); err != nil {
		t.Fatalf("failed to write global config: %v", err)
	}

	projectContent := `
[[llm.providers]]
name = "anthropic"
api = "anthropic-messages"
base-url = "https://project.anthropic.com"
api-key-command = "project-key-cmd"
models = ["project-model"]
`
	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, "incrementum.toml"), []byte(projectContent), 0o644); err != nil {
		t.Fatalf("failed to write project config: %v", err)
	}

	cfg, err := config.Load(repoDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Should have 2 providers: project anthropic + global openai
	if len(cfg.LLM.Providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(cfg.LLM.Providers))
	}

	// Project provider should come first and override global
	anthropic := cfg.LLM.Providers[0]
	if anthropic.Name != "anthropic" {
		t.Errorf("expected first provider 'anthropic', got %q", anthropic.Name)
	}
	if anthropic.BaseURL != "https://project.anthropic.com" {
		t.Errorf("expected project base URL, got %q", anthropic.BaseURL)
	}
	if anthropic.APIKeyCommand != "project-key-cmd" {
		t.Errorf("expected project api-key-command, got %q", anthropic.APIKeyCommand)
	}
	if len(anthropic.Models) != 1 || anthropic.Models[0] != "project-model" {
		t.Errorf("expected project models, got %v", anthropic.Models)
	}

	// Global-only provider should still be present
	openai := cfg.LLM.Providers[1]
	if openai.Name != "openai" {
		t.Errorf("expected second provider 'openai', got %q", openai.Name)
	}
}

func TestLoadGlobal(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	globalContent := `
[[llm.providers]]
name = "anthropic"
api = "anthropic-messages"
base-url = "https://api.anthropic.com"
models = ["claude-haiku-4-5-20251001"]
`
	globalPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(globalPath, []byte(globalContent), 0o644); err != nil {
		t.Fatalf("failed to write global config: %v", err)
	}

	cfg, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("failed to load global config: %v", err)
	}

	if len(cfg.LLM.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(cfg.LLM.Providers))
	}

	if cfg.LLM.Providers[0].Name != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", cfg.LLM.Providers[0].Name)
	}
}

func TestLoadGlobal_NotFound(t *testing.T) {
	testsupport.SetupTestHome(t)

	cfg, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	if len(cfg.LLM.Providers) != 0 {
		t.Errorf("expected 0 providers, got %d", len(cfg.LLM.Providers))
	}
}
