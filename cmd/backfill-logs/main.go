package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"monks.co/pkg/database"
	"monks.co/pkg/logs"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	const trafficPath = "/data/traffic.db"
	const logsPath = "/data/logs.db"

	// Open traffic DB read-only.
	trafficDB, err := database.Open(trafficPath)
	if err != nil {
		return fmt.Errorf("opening traffic db: %w", err)
	}
	defer trafficDB.Close()

	// Open/create logs DB (runs migrations to create tables+indexes).
	logsDB, err := logs.OpenPath(logsPath)
	if err != nil {
		return fmt.Errorf("opening logs db: %w", err)
	}
	defer logsDB.Close()

	// Get underlying *sql.DB for raw performance.
	gormDB := logsDB.DB.DB
	sqlDB, err := gormDB.DB()
	if err != nil {
		return fmt.Errorf("getting sql.DB: %w", err)
	}

	// Speed up bulk import.
	sqlDB.Exec("PRAGMA synchronous = OFF")
	sqlDB.Exec("PRAGMA journal_mode = WAL")
	sqlDB.Exec("PRAGMA cache_size = -512000") // 512MB cache

	// Drop indexes for faster bulk insert; we'll recreate after.
	sqlDB.Exec("DROP INDEX IF EXISTS idx_events_timestamp")
	sqlDB.Exec("DROP INDEX IF EXISTS idx_events_request_id")
	sqlDB.Exec("DROP INDEX IF EXISTS idx_events_app_timestamp")

	// Read all requests from traffic.
	type TrafficRequest struct {
		ID         uint
		CreatedAt  *time.Time
		Host       *string
		Path       *string
		Query      *string
		Method     *string
		RemoteAddr *string
		UserAgent  *string
		Referer    *string
		StatusCode *int
		Duration   *int64 // nanoseconds
	}

	rows, err := trafficDB.Raw(`SELECT id, created_at, host, path, query, method, remote_addr, user_agent, referer, status_code, duration FROM requests ORDER BY created_at ASC`).Rows()
	if err != nil {
		return fmt.Errorf("reading traffic requests: %w", err)
	}
	defer rows.Close()

	total := 0

	deref := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}

	// Process in batches with raw prepared statements.
	const batchSize = 50000

	type event struct {
		ts   time.Time
		data string
	}
	batch := make([]event, 0, batchSize)

	flushBatch := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := sqlDB.Begin()
		if err != nil {
			return err
		}
		stmt, err := tx.Prepare("INSERT INTO events (timestamp, data) VALUES (?, ?)")
		if err != nil {
			tx.Rollback()
			return err
		}
		for _, e := range batch {
			if _, err := stmt.Exec(e.ts, e.data); err != nil {
				stmt.Close()
				tx.Rollback()
				return err
			}
		}
		stmt.Close()
		if err := tx.Commit(); err != nil {
			return err
		}
		total += len(batch)
		log.Printf("backfill: inserted %d events", total)
		batch = batch[:0]
		return nil
	}

	for rows.Next() {
		var r TrafficRequest
		if err := rows.Scan(&r.ID, &r.CreatedAt, &r.Host, &r.Path, &r.Query, &r.Method, &r.RemoteAddr, &r.UserAgent, &r.Referer, &r.StatusCode, &r.Duration); err != nil {
			return fmt.Errorf("scanning row: %w", err)
		}

		ts := time.Now()
		if r.CreatedAt != nil {
			ts = *r.CreatedAt
		}
		var durationMs float64
		if r.Duration != nil {
			durationMs = float64(*r.Duration) / 1e6
		}
		statusCode := 0
		if r.StatusCode != nil {
			statusCode = *r.StatusCode
		}

		m := map[string]interface{}{
			"time":             ts.Format(time.RFC3339Nano),
			"level":            "INFO",
			"msg":              "request",
			"app.name":         "proxy",
			"req.id":           "",
			"http.method":      deref(r.Method),
			"http.host":        deref(r.Host),
			"http.path":        deref(r.Path),
			"http.status":      statusCode,
			"http.duration_ms": durationMs,
			"http.remote_addr": deref(r.RemoteAddr),
		}
		if ua := deref(r.UserAgent); ua != "" {
			m["http.user_agent"] = ua
		}
		if ref := deref(r.Referer); ref != "" {
			m["http.referer"] = ref
		}
		if q := deref(r.Query); q != "" {
			m["http.query"] = q
		}

		raw, err := json.Marshal(m)
		if err != nil {
			return fmt.Errorf("marshaling event: %w", err)
		}
		batch = append(batch, event{ts: ts, data: string(raw)})

		if len(batch) >= batchSize {
			if err := flushBatch(); err != nil {
				return fmt.Errorf("flushing batch: %w", err)
			}
		}
	}

	if err := flushBatch(); err != nil {
		return fmt.Errorf("flushing final batch: %w", err)
	}
	log.Printf("backfill: all %d events inserted, rebuilding indexes...", total)

	// Recreate indexes.
	for _, q := range []string{
		"CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_events_request_id ON events(request_id) WHERE request_id IS NOT NULL",
		"CREATE INDEX IF NOT EXISTS idx_events_app_timestamp ON events(app, timestamp)",
	} {
		log.Printf("backfill: creating index: %s", q[:60])
		if _, err := sqlDB.Exec(q); err != nil {
			return fmt.Errorf("creating index: %w", err)
		}
	}
	log.Printf("backfill: indexes rebuilt, populating aggregation tables...")

	// Populate daily_stats from events in one query.
	if _, err := sqlDB.Exec(`
		INSERT INTO daily_stats (day, app, host, method, status, duration_bucket, count)
		SELECT
			date(timestamp) as day,
			COALESCE(app, '') as app,
			COALESCE(LOWER(host), '') as host,
			COALESCE(method, 'unknown') as method,
			COALESCE(status, 0) as status,
			COALESCE(duration_bucket, 0) as duration_bucket,
			COUNT(*) as count
		FROM events
		WHERE msg = 'request'
		GROUP BY date(timestamp), app, LOWER(host), method, status, duration_bucket
		ON CONFLICT (day, app, host, method, status, duration_bucket)
		DO UPDATE SET count = count + excluded.count
	`); err != nil {
		return fmt.Errorf("populating daily_stats: %w", err)
	}
	log.Printf("backfill: daily_stats populated")

	// Populate page_daily from events in one query.
	if _, err := sqlDB.Exec(`
		INSERT INTO page_daily (day, host, path, count)
		SELECT
			date(timestamp) as day,
			COALESCE(LOWER(host), '') as host,
			path,
			COUNT(*) as count
		FROM events
		WHERE msg = 'request' AND path IS NOT NULL AND path != ''
		GROUP BY date(timestamp), LOWER(host), path
		ON CONFLICT (day, host, path)
		DO UPDATE SET count = count + excluded.count
	`); err != nil {
		return fmt.Errorf("populating page_daily: %w", err)
	}
	log.Printf("backfill: page_daily populated")

	// Restore safe pragmas.
	sqlDB.Exec("PRAGMA synchronous = NORMAL")

	log.Printf("backfill: complete, %d total events", total)
	return nil
}

