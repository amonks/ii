package node

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"monks.co/pkg/serve"
)

//go:embed web/*
var webFS embed.FS

var templates *template.Template

func init() {
	funcMap := template.FuncMap{
		"formatTime": func(ts int64) string {
			return time.Unix(ts, 0).Local().Format("Mon Jan 2 3:04 PM")
		},
		"formatDuration": func(secs float64) string {
			h := int(secs) / 3600
			m := (int(secs) % 3600) / 60
			if h > 0 {
				return fmt.Sprintf("%dh %dm", h, m)
			}
			return fmt.Sprintf("%dm", m)
		},
		"periodMinutes": func(secs int64) int64 {
			return secs / 60
		},
		"highlightTags": func(blurb string) template.HTML {
			words := strings.Fields(blurb)
			for i, w := range words {
				if strings.HasPrefix(w, "#") && len(w) > 1 {
					words[i] = `<span class="tag">` + template.HTMLEscapeString(w) + `</span>`
				} else {
					words[i] = template.HTMLEscapeString(w)
				}
			}
			return template.HTML(strings.Join(words, " "))
		},
	}

	templates = template.Must(template.New("").Funcs(funcMap).ParseFS(webFS, "web/*.html"))
}

type handler struct {
	store          *Store
	changes        func() []PeriodChange
	refreshChanges func()
	syncNow        func(context.Context) error
	nodeID         string
	upstream       string
}

func newHandler(store *Store, changes func() []PeriodChange, refreshChanges func(), syncNow func(context.Context) error, nodeID, upstream string) http.Handler {
	h := &handler{store: store, changes: changes, refreshChanges: refreshChanges, syncNow: syncNow, nodeID: nodeID, upstream: upstream}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.handleIndex)
	mux.HandleFunc("GET /pings", h.handlePingsJSON)
	mux.HandleFunc("POST /answer", h.handleAnswer)
	mux.HandleFunc("POST /batch-answer", h.handleBatchAnswer)
	mux.HandleFunc("GET /search", h.handleSearch)
	mux.HandleFunc("GET /search/data", h.handleSearchData)
	mux.HandleFunc("GET /graphs", h.handleGraphs)
	mux.HandleFunc("GET /graphs/data", h.handleGraphsData)
	mux.HandleFunc("GET /settings", h.handleSettings)
	mux.HandleFunc("POST /settings/period", h.handleSettingsPeriod)
	mux.HandleFunc("POST /sync/push", h.handleSyncPush)
	mux.HandleFunc("GET /sync/pull", h.handleSyncPull)
	mux.HandleFunc("GET /sync/period-changes", h.handleSyncPeriodChanges)
	mux.HandleFunc("GET /sync/status", h.handleSyncStatus)
	mux.HandleFunc("POST /sync/now", h.handleSyncNow)
	mux.HandleFunc("GET /next-ping", h.handleNextPing)
	mux.HandleFunc("GET /style.css", h.handleStatic)
	mux.HandleFunc("GET /graphs.js", h.handleStatic)
	return mux
}

type indexData struct {
	BasePath      string
	Pending       []Ping
	Recent        []Ping
	InitializedAt time.Time
	NextPingUnix  int64
}

func (h *handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()

	// Ensure pings exist for the recent schedule.
	changes := h.changes()
	start := now.Add(-24 * time.Hour)
	scheduledPings := PingsInRange(changes, start, now)
	if len(scheduledPings) > 0 {
		timestamps := make([]int64, len(scheduledPings))
		for i, p := range scheduledPings {
			timestamps[i] = p.Unix()
		}
		if err := h.store.EnsurePingsExist(ctx, timestamps); err != nil {
			slog.Error("ensuring pings exist", "error", err)
		}
	}

	pending, err := h.store.PendingPings(ctx, now)
	if err != nil {
		serve.InternalServerError(w, r, err)
		return
	}
	recent, err := h.store.RecentPings(ctx, 20)
	if err != nil {
		serve.InternalServerError(w, r, err)
		return
	}

	var initializedAt time.Time
	if len(changes) > 0 {
		initializedAt = time.Unix(changes[0].Timestamp, 0)
	}

	nextPing := NextPing(changes, now)

	templates.ExecuteTemplate(w, "index.html", indexData{
		BasePath:      serve.BasePath(r),
		Pending:       pending,
		Recent:        recent,
		InitializedAt: initializedAt,
		NextPingUnix:  nextPing.Unix(),
	})
}

