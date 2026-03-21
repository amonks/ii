package node

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	pb "monks.co/apps/breadcrumbs/proto"
	"monks.co/pkg/migrate"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store manages point storage in SQLite with an R*tree spatial index.
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

// InsertPoint stores a point with its significance. If a point with the same
// timestamp already exists, it is updated (upsert).
func (s *Store) InsertPoint(ctx context.Context, p *pb.Point, significance float64, subscribed bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UnixNano()
	sub := 0
	if subscribed {
		sub = 1
	}

	// Check if a point with this timestamp already exists.
	var existingID int64
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM points WHERE timestamp = ?`, p.Timestamp,
	).Scan(&existingID)

	if err == sql.ErrNoRows {
		// Insert new point.
		res, err := tx.ExecContext(ctx,
			`INSERT INTO points (timestamp, subscribed, touched_at, lat, lon, alt, ellipsoidal_alt,
				h_accuracy, v_accuracy, speed, speed_accuracy, course, course_accuracy,
				floor, is_simulated, is_from_accessory)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			p.Timestamp, sub, now, p.Latitude, p.Longitude, p.Altitude, p.EllipsoidalAltitude,
			p.HorizontalAccuracy, p.VerticalAccuracy, p.Speed, p.SpeedAccuracy,
			p.Course, p.CourseAccuracy, p.Floor, boolToInt(p.IsSimulated), boolToInt(p.IsFromAccessory),
		)
		if err != nil {
			return fmt.Errorf("inserting point: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO points_idx (id, min_lat, max_lat, min_lon, max_lon, min_sig, max_sig)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, p.Latitude, p.Latitude, p.Longitude, p.Longitude, significance, significance,
		); err != nil {
			return fmt.Errorf("inserting rtree: %w", err)
		}
	} else if err != nil {
		return err
	} else {
		// Update existing point.
		if _, err := tx.ExecContext(ctx,
			`UPDATE points SET subscribed=?, touched_at=?, lat=?, lon=?, alt=?, ellipsoidal_alt=?,
				h_accuracy=?, v_accuracy=?, speed=?, speed_accuracy=?, course=?, course_accuracy=?,
				floor=?, is_simulated=?, is_from_accessory=?
			WHERE id=?`,
			sub, now, p.Latitude, p.Longitude, p.Altitude, p.EllipsoidalAltitude,
			p.HorizontalAccuracy, p.VerticalAccuracy, p.Speed, p.SpeedAccuracy,
			p.Course, p.CourseAccuracy, p.Floor, boolToInt(p.IsSimulated), boolToInt(p.IsFromAccessory),
			existingID,
		); err != nil {
			return fmt.Errorf("updating point: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE points_idx SET min_lat=?, max_lat=?, min_lon=?, max_lon=?, min_sig=?, max_sig=?
			WHERE id=?`,
			p.Latitude, p.Latitude, p.Longitude, p.Longitude, significance, significance,
			existingID,
		); err != nil {
			return fmt.Errorf("updating rtree: %w", err)
		}
	}

	return tx.Commit()
}

// UpdateSignificance updates the significance of a point identified by timestamp.
func (s *Store) UpdateSignificance(ctx context.Context, timestamp int64, significance float64) error {
	var id int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT id FROM points WHERE timestamp = ?`, timestamp,
	).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return nil // point may have been evicted
		}
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE points_idx SET min_sig=?, max_sig=? WHERE id=?`,
		significance, significance, id,
	)
	return err
}

// QueryTile returns all points within the bounding box with significance >= minSig.
func (s *Store) QueryTile(ctx context.Context, south, north, west, east, minSig float64) ([]*pb.Point, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT p.timestamp, p.lat, p.lon, p.alt, p.ellipsoidal_alt,
			p.h_accuracy, p.v_accuracy, p.speed, p.speed_accuracy,
			p.course, p.course_accuracy, p.floor, p.is_simulated, p.is_from_accessory
		FROM points_idx i JOIN points p ON i.id = p.id
		WHERE i.max_lat >= ? AND i.min_lat <= ?
		  AND i.max_lon >= ? AND i.min_lon <= ?
		  AND i.max_sig >= ?
		ORDER BY p.timestamp`,
		south, north, west, east, minSig,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []*pb.Point
	for rows.Next() {
		p := &pb.Point{}
		var isSim, isAcc sql.NullInt64
		if err := rows.Scan(
			&p.Timestamp, &p.Latitude, &p.Longitude, &p.Altitude, &p.EllipsoidalAltitude,
			&p.HorizontalAccuracy, &p.VerticalAccuracy, &p.Speed, &p.SpeedAccuracy,
			&p.Course, &p.CourseAccuracy, &p.Floor, &isSim, &isAcc,
		); err != nil {
			return nil, err
		}
		p.IsSimulated = isSim.Valid && isSim.Int64 != 0
		p.IsFromAccessory = isAcc.Valid && isAcc.Int64 != 0
		points = append(points, p)
	}
	return points, rows.Err()
}

