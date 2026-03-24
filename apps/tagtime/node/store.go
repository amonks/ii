package node

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"monks.co/pkg/migrate"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Ping represents a single time sample.
type Ping struct {
	Timestamp  int64  `json:"timestamp"`   // unix seconds
	Blurb      string `json:"blurb"`
	NodeID     string `json:"node_id"`
	UpdatedAt  int64  `json:"updated_at"`  // unix nanos, LWW clock
	SyncedAt   int64  `json:"synced_at"`   // unix nanos
	ReceivedAt int64  `json:"received_at"` // unix nanos, server-assigned on push receipt
}

// Store manages ping storage in SQLite with FTS5 search.
type Store struct {
	db *sql.DB
}

// OpenStore opens (or creates) a SQLite database at dbPath and runs migrations.
func OpenStore(ctx context.Context, dbPath string) (*Store, error) {
	dsn := dbPath + "?_journal_mode=WAL&_busy_timeout=5000"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening db %s: %w", dbPath, err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.ExecContext(ctx, "PRAGMA synchronous = FULL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting synchronous mode: %w", err)
	}

	if err := migrate.Run(ctx, migrate.Config{
		DB:  db,
		FS:  migrationsFS,
		Dir: "migrations",
	}); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// UpsertPing stores a ping using LWW semantics: only writes if incoming
