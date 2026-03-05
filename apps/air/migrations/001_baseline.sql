CREATE TABLE IF NOT EXISTS data_points (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at DATETIME,
	room TEXT,
	device TEXT,
	parameter TEXT,
	value REAL
);

CREATE TABLE IF NOT EXISTS window_aggregates (
	room TEXT,
	device TEXT,
	parameter TEXT,
	window_duration INTEGER,
	window_start_at DATETIME,
	min REAL,
	max REAL,
	mean REAL,
	count INTEGER,
	PRIMARY KEY (room, device, parameter, window_duration, window_start_at)
);
