package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"monks.co/incrementum/internal/config"
	"monks.co/incrementum/internal/testsupport"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestAgentScripts(t *testing.T) {
	// Find the module root where incrementum.toml should be
	moduleRoot := findModuleRootForTest(t)
	configPath := filepath.Join(moduleRoot, "incrementum.toml")

	// Verify incrementum.toml exists
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("incrementum.toml not found at %s: %v", configPath, err)
	}

	// Load and verify the config has providers that support:
	// - claude-haiku-4-5 (explicitly used by several scripts)
	// - gpt-5.2 (the default llm.model used when scripts omit --model)
	cfg, err := config.Load(moduleRoot)
	if err != nil {
		t.Fatalf("failed to load incrementum.toml: %v", err)
	}

	hasHaiku := false
	hasGPT52 := false
	for _, provider := range cfg.LLM.Providers {
		for _, model := range provider.Models {
			if strings.Contains(model, "claude-haiku-4-5") {
				hasHaiku = true
			}
			if model == "gpt-5.2" {
				hasGPT52 = true
			}
		}
	}
	if !hasHaiku {
		t.Fatal("config does not have a provider configured with claude-haiku-4-5 (check global config)")
	}
	if !hasGPT52 {
		t.Fatal("config does not have a provider configured with gpt-5.2 (check global config)")
	}

	// Read the project config file content to pass to tests
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read incrementum.toml: %v", err)
	}

	// Read the global config so testscript environments (which use a temp HOME)
	// can still resolve LLM providers.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home directory: %v", err)
	}
	globalConfigPath := filepath.Join(homeDir, ".config", "incrementum", "config.toml")
	globalConfigContent, err := os.ReadFile(globalConfigPath)
	if err != nil {
		t.Fatalf("failed to read global config at %s: %v", globalConfigPath, err)
	}

	testscript.Run(t, testscript.Params{
		Dir: "testdata/agent",
		Setup: func(env *testscript.Env) error {
			if err := testsupport.SetupScriptEnv(t, env); err != nil {
				return err
			}
			// Avoid any external environment overrides changing model selection.
			env.Setenv("INCREMENTUM_AGENT_MODEL", "")
			// Write the real project config to a file that tests can copy
			configFile := filepath.Join(env.WorkDir, "incrementum.toml")
			if err := os.WriteFile(configFile, configContent, 0644); err != nil {
				return err
			}
			// Write the global config into the testscript's HOME so the ii
			// binary can resolve LLM providers.
			testHome := env.Getenv("HOME")
			globalDir := filepath.Join(testHome, ".config", "incrementum")
			if err := os.MkdirAll(globalDir, 0755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(globalDir, "config.toml"), globalConfigContent, 0644); err != nil {
				return err
			}
			return nil
		},
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"todoid": testsupport.CmdTodoID,
		},
	})
}

func findModuleRootForTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find module root (go.mod)")
		}
		dir = parent
	}
}
