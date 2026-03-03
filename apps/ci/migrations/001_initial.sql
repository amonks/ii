CREATE TABLE IF NOT EXISTS runs (
    id          INTEGER PRIMARY KEY,
    head_sha    TEXT NOT NULL,
    base_sha    TEXT NOT NULL,
    machine_id  TEXT,
    started_at  TEXT NOT NULL,
    finished_at TEXT,
    status      TEXT NOT NULL DEFAULT 'running',
    trigger     TEXT NOT NULL DEFAULT 'webhook'
);

CREATE TABLE IF NOT EXISTS jobs (
    id          INTEGER PRIMARY KEY,
    run_id      INTEGER NOT NULL REFERENCES runs(id),
    kind        TEXT NOT NULL,
    name        TEXT NOT NULL,
    started_at  TEXT,
    finished_at TEXT,
    duration_ms INTEGER,
    status      TEXT NOT NULL DEFAULT 'pending',
    error       TEXT,
    output_path TEXT
);

CREATE TABLE IF NOT EXISTS deploy_jobs (
    job_id          INTEGER PRIMARY KEY REFERENCES jobs(id),
    app             TEXT NOT NULL,
    image_ref       TEXT NOT NULL,
    previous_image  TEXT,
    binary_bytes    INTEGER,
    image_bytes     INTEGER,
    compile_ms      INTEGER,
    push_ms         INTEGER,
    deploy_ms       INTEGER,
    packages_changed TEXT
);

CREATE TABLE IF NOT EXISTS terraform_jobs (
    job_id              INTEGER PRIMARY KEY REFERENCES jobs(id),
    resources_added     INTEGER,
    resources_changed   INTEGER,
    resources_destroyed INTEGER
);

CREATE TABLE IF NOT EXISTS deployments (
    id          INTEGER PRIMARY KEY,
    job_id      INTEGER REFERENCES jobs(id),
    app         TEXT NOT NULL,
    commit_sha  TEXT NOT NULL,
    image_ref   TEXT NOT NULL,
    binary_bytes INTEGER,
    deployed_at TEXT NOT NULL
);
