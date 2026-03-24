package node

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"monks.co/pkg/migrate"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Ping represents a single time sample.
type Ping struct {
	Timestamp  int64  `json:"timestamp"` // unix seconds
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

	s := &Store{db: db}
	if err := s.backfillPingTags(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("backfilling ping_tags: %w", err)
	}
	return s, nil
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
// Also maintains ping_tags and tags tables.
func (s *Store) SetBlurb(ctx context.Context, timestamp int64, blurb, nodeID string) error {
	now := time.Now().UnixNano()
	if err := s.UpsertPing(ctx, Ping{
		Timestamp:  timestamp,
		Blurb:      blurb,
		NodeID:     nodeID,
		UpdatedAt:  now,
		ReceivedAt: now,
	}); err != nil {
		return err
	}
	return s.ensureTagsFromBlurb(ctx, timestamp, blurb)
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
	if err := tx.Commit(); err != nil {
		return err
	}
	// Maintain ping_tags outside the transaction (UpsertPing already committed).
	for _, ts := range timestamps {
		if err := s.ensureTagsFromBlurb(ctx, ts, blurb); err != nil {
			return err
		}
	}
	return nil
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

// UnsyncedPingCount returns the number of pings that haven't been synced yet.
func (s *Store) UnsyncedPingCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pings WHERE synced_at = 0 AND updated_at > 0`,
	).Scan(&count)
	return count, err
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

	changes := []PeriodChange{}
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

// PingTagsInTimeRange returns a map of ping_timestamp → tag names for the given range.
func (s *Store) PingTagsInTimeRange(ctx context.Context, start, end time.Time) (map[int64][]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT ping_timestamp, tag_name FROM ping_tags
		 WHERE ping_timestamp >= ? AND ping_timestamp < ?
		 ORDER BY ping_timestamp, tag_name`,
		start.Unix(), end.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int64][]string)
	for rows.Next() {
		var ts int64
		var tag string
		if err := rows.Scan(&ts, &tag); err != nil {
			return nil, err
		}
		result[ts] = append(result[ts], tag)
	}
	return result, rows.Err()
}

// backfillPingTags populates ping_tags and tags from existing blurbs.
// It's idempotent: rows that already exist are skipped.
func (s *Store) backfillPingTags(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx,
		`SELECT timestamp, blurb FROM pings WHERE blurb != ''`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type pingBlurb struct {
		ts    int64
		blurb string
	}
	var pbs []pingBlurb
	for rows.Next() {
		var pb pingBlurb
		if err := rows.Scan(&pb.ts, &pb.blurb); err != nil {
			return err
		}
		pbs = append(pbs, pb)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, pb := range pbs {
		if err := s.ensureTagsFromBlurb(ctx, pb.ts, pb.blurb); err != nil {
			return err
		}
	}
	return nil
}

// ensureTagsFromBlurb extracts tags from a blurb and inserts them into
// ping_tags and tags. Idempotent via INSERT OR IGNORE.
func (s *Store) ensureTagsFromBlurb(ctx context.Context, timestamp int64, blurb string) error {
	tags := ExtractTags(blurb)
	for _, tag := range tags {
		if _, err := s.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO tags (name) VALUES (?)`, tag); err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO ping_tags (ping_timestamp, tag_name) VALUES (?, ?)`,
			timestamp, tag); err != nil {
			return err
		}
	}
	// Remove stale ping_tags rows that no longer match the blurb.
	if len(tags) == 0 {
		_, err := s.db.ExecContext(ctx,
			`DELETE FROM ping_tags WHERE ping_timestamp = ?`, timestamp)
		return err
	}
	args := []any{timestamp}
	var placeholders strings.Builder
	for i, tag := range tags {
		if i > 0 {
			placeholders.WriteString(", ")
		}
		placeholders.WriteString("?")
		args = append(args, tag)
	}
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM ping_tags WHERE ping_timestamp = ? AND tag_name NOT IN (`+placeholders.String()+`)`,
		args...)
	return err
}

// ListTags returns all known tag names, sorted alphabetically.
func (s *Store) ListTags(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name FROM tags ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tags = append(tags, name)
	}
	return tags, rows.Err()
}

// TagsForPing returns the tag names associated with a ping.
func (s *Store) TagsForPing(ctx context.Context, timestamp int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT tag_name FROM ping_tags WHERE ping_timestamp = ? ORDER BY tag_name`,
		timestamp)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tags = append(tags, name)
	}
	return tags, rows.Err()
}

// RenameTag records a time-scoped tag rename and applies it to ping_tags.
// Only pings at or before the current time are affected.
func (s *Store) RenameTag(ctx context.Context, oldName, newName, nodeID string) error {
	now := time.Now().Unix()
	rename := TagRename{
		OldName:   oldName,
		NewName:   newName,
		RenamedAt: now,
		NodeID:    nodeID,
	}
	if err := s.AddTagRename(ctx, rename); err != nil {
		return err
	}
	return s.ApplyTagRename(ctx, rename)
}

