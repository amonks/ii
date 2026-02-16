package logs

import (
	"encoding/json"
	"testing"
	"time"
)

func TestIsValidColumn_Msg(t *testing.T) {
	if !isValidColumn("msg") {
		t.Error("expected 'msg' to be a valid column")
	}
}

func TestGetFilteredEvents_MsgFilter(t *testing.T) {
	m, err := OpenPath(":memory:")
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().Truncate(time.Second)

	// Insert a "request" event and a "started" event.
	requestEvent := json.RawMessage(`{"time":"` + now.Format(time.RFC3339) + `","msg":"request","app.name":"test","http.method":"GET","http.host":"example.com","http.path":"/","http.status":200,"http.duration_ms":10}`)
	startedEvent := json.RawMessage(`{"time":"` + now.Format(time.RFC3339) + `","msg":"started","app.name":"test"}`)

	if err := m.IngestTx([]json.RawMessage{requestEvent, startedEvent}); err != nil {
		t.Fatal(err)
	}

	tr := TimeRange{
		start: now.Add(-time.Hour),
		end:   now.Add(time.Hour),
	}

	// Filter for msg=started should return only the started event.
	q := Query{
		GroupBy: "app",
		Filters: []Filter{{Column: "msg", Values: []string{"started"}}},
	}
	events, total, err := m.GetFilteredEvents(tr, q, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("expected 1 event with msg=started, got %d", total)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Msg == nil || *events[0].Msg != "started" {
		t.Errorf("expected msg=started, got %v", events[0].Msg)
	}
}

func TestGetFilteredEvents_DefaultMsgFilter(t *testing.T) {
	m, err := OpenPath(":memory:")
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().Truncate(time.Second)

	requestEvent := json.RawMessage(`{"time":"` + now.Format(time.RFC3339) + `","msg":"request","app.name":"test","http.method":"GET","http.host":"example.com","http.path":"/","http.status":200,"http.duration_ms":10}`)
	startedEvent := json.RawMessage(`{"time":"` + now.Format(time.RFC3339) + `","msg":"started","app.name":"test"}`)

	if err := m.IngestTx([]json.RawMessage{requestEvent, startedEvent}); err != nil {
		t.Fatal(err)
	}

	tr := TimeRange{
		start: now.Add(-time.Hour),
		end:   now.Add(time.Hour),
	}

	// No msg filter specified: should default to msg=request.
	q := Query{GroupBy: "app"}
	events, total, err := m.GetFilteredEvents(tr, q, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("expected 1 event (default msg=request), got %d", total)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Msg == nil || *events[0].Msg != "request" {
		t.Errorf("expected msg=request, got %v", events[0].Msg)
	}
}
