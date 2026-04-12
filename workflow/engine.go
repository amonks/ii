package workflow

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

)

// RunOptions configures a workflow execution.
type RunOptions struct {
	// DB is the SQLite database for state persistence.
	DB *sql.DB
	// Workspace is the working directory for node commands (cwd at invocation).
	Workspace string
	// ScratchpadBase is the base directory for scratchpad storage.
	// Default: ~/.local/state/incrementum/scratchpads
	ScratchpadBase string
	// Inputs provides values for workflow inputs, keyed by input name.
	Inputs map[string]string
	// Repo is the repo identifier for state tracking.
	Repo string
}

// Execution represents a running or completed workflow execution.
type Execution struct {
	ID           string
	WorkflowName string
	Status       ExecutionStatus
	CurrentNode  string
}

// ExecutionStatus is the status of a workflow execution.
type ExecutionStatus string

const (
	StatusActive      ExecutionStatus = "active"
	StatusCompleted   ExecutionStatus = "completed"
	StatusFailed      ExecutionStatus = "failed"
	StatusInterrupted ExecutionStatus = "interrupted"
)

// Run executes a workflow to completion.
func Run(ctx context.Context, wf Workflow, opts RunOptions) (*Execution, error) {
	if err := validate(wf); err != nil {
		return nil, fmt.Errorf("invalid workflow: %w", err)
	}

	// Resolve workspace.
	workspace := opts.Workspace
	if workspace == "" {
		var err error
		workspace, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}
	workspace, err := filepath.Abs(workspace)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace: %w", err)
	}

	// Resolve inputs.
	inputValues, err := resolveInputs(wf.Inputs, opts.Inputs)
	if err != nil {
		return nil, err
	}

	// Generate execution ID.
	execID := generateID(wf.Name, time.Now())

	// Create scratchpad.
	scratchpadDir := filepath.Join(opts.ScratchpadBase, execID)
	sp, err := newScratchpad(scratchpadDir)
	if err != nil {
		return nil, err
	}

	// Write inputs to scratchpad.
	for name, value := range inputValues {
		if err := sp.Write(name, value); err != nil {
			return nil, fmt.Errorf("write input %q: %w", name, err)
		}
	}

	// Record execution in DB.
	exec := &Execution{
		ID:           execID,
		WorkflowName: wf.Name,
		Status:       StatusActive,
		CurrentNode:  wf.Start,
	}
	if opts.DB != nil {
		if err := insertExecution(opts.DB, exec, opts.Repo, workspace); err != nil {
			return nil, err
		}
	}

	// Build lookup structures.
	nodeMap := make(map[string]Node, len(wf.Nodes))
	for _, n := range wf.Nodes {
		nodeMap[n.Name] = n
	}
	edgeMap := make(map[string][]Edge)
	for _, e := range wf.Edges {
		edgeMap[e.From] = append(edgeMap[e.From], e)
	}

	// Execute the graph.
	currentNode := wf.Start
	var lastExitCode int

	for {
		// Check for cancellation.
		if ctx.Err() != nil {
			exec.Status = StatusInterrupted
			exec.CurrentNode = currentNode
			if opts.DB != nil {
				updateExecution(opts.DB, exec)
			}
			return exec, ctx.Err()
		}

		node, ok := nodeMap[currentNode]
		if !ok {
			exec.Status = StatusFailed
			if opts.DB != nil {
				updateExecution(opts.DB, exec)
			}
			return exec, fmt.Errorf("node %q not found", currentNode)
		}

		// Run workspace hygiene (best-effort).
		runHygiene(workspace)

		// Snapshot scratchpad before.
		before, err := sp.Snapshot()
		if err != nil {
			return exec, fmt.Errorf("snapshot scratchpad: %w", err)
		}

		// Execute the node.
		nodeRunStart := time.Now()
		exitCode, err := executeNode(ctx, node, workspace, sp.Dir(), lastExitCode)
		nodeRunEnd := time.Now()
		if err != nil && ctx.Err() != nil {
			exec.Status = StatusInterrupted
			exec.CurrentNode = currentNode
			if opts.DB != nil {
				recordNodeRun(opts.DB, execID, currentNode, nil, nodeRunStart, nodeRunEnd)
				updateExecution(opts.DB, exec)
			}
			return exec, ctx.Err()
		}

		lastExitCode = exitCode

		// Snapshot scratchpad after and record diffs.
		after, err := sp.Snapshot()
		if err != nil {
			return exec, fmt.Errorf("snapshot scratchpad: %w", err)
		}
		changes := Diff(before, after)

		// Record node run.
		if opts.DB != nil {
			nodeRunID := recordNodeRun(opts.DB, execID, currentNode, &exitCode, nodeRunStart, nodeRunEnd)
			recordScratchpadDiffs(opts.DB, nodeRunID, changes)
		}

		// Update execution state.
		exec.CurrentNode = currentNode
		if opts.DB != nil {
			updateExecution(opts.DB, exec)
		}

		// Evaluate edges.
		outEdges := edgeMap[currentNode]
		if len(outEdges) == 0 {
			// Terminal node — workflow completes successfully.
			exec.Status = StatusCompleted
			if opts.DB != nil {
				updateExecution(opts.DB, exec)
			}
			sp.Remove()
			return exec, nil
		}

		nextNode, err := evaluateEdges(outEdges, workspace, sp.Dir(), lastExitCode)
		if err != nil {
			exec.Status = StatusFailed
			exec.CurrentNode = currentNode
			if opts.DB != nil {
				updateExecution(opts.DB, exec)
			}
			return exec, fmt.Errorf("no matching edge from node %q (exit code %d): %w", currentNode, lastExitCode, err)
		}

		currentNode = nextNode
	}
}

