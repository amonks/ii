-- Workflow executions
CREATE TABLE workflow_executions (
    id            TEXT PRIMARY KEY,
    workflow_name TEXT NOT NULL,
    repo          TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'completed', 'failed', 'interrupted')),
    current_node  TEXT NOT NULL,
    workspace     TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    completed_at  TEXT NOT NULL DEFAULT ''
);

-- Node execution trace
CREATE TABLE workflow_node_runs (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    execution_id TEXT NOT NULL REFERENCES workflow_executions(id),
    node_name    TEXT NOT NULL,
    exit_code    INTEGER,
    started_at   TEXT NOT NULL,
    completed_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_workflow_node_runs_exec ON workflow_node_runs(execution_id);

-- Scratchpad diffs per node run
CREATE TABLE workflow_scratchpad_diffs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    node_run_id INTEGER NOT NULL REFERENCES workflow_node_runs(id),
    path        TEXT NOT NULL,
    operation   TEXT NOT NULL CHECK (operation IN ('added', 'modified', 'deleted')),
    content     TEXT
);
CREATE INDEX idx_workflow_scratchpad_diffs_run ON workflow_scratchpad_diffs(node_run_id);
