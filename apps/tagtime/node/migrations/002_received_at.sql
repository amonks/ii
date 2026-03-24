ALTER TABLE pings ADD COLUMN received_at INTEGER NOT NULL DEFAULT 0; -- unix nanos, server-assigned on push receipt
