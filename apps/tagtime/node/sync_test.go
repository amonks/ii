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
			if p.Blurb != "" {
				if err := serverStore.ensureTagsFromBlurb(r.Context(), p.Timestamp, p.Blurb); err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				if err := serverStore.ApplyAllRenamesForPing(r.Context(), p.Timestamp); err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
			}
		}
		for _, c := range payload.PeriodChanges {
			if err := serverStore.AddPeriodChange(r.Context(), c); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		for _, rename := range payload.TagRenames {
			if err := serverStore.AddTagRename(r.Context(), rename); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			if err := serverStore.ApplyTagRename(r.Context(), rename); err != nil {
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
		tagRenames, err := serverStore.ListTagRenames(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(syncPayload{Pings: pings, PeriodChanges: changes, TagRenames: tagRenames})
	})

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client = NewSyncer(clientStore, ts.URL, "test-client", ts.Client(), nil)
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
			if p.Blurb != "" {
				if err := serverStore.ensureTagsFromBlurb(r.Context(), p.Timestamp, p.Blurb); err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				if err := serverStore.ApplyAllRenamesForPing(r.Context(), p.Timestamp); err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
			}
		}
		for _, c := range payload.PeriodChanges {
			if err := serverStore.AddPeriodChange(r.Context(), c); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		for _, rename := range payload.TagRenames {
			if err := serverStore.AddTagRename(r.Context(), rename); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			if err := serverStore.ApplyTagRename(r.Context(), rename); err != nil {
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
		tagRenames, err := serverStore.ListTagRenames(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(syncPayload{Pings: pings, PeriodChanges: changes, TagRenames: tagRenames})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	// Client A (will go offline).
	storeA, err := OpenStore(ctx, filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { storeA.Close() })
	syncerA := NewSyncer(storeA, ts.URL, "client-a", ts.Client(), nil)

	// Client C (stays online).
	storeC, err := OpenStore(ctx, filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { storeC.Close() })
	syncerC := NewSyncer(storeC, ts.URL, "client-c", ts.Client(), nil)

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

// TestSyncTagRename tests the multi-device rename scenario:
// - Both devices add #sleep
// - Device A renames #sleep → #sleeping
// - After sync, device B's ping_tags show "sleeping"
func TestSyncTagRename(t *testing.T) {
	ctx := context.Background()
	syncer, clientStore, serverStore := newTestSyncPair(t)

	// Client adds a ping with #sleep.
	if err := clientStore.SetBlurb(ctx, 1000, "#sleep", "client"); err != nil {
		t.Fatal(err)
	}

	// Push to server.
	if _, err := syncer.Push(ctx); err != nil {
		t.Fatal(err)
	}

	// Verify ping arrived with tag on server.
	serverTags, err := serverStore.TagsForPing(ctx, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(serverTags) != 1 || serverTags[0] != "sleep" {
		t.Fatalf("server TagsForPing(1000) = %v, want [sleep]", serverTags)
	}

	// Server renames sleep → sleeping (simulating device A).
	if err := serverStore.RenameTag(ctx, "sleep", "sleeping", "server"); err != nil {
		t.Fatal(err)
	}

	// Verify server's ping_tags updated.
	serverTags, err = serverStore.TagsForPing(ctx, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(serverTags) != 1 || serverTags[0] != "sleeping" {
		t.Fatalf("after rename, server TagsForPing(1000) = %v, want [sleeping]", serverTags)
	}

	// Client pulls — should get the rename and apply it.
	if _, err := syncer.Pull(ctx); err != nil {
		t.Fatal(err)
	}

	// Verify client's ping_tags were updated by the rename.
	clientTags, err := clientStore.TagsForPing(ctx, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(clientTags) != 1 || clientTags[0] != "sleeping" {
		t.Errorf("after pull, client TagsForPing(1000) = %v, want [sleeping]", clientTags)
	}

	// Verify rename record arrived on client.
	renames, err := clientStore.ListTagRenames(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(renames) != 1 || renames[0].OldName != "sleep" || renames[0].NewName != "sleeping" {
		t.Errorf("client renames = %+v, want [{sleep→sleeping}]", renames)
	}
}

// TestSyncTagRenameTimeScoped tests that renames synced across devices
// respect the time scope: pings created after the rename keep original tags.
func TestSyncTagRenameTimeScoped(t *testing.T) {
	ctx := context.Background()
	syncer, clientStore, serverStore := newTestSyncPair(t)

	// Client adds a ping with #sleep at T=1000.
	if err := clientStore.SetBlurb(ctx, 1000, "#sleep", "client"); err != nil {
		t.Fatal(err)
	}

	// Push to server.
	if _, err := syncer.Push(ctx); err != nil {
		t.Fatal(err)
	}

	// Server renames sleep → sleeping.
	if err := serverStore.RenameTag(ctx, "sleep", "sleeping", "server"); err != nil {
		t.Fatal(err)
	}

	// Client adds ANOTHER ping with #sleep at T=far future (after rename).
	if err := clientStore.SetBlurb(ctx, 9999999999, "#sleep", "client"); err != nil {
		t.Fatal(err)
	}

	// Push the new ping.
	if _, err := syncer.Push(ctx); err != nil {
		t.Fatal(err)
	}

	// Pull the rename.
	if _, err := syncer.Pull(ctx); err != nil {
		t.Fatal(err)
	}

	// Old ping should be renamed.
	tags, err := clientStore.TagsForPing(ctx, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0] != "sleeping" {
		t.Errorf("old ping TagsForPing(1000) = %v, want [sleeping]", tags)
	}

	// New ping (after rename time) should keep "sleep".
	tags, err = clientStore.TagsForPing(ctx, 9999999999)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0] != "sleep" {
		t.Errorf("new ping TagsForPing(9999999999) = %v, want [sleep]", tags)
	}
}

// TestSyncRenameThenLateArrival tests the offline rename scenario:
// - A and B are offline
// - A logs #sleep at T1, B logs #sleep at T2
// - A renames #sleep → #sleeping at T3
// - A logs #sleep at T4
// - After sync: T1 and T2 should be #sleeping, T4 should be #sleep
// This verifies that the server applies existing renames to late-arriving pings.
func TestSyncRenameThenLateArrival(t *testing.T) {
	ctx := context.Background()

	// We need two clients and a server. Use a slightly different setup
	// than newTestSyncPair so we can have two independent clients.
	serverStore, err := OpenStore(ctx, filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { serverStore.Close() })

	clientAStore, err := OpenStore(ctx, filepath.Join(t.TempDir(), "clientA.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clientAStore.Close() })

	clientBStore, err := OpenStore(ctx, filepath.Join(t.TempDir(), "clientB.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clientBStore.Close() })

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
			if p.Blurb != "" {
				if err := serverStore.ensureTagsFromBlurb(r.Context(), p.Timestamp, p.Blurb); err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				if err := serverStore.ApplyAllRenamesForPing(r.Context(), p.Timestamp); err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
			}
		}
		for _, c := range payload.PeriodChanges {
			if err := serverStore.AddPeriodChange(r.Context(), c); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		for _, rename := range payload.TagRenames {
			if err := serverStore.AddTagRename(r.Context(), rename); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			if err := serverStore.ApplyTagRename(r.Context(), rename); err != nil {
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
		renames, err := serverStore.ListTagRenames(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(syncPayload{Pings: pings, PeriodChanges: changes, TagRenames: renames})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	syncerA := NewSyncer(clientAStore, ts.URL, "A", http.DefaultClient, nil)
	syncerB := NewSyncer(clientBStore, ts.URL, "B", http.DefaultClient, nil)

	// Both offline: A logs #sleep at T=1000, B logs #sleep at T=2000.
	if err := clientAStore.SetBlurb(ctx, 1000, "#sleep", "A"); err != nil {
		t.Fatal(err)
	}
	if err := clientBStore.SetBlurb(ctx, 2000, "#sleep", "B"); err != nil {
		t.Fatal(err)
	}

	// A renames #sleep → #sleeping at T3 (current time).
	if err := clientAStore.RenameTag(ctx, "sleep", "sleeping", "A"); err != nil {
		t.Fatal(err)
	}

	// A logs #sleep again at T=9999999999 (far future, after rename).
	if err := clientAStore.SetBlurb(ctx, 9999999999, "#sleep", "A"); err != nil {
		t.Fatal(err)
	}

	// A syncs first (push + pull).
	if _, err := syncerA.Push(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := syncerA.Pull(ctx); err != nil {
		t.Fatal(err)
	}

	// B syncs (push + pull). B's ping T=2000 arrives on server after rename.
	if _, err := syncerB.Push(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := syncerB.Pull(ctx); err != nil {
		t.Fatal(err)
	}

	// Verify server state: T1 and T2 should be "sleeping", T4 should be "sleep".
	serverTagsT1, err := serverStore.TagsForPing(ctx, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(serverTagsT1) != 1 || serverTagsT1[0] != "sleeping" {
		t.Errorf("server TagsForPing(1000) = %v, want [sleeping]", serverTagsT1)
	}

	serverTagsT2, err := serverStore.TagsForPing(ctx, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if len(serverTagsT2) != 1 || serverTagsT2[0] != "sleeping" {
		t.Errorf("server TagsForPing(2000) = %v, want [sleeping]", serverTagsT2)
	}

	serverTagsT4, err := serverStore.TagsForPing(ctx, 9999999999)
	if err != nil {
		t.Fatal(err)
	}
	if len(serverTagsT4) != 1 || serverTagsT4[0] != "sleep" {
		t.Errorf("server TagsForPing(9999999999) = %v, want [sleep]", serverTagsT4)
	}

	// Verify client B state: should also be correct after pull.
	clientBTagsT1, err := clientBStore.TagsForPing(ctx, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(clientBTagsT1) != 1 || clientBTagsT1[0] != "sleeping" {
		t.Errorf("clientB TagsForPing(1000) = %v, want [sleeping]", clientBTagsT1)
	}

	clientBTagsT2, err := clientBStore.TagsForPing(ctx, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if len(clientBTagsT2) != 1 || clientBTagsT2[0] != "sleeping" {
		t.Errorf("clientB TagsForPing(2000) = %v, want [sleeping]", clientBTagsT2)
	}

	clientBTagsT4, err := clientBStore.TagsForPing(ctx, 9999999999)
	if err != nil {
		t.Fatal(err)
	}
	if len(clientBTagsT4) != 1 || clientBTagsT4[0] != "sleep" {
		t.Errorf("clientB TagsForPing(9999999999) = %v, want [sleep]", clientBTagsT4)
	}
}
