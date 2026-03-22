package node

import (
	"context"
	"fmt"
)

// Evict removes the oldest unsubscribed points (by touched_at) that exceed
// capacity. Points with timestamp > watermark are never evicted (they haven't
// been forwarded yet). Returns the number of evicted points.
func (s *Store) Evict(ctx context.Context, capacity int, watermark int64) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Count unsubscribed, forwardable points.
	var count int
	if err := tx.QueryRowContext(ctx,
		`SELECT count(*) FROM points WHERE subscribed = 0 AND timestamp <= ?`, watermark,
	).Scan(&count); err != nil {
		return 0, err
	}

	excess := count - capacity
	if excess <= 0 {
		return 0, nil
	}

	// Find IDs to evict: oldest by touched_at.
	rows, err := tx.QueryContext(ctx,
		`SELECT id FROM points WHERE subscribed = 0 AND timestamp <= ?
		ORDER BY touched_at LIMIT ?`, watermark, excess,
	)
	if err != nil {
		return 0, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	// Delete from both tables.
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `DELETE FROM points WHERE id = ?`, id); err != nil {
			return 0, fmt.Errorf("deleting point %d: %w", id, err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM points_idx WHERE id = ?`, id); err != nil {
			return 0, fmt.Errorf("deleting rtree %d: %w", id, err)
		}
	}

	return len(ids), tx.Commit()
}

// RecomputeSubscriptions updates the subscribed flag for all points based on
// the given subscription list. This runs in a single transaction.
func (s *Store) RecomputeSubscriptions(ctx context.Context, subs []Subscription) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear all subscribed flags.
	if _, err := tx.ExecContext(ctx, `UPDATE points SET subscribed = 0`); err != nil {
		return fmt.Errorf("clearing subscribed: %w", err)
	}

	// For each subscription, mark matching points.
	for _, sub := range subs {
		west, south, east, north := sub.BBox[0], sub.BBox[1], sub.BBox[2], sub.BBox[3]
		if _, err := tx.ExecContext(ctx,
			`UPDATE points SET subscribed = 1
			WHERE id IN (
				SELECT p.id FROM points_idx i JOIN points p ON i.id = p.id
				WHERE i.min_lat >= ? AND i.max_lat <= ?
				  AND i.min_lon >= ? AND i.max_lon <= ?
				  AND p.significance >= ?
			)`,
			south, north, west, east, sub.MinSignificance,
		); err != nil {
			return fmt.Errorf("applying subscription: %w", err)
		}
	}

	return tx.Commit()
}
