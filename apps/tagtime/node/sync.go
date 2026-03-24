package node

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Syncer handles bidirectional sync with an upstream server.
type Syncer struct {
	store          *Store
	upstream       string
	nodeID         string
	client         *http.Client
	refreshChanges func(context.Context)
}

// NewSyncer creates a syncer that pushes/pulls with the given upstream URL.
func NewSyncer(store *Store, upstream, nodeID string, client *http.Client, refreshChanges func(context.Context)) *Syncer {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Syncer{
		store:          store,
		upstream:       upstream,
		nodeID:         nodeID,
		client:         client,
		refreshChanges: refreshChanges,
	}
}

// syncPayload is the JSON wire format for push/pull.
type syncPayload struct {
	Pings         []Ping         `json:"pings,omitempty"`
	PeriodChanges []PeriodChange `json:"period_changes,omitempty"`
	TagRenames    []TagRename    `json:"tag_renames,omitempty"`
}

// Push sends unsynced pings and period changes to upstream. Returns the number of pings pushed.
func (s *Syncer) Push(ctx context.Context) (int, error) {
	pings, err := s.store.UnsyncedPings(ctx, 1000)
	if err != nil {
		return 0, fmt.Errorf("listing unsynced: %w", err)
	}
	changes, err := s.store.ListPeriodChanges(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing period changes: %w", err)
	}
	tagRenames, err := s.store.ListTagRenames(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing tag renames: %w", err)
	}
	if len(pings) == 0 && len(changes) == 0 && len(tagRenames) == 0 {
		return 0, nil
	}

	payload := syncPayload{Pings: pings, PeriodChanges: changes, TagRenames: tagRenames}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.upstream+"/sync/push", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("push request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("push failed: %s %s", resp.Status, respBody)
	}

	// Mark pings as synced.
	maxUpdatedAt := int64(0)
	for _, p := range pings {
		if p.UpdatedAt > maxUpdatedAt {
			maxUpdatedAt = p.UpdatedAt
		}
	}
	if err := s.store.MarkSynced(ctx, maxUpdatedAt); err != nil {
		return 0, fmt.Errorf("marking synced: %w", err)
	}

	if err := s.store.SetMeta(ctx, "last_push_at", fmt.Sprintf("%d", time.Now().Unix())); err != nil {
		return 0, fmt.Errorf("storing last_push_at: %w", err)
	}

	return len(pings), nil
}

// Pull fetches changed pings from upstream. Returns the number of pings received.
func (s *Syncer) Pull(ctx context.Context) (int, error) {
	since, err := s.store.GetMeta(ctx, "pull_watermark")
	if err != nil {
		return 0, err
	}
	if since == "" {
		since = "0"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", s.upstream+"/sync/pull?since="+since, nil)
	if err != nil {
		return 0, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("pull request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("pull failed: %s %s", resp.Status, respBody)
	}

	var payload syncPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, fmt.Errorf("decoding pull response: %w", err)
	}

	// Apply received pings with LWW, deriving ping_tags from blurbs.
	for _, p := range payload.Pings {
		if err := s.store.UpsertPing(ctx, p); err != nil {
			return 0, fmt.Errorf("upserting pulled ping: %w", err)
		}
		if p.Blurb != "" {
			if err := s.store.ensureTagsFromBlurb(ctx, p.Timestamp, p.Blurb); err != nil {
				return 0, fmt.Errorf("ensuring tags from pulled ping: %w", err)
			}
		}
	}

	// Apply period changes.
	for _, c := range payload.PeriodChanges {
		if err := s.store.AddPeriodChange(ctx, c); err != nil {
			return 0, fmt.Errorf("adding pulled period change: %w", err)
		}
	}
	if len(payload.PeriodChanges) > 0 && s.refreshChanges != nil {
		s.refreshChanges(ctx)
	}

	// Apply tag renames.
	for _, r := range payload.TagRenames {
		if err := s.store.AddTagRename(ctx, r); err != nil {
			return 0, fmt.Errorf("adding pulled tag rename: %w", err)
		}
		if err := s.store.ApplyTagRename(ctx, r); err != nil {
			return 0, fmt.Errorf("applying pulled tag rename: %w", err)
		}
	}

	// Advance watermark based on server-assigned received_at.
	maxReceivedAt := int64(0)
	for _, p := range payload.Pings {
		if p.ReceivedAt > maxReceivedAt {
			maxReceivedAt = p.ReceivedAt
		}
	}
	if maxReceivedAt > 0 {
		if err := s.store.SetMeta(ctx, "pull_watermark", fmt.Sprintf("%d", maxReceivedAt)); err != nil {
			return 0, err
		}
	}

	if err := s.store.SetMeta(ctx, "last_pull_at", fmt.Sprintf("%d", time.Now().Unix())); err != nil {
		return 0, fmt.Errorf("storing last_pull_at: %w", err)
	}

	return len(payload.Pings), nil
}

// RunPeriodicSync runs push then pull on the given interval until ctx is cancelled.
func (s *Syncer) RunPeriodicSync(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := s.Push(ctx); err != nil {
				slog.Error("sync push", "error", err)
			} else if n > 0 {
				slog.Info("sync pushed", "count", n)
			}
			if n, err := s.Pull(ctx); err != nil {
				slog.Error("sync pull", "error", err)
			} else if n > 0 {
				slog.Info("sync pulled", "count", n)
			}
		}
	}
}
