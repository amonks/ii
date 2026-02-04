package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amonks/incrementum/internal/testsupport"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestAgentTranscriptCommandRemoved(t *testing.T) {
	moduleRoot := findModuleRootForTest(t)
	configPath := filepath.Join(moduleRoot, "incrementum.toml")
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read incrementum.toml: %v", err)
	}

	testscript.Run(t, testscript.Params{
		Dir: "testdata/agent-cli",
		Setup: func(env *testscript.Env) error {
			if err := testsupport.SetupScriptEnv(t, env); err != nil {
				return err
			}
			configFile := filepath.Join(env.WorkDir, "incrementum.toml")
			return os.WriteFile(configFile, configContent, 0644)
		},
	})
}
