package testsupport

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rogpeppe/go-internal/testscript"
)

// CmdRunBG starts a command in the background.
//
// Usage:
//   runbg PIDVAR -- cmd args...
//
// The started process inherits the testscript environment and has its stdout and
// stderr redirected to files in the workdir:
//   <PIDVAR>.stdout, <PIDVAR>.stderr
func CmdRunBG(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("runbg does not support negation")
	}
	if len(args) < 3 || args[1] != "--" {
		ts.Fatalf("usage: runbg PIDVAR -- cmd args...")
	}

	pidVar := args[0]
	cmdArgs := args[2:]
	if len(cmdArgs) == 0 {
		ts.Fatalf("usage: runbg PIDVAR -- cmd args...")
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)

	// If the script has cd'd, prefer its current directory (exposed via PWD).
	if v := ts.Getenv("PWD"); v != "" {
		cmd.Dir = v
	} else {
		cmd.Dir = ts.Getenv("WORK")
	}
	cmd.Env = append([]string{}, os.Environ()...)
	cmd.Env = append(cmd.Env, "HOME="+ts.Getenv("HOME"))
	if v := ts.Getenv("PWD"); v != "" {
		cmd.Env = append(cmd.Env, "PWD="+v)
	}
	if v := ts.Getenv("INCREMENTUM_STATE_DIR"); v != "" {
		cmd.Env = append(cmd.Env, "INCREMENTUM_STATE_DIR="+v)
	}
	if v := ts.Getenv("INCREMENTUM_AGENT_EVENTS_DIR"); v != "" {
		cmd.Env = append(cmd.Env, "INCREMENTUM_AGENT_EVENTS_DIR="+v)
	}
	if v := ts.Getenv("INCREMENTUM_AGENT_MODEL"); v != "" {
		cmd.Env = append(cmd.Env, "INCREMENTUM_AGENT_MODEL="+v)
	}

	stdoutPath := filepath.Join(ts.Getenv("WORK"), pidVar+".stdout")
	stderrPath := filepath.Join(ts.Getenv("WORK"), pidVar+".stderr")
	stdoutF, err := os.Create(stdoutPath)
	if err != nil {
		ts.Fatalf("runbg: create stdout: %v", err)
	}
	defer stdoutF.Close()
	stderrF, err := os.Create(stderrPath)
	if err != nil {
		ts.Fatalf("runbg: create stderr: %v", err)
	}
	defer stderrF.Close()
	cmd.Stdout = stdoutF
	cmd.Stderr = stderrF

	if err := cmd.Start(); err != nil {
		ts.Fatalf("runbg: start %s: %v", strings.Join(cmdArgs, " "), err)
	}

	ts.Setenv(pidVar, strconv.Itoa(cmd.Process.Pid))
}
