// Command backfill-errlog migrates historical error reports from the errlog
// SQLite database into the logs database. Run on the logs fly machine via
// fly ssh after uploading both the binary and the errlog.db file.
//
// Usage:
//
//	/tmp/backfill-errlog -errlog-db /tmp/errlog.db -logs-db /data/logs.db
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

var (
	errlogDBPath = flag.String("errlog-db", "/tmp/errlog.db", "path to errlog SQLite database")
	logsDBPath   = flag.String("logs-db", "/data/logs.db", "path to logs SQLite database")
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	flag.Parse()

	errDB, err := sql.Open("sqlite", *errlogDBPath+"?mode=ro")
	if err != nil {
		return fmt.Errorf("opening errlog db: %w", err)
	}
	defer errDB.Close()

	logsDB, err := sql.Open("sqlite", *logsDBPath)
	if err != nil {
		return fmt.Errorf("opening logs db: %w", err)
	}
	defer logsDB.Close()

	rows, err := errDB.Query(`SELECT app, machine, status_code, happened_at, report FROM error_reports ORDER BY happened_at ASC`)
	if err != nil {
		return fmt.Errorf("querying error_reports: %w", err)
	}
	defer rows.Close()

	tx, err := logsDB.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	stmt, err := tx.Prepare("INSERT INTO events (timestamp, data) VALUES (?, ?)")
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	total := 0
	for rows.Next() {
		var (
			app        string
			machine    string
			statusCode int
			happenedAt time.Time
			report     string
		)
		if err := rows.Scan(&app, &machine, &statusCode, &happenedAt, &report); err != nil {
			tx.Rollback()
			return fmt.Errorf("scanning row: %w", err)
		}

		event := map[string]interface{}{
			"time":        happenedAt.Format(time.RFC3339Nano),
			"level":       "ERROR",
			"msg":         "error_report",
			"app.name":    app,
			"machine":     machine,
			"http.status": statusCode,
			"err.message": report,
		}

		raw, err := json.Marshal(event)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("marshaling event: %w", err)
		}

		if _, err := stmt.Exec(happenedAt, string(raw)); err != nil {
			tx.Rollback()
			return fmt.Errorf("inserting event: %w", err)
		}
		total++

		if total%500 == 0 {
			log.Printf("backfill-errlog: inserted %d events", total)
		}
	}
	if err := rows.Err(); err != nil {
		tx.Rollback()
		return fmt.Errorf("iterating rows: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	log.Printf("backfill-errlog: complete, %d total events migrated", total)
	return nil
}
