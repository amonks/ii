CREATE TABLE IF NOT EXISTS events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp DATETIME NOT NULL,
	data TEXT NOT NULL,

	app           TEXT GENERATED ALWAYS AS (json_extract(data, '$."app.name"')) STORED,
	level         TEXT GENERATED ALWAYS AS (json_extract(data, '$."level"')) STORED,
	msg           TEXT GENERATED ALWAYS AS (json_extract(data, '$."msg"')) STORED,
	request_id    TEXT GENERATED ALWAYS AS (json_extract(data, '$."req.id"')) STORED,
	method        TEXT GENERATED ALWAYS AS (json_extract(data, '$."http.method"')) STORED,
	host          TEXT GENERATED ALWAYS AS (json_extract(data, '$."http.host"')) STORED,
	path          TEXT GENERATED ALWAYS AS (json_extract(data, '$."http.path"')) STORED,
	route         TEXT GENERATED ALWAYS AS (json_extract(data, '$."http.route"')) STORED,
	status        INTEGER GENERATED ALWAYS AS (json_extract(data, '$."http.status"')) STORED,
	duration_ms   REAL GENERATED ALWAYS AS (json_extract(data, '$."http.duration_ms"')) STORED,
	remote_addr   TEXT GENERATED ALWAYS AS (json_extract(data, '$."http.remote_addr"')) STORED,
	proxy_upstream TEXT GENERATED ALWAYS AS (json_extract(data, '$."proxy.upstream"')) STORED,

	duration_bucket INTEGER GENERATED ALWAYS AS (
		CASE
			WHEN json_extract(data, '$."http.duration_ms"') IS NULL THEN NULL
			WHEN json_extract(data, '$."http.duration_ms"') <=    1 THEN 1
			WHEN json_extract(data, '$."http.duration_ms"') <=    2 THEN 2
			WHEN json_extract(data, '$."http.duration_ms"') <=    5 THEN 5
			WHEN json_extract(data, '$."http.duration_ms"') <=   10 THEN 10
			WHEN json_extract(data, '$."http.duration_ms"') <=   25 THEN 25
			WHEN json_extract(data, '$."http.duration_ms"') <=   50 THEN 50
			WHEN json_extract(data, '$."http.duration_ms"') <=  100 THEN 100
			WHEN json_extract(data, '$."http.duration_ms"') <=  250 THEN 250
			WHEN json_extract(data, '$."http.duration_ms"') <=  500 THEN 500
			WHEN json_extract(data, '$."http.duration_ms"') <= 1000 THEN 1000
			WHEN json_extract(data, '$."http.duration_ms"') <= 2500 THEN 2500
			WHEN json_extract(data, '$."http.duration_ms"') <= 5000 THEN 5000
			ELSE 10000
		END
	) STORED
);

CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_request_id ON events(request_id) WHERE request_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_events_app_timestamp ON events(app, timestamp);
CREATE INDEX IF NOT EXISTS idx_events_msg ON events(msg);
CREATE INDEX IF NOT EXISTS idx_events_msg_timestamp ON events(msg, timestamp DESC);

CREATE TABLE IF NOT EXISTS daily_stats (
	day DATETIME NOT NULL,
	app TEXT NOT NULL DEFAULT '',
	host TEXT NOT NULL DEFAULT '',
	method TEXT DEFAULT 'unknown',
	status INTEGER NOT NULL DEFAULT 0,
	duration_bucket INTEGER NOT NULL DEFAULT 0,
	count INTEGER DEFAULT 0,
	PRIMARY KEY (day, app, host, method, status, duration_bucket)
);

CREATE TABLE IF NOT EXISTS page_daily (
	day DATETIME NOT NULL,
	host TEXT NOT NULL,
	path TEXT NOT NULL,
	count INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (day, host, path)
);

DROP INDEX IF EXISTS idx_page_daily_host_path_day;
CREATE INDEX IF NOT EXISTS idx_page_daily_day_count ON page_daily(day, host, path, count);
CREATE INDEX IF NOT EXISTS idx_daily_stats_day_status ON daily_stats(day, status, count);
CREATE INDEX IF NOT EXISTS idx_daily_stats_day_duration ON daily_stats(day, duration_bucket, count);
