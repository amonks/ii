package server

import (
	"encoding/json"
	"sync"
)

// Hub manages per-map SSE subscriber lists for real-time broadcasting.
type Hub struct {
	mu          sync.Mutex
	subscribers map[uint][]chan []byte
}

func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[uint][]chan []byte),
	}
}

// Subscribe registers a new subscriber for a map and returns a channel
// that will receive SSE event data. Call Unsubscribe when done.
func (h *Hub) Subscribe(mapID uint) chan []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan []byte, 32)
	h.subscribers[mapID] = append(h.subscribers[mapID], ch)
	return ch
}

// Unsubscribe removes a subscriber channel for a map.
func (h *Hub) Unsubscribe(mapID uint, ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	subs := h.subscribers[mapID]
	for i, s := range subs {
		if s == ch {
			h.subscribers[mapID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			return
		}
	}
}

// Event represents an SSE event sent to subscribers.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// Publish sends an event to all subscribers of a map.
func (h *Hub) Publish(mapID uint, eventType string, data any) {
	evt := Event{Type: eventType, Data: data}
	bs, err := json.Marshal(evt)
	if err != nil {
		return
	}

	h.mu.Lock()
	subs := make([]chan []byte, len(h.subscribers[mapID]))
	copy(subs, h.subscribers[mapID])
	h.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- bs:
		default:
			// Drop if subscriber is too slow
		}
	}
}
