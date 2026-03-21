package node

import (
	"context"
	"fmt"
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
	simplifier := NewSimplifier()
	prev, tail, err := store.LastTwoPoints(ctx)
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("recovering simplifier: %w", err)
	}
	if tail != nil {
		simplifier.Recover(prev, tail)
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