// updated_at > existing updated_at.
func (s *Store) UpsertPing(ctx context.Context, p Ping) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO pings (timestamp, blurb, node_id, updated_at, synced_at, received_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(timestamp) DO UPDATE SET
		   blurb = excluded.blurb,
		   node_id = excluded.node_id,
		   updated_at = excluded.updated_at,
		   synced_at = excluded.synced_at,
		   received_at = excluded.received_at
		 WHERE excluded.updated_at > pings.updated_at`,
		p.Timestamp, p.Blurb, p.NodeID, p.UpdatedAt, p.SyncedAt, p.ReceivedAt,
	)
	return err
}

// GetPing returns a single ping by timestamp.
func (s *Store) GetPing(ctx context.Context, timestamp int64) (*Ping, error) {
	var p Ping
	err := s.db.QueryRowContext(ctx,
		`SELECT timestamp, blurb, node_id, updated_at, synced_at, received_at FROM pings WHERE timestamp = ?`,
		timestamp,
	).Scan(&p.Timestamp, &p.Blurb, &p.NodeID, &p.UpdatedAt, &p.SyncedAt, &p.ReceivedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// PendingPings returns scheduled pings up to now that have no blurb set,
// most recent first.
func (s *Store) PendingPings(ctx context.Context, now time.Time) ([]Ping, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT timestamp, blurb, node_id, updated_at, synced_at, received_at
		 FROM pings WHERE blurb = '' AND timestamp <= ?
		 ORDER BY timestamp DESC`,
		now.Unix(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPings(rows)
}

// RecentPings returns the most recent answered pings.
func (s *Store) RecentPings(ctx context.Context, limit int) ([]Ping, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT timestamp, blurb, node_id, updated_at, synced_at, received_at
		 FROM pings WHERE blurb != ''
		 ORDER BY timestamp DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPings(rows)
}

// SetBlurb sets the blurb for a ping, updating the LWW clock.
func (s *Store) SetBlurb(ctx context.Context, timestamp int64, blurb, nodeID string) error {
	now := time.Now().UnixNano()
	return s.UpsertPing(ctx, Ping{
		Timestamp:  timestamp,
		Blurb:      blurb,
		NodeID:     nodeID,
		UpdatedAt:  now,
		ReceivedAt: now,
	})
}

// BatchSetBlurb sets the blurb for multiple pings at once.
func (s *Store) BatchSetBlurb(ctx context.Context, timestamps []int64, blurb, nodeID string) error {
	now := time.Now().UnixNano()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO pings (timestamp, blurb, node_id, updated_at, synced_at, received_at)
		 VALUES (?, ?, ?, ?, 0, ?)
		 ON CONFLICT(timestamp) DO UPDATE SET
		   blurb = excluded.blurb,
		   node_id = excluded.node_id,
		   updated_at = excluded.updated_at,
		   received_at = excluded.received_at
		 WHERE excluded.updated_at > pings.updated_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, ts := range timestamps {
		if _, err := stmt.ExecContext(ctx, ts, blurb, nodeID, now, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SearchBlurbs performs a full-text search on ping blurbs.
func (s *Store) SearchBlurbs(ctx context.Context, query string, limit int) ([]Ping, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT p.timestamp, p.blurb, p.node_id, p.updated_at, p.synced_at, p.received_at
		 FROM pings_fts f
		 JOIN pings p ON f.rowid = p.timestamp
		 WHERE pings_fts MATCH ?
		 ORDER BY rank
		 LIMIT ?`,
		query, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPings(rows)
}

// PingsInTimeRange returns all pings with timestamps in [start, end).
func (s *Store) PingsInTimeRange(ctx context.Context, start, end time.Time) ([]Ping, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT timestamp, blurb, node_id, updated_at, synced_at, received_at
		 FROM pings WHERE timestamp >= ? AND timestamp < ?
		 ORDER BY timestamp`,
		start.Unix(), end.Unix(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPings(rows)
}

// PingsUpdatedAfter returns pings with updated_at > since, for sync pull.
func (s *Store) PingsUpdatedAfter(ctx context.Context, since int64, limit int) ([]Ping, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT timestamp, blurb, node_id, updated_at, synced_at, received_at
		 FROM pings WHERE updated_at > ?
		 ORDER BY updated_at LIMIT ?`,
		since, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPings(rows)
}

// PingsReceivedAfter returns pings with received_at > since, for sync pull.
func (s *Store) PingsReceivedAfter(ctx context.Context, since int64, limit int) ([]Ping, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT timestamp, blurb, node_id, updated_at, synced_at, received_at
		 FROM pings WHERE received_at > ?
		 ORDER BY received_at LIMIT ?`,
		since, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPings(rows)
}

// UnsyncedPings returns pings that haven't been synced yet (synced_at = 0 and updated_at > 0).
func (s *Store) UnsyncedPings(ctx context.Context, limit int) ([]Ping, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT timestamp, blurb, node_id, updated_at, synced_at, received_at
		 FROM pings WHERE synced_at = 0 AND updated_at > 0
		 ORDER BY updated_at LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPings(rows)
}

// MarkSynced sets synced_at for pings up to the given updated_at.
func (s *Store) MarkSynced(ctx context.Context, upToUpdatedAt int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE pings SET synced_at = ? WHERE synced_at = 0 AND updated_at > 0 AND updated_at <= ?`,
		time.Now().UnixNano(), upToUpdatedAt,
	)
	return err
}

// ListPeriodChanges returns all period changes ordered by timestamp.
func (s *Store) ListPeriodChanges(ctx context.Context) ([]PeriodChange, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT timestamp, seed, period_secs FROM period_changes ORDER BY timestamp`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var changes []PeriodChange
	for rows.Next() {
		var c PeriodChange
		if err := rows.Scan(&c.Timestamp, &c.Seed, &c.PeriodSecs); err != nil {
			return nil, err
		}
		changes = append(changes, c)
	}
	return changes, rows.Err()
}

// AddPeriodChange inserts a period change event. Idempotent by timestamp.
func (s *Store) AddPeriodChange(ctx context.Context, c PeriodChange) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO period_changes (timestamp, seed, period_secs)
		 VALUES (?, ?, ?)
		 ON CONFLICT(timestamp) DO UPDATE SET
		   seed = excluded.seed,
		   period_secs = excluded.period_secs`,
		c.Timestamp, c.Seed, c.PeriodSecs,
	)
	return err
}

// EnsurePingsExist creates empty ping rows for scheduled timestamps that
// don't already exist in the database.
func (s *Store) EnsurePingsExist(ctx context.Context, timestamps []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT OR IGNORE INTO pings (timestamp) VALUES (?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, ts := range timestamps {
		if _, err := stmt.ExecContext(ctx, ts); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetMeta reads a value from the meta table.
func (s *Store) GetMeta(ctx context.Context, key string) (string, error) {
	var val sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM meta WHERE key = ?`, key,
	).Scan(&val)
	if err == sql.ErrNoRows || !val.Valid {
		return "", nil
	}
	return val.String, err
}

// SetMeta writes a value to the meta table.
func (s *Store) SetMeta(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO meta (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}

func scanPings(rows *sql.Rows) ([]Ping, error) {
	var pings []Ping
	for rows.Next() {
		var p Ping
		if err := rows.Scan(&p.Timestamp, &p.Blurb, &p.NodeID, &p.UpdatedAt, &p.SyncedAt, &p.ReceivedAt); err != nil {
			return nil, err
		}
		pings = append(pings, p)
	}
	return pings, rows.Err()
}
