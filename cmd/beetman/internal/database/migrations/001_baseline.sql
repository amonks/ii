CREATE TABLE IF NOT EXISTS albums (
    directory_name TEXT PRIMARY KEY,
    discovery_time TEXT,
    mtime         TEXT,
    import_time   TEXT NULL,
    status        TEXT,
    failure_count INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_mtime ON albums(mtime);
