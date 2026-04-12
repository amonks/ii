package workflow

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Resume continues an interrupted or failed workflow execution.
func Resume(ctx context.Context, executionID string, opts ResumeOptions) (*Execution, error) {
	if opts.DB == nil {
		return nil, fmt.Errorf("database is required for resume")
	}

	// Load execution record.
	record, err := FindExecution(opts.DB, opts.Repo, executionID)
	if err != nil {
		return nil, err
	}

	if record.Status != StatusInterrupted && record.Status != StatusFailed {
		return nil, fmt.Errorf("cannot resume execution with status %q (must be interrupted or failed)", record.Status)
	}

	// Look up the workflow definition.
	wf, ok := Registry[record.WorkflowName]
	if !ok {
		return nil, fmt.Errorf("unknown workflow: %s", record.WorkflowName)
	}

	if err := validate(wf); err != nil {
		return nil, fmt.Errorf("invalid workflow: %w", err)
	}

	// Verify scratchpad exists.
	scratchpadDir := filepath.Join(opts.ScratchpadBase, record.ID)
	if _, err := os.Stat(scratchpadDir); err != nil {
		return nil, fmt.Errorf("scratchpad not found for execution %s: %w", record.ID, err)
	}

	sp := &scratchpad{dir: scratchpadDir}

	// Mark as active.
	exec := &Execution{
		ID:           record.ID,
		WorkflowName: record.WorkflowName,
		Status:       StatusActive,
		CurrentNode:  record.CurrentNode,
	}
	updateExecution(opts.DB, exec)

	// Build lookup structures.
	nodeMap := make(map[string]Node, len(wf.Nodes))
	for _, n := range wf.Nodes {
		nodeMap[n.Name] = n
	}
	edgeMap := make(map[string][]Edge)
	for _, e := range wf.Edges {
		edgeMap[e.From] = append(edgeMap[e.From], e)
	}

	// Resume at the current node.
	currentNode := record.CurrentNode
	var lastExitCode int

	for {
		if ctx.Err() != nil {
			exec.Status = StatusInterrupted
			exec.CurrentNode = currentNode
			updateExecution(opts.DB, exec)
			return exec, ctx.Err()
		}

		node, ok := nodeMap[currentNode]
		if !ok {
			exec.Status = StatusFailed
			updateExecution(opts.DB, exec)
			return exec, fmt.Errorf("node %q not found", currentNode)
		}

		runHygiene(record.Workspace)

		before, err := sp.Snapshot()
		if err != nil {
			return exec, fmt.Errorf("snapshot scratchpad: %w", err)
		}

		nodeRunStart := time.Now()
		exitCode, err := executeNode(ctx, node, record.Workspace, sp.Dir(), lastExitCode)
		nodeRunEnd := time.Now()
		if err != nil && ctx.Err() != nil {
			exec.Status = StatusInterrupted
			exec.CurrentNode = currentNode
			recordNodeRun(opts.DB, record.ID, currentNode, nil, nodeRunStart, nodeRunEnd)
			updateExecution(opts.DB, exec)
			return exec, ctx.Err()
		}

		lastExitCode = exitCode

		after, err := sp.Snapshot()
		if err != nil {
			return exec, fmt.Errorf("snapshot scratchpad: %w", err)
		}
		changes := Diff(before, after)

		nodeRunID := recordNodeRun(opts.DB, record.ID, currentNode, &exitCode, nodeRunStart, nodeRunEnd)
		recordScratchpadDiffs(opts.DB, nodeRunID, changes)

		exec.CurrentNode = currentNode
		updateExecution(opts.DB, exec)

		outEdges := edgeMap[currentNode]
		if len(outEdges) == 0 {
			exec.Status = StatusCompleted
			updateExecution(opts.DB, exec)
			sp.Remove()
			return exec, nil
		}

		nextNode, err := evaluateEdges(outEdges, record.Workspace, sp.Dir(), lastExitCode)
		if err != nil {
			exec.Status = StatusFailed
			exec.CurrentNode = currentNode
			updateExecution(opts.DB, exec)
			return exec, fmt.Errorf("no matching edge from node %q (exit code %d): %w", currentNode, lastExitCode, err)
		}

		currentNode = nextNode
	}
}

// ResumeOptions configures a workflow resume.
type ResumeOptions struct {
	DB             *sql.DB
	ScratchpadBase string
	Repo           string
}