// LastTwoPoints returns the two most recent points by timestamp for VW recovery.
// Returns (secondToLast, last). Either or both may be nil if fewer than 2 points exist.
func (s *Store) LastTwoPoints(ctx context.Context) (prev, tail *pb.Point, err error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT timestamp, lat, lon, alt, ellipsoidal_alt,
			h_accuracy, v_accuracy, speed, speed_accuracy,
			course, course_accuracy, floor, is_simulated, is_from_accessory
		FROM points ORDER BY timestamp DESC LIMIT 2`,
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var pts []*pb.Point
	for rows.Next() {
		p := &pb.Point{}
		var isSim, isAcc sql.NullInt64
		if err := rows.Scan(
			&p.Timestamp, &p.Latitude, &p.Longitude, &p.Altitude, &p.EllipsoidalAltitude,
			&p.HorizontalAccuracy, &p.VerticalAccuracy, &p.Speed, &p.SpeedAccuracy,
			&p.Course, &p.CourseAccuracy, &p.Floor, &isSim, &isAcc,
		); err != nil {
			return nil, nil, err
		}
		p.IsSimulated = isSim.Valid && isSim.Int64 != 0
		p.IsFromAccessory = isAcc.Valid && isAcc.Int64 != 0
		pts = append(pts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	switch len(pts) {
	case 0:
		return nil, nil, nil
	case 1:
		return nil, pts[0], nil
	default:
		// pts[0] is most recent (tail), pts[1] is second-to-last (prev)
		return pts[1], pts[0], nil
	}
}

// Stats returns the total point count and the most recent point (by timestamp).
// If the store is empty, latest is nil.
func (s *Store) Stats(ctx context.Context) (count int64, latest *pb.Point, err error) {
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM points`).Scan(&count); err != nil {
		return 0, nil, err
	}
	if count == 0 {
		return 0, nil, nil
	}

	row := s.db.QueryRowContext(ctx,
		`SELECT timestamp, lat, lon, alt, ellipsoidal_alt,
			h_accuracy, v_accuracy, speed, speed_accuracy,
			course, course_accuracy, floor, is_simulated, is_from_accessory
		FROM points ORDER BY timestamp DESC LIMIT 1`,
	)
	p := &pb.Point{}
	var isSim, isAcc sql.NullInt64
	if err := row.Scan(
		&p.Timestamp, &p.Latitude, &p.Longitude, &p.Altitude, &p.EllipsoidalAltitude,
		&p.HorizontalAccuracy, &p.VerticalAccuracy, &p.Speed, &p.SpeedAccuracy,
		&p.Course, &p.CourseAccuracy, &p.Floor, &isSim, &isAcc,
	); err != nil {
		return 0, nil, err
	}
	p.IsSimulated = isSim.Valid && isSim.Int64 != 0
	p.IsFromAccessory = isAcc.Valid && isAcc.Int64 != 0
	return count, p, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	if _, err := s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		// best effort
	}
	return s.db.Close()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
