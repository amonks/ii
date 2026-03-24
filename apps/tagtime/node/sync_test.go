package node

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// newTestSyncPair creates two stores and a test server from the "server" store,
// returning the client syncer and both stores.
func newTestSyncPair(t *testing.T) (client *Syncer, clientStore, serverStore *Store) {
	t.Helper()
	ctx := context.Background()

	serverStore, err := OpenStore(ctx, filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { serverStore.Close() })

	clientStore, err = OpenStore(ctx, filepath.Join(t.TempDir(), "client.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clientStore.Close() })

	// Create a test HTTP server that handles sync endpoints.
	mux := http.NewServeMux()
	mux.HandleFunc("POST /sync/push", func(w http.ResponseWriter, r *http.Request) {
		var payload syncPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		now := time.Now().UnixNano()
		for _, p := range payload.Pings {
			p.ReceivedAt = now
			if err := serverStore.UpsertPing(r.Context(), p); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		for _, c := range payload.PeriodChanges {
			if err := serverStore.AddPeriodChange(r.Context(), c); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /sync/pull", func(w http.ResponseWriter, r *http.Request) {
		sinceStr := r.URL.Query().Get("since")
		since, _ := strconv.ParseInt(sinceStr, 10, 64)
		pings, err := serverStore.PingsReceivedAfter(r.Context(), since, 1000)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		changes, err := serverStore.ListPeriodChanges(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(syncPayload{Pings: pings, PeriodChanges: changes})
	})

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client = NewSyncer(clientStore, ts.URL, "test-client", ts.Client())
	return client, clientStore, serverStore
}

func TestSyncPush(t *testing.T) {
	ctx := context.Background()
	syncer, clientStore, serverStore := newTestSyncPair(t)

	// Add a ping on the client.
	if err := clientStore.SetBlurb(ctx, 1000, "#working", "client"); err != nil {
		t.Fatal(err)
	}

	// Push to server.
	n, err := syncer.Push(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("pushed %d, want 1", n)
	}

	// Verify on server.
	p, err := serverStore.GetPing(ctx, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if p == nil || p.Blurb != "#working" {
		t.Errorf("server ping = %+v, want blurb=#working", p)
	}

	// Second push should be a no-op.
	n, err = syncer.Push(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("second push sent %d, want 0", n)
	}
}

func TestSyncPull(t *testing.T) {
	ctx := context.Background()
	syncer, clientStore, serverStore := newTestSyncPair(t)

	// Add pings on the server.
	if err := serverStore.UpsertPing(ctx, Ping{Timestamp: 2000, Blurb: "#meeting", NodeID: "server", UpdatedAt: 500, ReceivedAt: 500}); err != nil {
		t.Fatal(err)
	}

	// Pull to client.
	n, err := syncer.Pull(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("pulled %d, want 1", n)
	}

	// Verify on client.
	p, err := clientStore.GetPing(ctx, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if p == nil || p.Blurb != "#meeting" {
		t.Errorf("client ping = %+v, want blurb=#meeting", p)
	}

	// Second pull should be a no-op.
	n, err = syncer.Pull(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("second pull got %d, want 0", n)
	}
}

func TestSyncLWWConflict(t *testing.T) {
	ctx := context.Background()
	syncer, clientStore, serverStore := newTestSyncPair(t)

	ts := int64(3000)

	// Client writes first.
	if err := clientStore.UpsertPing(ctx, Ping{Timestamp: ts, Blurb: "#client", NodeID: "client", UpdatedAt: 100}); err != nil {
		t.Fatal(err)
	}

	// Server writes with higher timestamp.
	if err := serverStore.UpsertPing(ctx, Ping{Timestamp: ts, Blurb: "#server", NodeID: "server", UpdatedAt: 200, ReceivedAt: 200}); err != nil {
		t.Fatal(err)
	}

	// Push client's version to server — should be rejected (server has newer).
	if _, err := syncer.Push(ctx); err != nil {
		t.Fatal(err)
	}
	p, err := serverStore.GetPing(ctx, ts)
	if err != nil {
		t.Fatal(err)
	}
	if p.Blurb != "#server" {
		t.Errorf("server should keep its newer version, got blurb=%q", p.Blurb)
	}

	// Pull server's version to client — should win.
	if _, err := syncer.Pull(ctx); err != nil {
		t.Fatal(err)
	}
	p, err = clientStore.GetPing(ctx, ts)
	if err != nil {
		t.Fatal(err)
	}
	if p.Blurb != "#server" {
		t.Errorf("client should have server's version after pull, got blurb=%q", p.Blurb)
	}
}

func TestSyncPeriodChanges(t *testing.T) {
	ctx := context.Background()
	syncer, clientStore, serverStore := newTestSyncPair(t)

	// Add period change on server.
	if err := serverStore.AddPeriodChange(ctx, PeriodChange{Timestamp: 0, Seed: 42, PeriodSecs: 2700}); err != nil {
		t.Fatal(err)
	}

	// Pull to client.
	if _, err := syncer.Pull(ctx); err != nil {
		t.Fatal(err)
	}

	changes, err := clientStore.ListPeriodChanges(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].PeriodSecs != 2700 {
		t.Errorf("expected period change, got %+v", changes)
	}
}

func TestSyncPushSetsLastPushAt(t *testing.T) {
	ctx := context.Background()
	syncer, clientStore, _ := newTestSyncPair(t)

	if err := clientStore.SetBlurb(ctx, 1000, "#work", "client"); err != nil {
		t.Fatal(err)
	}

	before := time.Now().Unix()
	if _, err := syncer.Push(ctx); err != nil {
		t.Fatal(err)
	}
	after := time.Now().Unix()

	val, err := clientStore.GetMeta(ctx, "last_push_at")
	if err != nil {
		t.Fatal(err)
	}
	ts, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		t.Fatalf("last_push_at not a valid int: %q", val)
	}
	if ts < before || ts > after {
		t.Errorf("last_push_at=%d not in [%d, %d]", ts, before, after)
	}
}

func TestSyncPullSetsLastPullAt(t *testing.T) {
	ctx := context.Background()
	syncer, clientStore, serverStore := newTestSyncPair(t)

	if err := serverStore.AddPeriodChange(ctx, PeriodChange{Timestamp: 0, Seed: 42, PeriodSecs: 2700}); err != nil {
		t.Fatal(err)
	}

	before := time.Now().Unix()
	if _, err := syncer.Pull(ctx); err != nil {
		t.Fatal(err)
	}
	after := time.Now().Unix()

	val, err := clientStore.GetMeta(ctx, "last_pull_at")
	if err != nil {
		t.Fatal(err)
	}
	ts, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		t.Fatalf("last_pull_at not a valid int: %q", val)
	}
	if ts < before || ts > after {
		t.Errorf("last_pull_at=%d not in [%d, %d]", ts, before, after)
	}
}

// TestSyncLateClientPingsVisible tests that when client A goes offline,
// B pushes changes, C pulls them (advancing C's watermark), then A comes
// back and pushes old pings — C still sees A's pings on the next pull.
// This works because the server stamps received_at on push receipt, and
// pull filters on received_at rather than updated_at.
func TestSyncLateClientPingsVisible(t *testing.T) {
	ctx := context.Background()

	// Set up a shared server store and HTTP server.
	serverStore, err := OpenStore(ctx, filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { serverStore.Close() })

	mux := http.NewServeMux()
	mux.HandleFunc("POST /sync/push", func(w http.ResponseWriter, r *http.Request) {
		var payload syncPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		now := time.Now().UnixNano()
		for _, p := range payload.Pings {
			p.ReceivedAt = now
			if err := serverStore.UpsertPing(r.Context(), p); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		for _, c := range payload.PeriodChanges {
			if err := serverStore.AddPeriodChange(r.Context(), c); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /sync/pull", func(w http.ResponseWriter, r *http.Request) {
		sinceStr := r.URL.Query().Get("since")
		since, _ := strconv.ParseInt(sinceStr, 10, 64)
		pings, err := serverStore.PingsReceivedAfter(r.Context(), since, 1000)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		changes, err := serverStore.ListPeriodChanges(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(syncPayload{Pings: pings, PeriodChanges: changes})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	// Client A (will go offline).
	storeA, err := OpenStore(ctx, filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { storeA.Close() })
	syncerA := NewSyncer(storeA, ts.URL, "client-a", ts.Client())

	// Client C (stays online).
	storeC, err := OpenStore(ctx, filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { storeC.Close() })
	syncerC := NewSyncer(storeC, ts.URL, "client-c", ts.Client())

	// 1. Client A writes a ping while offline (doesn't push yet).
	if err := storeA.SetBlurb(ctx, 1000, "#offline-work", "client-a"); err != nil {
		t.Fatal(err)
	}

	// 2. Meanwhile, another client (simulated by direct server insert) pushes data.
	if err := serverStore.UpsertPing(ctx, Ping{
		Timestamp: 2000, Blurb: "#other-work", NodeID: "client-b",
		UpdatedAt: time.Now().UnixNano(), ReceivedAt: time.Now().UnixNano(),
	}); err != nil {
		t.Fatal(err)
	}

	// 3. Client C pulls — gets B's ping, advancing its watermark.
	n, err := syncerC.Pull(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("C should have pulled 1 ping, got %d", n)
	}

	// Verify C has B's ping.
	p, err := storeC.GetPing(ctx, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if p == nil || p.Blurb != "#other-work" {
		t.Fatalf("C should have B's ping, got %+v", p)
	}

	// 4. Client A comes back online and pushes its old ping.
	//    The server will stamp a fresh received_at on it.
	n, err = syncerA.Push(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("A should have pushed 1 ping, got %d", n)
	}

	// 5. Client C pulls again — should get A's ping even though A's
	//    updated_at is older than C's watermark. This works because
	//    the server assigned a new received_at when A pushed.
	n, err = syncerC.Pull(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("C should have pulled 1 new ping from A, got %d", n)
	}

	// Verify C has A's ping.
	p, err = storeC.GetPing(ctx, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if p == nil || p.Blurb != "#offline-work" {
		t.Fatalf("C should have A's offline ping, got %+v", p)
	}
}

func TestSyncPushPeriodChanges(t *testing.T) {
	ctx := context.Background()
	syncer, clientStore, serverStore := newTestSyncPair(t)

	// Add a period change on the client (simulating iOS user changing period).
	if err := clientStore.AddPeriodChange(ctx, PeriodChange{Timestamp: 100, Seed: 42, PeriodSecs: 1800}); err != nil {
		t.Fatal(err)
	}

	// Also add a ping so Push has something to send (Push requires unsynced pings).
	if err := clientStore.SetBlurb(ctx, 1000, "#work", "client"); err != nil {
		t.Fatal(err)
	}

	// Push to server.
	if _, err := syncer.Push(ctx); err != nil {
		t.Fatal(err)
	}

	// Verify period change arrived on server.
	changes, err := serverStore.ListPeriodChanges(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].PeriodSecs != 1800 {
		t.Errorf("expected pushed period change with 1800s, got %+v", changes)
	}
}
