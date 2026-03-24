package node

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := OpenStore(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestOpenStore(t *testing.T) {
	store := openTestStore(t)
	if store == nil {
		t.Fatal("store is nil")
	}
}

func TestUpsertPingLWW(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	ts := int64(1000000)

	// Insert initial ping.
	err := store.UpsertPing(ctx, Ping{
		Timestamp: ts, Blurb: "first", NodeID: "a", UpdatedAt: 100,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify.
	p, err := store.GetPing(ctx, ts)
	if err != nil {
		t.Fatal(err)
	}
	if p.Blurb != "first" {
		t.Errorf("blurb = %q, want %q", p.Blurb, "first")
	}

	// Older update should be rejected.
	err = store.UpsertPing(ctx, Ping{
		Timestamp: ts, Blurb: "older", NodeID: "b", UpdatedAt: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err = store.GetPing(ctx, ts)
	if err != nil {
		t.Fatal(err)
	}
	if p.Blurb != "first" {
		t.Errorf("older write should be rejected, blurb = %q, want %q", p.Blurb, "first")
	}

	// Newer update should win.
	err = store.UpsertPing(ctx, Ping{
		Timestamp: ts, Blurb: "newer", NodeID: "c", UpdatedAt: 200,
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err = store.GetPing(ctx, ts)
	if err != nil {
		t.Fatal(err)
	}
	if p.Blurb != "newer" {
		t.Errorf("newer write should win, blurb = %q, want %q", p.Blurb, "newer")
	}
}

func TestPendingPings(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	now := time.Now()

	// Create some pings: two pending (no blurb), one answered, one in the future.
	for _, p := range []Ping{
		{Timestamp: now.Add(-2 * time.Hour).Unix(), Blurb: "", NodeID: "a", UpdatedAt: 0},
		{Timestamp: now.Add(-1 * time.Hour).Unix(), Blurb: "", NodeID: "a", UpdatedAt: 0},
		{Timestamp: now.Add(-30 * time.Minute).Unix(), Blurb: "#working", NodeID: "a", UpdatedAt: 1},
		{Timestamp: now.Add(1 * time.Hour).Unix(), Blurb: "", NodeID: "a", UpdatedAt: 0},
	} {
		if err := store.UpsertPing(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	pending, err := store.PendingPings(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 2 {
		t.Errorf("got %d pending pings, want 2", len(pending))
	}
}

func TestBatchSetBlurb(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	timestamps := []int64{1000, 2000, 3000}
	for _, ts := range timestamps {
		if err := store.UpsertPing(ctx, Ping{Timestamp: ts}); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.BatchSetBlurb(ctx, timestamps, "#sleeping", "phone"); err != nil {
		t.Fatal(err)
	}

	for _, ts := range timestamps {
		p, err := store.GetPing(ctx, ts)
		if err != nil {
			t.Fatal(err)
		}
		if p.Blurb != "#sleeping" {
			t.Errorf("ping %d: blurb = %q, want #sleeping", ts, p.Blurb)
		}
	}
}

func TestSearchBlurbs(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	pings := []Ping{
		{Timestamp: 1000, Blurb: "working on #code for the frontend", NodeID: "a", UpdatedAt: 1},
		{Timestamp: 2000, Blurb: "#meeting with design team", NodeID: "a", UpdatedAt: 2},
		{Timestamp: 3000, Blurb: "more #code backend stuff", NodeID: "a", UpdatedAt: 3},
	}
	for _, p := range pings {
		if err := store.UpsertPing(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	results, err := store.SearchBlurbs(ctx, "code", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("got %d results for 'code', want 2", len(results))
	}

	results, err = store.SearchBlurbs(ctx, "meeting", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("got %d results for 'meeting', want 1", len(results))
	}
}

func TestPeriodChanges(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	changes := []PeriodChange{
		{Timestamp: 1000, Seed: 42, PeriodSecs: 2700},
		{Timestamp: 5000, Seed: 42, PeriodSecs: 900},
	}
	for _, c := range changes {
		if err := store.AddPeriodChange(ctx, c); err != nil {
			t.Fatal(err)
		}
	}

	got, err := store.ListPeriodChanges(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d changes, want 2", len(got))
	}
	if got[0].PeriodSecs != 2700 || got[1].PeriodSecs != 900 {
		t.Errorf("unexpected period changes: %+v", got)
	}

	// Idempotent: re-insert same timestamp overwrites.
	if err := store.AddPeriodChange(ctx, PeriodChange{Timestamp: 1000, Seed: 99, PeriodSecs: 1800}); err != nil {
		t.Fatal(err)
	}
	got, err = store.ListPeriodChanges(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 changes after idempotent insert, got %d", len(got))
	}
	if got[0].Seed != 99 {
		t.Errorf("seed should have been updated to 99, got %d", got[0].Seed)
	}
}

func TestEnsurePingsExist(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	// Pre-existing ping with a blurb.
	if err := store.UpsertPing(ctx, Ping{Timestamp: 1000, Blurb: "#working", UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}

	// EnsurePingsExist should not overwrite existing pings.
	if err := store.EnsurePingsExist(ctx, []int64{1000, 2000, 3000}); err != nil {
		t.Fatal(err)
	}

	p, err := store.GetPing(ctx, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if p.Blurb != "#working" {
		t.Errorf("existing ping overwritten: blurb = %q", p.Blurb)
	}

	p, err = store.GetPing(ctx, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Error("new ping 2000 was not created")
	}
}

func TestMetaGetSet(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	val, err := store.GetMeta(ctx, "test_key")
	if err != nil {
		t.Fatal(err)
	}
	if val != "" {
		t.Errorf("expected empty, got %q", val)
	}

	if err := store.SetMeta(ctx, "test_key", "test_value"); err != nil {
		t.Fatal(err)
	}

	val, err = store.GetMeta(ctx, "test_key")
	if err != nil {
		t.Fatal(err)
	}
	if val != "test_value" {
		t.Errorf("got %q, want %q", val, "test_value")
	}
}

func TestUnsyncedPings(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	// Insert two pings: one synced, one not.
	if err := store.UpsertPing(ctx, Ping{Timestamp: 1000, Blurb: "a", UpdatedAt: 100, SyncedAt: 50}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertPing(ctx, Ping{Timestamp: 2000, Blurb: "b", UpdatedAt: 200, SyncedAt: 0}); err != nil {
		t.Fatal(err)
	}
	// Empty ping (updated_at = 0) should not be considered unsynced.
	if err := store.UpsertPing(ctx, Ping{Timestamp: 3000, UpdatedAt: 0, SyncedAt: 0}); err != nil {
		t.Fatal(err)
	}

	unsynced, err := store.UnsyncedPings(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(unsynced) != 1 {
		t.Errorf("got %d unsynced pings, want 1", len(unsynced))
	}
	if len(unsynced) > 0 && unsynced[0].Timestamp != 2000 {
		t.Errorf("expected timestamp 2000, got %d", unsynced[0].Timestamp)
	}
}

func TestBackfillPingTags(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	// Insert pings with blurbs before the backfill already ran in OpenStore.
	// Since OpenStore already backfilled, insert new pings and verify
	// ensureTagsFromBlurb works via SetBlurb.
	if err := store.SetBlurb(ctx, 1000, "#work on #code", "a"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetBlurb(ctx, 2000, "#sleeping", "a"); err != nil {
		t.Fatal(err)
	}

	tags, err := store.ListTags(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"code", "sleeping", "work"}
	if len(tags) != len(want) {
		t.Fatalf("ListTags = %v, want %v", tags, want)
	}
	for i, tag := range tags {
		if tag != want[i] {
			t.Errorf("tag[%d] = %q, want %q", i, tag, want[i])
		}
	}
}

func TestTagsForPing(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	if err := store.SetBlurb(ctx, 1000, "#work on #code", "a"); err != nil {
		t.Fatal(err)
	}

	tags, err := store.TagsForPing(ctx, 1000)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"code", "work"}
	if len(tags) != len(want) {
		t.Fatalf("TagsForPing = %v, want %v", tags, want)
	}
	for i, tag := range tags {
		if tag != want[i] {
			t.Errorf("tag[%d] = %q, want %q", i, tag, want[i])
		}
	}
}

func TestRenameTagTimeScoped(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	// T1: add #sleep
	if err := store.SetBlurb(ctx, 1000, "#sleep", "a"); err != nil {
		t.Fatal(err)
	}

	// T3: rename sleep → sleeping (applies to pings at or before now)
	if err := store.RenameTag(ctx, "sleep", "sleeping", "a"); err != nil {
		t.Fatal(err)
	}

	// T1's tag should now be "sleeping"
	tags, err := store.TagsForPing(ctx, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0] != "sleeping" {
		t.Errorf("after rename, TagsForPing(1000) = %v, want [sleeping]", tags)
	}

	// T4: add #sleep again (after rename)
	if err := store.SetBlurb(ctx, 9999999999, "#sleep", "a"); err != nil {
		t.Fatal(err)
	}

	// T4's tag should still be "sleep" (not renamed, it's after T3)
	tags, err = store.TagsForPing(ctx, 9999999999)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0] != "sleep" {
		t.Errorf("post-rename ping TagsForPing(9999999999) = %v, want [sleep]", tags)
	}

	// Both tags should exist
	allTags, err := store.ListTags(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(allTags) != 2 {
		t.Errorf("ListTags = %v, want [sleep, sleeping]", allTags)
	}
}

func TestRenameTagRemovesOrphan(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	// Only one ping with #sleep
	if err := store.SetBlurb(ctx, 1000, "#sleep", "a"); err != nil {
		t.Fatal(err)
	}

	// Rename: sleep → sleeping. Since all pings are before now, "sleep" has no remaining refs.
	if err := store.RenameTag(ctx, "sleep", "sleeping", "a"); err != nil {
		t.Fatal(err)
	}

	tags, err := store.ListTags(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0] != "sleeping" {
		t.Errorf("ListTags = %v, want [sleeping]", tags)
	}
}

func TestListTagRenames(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	if err := store.SetBlurb(ctx, 1000, "#sleep", "a"); err != nil {
		t.Fatal(err)
	}
	if err := store.RenameTag(ctx, "sleep", "sleeping", "a"); err != nil {
		t.Fatal(err)
	}

	renames, err := store.ListTagRenames(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(renames) != 1 {
		t.Fatalf("got %d renames, want 1", len(renames))
	}
	if renames[0].OldName != "sleep" || renames[0].NewName != "sleeping" {
		t.Errorf("rename = %+v, want sleep→sleeping", renames[0])
	}
}

func TestBatchSetBlurbMaintainsTags(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	timestamps := []int64{1000, 2000}
	for _, ts := range timestamps {
		if err := store.UpsertPing(ctx, Ping{Timestamp: ts}); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.BatchSetBlurb(ctx, timestamps, "#sleeping", "a"); err != nil {
		t.Fatal(err)
	}

	for _, ts := range timestamps {
		tags, err := store.TagsForPing(ctx, ts)
		if err != nil {
			t.Fatal(err)
		}
		if len(tags) != 1 || tags[0] != "sleeping" {
			t.Errorf("TagsForPing(%d) = %v, want [sleeping]", ts, tags)
		}
	}

	allTags, err := store.ListTags(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(allTags) != 1 || allTags[0] != "sleeping" {
		t.Errorf("ListTags = %v, want [sleeping]", allTags)
	}
}
