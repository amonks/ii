package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amonks/incrementum/internal/config"
	"github.com/amonks/incrementum/internal/testsupport"
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
		t.Fatal("incrementum.toml does not have a provider configured with claude-haiku-4-5")
	}
	if !hasGPT52 {
		t.Fatal("incrementum.toml does not have a provider configured with gpt-5.2")
	}

	// Read the config file content to pass to tests
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read incrementum.toml: %v", err)
	}

	testscript.Run(t, testscript.Params{
		Dir: "testdata/agent",
		Setup: func(env *testscript.Env) error {
			if err := testsupport.SetupScriptEnv(t, env); err != nil {
				return err
			}
			// Avoid any external environment overrides changing model selection.
			env.Setenv("INCREMENTUM_AGENT_MODEL", "")
			// Write the real config to a file that tests can copy
			configFile := filepath.Join(env.WorkDir, "incrementum.toml")
			if err := os.WriteFile(configFile, configContent, 0644); err != nil {
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
