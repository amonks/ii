package workflow

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ExecutionRecord is a workflow execution read from the database.
type ExecutionRecord struct {
	ID           string
	WorkflowName string
	Repo         string
	Status       ExecutionStatus
	CurrentNode  string
	Workspace    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CompletedAt  time.Time
}

// NodeRunRecord is a node execution read from the database.
type NodeRunRecord struct {
	ID          int64
	NodeName    string
	ExitCode    *int
	StartedAt   time.Time
	CompletedAt time.Time
}

// DiffRecord is a scratchpad diff read from the database.
type DiffRecord struct {
	Path    string
	Op      ChangeOp
	Content string
}

// ListExecutions returns executions for a repo, most recent first.
func ListExecutions(db *sql.DB, repo string, all bool) ([]ExecutionRecord, error) {
	query := `SELECT id, workflow_name, repo, status, current_node, workspace,
		created_at, updated_at, completed_at
		FROM workflow_executions WHERE repo = ?`
	if !all {
		query += ` AND status = 'active'`
	}
	query += ` ORDER BY created_at DESC`

	rows, err := db.Query(query, repo)
	if err != nil {
		return nil, fmt.Errorf("list executions: %w", err)
	}
	defer rows.Close()

	var records []ExecutionRecord
	for rows.Next() {
		r, err := scanExecution(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// FindExecution finds an execution by ID or prefix.
func FindExecution(db *sql.DB, repo, idOrPrefix string) (ExecutionRecord, error) {
	// Try exact match.
	row := db.QueryRow(`SELECT id, workflow_name, repo, status, current_node, workspace,
		created_at, updated_at, completed_at
		FROM workflow_executions WHERE repo = ? AND id = ?`, repo, idOrPrefix)
	r, err := scanExecutionRow(row)
	if err == nil {
		return r, nil
	}

	// Try prefix match.
	rows, err := db.Query(`SELECT id, workflow_name, repo, status, current_node, workspace,
		created_at, updated_at, completed_at
		FROM workflow_executions WHERE repo = ? AND id LIKE ?
		ORDER BY id`, repo, idOrPrefix+"%")
	if err != nil {
		return ExecutionRecord{}, fmt.Errorf("find execution: %w", err)
	}
	defer rows.Close()

	var matches []ExecutionRecord
	for rows.Next() {
		r, err := scanExecution(rows)
		if err != nil {
			return ExecutionRecord{}, err
		}
		matches = append(matches, r)
	}
	if err := rows.Err(); err != nil {
		return ExecutionRecord{}, err
	}
	if len(matches) == 0 {
		return ExecutionRecord{}, fmt.Errorf("execution not found: %s", idOrPrefix)
	}
	if len(matches) > 1 {
		return ExecutionRecord{}, fmt.Errorf("ambiguous execution ID: %s matches %d executions", idOrPrefix, len(matches))
	}
	return matches[0], nil
}

// ListNodeRuns returns the node trace for an execution.
func ListNodeRuns(db *sql.DB, executionID string) ([]NodeRunRecord, error) {
	rows, err := db.Query(`SELECT id, node_name, exit_code, started_at, completed_at
		FROM workflow_node_runs WHERE execution_id = ?
		ORDER BY id`, executionID)
	if err != nil {
		return nil, fmt.Errorf("list node runs: %w", err)
	}
	defer rows.Close()

	var records []NodeRunRecord
	for rows.Next() {
		var r NodeRunRecord
		var exitCode sql.NullInt64
		var startedAt, completedAt string
		if err := rows.Scan(&r.ID, &r.NodeName, &exitCode, &startedAt, &completedAt); err != nil {
			return nil, fmt.Errorf("scan node run: %w", err)
		}
		if exitCode.Valid {
			code := int(exitCode.Int64)
			r.ExitCode = &code
		}
		r.StartedAt, _ = time.Parse(time.RFC3339Nano, startedAt)
		if completedAt != "" {
			r.CompletedAt, _ = time.Parse(time.RFC3339Nano, completedAt)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// ListDiffs returns scratchpad diffs for a node run.
func ListDiffs(db *sql.DB, nodeRunID int64) ([]DiffRecord, error) {
	rows, err := db.Query(`SELECT path, operation, content
		FROM workflow_scratchpad_diffs WHERE node_run_id = ?
		ORDER BY id`, nodeRunID)
	if err != nil {
		return nil, fmt.Errorf("list diffs: %w", err)
	}
	defer rows.Close()

	var records []DiffRecord
	for rows.Next() {
		var r DiffRecord
		var op string
		var content sql.NullString
		if err := rows.Scan(&r.Path, &op, &content); err != nil {
			return nil, fmt.Errorf("scan diff: %w", err)
		}
		r.Op = ChangeOp(op)
		if content.Valid {
			r.Content = content.String
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func scanExecution(rows *sql.Rows) (ExecutionRecord, error) {
	var r ExecutionRecord
	var status, createdAt, updatedAt, completedAt string
	if err := rows.Scan(&r.ID, &r.WorkflowName, &r.Repo, &status,
		&r.CurrentNode, &r.Workspace, &createdAt, &updatedAt, &completedAt); err != nil {
		return r, fmt.Errorf("scan execution: %w", err)
	}
	r.Status = ExecutionStatus(status)
	r.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	r.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	if strings.TrimSpace(completedAt) != "" {
		r.CompletedAt, _ = time.Parse(time.RFC3339Nano, completedAt)
	}
	return r, nil
}

func scanExecutionRow(row *sql.Row) (ExecutionRecord, error) {
	var r ExecutionRecord
	var status, createdAt, updatedAt, completedAt string
	if err := row.Scan(&r.ID, &r.WorkflowName, &r.Repo, &status,
		&r.CurrentNode, &r.Workspace, &createdAt, &updatedAt, &completedAt); err != nil {
		return r, err
	}
	r.Status = ExecutionStatus(status)
	r.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	r.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	if strings.TrimSpace(completedAt) != "" {
		r.CompletedAt, _ = time.Parse(time.RFC3339Nano, completedAt)
	}
	return r, nil
}