// AddTagRename inserts a rename record. Idempotent by primary key.
func (s *Store) AddTagRename(ctx context.Context, r TagRename) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO tag_renames (old_name, new_name, renamed_at, node_id)
		 VALUES (?, ?, ?, ?)`,
		r.OldName, r.NewName, r.RenamedAt, r.NodeID)
	return err
}

// ApplyTagRename updates ping_tags for a time-scoped rename.
// Pings with timestamp <= renamed_at have their tag updated.
// Also ensures the new tag exists in the tags table.
func (s *Store) ApplyTagRename(ctx context.Context, r TagRename) error {
	// Remove old_name entries where new_name already exists for the same ping
	// (can happen when ensureTagsFromBlurb re-derives tags after a rename).
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM ping_tags WHERE tag_name = ? AND ping_timestamp <= ?
		 AND ping_timestamp IN (SELECT ping_timestamp FROM ping_tags WHERE tag_name = ? AND ping_timestamp <= ?)`,
		r.OldName, r.RenamedAt, r.NewName, r.RenamedAt); err != nil {
		return err
	}
	// Rename remaining old_name entries.
	if _, err := s.db.ExecContext(ctx,
		`UPDATE ping_tags SET tag_name = ? WHERE tag_name = ? AND ping_timestamp <= ?`,
		r.NewName, r.OldName, r.RenamedAt); err != nil {
		return err
	}
	// Ensure new tag name exists.
	if _, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO tags (name) VALUES (?)`, r.NewName); err != nil {
		return err
	}
	// Remove old tag if no ping_tags reference it.
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM tags WHERE name = ? AND NOT EXISTS (
			SELECT 1 FROM ping_tags WHERE tag_name = ?
		)`, r.OldName, r.OldName)
	return err
}

// PingsByTagInTimeRange returns pings that have the given tag in [start, end), ordered by timestamp desc.
func (s *Store) PingsByTagInTimeRange(ctx context.Context, tagName string, start, end time.Time) ([]Ping, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT p.timestamp, p.blurb, p.node_id, p.updated_at, p.synced_at, p.received_at
		 FROM pings p
		 JOIN ping_tags pt ON pt.ping_timestamp = p.timestamp
		 WHERE pt.tag_name = ? AND p.timestamp >= ? AND p.timestamp < ?
		 ORDER BY p.timestamp DESC`,
		tagName, start.Unix(), end.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPings(rows)
}

// TagRenamesForTag returns renames where old_name or new_name matches tagName, ordered by renamed_at.
func (s *Store) TagRenamesForTag(ctx context.Context, tagName string) ([]TagRename, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT old_name, new_name, renamed_at, node_id FROM tag_renames
		 WHERE old_name = ? OR new_name = ?
		 ORDER BY renamed_at`,
		tagName, tagName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var renames []TagRename
	for rows.Next() {
		var r TagRename
		if err := rows.Scan(&r.OldName, &r.NewName, &r.RenamedAt, &r.NodeID); err != nil {
			return nil, err
		}
		renames = append(renames, r)
	}
	return renames, rows.Err()
}

// ListTagRenames returns all tag renames, ordered by renamed_at.
func (s *Store) ListTagRenames(ctx context.Context) ([]TagRename, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT old_name, new_name, renamed_at, node_id FROM tag_renames ORDER BY renamed_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var renames []TagRename
	for rows.Next() {
		var r TagRename
		if err := rows.Scan(&r.OldName, &r.NewName, &r.RenamedAt, &r.NodeID); err != nil {
			return nil, err
		}
		renames = append(renames, r)
	}
	return renames, rows.Err()
}

// ApplyAllRenamesForPing applies all relevant renames to a ping's tags.
// This is used after syncing a ping to ensure its tags reflect all renames
// that happened while the ping existed.
func (s *Store) ApplyAllRenamesForPing(ctx context.Context, pingTimestamp int64) error {
	renames, err := s.ListTagRenames(ctx)
	if err != nil {
		return err
	}
	for _, r := range renames {
		if r.RenamedAt >= pingTimestamp {
			// This rename applies to this ping (rename happened after ping was created).
			// Delete old_name if new_name already exists for this ping.
			if _, err := s.db.ExecContext(ctx,
				`DELETE FROM ping_tags WHERE tag_name = ? AND ping_timestamp = ?
				 AND EXISTS (SELECT 1 FROM ping_tags WHERE tag_name = ? AND ping_timestamp = ?)`,
				r.OldName, pingTimestamp, r.NewName, pingTimestamp); err != nil {
				return err
			}
			// Rename remaining.
			if _, err := s.db.ExecContext(ctx,
				`UPDATE ping_tags SET tag_name = ? WHERE tag_name = ? AND ping_timestamp = ?`,
				r.NewName, r.OldName, pingTimestamp); err != nil {
				return err
			}
		}
	}
	return nil
}

func scanPings(rows *sql.Rows) ([]Ping, error) {
	pings := []Ping{}
	for rows.Next() {
		var p Ping
		if err := rows.Scan(&p.Timestamp, &p.Blurb, &p.NodeID, &p.UpdatedAt, &p.SyncedAt, &p.ReceivedAt); err != nil {
			return nil, err
		}
		pings = append(pings, p)
	}
	return pings, rows.Err()
}
