package node

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Node is a tagtime instance with storage, schedule, sync, and HTTP handler.
type Node struct {
	store   *Store
	syncer  *Syncer
	handler http.Handler
	cancel  context.CancelFunc

	mu      sync.RWMutex
	changes []PeriodChange
}

// NewNode creates and starts a tagtime node.
func NewNode(ctx context.Context, config Config) (*Node, error) {
	store, err := OpenStore(ctx, config.DBPath)
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}

	// Load or initialize period changes.
	changes, err := store.ListPeriodChanges(ctx)
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("listing period changes: %w", err)
	}

	var syncer *Syncer
	if config.Upstream != "" {
		syncer = NewSyncer(store, config.Upstream, config.NodeID, nil)
	}

	if len(changes) == 0 {
		if syncer != nil {
			// Client: pull period changes from server on first launch.
			if _, err := syncer.Pull(ctx); err != nil {
				store.Close()
				return nil, fmt.Errorf("first launch sync failed (is the server reachable?): %w", err)
			}
			changes, err = store.ListPeriodChanges(ctx)
			if err != nil {
				store.Close()
				return nil, fmt.Errorf("listing period changes after sync: %w", err)
			}
			if len(changes) == 0 {
				store.Close()
				return nil, fmt.Errorf("server has no period changes; initialize the server first")
			}
		} else {
			// Server: create the initial period change.
			initial := PeriodChange{
				Timestamp:  time.Now().Unix(),
				Seed:       config.DefaultSeed,
				PeriodSecs: config.DefaultPeriodSecs,
			}
			if err := store.AddPeriodChange(ctx, initial); err != nil {
				store.Close()
				return nil, fmt.Errorf("adding initial period change: %w", err)
			}
			changes = []PeriodChange{initial}
		}
	}

	ctx, cancel := context.WithCancel(ctx)

	n := &Node{
		store:   store,
		cancel:  cancel,
		changes: changes,
	}

	n.handler = newHandler(store, n.getChanges, config.NodeID, config.Upstream)

	// Start periodic sync if upstream configured.
	if syncer != nil {
		n.syncer = syncer
		go n.syncer.RunPeriodicSync(ctx, 5*time.Minute)
	}

	// Periodically ensure scheduled pings exist in the DB.
	go n.ensureScheduledPings(ctx)

	return n, nil
}

// Handler returns the HTTP handler for this node.
func (n *Node) Handler() http.Handler {
	return n.handler
}

// Close shuts down the node.
func (n *Node) Close() error {
	n.cancel()
	return n.store.Close()
}

func (n *Node) getChanges() []PeriodChange {
	n.mu.RLock()
	defer n.mu.RUnlock()
	out := make([]PeriodChange, len(n.changes))
	copy(out, n.changes)
	return out
}

func (n *Node) refreshChanges(ctx context.Context) {
	changes, err := n.store.ListPeriodChanges(ctx)
	if err != nil {
		slog.Error("refreshing period changes", "error", err)
		return
	}
	n.mu.Lock()
	n.changes = changes
	n.mu.Unlock()
}

// ensureScheduledPings periodically materializes upcoming scheduled pings
// into the database so they appear as pending.
func (n *Node) ensureScheduledPings(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Run once immediately.
	n.materializePings(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.refreshChanges(ctx)
			n.materializePings(ctx)
		}
	}
}

func (n *Node) materializePings(ctx context.Context) {
	changes := n.getChanges()
	now := time.Now()
	start := now.Add(-24 * time.Hour)
	end := now.Add(1 * time.Hour) // materialize slightly ahead

	pings := PingsInRange(changes, start, end)
	if len(pings) == 0 {
		return
	}

	timestamps := make([]int64, len(pings))
	for i, p := range pings {
		timestamps[i] = p.Unix()
	}
	if err := n.store.EnsurePingsExist(ctx, timestamps); err != nil {
		slog.Error("materializing pings", "error", err)
	}
}
