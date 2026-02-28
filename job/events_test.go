package job

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestEventLogAppendsEvents(t *testing.T) {
	eventsDir := t.TempDir()
	log, err := OpenEventLog("job-events", EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}

	if err := log.Append(Event{Name: "job.stage", Data: "{\"stage\":\"implementing\"}"}); err != nil {
		_ = log.Close()
		t.Fatalf("append event: %v", err)
	}
	if err := log.Append(Event{ID: "2", Name: "job.prompt", Data: "prompt"}); err != nil {
		_ = log.Close()
		t.Fatalf("append event: %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("close event log: %v", err)
	}

	path := filepath.Join(eventsDir, "job-events.jsonl")
	events := readEventLogFile(t, path)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Name != "job.stage" || events[0].Data == "" {
		t.Fatalf("unexpected first event: %#v", events[0])
	}
	if events[1].ID != "2" || events[1].Name != "job.prompt" || events[1].Data != "prompt" {
		t.Fatalf("unexpected second event: %#v", events[1])
	}
}

func TestEventLogAppendAppendsEvents(t *testing.T) {
	eventsDir := t.TempDir()
	log, err := OpenEventLog("job-append", EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	if err := log.Append(Event{Name: "job.stage", Data: "{\"stage\":\"implementing\"}"}); err != nil {
		_ = log.Close()
		t.Fatalf("append event: %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("close event log: %v", err)
	}

	appendLog, err := OpenEventLogAppend("job-append", EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("open event log append: %v", err)
	}
	if err := appendLog.Append(Event{ID: "2", Name: "job.prompt", Data: "prompt"}); err != nil {
		_ = appendLog.Close()
		t.Fatalf("append event: %v", err)
	}
	if err := appendLog.Close(); err != nil {
		t.Fatalf("close append event log: %v", err)
	}

	path := filepath.Join(eventsDir, "job-append.jsonl")
	events := readEventLogFile(t, path)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Name != "job.stage" {
		t.Fatalf("unexpected first event: %#v", events[0])
	}
	if events[1].ID != "2" || events[1].Name != "job.prompt" || events[1].Data != "prompt" {
		t.Fatalf("unexpected second event: %#v", events[1])
	}
}

func TestEventSnapshotReadsEvents(t *testing.T) {
	eventsDir := t.TempDir()
	log, err := OpenEventLog("job-snapshot", EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	if err := log.Append(Event{Name: "job.stage", Data: "{\"stage\":\"implementing\"}"}); err != nil {
		_ = log.Close()
		t.Fatalf("append event: %v", err)
	}
	if err := log.Append(Event{Name: "job.prompt", Data: "prompt"}); err != nil {
		_ = log.Close()
		t.Fatalf("append event: %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("close event log: %v", err)
	}

	events, err := EventSnapshot("job-snapshot", EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("event snapshot: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Name != "job.stage" {
		t.Fatalf("unexpected first event: %#v", events[0])
	}
}

func TestEventLogStreamsEvents(t *testing.T) {
	eventsDir := t.TempDir()
	log, err := OpenEventLog("job-stream", EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	stream := make(chan Event, 2)
	log.SetStream(stream)

	first := Event{Name: "job.stage", Data: "{\"stage\":\"implementing\"}"}
	second := Event{ID: "2", Name: "job.prompt", Data: "prompt"}
	if err := log.Append(first); err != nil {
		_ = log.Close()
		t.Fatalf("append event: %v", err)
	}
	if err := log.Append(second); err != nil {
		_ = log.Close()
		t.Fatalf("append event: %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("close event log: %v", err)
	}

	gotFirst := <-stream
	gotSecond := <-stream
	if gotFirst != first {
		t.Fatalf("unexpected first stream event: %#v", gotFirst)
	}
	if gotSecond != second {
		t.Fatalf("unexpected second stream event: %#v", gotSecond)
	}
}

func TestEventSnapshotMissingFileReturnsEmpty(t *testing.T) {
	eventsDir := t.TempDir()
	events, err := EventSnapshot("missing-log", EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("event snapshot: %v", err)
	}
	if events == nil {
		t.Fatal("expected empty events slice, got nil")
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func readEventLogFile(t *testing.T, path string) []Event {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Fatalf("close event log: %v", err)
		}
	}()

	decoder := json.NewDecoder(file)
	var events []Event
	for {
		var event Event
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("decode event: %v", err)
		}
		events = append(events, event)
	}
	return events
}
