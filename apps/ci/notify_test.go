package main

import (
	"testing"
	"time"
)

func TestOutputHubSubscribeReceivesPublished(t *testing.T) {
	hub := NewOutputHub()

	ch, unsub := hub.Subscribe("1/test/stdout")
	defer unsub()

	hub.Publish("1/test/stdout", []byte("hello\n"))

	select {
	case data := <-ch:
		if string(data) != "hello\n" {
			t.Errorf("expected %q, got %q", "hello\n", string(data))
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for data")
	}
}

func TestOutputHubMultipleSubscribers(t *testing.T) {
	hub := NewOutputHub()

	ch1, unsub1 := hub.Subscribe("1/test/stdout")
	defer unsub1()
	ch2, unsub2 := hub.Subscribe("1/test/stdout")
	defer unsub2()

	hub.Publish("1/test/stdout", []byte("data"))

	for i, ch := range []chan []byte{ch1, ch2} {
		select {
		case data := <-ch:
			if string(data) != "data" {
				t.Errorf("subscriber %d: expected %q, got %q", i, "data", string(data))
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out", i)
		}
	}
}

func TestOutputHubUnsubscribe(t *testing.T) {
	hub := NewOutputHub()

	ch, unsub := hub.Subscribe("1/test/stdout")
	unsub()

	hub.Publish("1/test/stdout", []byte("data"))

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed after unsub")
		}
	case <-time.After(100 * time.Millisecond):
		// Channel is closed and drained, this is fine too
	}
}

func TestOutputHubDifferentKeys(t *testing.T) {
	hub := NewOutputHub()

	ch1, unsub1 := hub.Subscribe("1/test/stdout")
	defer unsub1()
	ch2, unsub2 := hub.Subscribe("1/test/stderr")
	defer unsub2()

	hub.Publish("1/test/stdout", []byte("out"))

	select {
	case data := <-ch1:
		if string(data) != "out" {
			t.Errorf("expected %q, got %q", "out", string(data))
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stdout data")
	}

	select {
	case <-ch2:
		t.Error("stderr should not have received data")
	case <-time.After(50 * time.Millisecond):
		// Good, nothing received
	}
}

func TestOutputHubCloseAll(t *testing.T) {
	hub := NewOutputHub()

	ch1, _ := hub.Subscribe("1/test/stdout")
	ch2, _ := hub.Subscribe("1/test/stderr")
	ch3, unsub3 := hub.Subscribe("2/test/stdout")
	defer unsub3()

	hub.CloseAll("1/test/")

	// Both channels for job "1/test" should be closed.
	for i, ch := range []chan []byte{ch1, ch2} {
		select {
		case _, ok := <-ch:
			if ok {
				t.Errorf("channel %d: expected closed", i)
			}
		case <-time.After(time.Second):
			t.Fatalf("channel %d: timed out waiting for close", i)
		}
	}

	// Channel for different job should still be open.
	hub.Publish("2/test/stdout", []byte("still open"))
	select {
	case data := <-ch3:
		if string(data) != "still open" {
			t.Errorf("expected %q, got %q", "still open", string(data))
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for unrelated channel data")
	}
}

func TestOutputHubPublishNoSubscribers(t *testing.T) {
	hub := NewOutputHub()
	// Should not panic.
	hub.Publish("1/test/stdout", []byte("nobody listening"))
}
