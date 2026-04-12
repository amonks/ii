package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunSimpleWorkflow(t *testing.T) {
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	os.MkdirAll(workspace, 0o755)
	scratchpadBase := filepath.Join(tmp, "scratchpads")

	wf := Workflow{
		Name:  "test",
		Start: "hello",
		Nodes: []Node{
			{Name: "hello", Command: `echo "hello world"`},
		},
	}

	exec, err := Run(context.Background(), wf, RunOptions{
		Workspace:      workspace,
		ScratchpadBase: scratchpadBase,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.Status != StatusCompleted {
		t.Errorf("expected status completed, got %s", exec.Status)
	}
	if exec.CurrentNode != "hello" {
		t.Errorf("expected current node hello, got %s", exec.CurrentNode)
	}
}

func TestRunWithInputs(t *testing.T) {
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	os.MkdirAll(workspace, 0o755)
	scratchpadBase := filepath.Join(tmp, "scratchpads")

	wf := Workflow{
		Name:  "test-inputs",
		Start: "greet",
		Inputs: []Input{
			{Name: "name", Required: true},
		},
		Nodes: []Node{
			{Name: "greet", Command: `cat $SCRATCHPAD/name > $SCRATCHPAD/greeting`},
		},
	}

	exec, err := Run(context.Background(), wf, RunOptions{
		Workspace:      workspace,
		ScratchpadBase: scratchpadBase,
		Inputs:         map[string]string{"name": "world"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.Status != StatusCompleted {
		t.Errorf("expected completed, got %s", exec.Status)
	}
}

func TestRunMissingRequiredInput(t *testing.T) {
	tmp := t.TempDir()

	wf := Workflow{
		Name:  "test",
		Start: "a",
		Inputs: []Input{
			{Name: "required-thing", Required: true},
		},
		Nodes: []Node{
			{Name: "a", Command: "true"},
		},
	}

	_, err := Run(context.Background(), wf, RunOptions{
		Workspace:      tmp,
		ScratchpadBase: filepath.Join(tmp, "sp"),
	})
	if err == nil {
		t.Fatal("expected error for missing required input")
	}
}

func TestRunConditionalEdges(t *testing.T) {
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	os.MkdirAll(workspace, 0o755)
	scratchpadBase := filepath.Join(tmp, "scratchpads")

	wf := Workflow{
		Name:  "conditional",
		Start: "check",
		Nodes: []Node{
			{Name: "check", Command: `echo "yes" > $SCRATCHPAD/result`},
			{Name: "yes-branch", Command: `echo "took yes branch" > $SCRATCHPAD/outcome`},
			{Name: "no-branch", Command: `echo "took no branch" > $SCRATCHPAD/outcome`},
		},
		Edges: []Edge{
			{From: "check", To: "yes-branch", Condition: `grep -q "yes" $SCRATCHPAD/result`},
			{From: "check", To: "no-branch"},
		},
	}

	exec, err := Run(context.Background(), wf, RunOptions{
		Workspace:      workspace,
		ScratchpadBase: scratchpadBase,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.Status != StatusCompleted {
		t.Errorf("expected completed, got %s", exec.Status)
	}
	if exec.CurrentNode != "yes-branch" {
		t.Errorf("expected yes-branch, got %s", exec.CurrentNode)
	}
}

func TestRunExitCodeCondition(t *testing.T) {
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	os.MkdirAll(workspace, 0o755)
	scratchpadBase := filepath.Join(tmp, "scratchpads")

	wf := Workflow{
		Name:  "exit-code",
		Start: "fail",
		Nodes: []Node{
			{Name: "fail", Command: "exit 1"},
			{Name: "on-failure", Command: "true"},
			{Name: "on-success", Command: "true"},
		},
		Edges: []Edge{
			{From: "fail", To: "on-success", Condition: `[ $EXIT_CODE -eq 0 ]`},
			{From: "fail", To: "on-failure", Condition: `[ $EXIT_CODE -ne 0 ]`},
		},
	}

	exec, err := Run(context.Background(), wf, RunOptions{
		Workspace:      workspace,
		ScratchpadBase: scratchpadBase,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.CurrentNode != "on-failure" {
		t.Errorf("expected on-failure, got %s", exec.CurrentNode)
	}
}

func TestRunCycle(t *testing.T) {
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	os.MkdirAll(workspace, 0o755)
	scratchpadBase := filepath.Join(tmp, "scratchpads")

	// Workflow that loops 3 times then exits.
	wf := Workflow{
		Name:  "cycle",
		Start: "increment",
		Nodes: []Node{
			{Name: "increment", Command: `
				count=$(cat $SCRATCHPAD/count 2>/dev/null || echo 0)
				count=$((count + 1))
				echo $count > $SCRATCHPAD/count
			`},
			{Name: "done", Command: "true"},
		},
		Edges: []Edge{
			{From: "increment", To: "done", Condition: `[ "$(cat $SCRATCHPAD/count)" -ge 3 ]`},
			{From: "increment", To: "increment"},
		},
	}

	exec, err := Run(context.Background(), wf, RunOptions{
		Workspace:      workspace,
		ScratchpadBase: scratchpadBase,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.Status != StatusCompleted {
		t.Errorf("expected completed, got %s", exec.Status)
	}
	if exec.CurrentNode != "done" {
		t.Errorf("expected done, got %s", exec.CurrentNode)
	}
}

func TestRunNoMatchingEdge(t *testing.T) {
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	os.MkdirAll(workspace, 0o755)

	wf := Workflow{
		Name:  "no-match",
		Start: "start",
		Nodes: []Node{
			{Name: "start", Command: "true"},
			{Name: "unreachable", Command: "true"},
		},
		Edges: []Edge{
			{From: "start", To: "unreachable", Condition: "false"},
		},
	}

	exec, err := Run(context.Background(), wf, RunOptions{
		Workspace:      workspace,
		ScratchpadBase: filepath.Join(tmp, "sp"),
	})
	if err == nil {
		t.Fatal("expected error for no matching edge")
	}
	if exec.Status != StatusFailed {
		t.Errorf("expected failed, got %s", exec.Status)
	}
}

func TestRunCancellation(t *testing.T) {
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	os.MkdirAll(workspace, 0o755)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	wf := Workflow{
		Name:  "cancel",
		Start: "slow",
		Nodes: []Node{
			{Name: "slow", Command: "sleep 10"},
		},
	}

	exec, err := Run(ctx, wf, RunOptions{
		Workspace:      workspace,
		ScratchpadBase: filepath.Join(tmp, "sp"),
	})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if exec.Status != StatusInterrupted {
		t.Errorf("expected interrupted, got %s", exec.Status)
	}
}

func TestRunDefaultInput(t *testing.T) {
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	os.MkdirAll(workspace, 0o755)

	wf := Workflow{
		Name:  "defaults",
		Start: "check",
		Inputs: []Input{
			{Name: "target", Default: "main"},
		},
		Nodes: []Node{
			{Name: "check", Command: `grep -q "main" $SCRATCHPAD/target`},
		},
	}

	exec, err := Run(context.Background(), wf, RunOptions{
		Workspace:      workspace,
		ScratchpadBase: filepath.Join(tmp, "sp"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.Status != StatusCompleted {
		t.Errorf("expected completed, got %s", exec.Status)
	}
}
