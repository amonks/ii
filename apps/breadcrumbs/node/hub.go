package node

import "sync"

// Hub is a client-keyed pub/sub hub for SSE events.
type Hub struct {
	mu   sync.Mutex
	subs map[string]chan []byte
}

func NewHub() *Hub {
	return &Hub{subs: make(map[string]chan []byte)}
}

// Subscribe registers a channel for the given client ID. If a channel already
// exists for this ID (reconnect), the old channel is closed and replaced.
// Returns the channel and an unsubscribe function.
func (h *Hub) Subscribe(clientID string) (<-chan []byte, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if old, ok := h.subs[clientID]; ok {
		close(old)
	}

	ch := make(chan []byte, 16)
	h.subs[clientID] = ch

	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.subs[clientID] == ch {
			delete(h.subs, clientID)
			close(ch)
		}
	}
}

// Publish sends data to the client with the given ID. No-op if the client
// is not connected.
func (h *Hub) Publish(clientID string, data []byte) {
	h.mu.Lock()
	ch, ok := h.subs[clientID]
	h.mu.Unlock()

	if ok {
		select {
		case ch <- data:
		default:
			// drop if buffer full
		}
	}
}
