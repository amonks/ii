DROP TABLE IF EXISTS deploy_jobs;
DROP TABLE IF EXISTS terraform_jobs;

CREATE TABLE streams (
    id          INTEGER PRIMARY KEY,
    job_id      INTEGER NOT NULL REFERENCES jobs(id),
    name        TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    started_at  TEXT,
    finished_at TEXT,
    duration_ms INTEGER,
    error       TEXT,
    UNIQUE(job_id, name)
);
