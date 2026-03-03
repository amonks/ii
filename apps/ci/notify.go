package main

import (
	"strings"
	"sync"
)

// OutputHub is a channel-per-key pub/sub for streaming build output.
// Keys are "runID/jobName/stream".
type OutputHub struct {
	mu   sync.Mutex
	subs map[string]map[*chan []byte]struct{}
}

// NewOutputHub creates a new OutputHub.
func NewOutputHub() *OutputHub {
	return &OutputHub{
		subs: make(map[string]map[*chan []byte]struct{}),
	}
}

// Subscribe returns a channel that receives published bytes for the given key,
// and an unsubscribe function that closes the channel and removes it.
func (h *OutputHub) Subscribe(key string) (chan []byte, func()) {
	ch := make(chan []byte, 64)
	h.mu.Lock()
	if h.subs[key] == nil {
		h.subs[key] = make(map[*chan []byte]struct{})
	}
	h.subs[key][&ch] = struct{}{}
	h.mu.Unlock()

	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.subs[key][&ch]; ok {
			delete(h.subs[key], &ch)
			close(ch)
		}
	}
}

// Publish sends data to all subscribers of the given key. Non-blocking: if a
// subscriber's channel is full, the data is dropped for that subscriber.
func (h *OutputHub) Publish(key string, data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for chp := range h.subs[key] {
		select {
		case *chp <- data:
		default:
		}
	}
}

// CloseAll closes all subscriber channels whose key starts with prefix and
// removes them. Used to signal EOF when a job finishes.
func (h *OutputHub) CloseAll(prefix string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for key, chs := range h.subs {
		if strings.HasPrefix(key, prefix) {
			for chp := range chs {
				close(*chp)
			}
			delete(h.subs, key)
		}
	}
}
