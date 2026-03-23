package node

import (
	"sync"
	"testing"
)

func TestHubPubSub(t *testing.T) {
	h := NewHub()
	ch, unsub := h.Subscribe("client1")
	defer unsub()

	h.Publish("client1", []byte("hello"))

	msg := <-ch
	if string(msg) != "hello" {
		t.Errorf("got %q, want %q", msg, "hello")
	}
}

func TestHubTwoClients(t *testing.T) {
	h := NewHub()
	ch1, unsub1 := h.Subscribe("c1")
	defer unsub1()
	ch2, unsub2 := h.Subscribe("c2")
	defer unsub2()

	h.Publish("c1", []byte("for-c1"))
	h.Publish("c2", []byte("for-c2"))

	if string(<-ch1) != "for-c1" {
		t.Error("c1 got wrong message")
	}
	if string(<-ch2) != "for-c2" {
		t.Error("c2 got wrong message")
	}
}

func TestHubPublishUnknown(t *testing.T) {
	h := NewHub()
	// Should not panic.
	h.Publish("nobody", []byte("data"))
}

func TestHubUnsubscribe(t *testing.T) {
	h := NewHub()
	ch, unsub := h.Subscribe("c1")
	unsub()

	// Channel should be closed.
	_, ok := <-ch
	if ok {
		t.Error("channel should be closed after unsub")
	}

	// Publish after unsub should not panic.
	h.Publish("c1", []byte("data"))
}

func TestHubReconnect(t *testing.T) {
	h := NewHub()
	ch1, _ := h.Subscribe("c1")

	// Reconnect replaces the channel.
	ch2, unsub2 := h.Subscribe("c1")
	defer unsub2()

	// Old channel should be closed.
	_, ok := <-ch1
	if ok {
		t.Error("old channel should be closed on reconnect")
	}

	// New channel should work.
	h.Publish("c1", []byte("new"))
	if string(<-ch2) != "new" {
		t.Error("new channel should receive messages")
	}
}

func TestHubConcurrent(t *testing.T) {
	h := NewHub()
	var wg sync.WaitGroup

	for i := range 10 {
		wg.Go(func() {
			id := string(rune('a' + i))
			ch, unsub := h.Subscribe(id)
			h.Publish(id, []byte("msg"))
			<-ch
			unsub()
		})
	}

	wg.Wait()
}
