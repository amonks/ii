package node

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestHandler(t *testing.T) (http.Handler, *Store) {
	t.Helper()
	ctx := context.Background()
	store, err := OpenStore(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	// Add a default period change.
	if err := store.AddPeriodChange(ctx, PeriodChange{Timestamp: 0, Seed: 42, PeriodSecs: 2700}); err != nil {
		t.Fatal(err)
	}

	changes := func() []PeriodChange {
		c, _ := store.ListPeriodChanges(ctx)
		return c
	}
	h := newHandler(store, changes, func() {}, "test", "")
	return h, store
}

func TestHandlerIndex(t *testing.T) {
	h, _ := newTestHandler(t)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("GET / = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "TagTime") {
		t.Error("index page missing title")
	}
	if !strings.Contains(body, "next-ping-time") {
		t.Error("index page missing next ping display")
	}
}

func TestHandlerPingsJSON(t *testing.T) {
	h, store := newTestHandler(t)
	ctx := context.Background()

	now := time.Now()
	pastTS := now.Add(-1 * time.Hour).Unix()
	answeredTS := now.Add(-2 * time.Hour).Unix()

	// Create a pending ping and an answered ping.
	if err := store.EnsurePingsExist(ctx, []int64{pastTS}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertPing(ctx, Ping{Timestamp: answeredTS, Blurb: "#test", NodeID: "test", UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/pings", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("GET /pings = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"pending"`) {
		t.Error("response missing pending field")
	}
	if !strings.Contains(body, `"recent"`) {
		t.Error("response missing recent field")
	}
	if !strings.Contains(body, "#test") {
		t.Error("response missing answered ping in recent")
	}
}

func TestHandlerAnswer(t *testing.T) {
	h, store := newTestHandler(t)
	ctx := context.Background()

	// Create a pending ping.
	if err := store.EnsurePingsExist(ctx, []int64{1000}); err != nil {
		t.Fatal(err)
	}

	form := url.Values{
		"timestamp": {"1000"},
		"blurb":     {"#working on tests"},
	}
	req := httptest.NewRequest("POST", "/answer", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("POST /answer = %d, want 303", w.Code)
	}

	p, err := store.GetPing(ctx, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if p.Blurb != "#working on tests" {
		t.Errorf("blurb = %q, want %q", p.Blurb, "#working on tests")
	}
}

func TestHandlerEditAnswer(t *testing.T) {
	h, store := newTestHandler(t)
	ctx := context.Background()

	// Create and answer a ping.
	if err := store.SetBlurb(ctx, 1000, "#working", "test"); err != nil {
		t.Fatal(err)
	}

	// Edit the answer via POST /answer.
	form := url.Values{
		"timestamp": {"1000"},
		"blurb":     {"#meeting with team"},
	}
	req := httptest.NewRequest("POST", "/answer", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("POST /answer (edit) = %d, want 303", w.Code)
	}

	p, err := store.GetPing(ctx, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if p.Blurb != "#meeting with team" {
		t.Errorf("edited blurb = %q, want %q", p.Blurb, "#meeting with team")
	}
}

func TestHandlerIndexShowsEditForm(t *testing.T) {
	h, store := newTestHandler(t)
	ctx := context.Background()

	// Create an answered ping.
	if err := store.SetBlurb(ctx, 1000, "#coding", "test"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "edit-form") {
		t.Error("recent pings should have edit forms")
	}
}

func TestHandlerBatchAnswer(t *testing.T) {
	h, store := newTestHandler(t)
	ctx := context.Background()

	if err := store.EnsurePingsExist(ctx, []int64{1000, 2000, 3000}); err != nil {
		t.Fatal(err)
	}

	form := url.Values{
		"timestamps": {"1000", "2000", "3000"},
		"blurb":      {"#sleeping"},
	}
	req := httptest.NewRequest("POST", "/batch-answer", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("POST /batch-answer = %d, want 303", w.Code)
	}

	for _, ts := range []int64{1000, 2000, 3000} {
		p, _ := store.GetPing(ctx, ts)
		if p.Blurb != "#sleeping" {
			t.Errorf("ping %d blurb = %q, want #sleeping", ts, p.Blurb)
		}
	}
}

func TestHandlerSearch(t *testing.T) {
	h, store := newTestHandler(t)
	ctx := context.Background()

	if err := store.UpsertPing(ctx, Ping{Timestamp: 1000, Blurb: "working on #code", UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/search?q=code", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("GET /search = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "#code") {
		t.Error("search results missing expected content")
	}
}

func TestHandlerGraphsData(t *testing.T) {
	h, store := newTestHandler(t)
	ctx := context.Background()

	if err := store.UpsertPing(ctx, Ping{Timestamp: 1711929600, Blurb: "#code", UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/graphs/data?window=day&start=2024-04-01T00:00:00Z&end=2024-04-02T00:00:00Z", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("GET /graphs/data = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "buckets") {
		t.Error("response missing buckets")
	}
}

func TestHandlerSettings(t *testing.T) {
	h, _ := newTestHandler(t)
	req := httptest.NewRequest("GET", "/settings", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("GET /settings = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Settings") {
		t.Error("settings page missing title")
	}
}

func TestHandlerNextPing(t *testing.T) {
	h, _ := newTestHandler(t)
	req := httptest.NewRequest("GET", "/next-ping", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("GET /next-ping = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "timestamp") {
		t.Error("response missing timestamp field")
	}
}

func TestHandlerSyncStatus(t *testing.T) {
	h, _ := newTestHandler(t)
	req := httptest.NewRequest("GET", "/sync/status", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("GET /sync/status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "has_upstream") {
		t.Error("response missing has_upstream field")
	}
	if !strings.Contains(body, "unsynced_count") {
		t.Error("response missing unsynced_count field")
	}
}

func TestHandlerSyncStatusWithUpstream(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	if err := store.AddPeriodChange(ctx, PeriodChange{Timestamp: 0, Seed: 42, PeriodSecs: 2700}); err != nil {
		t.Fatal(err)
	}

	changes := func() []PeriodChange {
		c, _ := store.ListPeriodChanges(ctx)
		return c
	}
	h := newHandler(store, changes, func() {}, "test", "http://upstream:8080")

	req := httptest.NewRequest("GET", "/sync/status", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("GET /sync/status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"has_upstream":true`) {
		t.Errorf("expected has_upstream:true, got %s", body)
	}
	if !strings.Contains(body, "upstream:8080") {
		t.Errorf("expected upstream URL in response, got %s", body)
	}
}