// executeNode runs a node's command and returns the exit code.
func executeNode(ctx context.Context, node Node, workspace, scratchpadDir string, lastExitCode int) (int, error) {
	env := append(os.Environ(),
		"SCRATCHPAD="+scratchpadDir,
		"WORKSPACE="+workspace,
		fmt.Sprintf("EXIT_CODE=%d", lastExitCode),
	)

	cmd := exec.CommandContext(ctx, "bash", "-c", node.Command)
	cmd.Dir = workspace
	cmd.Env = env

	if node.TTY {
		// TTY passthrough: use creack/pty to allocate a pseudo-terminal.
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return exitErr.ExitCode(), nil
			}
			return -1, err
		}
		return 0, nil
	}

	// Non-TTY: stdout/stderr pass through to the terminal.
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}

// evaluateEdges returns the name of the next node based on edge conditions.
func evaluateEdges(edges []Edge, workspace, scratchpadDir string, exitCode int) (string, error) {
	for _, edge := range edges {
		if edge.Condition == "" {
			return edge.To, nil
		}
		if conditionMatches(edge.Condition, workspace, scratchpadDir, exitCode) {
			return edge.To, nil
		}
	}
	return "", fmt.Errorf("no edge condition matched")
}

// conditionMatches runs a condition command and returns true if it exits 0.
func conditionMatches(condition, workspace, scratchpadDir string, exitCode int) bool {
	cmd := exec.Command("bash", "-c", condition)
	cmd.Dir = workspace
	cmd.Env = append(os.Environ(),
		"SCRATCHPAD="+scratchpadDir,
		"WORKSPACE="+workspace,
		fmt.Sprintf("EXIT_CODE=%d", exitCode),
	)
	return cmd.Run() == nil
}

// runHygiene runs jj workspace update-stale and snapshot (best-effort).
func runHygiene(workspace string) {
	cmd := exec.Command("jj", "workspace", "update-stale")
	cmd.Dir = workspace
	cmd.Run()

	cmd = exec.Command("jj", "debug", "snapshot")
	cmd.Dir = workspace
	cmd.Run()
}

// resolveInputs validates and resolves workflow inputs.
func resolveInputs(inputs []Input, provided map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(inputs))
	for _, input := range inputs {
		if val, ok := provided[input.Name]; ok {
			result[input.Name] = val
		} else if input.Default != "" {
			result[input.Name] = input.Default
		} else if input.Required {
			return nil, fmt.Errorf("missing required input: %s", input.Name)
		}
	}
	return result, nil
}

// generateID creates a unique execution ID.
func generateID(workflowName string, t time.Time) string {
	return fmt.Sprintf("%s-%d", workflowName, t.UnixNano())
}

