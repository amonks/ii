package node

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"time"

	pb "monks.co/apps/breadcrumbs/proto"
	"google.golang.org/protobuf/proto"
)

// GetWatermark returns the forward watermark (timestamp of the last
// successfully forwarded point). Returns 0 if no watermark is set.
func (s *Store) GetWatermark(ctx context.Context) (int64, error) {
	var val sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM meta WHERE key = 'forward_watermark'`,
	).Scan(&val)
	if err == sql.ErrNoRows || !val.Valid {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var wm int64
	if _, err := fmt.Sscanf(val.String, "%d", &wm); err != nil {
		return 0, fmt.Errorf("parsing watermark %q: %w", val.String, err)
	}
	return wm, nil
}

// SetWatermark advances the forward watermark.
func (s *Store) SetWatermark(ctx context.Context, watermark int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO meta (key, value) VALUES ('forward_watermark', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		fmt.Sprintf("%d", watermark),
	)
	return err
}

// ForwardQueueSize returns the number of points with timestamp > watermark.
func (s *Store) ForwardQueueSize(ctx context.Context, watermark int64) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM points WHERE timestamp > ?`, watermark,
	).Scan(&count)
	return count, err
}

// ForwardablePoints returns all points with timestamp > watermark, ordered
// by timestamp. These are points that have not yet been forwarded upstream.
func (s *Store) ForwardablePoints(ctx context.Context, watermark int64) ([]*pb.Point, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT timestamp, lat, lon, alt, ellipsoidal_alt,
			h_accuracy, v_accuracy, speed, speed_accuracy,
			course, course_accuracy, floor, is_simulated, is_from_accessory
		FROM points WHERE timestamp > ?
		ORDER BY timestamp`,
		watermark,
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

// Forwarder manages forwarding points to an upstream node.
type Forwarder struct {
	store    *Store
	upstream string
	capacity int
	client   *http.Client
}

func newForwarder(store *Store, upstream string, capacity int) *Forwarder {
	return &Forwarder{
		store:    store,
		upstream: upstream,
		capacity: capacity,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

// Forward sends all unforwarded points to the upstream node. Returns the
// number of points forwarded and the new watermark.
func (f *Forwarder) Forward(ctx context.Context) (int32, int64, error) {
	watermark, err := f.store.GetWatermark(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("getting watermark: %w", err)
	}

	points, err := f.store.ForwardablePoints(ctx, watermark)
	if err != nil {
		return 0, 0, fmt.Errorf("getting forwardable points: %w", err)
	}

	if len(points) == 0 {
		return 0, watermark, nil
	}

	track := &pb.Track{Points: points}
	body, err := proto.Marshal(track)
	if err != nil {
		return 0, 0, fmt.Errorf("marshaling track: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", f.upstream+"/ingest", bytes.NewReader(body))
	if err != nil {
		return 0, 0, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/protobuf")

	resp, err := f.client.Do(req)
	if err != nil {
		return 0, watermark, fmt.Errorf("forwarding to upstream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, watermark, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, respBody)
	}

	// Advance watermark to the latest forwarded point.
	newWatermark := points[len(points)-1].Timestamp
	if err := f.store.SetWatermark(ctx, newWatermark); err != nil {
		return 0, watermark, fmt.Errorf("setting watermark: %w", err)
	}

	// Run eviction now that points have been forwarded.
	if f.capacity > 0 {
		f.store.Evict(ctx, f.capacity, newWatermark)
	}

	return int32(len(points)), newWatermark, nil
}

// RunPeriodicForward starts a background goroutine that forwards every
// interval when there are unsent points. Stops when ctx is cancelled.
func (f *Forwarder) RunPeriodicForward(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, _, err := f.Forward(ctx)
				if err != nil {
					slog.Warn("periodic forward failed", "error", err)
				} else if n > 0 {
					slog.Info("periodic forward", "points", n)
				}
			}
		}
	}()
}

// FetchTileFromUpstream fetches a tile from the upstream node as protobuf.
func FetchTileFromUpstream(ctx context.Context, client *http.Client, upstream string, z, x, y int) ([]*pb.Point, error) {
	url := fmt.Sprintf("%s/tiles/%d/%d/%d", upstream, z, x, y)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/protobuf")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream tile %d/%d/%d returned %d", z, x, y, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var track pb.Track
	if err := proto.Unmarshal(body, &track); err != nil {
		return nil, fmt.Errorf("decoding upstream tile: %w", err)
	}

	return track.Points, nil
}

// WriteUpstreamPoints writes points received from upstream into the store.
// Upstream points replace local significance (server is authoritative).
// Returns true if any new data was written.
func WriteUpstreamPoints(ctx context.Context, store *Store, points []*pb.Point, subs []Subscription) (bool, error) {
	changed := false
	for _, p := range points {
		// Use MaxFloat64 significance for upstream points — the server's
		// simplification is authoritative and we trust its tile-level filtering.
		subscribed := matchesSubscription(p, math.MaxFloat64, subs)
		if err := store.InsertPoint(ctx, p, math.MaxFloat64, subscribed); err != nil {
			return changed, err
		}
		changed = true
	}
	return changed, nil
}
