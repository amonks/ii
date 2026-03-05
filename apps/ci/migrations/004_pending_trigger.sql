-- Pending trigger: LWW register for the most-recently-requested SHA.
-- At most one row exists at any time.
CREATE TABLE IF NOT EXISTS pending_trigger (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    sha TEXT NOT NULL,
    created_at TEXT NOT NULL
);
