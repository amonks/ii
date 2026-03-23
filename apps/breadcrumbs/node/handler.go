package node

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	pb "monks.co/apps/breadcrumbs/proto"
	"monks.co/pkg/serve"
	"google.golang.org/protobuf/proto"
)

//go:embed web
var webContent embed.FS

// handler wires together the store, simplifier, and hub to serve HTTP.
type handler struct {
	store      *Store
	simplifier *Simplifier
	hub        *Hub
	config     *Config
	forwarder  *Forwarder // nil if no upstream
	mux        *http.ServeMux
	httpClient *http.Client
}

func newHandler(store *Store, simplifier *Simplifier, hub *Hub, config *Config, forwarder *Forwarder) http.Handler {
	h := &handler{
		store:      store,
		simplifier: simplifier,
		hub:        hub,
		config:     config,
		forwarder:  forwarder,
		mux:        http.NewServeMux(),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	h.mux.HandleFunc("GET /{$}", h.handleIndex)
	h.mux.HandleFunc("POST /ingest", h.handleIngest)
	h.mux.HandleFunc("GET /tiles/{z}/{x}/{y}", h.handleTile)
	h.mux.HandleFunc("GET /events", h.handleEvents)
	h.mux.HandleFunc("POST /flush", h.handleFlush)
	h.mux.HandleFunc("GET /stats", h.handleStats)
	h.mux.HandleFunc("GET /debug/significance", h.handleDebugSignificance)
	h.mux.HandleFunc("POST /recompute", h.handleRecompute)
	return h
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *handler) handleIngest(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "reading body: "+err.Error(), http.StatusBadRequest)
		return
	}

	var track pb.Track
	if err := proto.Unmarshal(body, &track); err != nil {
		http.Error(w, "invalid protobuf: "+err.Error(), http.StatusBadRequest)
		return
	}

	var watermark int64
	for _, p := range track.Points {
		result := h.simplifier.Append(p)

		// Check if point matches any subscription.
		subscribed := matchesSubscription(p, result.NewPointSig, h.config.Subscriptions)

		if err := h.store.InsertPoint(r.Context(), p, result.NewPointSig, subscribed); err != nil {
			http.Error(w, "storing point: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if result.HasPrevUpdate {
			if err := h.store.UpdateSignificance(r.Context(), result.PrevTailTimestamp, result.PrevTailSig); err != nil {
				http.Error(w, "updating significance: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		if p.Timestamp > watermark {
			watermark = p.Timestamp
		}
	}

	// Run eviction if capacity is configured.
	if h.config.Capacity > 0 {
		// For now (no upstream), all points are eligible for eviction.
		h.store.Evict(r.Context(), h.config.Capacity, math.MaxInt64)
	}

	resp := &pb.IngestResponse{Watermark: watermark}
	data, err := proto.Marshal(resp)
	if err != nil {
		http.Error(w, "encoding response: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/protobuf")
	w.Write(data)
}

func (h *handler) handleTile(w http.ResponseWriter, r *http.Request) {
	z, err := strconv.Atoi(r.PathValue("z"))
	if err != nil {
		http.Error(w, "invalid z", http.StatusBadRequest)
		return
	}
	x, err := strconv.Atoi(r.PathValue("x"))
	if err != nil {
		http.Error(w, "invalid x", http.StatusBadRequest)
		return
	}
	y, err := strconv.Atoi(r.PathValue("y"))
	if err != nil {
		http.Error(w, "invalid y", http.StatusBadRequest)
		return
	}

	south, north, west, east, err := TileBBox(z, x, y)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	detail := 5.0 // default
	if d := r.URL.Query().Get("detail"); d != "" {
		if parsed, err := strconv.ParseFloat(d, 64); err == nil {
			detail = parsed
		}
	}
	minSig := SignificanceThreshold(z, detail)

	// Expand the query bbox by a buffer so the line extends past tile edges.
	// This eliminates gaps at tile boundaries. The MVT encoder will produce
	// coordinates outside [0, extent] for buffered points, which is valid —
	// MapLibre clips them to the tile.
	latBuf := (north - south) * 0.1
	lonBuf := (east - west) * 0.1
	points, err := h.store.QueryTile(r.Context(),
		south-latBuf, north+latBuf, west-lonBuf, east+lonBuf, minSig)
	if err != nil {
		http.Error(w, "querying tile: "+err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Debug("tile",
		"z", z, "x", x, "y", y,
		"detail", detail,
		"threshold", minSig,
		"points", len(points),
	)

	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/protobuf") {
		// Node-to-node requests explicitly ask for protobuf.
		track := &pb.Track{Points: points}
		data, err := proto.Marshal(track)
		if err != nil {
			http.Error(w, "encoding protobuf: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/protobuf")
		w.Write(data)
	} else {
		// Default to MVT for map clients (MapLibre, browsers, etc.).
		data, err := EncodeMVT(points, z, x, y)
		if err != nil {
			http.Error(w, "encoding MVT: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.mapbox-vector-tile")
		w.Write(data)
	}

	// Background read-through: fetch from upstream and notify client if data changed.
	if h.config.Upstream != "" {
		clientID := r.URL.Query().Get("client")
		go h.readThrough(z, x, y, clientID)
	}
}

func (h *handler) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	clientID := r.URL.Query().Get("client")
	if clientID == "" {
		http.Error(w, "missing client query parameter", http.StatusBadRequest)
		return
	}

	ch, unsub := h.hub.Subscribe(clientID)
	defer unsub()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	keepalive := time.NewTicker(30 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: tile-updated\ndata: %s\n\n", msg)
			flusher.Flush()
		case <-keepalive.C:
			fmt.Fprintf(w, ":keepalive\n\n")
			flusher.Flush()
		}
	}
}

func (h *handler) handleFlush(w http.ResponseWriter, r *http.Request) {
	var forwarded int32
	var watermark int64

	if h.forwarder != nil {
		var err error
		forwarded, watermark, err = h.forwarder.Forward(r.Context())
		if err != nil {
			http.Error(w, "forwarding: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	resp := &pb.FlushResponse{Watermark: watermark, PointsForwarded: forwarded}
	data, err := proto.Marshal(resp)
	if err != nil {
		http.Error(w, "encoding response: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/protobuf")
	w.Write(data)
}

func (h *handler) handleStats(w http.ResponseWriter, r *http.Request) {
	count, latest, err := h.store.Stats(r.Context())
	if err != nil {
		http.Error(w, "querying stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	watermark, err := h.store.GetWatermark(r.Context())
	if err != nil {
		http.Error(w, "querying watermark: "+err.Error(), http.StatusInternalServerError)
		return
	}

	queueSize, err := h.store.ForwardQueueSize(r.Context(), watermark)
	if err != nil {
		http.Error(w, "querying queue size: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := &pb.StatsResponse{
		Count:            count,
		LatestPoint:      latest,
		ForwardWatermark: watermark,
		ForwardQueueSize: queueSize,
	}

	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		out := map[string]any{
			"count":              resp.Count,
			"forward_watermark":  resp.ForwardWatermark,
			"forward_queue_size": resp.ForwardQueueSize,
		}
		if resp.LatestPoint != nil {
			out["latest_point"] = map[string]any{
				"timestamp": resp.LatestPoint.Timestamp,
				"latitude":  resp.LatestPoint.Latitude,
				"longitude": resp.LatestPoint.Longitude,
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
		return
	}

	data, err := proto.Marshal(resp)
	if err != nil {
		http.Error(w, "encoding response: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/protobuf")
	w.Write(data)
}

func matchesSubscription(p *pb.Point, significance float64, subs []Subscription) bool {
	for _, sub := range subs {
		west, south, east, north := sub.BBox[0], sub.BBox[1], sub.BBox[2], sub.BBox[3]
		if p.Latitude >= south && p.Latitude <= north &&
			p.Longitude >= west && p.Longitude <= east &&
			significance >= sub.MinSignificance {
			return true
		}
	}
	return false
}

// notifyTileUpdated sends a tile-updated SSE event to the specified client.
func (h *handler) notifyTileUpdated(clientID string, z, x, y int) {
	data, _ := json.Marshal(pb.TileUpdated{Z: int32(z), X: int32(x), Y: int32(y)})
	h.hub.Publish(clientID, data)
}

func (h *handler) handleRecompute(w http.ResponseWriter, r *http.Request) {
	method := r.URL.Query().Get("method")
	if !ValidSimplifyMethod(method) {
		http.Error(w, fmt.Sprintf("invalid method %q, want \"area\", \"distance\", or \"distance_floor\"", method), http.StatusBadRequest)
		return
	}

	slog.Info("recomputing significance", "method", method)
	start := time.Now()
	n, err := h.store.RecomputeSignificance(r.Context(), SimplifyMethod(method))
	elapsed := time.Since(start)
	if err != nil {
		http.Error(w, "recompute failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Update the simplifier so new ingests use the same method.
	h.simplifier.Method = SimplifyMethod(method)
	_ = h.store.SetMeta(r.Context(), "simplify_method", method)

	slog.Info("recompute complete", "method", method, "points", n, "duration", elapsed)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"points":      n,
		"method":      method,
		"duration_ms": elapsed.Milliseconds(),
	})
}

func (h *handler) handleDebugSignificance(w http.ResponseWriter, r *http.Request) {
	stats, err := h.store.SignificanceStats(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	detail := 5.0
	if d := r.URL.Query().Get("detail"); d != "" {
		if parsed, err := strconv.ParseFloat(d, 64); err == nil {
			detail = parsed
		}
	}

	type zoomInfo struct {
		Zoom      int     `json:"zoom"`
		Threshold float64 `json:"threshold"`
	}

	// For each percentile, find the zoom where that fraction of points first appears.
	type percentileInfo struct {
		Label string  `json:"label"`
		Sig   float64 `json:"sig"`
		Zoom  int     `json:"visible_at_zoom"`
	}
	var percentiles []percentileInfo
	for _, p := range []struct {
		label string
		sig   float64
	}{
		{"min", stats.Min},
		{"p25", stats.P25},
		{"p50", stats.P50},
		{"p75", stats.P75},
		{"max", stats.Max},
	} {
		z := 22
		if p.sig > 0 {
			z = zoomForSigAtDetail(p.sig, detail)
		}
		percentiles = append(percentiles, percentileInfo{p.label, p.sig, z})
	}

	// Thresholds at each zoom level.
	var zooms []zoomInfo
	for z := 0; z <= 22; z++ {
		zooms = append(zooms, zoomInfo{z, SignificanceThreshold(z, detail)})
	}

	resp := struct {
		Count       int64            `json:"count"`
		Detail      float64          `json:"detail"`
		Percentiles []percentileInfo `json:"percentiles"`
		Zooms       []zoomInfo       `json:"zooms"`
	}{stats.Count, detail, percentiles, zooms}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := webContent.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "reading index: "+err.Error(), http.StatusInternalServerError)
		return
	}
	basePath := serve.BasePathFromContext(r.Context())
	html := strings.Replace(string(data), "{{BASE_PATH}}", basePath, 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// readThrough fetches a tile from upstream, writes new points, and notifies
// the client if data changed. Runs in a background goroutine.
func (h *handler) readThrough(z, x, y int, clientID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	points, err := FetchTileFromUpstream(ctx, h.httpClient, h.config.Upstream, z, x, y)
	if err != nil {
		// Upstream fetch failed silently — offline is fine.
		return
	}

	changed, err := WriteUpstreamPoints(ctx, h.store, points, h.config.Subscriptions)
	if err != nil {
		return
	}

	if changed && clientID != "" {
		h.notifyTileUpdated(clientID, z, x, y)
	}
}
