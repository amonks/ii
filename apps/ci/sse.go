package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// runStateEvent is the JSON structure sent to SSE clients.
type runStateEvent struct {
	Run     runJSON                 `json:"run"`
	Jobs    []jobJSON               `json:"jobs"`
	Streams map[string][]streamJSON `json:"streams"`
}

type runJSON struct {
	ID         int64   `json:"id"`
	Status     string  `json:"status"`
	HeadSHA    string  `json:"head_sha"`
	BaseSHA    string  `json:"base_sha"`
	Trigger    string  `json:"trigger"`
	StartedAt  string  `json:"started_at"`
	FinishedAt *string `json:"finished_at,omitempty"`
	MachineID  *string `json:"machine_id,omitempty"`
	Error      *string `json:"error,omitempty"`
}

type jobJSON struct {
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	Status     string  `json:"status"`
	DurationMs *int64  `json:"duration_ms,omitempty"`
	Error      *string `json:"error,omitempty"`
}

type streamJSON struct {
	Name        string  `json:"name"`
	DisplayName string  `json:"display_name"`
	Status      string  `json:"status"`
	DurationMs  *int64  `json:"duration_ms,omitempty"`
	Error       *string `json:"error,omitempty"`
}

// durationFromTimestamps computes duration in milliseconds from RFC3339 timestamps.
// Returns nil if either timestamp is missing or unparseable.
func durationFromTimestamps(startedAt, finishedAt *string) *int64 {
	if startedAt == nil || finishedAt == nil {
		return nil
	}
	start, err := time.Parse(time.RFC3339, *startedAt)
	if err != nil {
		return nil
	}
	end, err := time.Parse(time.RFC3339, *finishedAt)
	if err != nil {
		return nil
	}
	ms := end.Sub(start).Milliseconds()
	return &ms
}

// buildRunState queries the model and output directory to build a full state snapshot.
func buildRunState(model *Model, outputDir string, runID int64) (*runStateEvent, error) {
	run, jobs, err := model.RunWithJobs(runID)
	if err != nil {
		return nil, err
	}

	// Load all streams for this run from DB.
	dbStreams, _ := model.StreamsForRun(runID)

	// Index DB streams by job ID.
	streamsByJobID := make(map[int64][]Stream)
	for _, s := range dbStreams {
		streamsByJobID[s.JobID] = append(streamsByJobID[s.JobID], s)
	}

	state := &runStateEvent{
		Run: runJSON{
			ID:         run.ID,
			Status:     run.Status,
			HeadSHA:    run.HeadSHA,
			BaseSHA:    run.BaseSHA,
			Trigger:    run.Trigger,
			StartedAt:  run.StartedAt,
			FinishedAt: run.FinishedAt,
			MachineID:  run.MachineID,
			Error:      run.Error,
		},
		Jobs:    make([]jobJSON, 0, len(jobs)),
		Streams: make(map[string][]streamJSON),
	}

	for _, j := range jobs {
		dur := j.DurationMs
		if dur == nil {
			dur = durationFromTimestamps(j.StartedAt, j.FinishedAt)
		}
		state.Jobs = append(state.Jobs, jobJSON{
			Name:       j.Name,
			Kind:       j.Kind,
			Status:     j.Status,
			DurationMs: dur,
			Error:       j.Error,
		})

		// Use DB streams if available.
		if jobStreams, ok := streamsByJobID[j.ID]; ok {
			for _, s := range jobStreams {
				sdur := s.DurationMs
				if sdur == nil {
					sdur = durationFromTimestamps(s.StartedAt, s.FinishedAt)
				}
				state.Streams[j.Name] = append(state.Streams[j.Name], streamJSON{
					Name:        s.Name,
					DisplayName: decodeStreamName(s.Name),
					Status:      s.Status,
					DurationMs:  sdur,
					Error:       s.Error,
				})
			}
			continue
		}

		// Fallback: scan output directory for streams (legacy data).
		if j.OutputPath == nil {
			continue
		}
		dir := filepath.Join(outputDir, fmt.Sprintf("%d", runID), j.Name)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := strings.TrimSuffix(e.Name(), ".log")
			state.Streams[j.Name] = append(state.Streams[j.Name], streamJSON{
				Name:        name,
				DisplayName: decodeStreamName(name),
				Status:      j.Status,
			})
		}
	}

	return state, nil
}

func sseRunEventsKey(runID int64) string {
	return fmt.Sprintf("run-events:%d", runID)
}

// publishRunState builds and publishes the current run state to SSE subscribers.
func (a *apiHandler) publishRunState(runID int64) {
	if a.hub == nil {
		return
	}
	state, err := buildRunState(a.model, a.outputDir, runID)
	if err != nil {
		return
	}
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	a.hub.Publish(sseRunEventsKey(runID), data)
}

// closeRunEvents closes all SSE subscriber channels for a run.
func (a *apiHandler) closeRunEvents(runID int64) {
	if a.hub == nil {
		return
	}
	a.hub.CloseAll(sseRunEventsKey(runID))
}

// knownStreams tracks which output streams have been seen so we only publish
// run state on first write to a new stream, not every append.
var knownStreams sync.Map // key: "runID/jobName/stream" → bool

func sseHandler(model *Model, outputDir string, hub *OutputHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid run ID", http.StatusBadRequest)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Send initial state.
		state, err := buildRunState(model, outputDir, id)
		if err != nil {
			http.Error(w, "run not found", http.StatusNotFound)
			return
		}
		data, err := json.Marshal(state)
		if err != nil {
			http.Error(w, "error encoding state", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()

		// If run is already finished, close immediately.
		if state.Run.Status != "running" {
			return
		}

		// Subscribe for updates.
		key := sseRunEventsKey(id)
		ch, unsub := hub.Subscribe(key)
		defer unsub()

		for {
			select {
			case <-r.Context().Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", msg)
				flusher.Flush()
			}
		}
	}
}
