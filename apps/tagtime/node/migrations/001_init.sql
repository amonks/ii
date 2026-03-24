CREATE TABLE pings (
    timestamp  INTEGER PRIMARY KEY,  -- unix seconds, from schedule
    blurb      TEXT    NOT NULL DEFAULT '',
    node_id    TEXT    NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL DEFAULT 0,  -- unix nanos, LWW clock
    synced_at  INTEGER NOT NULL DEFAULT 0   -- unix nanos, last sync
);

CREATE TABLE period_changes (
    timestamp   INTEGER PRIMARY KEY, -- unix seconds, when this takes effect
    seed        INTEGER NOT NULL,
    period_secs INTEGER NOT NULL
);

CREATE VIRTUAL TABLE pings_fts USING fts5(
    blurb,
    content='pings',
    content_rowid='timestamp'
);

CREATE TRIGGER pings_ai AFTER INSERT ON pings BEGIN
    INSERT INTO pings_fts(rowid, blurb) VALUES (new.timestamp, new.blurb);
END;

CREATE TRIGGER pings_ad AFTER DELETE ON pings BEGIN
    INSERT INTO pings_fts(pings_fts, rowid, blurb) VALUES('delete', old.timestamp, old.blurb);
END;

CREATE TRIGGER pings_au AFTER UPDATE ON pings BEGIN
    INSERT INTO pings_fts(pings_fts, rowid, blurb) VALUES('delete', old.timestamp, old.blurb);
    INSERT INTO pings_fts(rowid, blurb) VALUES (new.timestamp, new.blurb);
END;

CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT);
