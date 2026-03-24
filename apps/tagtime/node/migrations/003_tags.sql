CREATE TABLE tags (
    name TEXT PRIMARY KEY
);

CREATE TABLE ping_tags (
    ping_timestamp INTEGER NOT NULL REFERENCES pings(timestamp),
    tag_name       TEXT    NOT NULL,
    PRIMARY KEY (ping_timestamp, tag_name)
);

CREATE INDEX idx_ping_tags_tag ON ping_tags(tag_name);

CREATE TABLE tag_renames (
    old_name   TEXT    NOT NULL,
    new_name   TEXT    NOT NULL,
    renamed_at INTEGER NOT NULL,  -- unix seconds
    node_id    TEXT    NOT NULL,
    PRIMARY KEY (old_name, renamed_at)
);
