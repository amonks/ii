package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"monks.co/pkg/tailnet"
)

// LogEvent represents a single log event from the logs service.
type LogEvent struct {
	Time       string `json:"time"`
	Level      string `json:"level"`
	Msg        string `json:"msg"`
	App        string `json:"app.name"`
	Method     string `json:"http.method"`
	Path       string `json:"http.path"`
	Status     int    `json:"http.status"`
	DurationMs float64 `json:"http.duration_ms"`
	RequestID  string `json:"req.id"`

	// CI-specific fields
	TriggerSHA    string `json:"trigger.sha"`
	TriggerRunID  any    `json:"trigger.run_id"`
	TriggerBase   string `json:"trigger.base_sha"`
	APIRunID      any    `json:"api.run_id"`
	APIJobName    string `json:"api.job_name"`
	ErrorMessage  string `json:"err.message"`

	// Raw JSON for display
	Raw json.RawMessage `json:"-"`
}

type eventsResponse struct {
	Events []json.RawMessage `json:"events"`
	Total  int               `json:"total"`
}

// FetchRunLogs queries the logs service for events related to a CI run.
func FetchRunLogs(run *Run) ([]LogEvent, error) {
	client := tailnet.Client()
	if client == nil {
		return nil, fmt.Errorf("no tailnet client")
	}

	// Build time range: from run start to finish (or now).
	start, err := time.Parse(time.RFC3339, run.StartedAt)
	if err != nil {
		return nil, fmt.Errorf("parsing start time: %w", err)
	}
	// Pad by 1 minute before.
	start = start.Add(-1 * time.Minute)

	end := time.Now().UTC()
	if run.FinishedAt != nil {
		if t, err := time.Parse(time.RFC3339, *run.FinishedAt); err == nil {
			// Pad by 1 minute after.
			end = t.Add(1 * time.Minute)
		}
	}

	params := url.Values{
		"start": {start.Format(time.RFC3339)},
		"end":   {end.Format(time.RFC3339)},
		"q":     {"group:app,app:ci"},
		"limit": {"200"},
	}

	reqURL := "http://monks-logs-fly-ord/events?" + params.Encode()
	resp, err := client.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("querying logs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("logs service returned %d: %s", resp.StatusCode, string(body))
	}

	var result eventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding logs response: %w", err)
	}

	// Parse events and filter to those related to this run.
	runID := float64(run.ID) // JSON numbers are float64
	var events []LogEvent
	for _, raw := range result.Events {
		var ev LogEvent
		if err := json.Unmarshal(raw, &ev); err != nil {
			continue
		}
		ev.Raw = raw

		// Keep events that mention this run ID.
		if matchesRunID(ev.TriggerRunID, runID) || matchesRunID(ev.APIRunID, runID) {
			events = append(events, ev)
		}
	}

	return events, nil
}

func matchesRunID(field any, runID float64) bool {
	switch v := field.(type) {
	case float64:
		return v == runID
	case string:
		return v == fmt.Sprintf("%d", int(runID))
	}
	return false
}
