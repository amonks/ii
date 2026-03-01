package server

import (
	"encoding/json"
	"testing"
	"time"
)

func TestHubSubscribePublish(t *testing.T) {
	hub := NewHub()

	ch := hub.Subscribe(1)
	defer hub.Unsubscribe(1, ch)

	hub.Publish(1, "cell_update", map[string]int{"x": 0, "y": 0})

	select {
	case data := <-ch:
		var evt Event
		if err := json.Unmarshal(data, &evt); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if evt.Type != "cell_update" {
			t.Errorf("type = %q, want %q", evt.Type, "cell_update")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestHubMultipleSubscribers(t *testing.T) {
	hub := NewHub()

	ch1 := hub.Subscribe(1)
	defer hub.Unsubscribe(1, ch1)
	ch2 := hub.Subscribe(1)
	defer hub.Unsubscribe(1, ch2)

	hub.Publish(1, "test", nil)

	for _, ch := range []chan []byte{ch1, ch2} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatal("subscriber did not receive event")
		}
	}
}

func TestHubIsolation(t *testing.T) {
	hub := NewHub()

	ch1 := hub.Subscribe(1)
	defer hub.Unsubscribe(1, ch1)
	ch2 := hub.Subscribe(2)
	defer hub.Unsubscribe(2, ch2)

	hub.Publish(1, "test", nil)

	select {
	case <-ch1:
	case <-time.After(time.Second):
		t.Fatal("map 1 subscriber should receive")
	}

	select {
	case <-ch2:
		t.Fatal("map 2 subscriber should not receive map 1 events")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestHubUnsubscribe(t *testing.T) {
	hub := NewHub()

	ch := hub.Subscribe(1)
	hub.Unsubscribe(1, ch)

	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Fatal("expected channel to be closed")
	}
}
