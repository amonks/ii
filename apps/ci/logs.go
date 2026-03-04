package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"monks.co/pkg/tailnet"
)

// LogEvent represents a single task log event from the logs service.
// It carries the raw JSON data and a few parsed fields for display.
type LogEvent struct {
	Time string
	Data map[string]any
}

type eventsResponse struct {
	Events []json.RawMessage `json:"events"`
	Total  int               `json:"total"`
}

type eventEnvelope struct {
	Data json.RawMessage `json:"data"`
}

// FetchRunLogs queries the logs service for the task event associated with a CI run.
func FetchRunLogs(run *Run) ([]LogEvent, error) {
	client := tailnet.Client()
	if client == nil {
		return nil, fmt.Errorf("no tailnet client")
	}
	return fetchRunLogsFrom("http://monks-logs-fly-ord", client, run)
}

func fetchRunLogsFrom(baseURL string, client *http.Client, run *Run) ([]LogEvent, error) {
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
		"q":     {"group:app,app:ci,msg:task"},
		"limit": {"50"},
	}

	reqURL := baseURL + "/events?" + params.Encode()
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

	// Parse events and filter to those matching this run's ID.
	runID := float64(run.ID) // JSON numbers are float64
	var events []LogEvent
	for _, raw := range result.Events {
		var envelope eventEnvelope
		if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Data == nil {
			continue
		}

		var data map[string]any
		if err := json.Unmarshal(envelope.Data, &data); err != nil {
			continue
		}

		// Filter: must have run.id matching this run.
		if rid, ok := data["run.id"]; ok {
			ridFloat, isFloat := rid.(float64)
			if !isFloat || ridFloat != runID {
				continue
			}
		} else {
			continue
		}

		timeStr, _ := data["time"].(string)
		events = append(events, LogEvent{
			Time: timeStr,
			Data: data,
		})
	}

	return events, nil
}

// SortedDataKeys returns the keys from a LogEvent's Data map in a display-friendly order.
// Keys are grouped by prefix (task.*, run.*, job.*, deploy.*, terraform.*) with
// ungrouped keys like "time", "level", "msg" omitted since they're metadata.
func SortedDataKeys(data map[string]any) []string {
	groupOrder := map[string]int{
		"task":      0,
		"run":       1,
		"job":       2,
		"deploy":    3,
		"terraform": 4,
	}

	skip := map[string]bool{
		"time":     true,
		"level":    true,
		"msg":      true,
		"app.name": true,
	}

	var keys []string
	for k := range data {
		if skip[k] {
			continue
		}
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		gi := groupForKey(keys[i], groupOrder)
		gj := groupForKey(keys[j], groupOrder)
		if gi != gj {
			return gi < gj
		}
		return keys[i] < keys[j]
	})

	return keys
}

func groupForKey(key string, order map[string]int) int {
	if idx := strings.IndexByte(key, '.'); idx >= 0 {
		if o, ok := order[key[:idx]]; ok {
			return o
		}
	}
	return 99
}