func (h *handler) handlePingsJSON(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()

	// Ensure pings exist for the recent schedule.
	changes := h.changes()
	start := now.Add(-24 * time.Hour)
	scheduledPings := PingsInRange(changes, start, now)
	if len(scheduledPings) > 0 {
		timestamps := make([]int64, len(scheduledPings))
		for i, p := range scheduledPings {
			timestamps[i] = p.Unix()
		}
		if err := h.store.EnsurePingsExist(ctx, timestamps); err != nil {
			slog.Error("ensuring pings exist", "error", err)
		}
	}

	pending, err := h.store.PendingPings(ctx, now)
	if err != nil {
		serve.InternalServerError(w, r, err)
		return
	}
	recent, err := h.store.RecentPings(ctx, 20)
	if err != nil {
		serve.InternalServerError(w, r, err)
		return
	}

	var initializedAt int64
	if len(changes) > 0 {
		initializedAt = changes[0].Timestamp
	}

	serve.JSON(w, r, struct {
		Pending       []Ping `json:"pending"`
		Recent        []Ping `json:"recent"`
		InitializedAt int64  `json:"initialized_at"`
	}{Pending: pending, Recent: recent, InitializedAt: initializedAt})
}

func (h *handler) handleAnswer(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	ts, err := strconv.ParseInt(r.FormValue("timestamp"), 10, 64)
	if err != nil {
		http.Error(w, "invalid timestamp", 400)
		return
	}
	blurb := r.FormValue("blurb")
	if err := h.store.SetBlurb(r.Context(), ts, blurb, h.nodeID); err != nil {
		serve.InternalServerError(w, r, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *handler) handleBatchAnswer(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	blurb := r.FormValue("blurb")
	tsStrs := r.Form["timestamps"]
	var timestamps []int64
	for _, s := range tsStrs {
		ts, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			continue
		}
		timestamps = append(timestamps, ts)
	}
	if len(timestamps) == 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if err := h.store.BatchSetBlurb(r.Context(), timestamps, blurb, h.nodeID); err != nil {
		serve.InternalServerError(w, r, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

type searchData struct {
	BasePath string
	Query    string
	Results  []Ping
}

func (h *handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	var results []Ping
	if q != "" {
		var err error
		results, err = h.store.SearchBlurbs(r.Context(), q, 50)
		if err != nil {
			serve.InternalServerError(w, r, err)
			return
		}
	}
	templates.ExecuteTemplate(w, "search.html", searchData{
		BasePath: serve.BasePath(r),
		Query:    q,
		Results:  results,
	})
}

func (h *handler) handleSearchData(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	var results []Ping
	if q != "" {
		var err error
		results, err = h.store.SearchBlurbs(r.Context(), q, 50)
		if err != nil {
			serve.InternalServerError(w, r, err)
			return
		}
	}
	serve.JSON(w, r, struct {
		Query   string `json:"query"`
		Results []Ping `json:"results"`
	}{Query: q, Results: results})
}

type graphsPageData struct {
	BasePath string
	Window   string
}

func (h *handler) handleGraphs(w http.ResponseWriter, r *http.Request) {
	window := r.URL.Query().Get("window")
	if window == "" {
		window = "day"
	}
	templates.ExecuteTemplate(w, "graphs.html", graphsPageData{
		BasePath: serve.BasePath(r),
		Window:   window,
	})
}

func (h *handler) handleGraphsData(w http.ResponseWriter, r *http.Request) {
	window := r.URL.Query().Get("window")
	if window == "" {
		window = "day"
	}

	now := time.Now().UTC()
	var start, end time.Time
	if s := r.URL.Query().Get("start"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			start = t
		}
	}
	if e := r.URL.Query().Get("end"); e != "" {
		if t, err := time.Parse(time.RFC3339, e); err == nil {
			end = t
		}
	}

	if start.IsZero() || end.IsZero() {
		switch window {
		case "hour":
			start = now.Add(-24 * time.Hour)
		case "day":
			start = now.Add(-30 * 24 * time.Hour)
		case "week":
			start = now.Add(-12 * 7 * 24 * time.Hour)
		default:
			start = now.Add(-30 * 24 * time.Hour)
		}
		end = now
	}

	changes := h.changes()
	pings, err := h.store.PingsInTimeRange(r.Context(), start, end)
	if err != nil {
		serve.InternalServerError(w, r, err)
		return
	}

	data := ComputeGraphData(pings, changes, window, start, end)
	// Sort all_tags deterministically.
	sort.Strings(data.AllTags)
	serve.JSON(w, r, data)
}

type settingsData struct {
	BasePath string
	Changes  []PeriodChange
	Current  *PeriodChange
}

func (h *handler) handleSettings(w http.ResponseWriter, r *http.Request) {
	changes := h.changes()
	var current *PeriodChange
	if len(changes) > 0 {
		current = &changes[len(changes)-1]
	}
	templates.ExecuteTemplate(w, "settings.html", settingsData{
		BasePath: serve.BasePath(r),
		Changes:  changes,
		Current:  current,
	})
}

func (h *handler) handleSettingsPeriod(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	periodMins, err := strconv.ParseInt(r.FormValue("period_minutes"), 10, 64)
	if err != nil || periodMins < 1 {
		http.Error(w, "invalid period", 400)
		return
	}

	changes := h.changes()
	seed := uint64(12345)
	if len(changes) > 0 {
		seed = changes[len(changes)-1].Seed
	}

	change := PeriodChange{
		Timestamp:  time.Now().Unix(),
		Seed:       seed,
		PeriodSecs: periodMins * 60,
	}
	if err := h.store.AddPeriodChange(r.Context(), change); err != nil {
		serve.InternalServerError(w, r, err)
		return
	}
	h.refreshChanges()
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// Sync endpoints.

func (h *handler) handleSyncPush(w http.ResponseWriter, r *http.Request) {
	var payload syncPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	now := time.Now().UnixNano()
	for _, p := range payload.Pings {
		p.ReceivedAt = now
		if err := h.store.UpsertPing(r.Context(), p); err != nil {
			serve.InternalServerError(w, r, err)
			return
		}
	}
	for _, c := range payload.PeriodChanges {
		if err := h.store.AddPeriodChange(r.Context(), c); err != nil {
			serve.InternalServerError(w, r, err)
			return
		}
	}
	if len(payload.PeriodChanges) > 0 {
		h.refreshChanges()
	}
	w.WriteHeader(http.StatusOK)
}

func (h *handler) handleSyncPull(w http.ResponseWriter, r *http.Request) {
	sinceStr := r.URL.Query().Get("since")
	since, _ := strconv.ParseInt(sinceStr, 10, 64)

	pings, err := h.store.PingsReceivedAfter(r.Context(), since, 1000)
	if err != nil {
		serve.InternalServerError(w, r, err)
		return
	}
	changes, err := h.store.ListPeriodChanges(r.Context())
	if err != nil {
		serve.InternalServerError(w, r, err)
		return
	}
	serve.JSON(w, r, syncPayload{Pings: pings, PeriodChanges: changes})
}

func (h *handler) handleSyncPeriodChanges(w http.ResponseWriter, r *http.Request) {
	changes, err := h.store.ListPeriodChanges(r.Context())
	if err != nil {
		serve.InternalServerError(w, r, err)
		return
	}
	serve.JSON(w, r, changes)
}

type syncStatus struct {
	HasUpstream   bool   `json:"has_upstream"`
	Upstream      string `json:"upstream,omitempty"`
	UnsyncedCount int    `json:"unsynced_count"`
	PullWatermark string `json:"pull_watermark"`
	LastPushAt    string `json:"last_push_at"`
	LastPullAt    string `json:"last_pull_at"`
}

func (h *handler) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	unsynced, err := h.store.UnsyncedPings(ctx, 10000)
	if err != nil {
		serve.InternalServerError(w, r, err)
		return
	}
	watermark, err := h.store.GetMeta(ctx, "pull_watermark")
	if err != nil {
		serve.InternalServerError(w, r, err)
		return
	}
	lastPushAt, err := h.store.GetMeta(ctx, "last_push_at")
	if err != nil {
		serve.InternalServerError(w, r, err)
		return
	}
	lastPullAt, err := h.store.GetMeta(ctx, "last_pull_at")
	if err != nil {
		serve.InternalServerError(w, r, err)
		return
	}
	serve.JSON(w, r, syncStatus{
		HasUpstream:   h.upstream != "",
		Upstream:      h.upstream,
		UnsyncedCount: len(unsynced),
		PullWatermark: watermark,
		LastPushAt:    lastPushAt,
		LastPullAt:    lastPullAt,
	})
}

func (h *handler) handleSyncNow(w http.ResponseWriter, r *http.Request) {
	if h.syncNow == nil {
		http.Error(w, "no upstream configured", http.StatusBadRequest)
		return
	}
	if err := h.syncNow(r.Context()); err != nil {
		serve.InternalServerError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *handler) handleNextPing(w http.ResponseWriter, r *http.Request) {
	changes := h.changes()
	nStr := r.URL.Query().Get("n")
	n, _ := strconv.Atoi(nStr)
	if n <= 1 {
		next := NextPing(changes, time.Now())
		serve.JSON(w, r, map[string]int64{"timestamp": next.Unix()})
		return
	}
	if n > 64 {
		n = 64
	}
	var timestamps []int64
	after := time.Now()
	for range n {
		next := NextPing(changes, after)
		timestamps = append(timestamps, next.Unix())
		after = next
	}
	serve.JSON(w, r, map[string]any{"timestamps": timestamps})
}

func (h *handler) handleStatic(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	data, err := webFS.ReadFile("web/" + name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch {
	case strings.HasSuffix(name, ".css"):
		w.Header().Set("Content-Type", "text/css")
	case strings.HasSuffix(name, ".js"):
		w.Header().Set("Content-Type", "application/javascript")
	}
	w.Write(data)
}
