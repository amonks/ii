CREATE VIRTUAL TABLE points_idx USING rtree(
    id,
    min_lat, max_lat,
    min_lon, max_lon,
    min_sig, max_sig
);

CREATE TABLE points (
    id INTEGER PRIMARY KEY,
    timestamp INTEGER NOT NULL UNIQUE,
    subscribed INTEGER NOT NULL DEFAULT 0,
    touched_at INTEGER NOT NULL,
    lat REAL NOT NULL, lon REAL NOT NULL,
    alt REAL, ellipsoidal_alt REAL,
    h_accuracy REAL, v_accuracy REAL,
    speed REAL, speed_accuracy REAL,
    course REAL, course_accuracy REAL,
    floor INTEGER,
    is_simulated INTEGER,
    is_from_accessory INTEGER
);

CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT);
