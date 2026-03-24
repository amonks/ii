-- Repos (replaces the Repos map in state.json)
CREATE TABLE repos (
    name        TEXT PRIMARY KEY,
    source_path TEXT NOT NULL UNIQUE
);

-- Workspaces
CREATE TABLE workspaces (
    repo            TEXT NOT NULL REFERENCES repos(name),
    name            TEXT NOT NULL,
    path            TEXT NOT NULL,
    purpose         TEXT NOT NULL DEFAULT '',
    rev             TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'available'
        CHECK (status IN ('available', 'acquired')),
    acquired_by_pid INTEGER,
    provisioned     INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    acquired_at     TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (repo, name)
);

-- Agent sessions
CREATE TABLE agent_sessions (
    repo             TEXT NOT NULL REFERENCES repos(name),
    id               TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'completed', 'failed')),
    model            TEXT NOT NULL DEFAULT '',
    created_at       TEXT NOT NULL,
    started_at       TEXT NOT NULL DEFAULT '',
    updated_at       TEXT NOT NULL,
    completed_at     TEXT NOT NULL DEFAULT '',
    exit_code        INTEGER,
    duration_seconds INTEGER NOT NULL DEFAULT 0,
    tokens_used      INTEGER NOT NULL DEFAULT 0,
    cost             REAL NOT NULL DEFAULT 0,
    PRIMARY KEY (repo, id)
);

-- Jobs
CREATE TABLE jobs (
    repo                 TEXT NOT NULL REFERENCES repos(name),
    id                   TEXT NOT NULL,
    todo_id              TEXT NOT NULL,
    agent                TEXT NOT NULL DEFAULT '',
    implementation_model TEXT NOT NULL DEFAULT '',
    code_review_model    TEXT NOT NULL DEFAULT '',
    project_review_model TEXT NOT NULL DEFAULT '',
    stage                TEXT NOT NULL DEFAULT 'implementing'
        CHECK (stage IN ('implementing', 'testing', 'reviewing', 'committing')),
    status               TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'completed', 'failed', 'abandoned')),
    feedback             TEXT NOT NULL DEFAULT '',
    project_review_outcome          TEXT CHECK (project_review_outcome IN ('ACCEPT', 'REQUEST_CHANGES', 'ABANDON')),
    project_review_comments         TEXT NOT NULL DEFAULT '',
    project_review_agent_session_id TEXT NOT NULL DEFAULT '',
    project_review_reviewed_at      TEXT NOT NULL DEFAULT '',
    created_at           TEXT NOT NULL,
    started_at           TEXT NOT NULL DEFAULT '',
    updated_at           TEXT NOT NULL,
    completed_at         TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (repo, id)
);

-- Job-to-agent-session link (ordered list)
CREATE TABLE job_agent_sessions (
    repo       TEXT NOT NULL,
    job_id     TEXT NOT NULL,
    session_id TEXT NOT NULL,
    purpose    TEXT NOT NULL DEFAULT '',
    position   INTEGER NOT NULL,
    PRIMARY KEY (repo, job_id, session_id),
    FOREIGN KEY (repo, job_id) REFERENCES jobs(repo, id),
    FOREIGN KEY (repo, session_id) REFERENCES agent_sessions(repo, id)
);

-- Job changes (one per jj change)
CREATE TABLE job_changes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    repo       TEXT NOT NULL,
    job_id     TEXT NOT NULL,
    change_id  TEXT NOT NULL,
    created_at TEXT NOT NULL,
    position   INTEGER NOT NULL,
    FOREIGN KEY (repo, job_id) REFERENCES jobs(repo, id)
);
CREATE INDEX idx_job_changes_job ON job_changes(repo, job_id);

-- Job commits (one per iteration within a change)
CREATE TABLE job_commits (
    id                        INTEGER PRIMARY KEY AUTOINCREMENT,
    job_change_id             INTEGER NOT NULL REFERENCES job_changes(id),
    commit_id                 TEXT NOT NULL,
    draft_message             TEXT NOT NULL DEFAULT '',
    tests_passed              INTEGER,  -- NULL = unknown, 0 = failed, 1 = passed
    agent_session_id          TEXT NOT NULL DEFAULT '',
    review_outcome            TEXT CHECK (review_outcome IN ('ACCEPT', 'REQUEST_CHANGES', 'ABANDON')),
    review_comments           TEXT NOT NULL DEFAULT '',
    review_agent_session_id   TEXT NOT NULL DEFAULT '',
    review_reviewed_at        TEXT NOT NULL DEFAULT '',
    created_at                TEXT NOT NULL,
    position                  INTEGER NOT NULL
);
CREATE INDEX idx_job_commits_change ON job_commits(job_change_id);
