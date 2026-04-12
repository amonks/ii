package workflow

import (
	"database/sql"
	"time"
)

func insertExecution(db *sql.DB, exec *Execution, repo, workspace string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := db.Exec(`INSERT INTO workflow_executions
		(id, workflow_name, repo, status, current_node, workspace, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		exec.ID, exec.WorkflowName, repo, string(exec.Status), exec.CurrentNode, workspace, now, now)
	return err
}

func updateExecution(db *sql.DB, exec *Execution) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	completedAt := ""
	if exec.Status == StatusCompleted || exec.Status == StatusFailed || exec.Status == StatusInterrupted {
		completedAt = now
	}
	db.Exec(`UPDATE workflow_executions
		SET status = ?, current_node = ?, updated_at = ?, completed_at = ?
		WHERE id = ?`,
		string(exec.Status), exec.CurrentNode, now, completedAt, exec.ID)
}

func recordNodeRun(db *sql.DB, executionID, nodeName string, exitCode *int, startedAt, completedAt time.Time) int64 {
	result, err := db.Exec(`INSERT INTO workflow_node_runs
		(execution_id, node_name, exit_code, started_at, completed_at)
		VALUES (?, ?, ?, ?, ?)`,
		executionID, nodeName, exitCode,
		startedAt.UTC().Format(time.RFC3339Nano),
		completedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0
	}
	id, _ := result.LastInsertId()
	return id
}

func recordScratchpadDiffs(db *sql.DB, nodeRunID int64, changes []ScratchpadChange) {
	if nodeRunID == 0 {
		return
	}
	for _, c := range changes {
		var content *string
		if c.Op != OpDeleted {
			content = &c.Content
		}
		db.Exec(`INSERT INTO workflow_scratchpad_diffs
			(node_run_id, path, operation, content)
			VALUES (?, ?, ?, ?)`,
			nodeRunID, c.Path, string(c.Op), content)
	}
}
