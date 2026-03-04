package testsupport

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/amonks/incrementum/internal/paths"
	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/todo"
	"github.com/rogpeppe/go-internal/testscript"
)

var (
	buildOnce sync.Once
	iiPath    string
	buildErr  error
)

// BuildII builds the ii binary once and returns its path.
func BuildII(t testing.TB) string {
	t.Helper()

	buildOnce.Do(func() {
		moduleRoot, err := findModuleRoot()
		if err != nil {
			buildErr = err
			return
		}

		binDir, err := os.MkdirTemp("", "ii-bin-")
		if err != nil {
			buildErr = err
			return
		}

		iiPath = filepath.Join(binDir, "ii")
		cmd := exec.Command("go", "build", "-o", iiPath, "./cmd/ii")
		cmd.Dir = moduleRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			buildErr = fmt.Errorf("build ii: %w: %s", err, internalstrings.TrimSpace(string(output)))
		}
	})

	if buildErr != nil {
		t.Fatalf("%v", buildErr)
	}

	return iiPath
}

// SetupScriptEnv configures common environment variables for testscript.
func SetupScriptEnv(t testing.TB, env *testscript.Env) error {
	t.Helper()

	env.Setenv("II", BuildII(t))

	homeDir := filepath.Join(env.WorkDir, "home")
	if err := EnsureHomeDirs(homeDir); err != nil {
		return err
	}
	env.Setenv("HOME", homeDir)
	return nil
}

// CmdEnvSet stores the trimmed contents of a file in an env var.
func CmdEnvSet(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("envset does not support negation")
	}
	if len(args) != 2 {
		ts.Fatalf("usage: envset VAR FILE")
	}

	value := internalstrings.TrimSpace(ts.ReadFile(args[1]))
	ts.Setenv(args[0], value)
}

// CmdTodoID finds a todo by title and stores its ID in an env var.
func CmdTodoID(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("todoid does not support negation")
	}
	if len(args) != 3 {
		ts.Fatalf("usage: todoid FILE TITLE VAR")
	}

	var items []todo.Todo
	data := ts.ReadFile(args[0])
	if err := json.Unmarshal([]byte(data), &items); err != nil {
		ts.Fatalf("parse todo list: %v", err)
	}

	title := args[1]
	for _, item := range items {
		if item.Title == title {
			ts.Setenv(args[2], item.ID)
			return
		}
	}

	ts.Fatalf("todo with title %q not found", title)
}

func findModuleRoot() (string, error) {
	dir, err := paths.WorkingDir()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find module root (go.mod)")
		}
		dir = parent
	}
}
