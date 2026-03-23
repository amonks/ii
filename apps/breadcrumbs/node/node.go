package node

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Node is a breadcrumbs node that ingests, stores, simplifies, and serves
// location data.
type Node struct {
	store   *Store
	handler http.Handler
	cancel  context.CancelFunc
}

// NewNode creates a new breadcrumbs node with the given configuration.
func NewNode(ctx context.Context, config Config) (*Node, error) {
	store, err := OpenStore(ctx, config.DBPath)
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}

	// Recover simplifier state from the last two points.
	method := config.simplifyMethod()
	simplifier := NewSimplifier(method)
	prev, tail, err := store.LastTwoPoints(ctx)
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("recovering simplifier: %w", err)
	}
	if tail != nil {
		simplifier.Recover(prev, tail)
	}

	// If the simplify method changed since last run, recompute all significance values.
	prevMethod, _ := store.GetMeta(ctx, "simplify_method")
	if prevMethod != string(method) {
		slog.Info("simplify method changed, recomputing significance",
			"old", prevMethod, "new", method)
		start := time.Now()
		n, err := store.RecomputeSignificance(ctx, method)
		if err != nil {
			store.Close()
			return nil, fmt.Errorf("recomputing significance: %w", err)
		}
		if err := store.SetMeta(ctx, "simplify_method", string(method)); err != nil {
			store.Close()
			return nil, fmt.Errorf("saving simplify method: %w", err)
		}
		slog.Info("significance recompute complete",
			"method", method, "points", n, "duration", time.Since(start))
	}

	// Recompute subscription flags.
	if err := store.RecomputeSubscriptions(ctx, config.Subscriptions); err != nil {
		store.Close()
		return nil, fmt.Errorf("recomputing subscriptions: %w", err)
	}

	hub := NewHub()

	// Create forwarder if upstream is configured.
	var forwarder *Forwarder
	nodeCtx, cancel := context.WithCancel(ctx)
	if config.Upstream != "" {
		forwarder = newForwarder(store, config.Upstream, config.Capacity)
		forwarder.RunPeriodicForward(nodeCtx, 5*time.Minute)

		// Backfill subscriptions from upstream.
		go backfillSubscriptions(nodeCtx, store, config)
	}

	// Log significance distribution to help debug threshold tuning.
	if stats, err := store.SignificanceStats(ctx); err == nil && stats.Count > 0 {
		slog.Info("significance distribution",
			"points", stats.Count,
			"min", stats.Min,
			"p25", stats.P25,
			"p50", stats.P50,
			"p75", stats.P75,
			"max", stats.Max,
		)
		// Show which zoom levels would show 25%/50%/75%/100% of points at detail=5.
		for _, pct := range []struct {
			label string
			sig   float64
		}{
			{"25%", stats.P25},
			{"50%", stats.P50},
			{"75%", stats.P75},
			{"all", stats.Min},
		} {
			if pct.sig > 0 {
				z := zoomForSigAtDetail(pct.sig, 5.0)
				slog.Info("zoom to show points",
					"fraction", pct.label,
					"sig", pct.sig,
					"zoom_at_detail5", z,
				)
			}
		}
	}

	handler := newHandler(store, simplifier, hub, &config, forwarder)

	return &Node{
		store:   store,
		handler: handler,
		cancel:  cancel,
	}, nil
}

// Handler returns the HTTP handler for this node.
func (n *Node) Handler() http.Handler {
	return n.handler
}

// Close shuts down the node and closes its database.
func (n *Node) Close() error {
	n.cancel()
	return n.store.Close()
}
