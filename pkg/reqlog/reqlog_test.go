package reqlog

import (
	"bytes"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestShutdown_NilClient(t *testing.T) {
	// Shutdown should be safe to call without SetupLogging.
	old := logsClient
	logsClient = nil
	defer func() { logsClient = old }()
	Shutdown() // must not panic
}

func TestMiddleware_NormalRequest(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

	handler := Middleware().ModifyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Set(r.Context(), "test.key", "test-value")
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/hello", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var event map[string]any
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("failed to parse log event: %v\nraw: %s", err, buf.String())
	}
	if event["msg"] != "request" {
		t.Errorf("expected msg=request, got %v", event["msg"])
	}
	if event["http.path"] != "/hello" {
		t.Errorf("expected http.path=/hello, got %v", event["http.path"])
	}
	if event["test.key"] != "test-value" {
		t.Errorf("expected test.key=test-value, got %v", event["test.key"])
	}
	if event["http.method"] != "GET" {
		t.Errorf("expected http.method=GET, got %v", event["http.method"])
	}
}

func TestMiddleware_PanicSetsErrorAttrs(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

	handler := Middleware().ModifyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/boom", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != 500 {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}

	var event map[string]any
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("failed to parse log event: %v\nraw: %s", err, buf.String())
	}
	if event["err.panic"] != "test panic" {
		t.Errorf("expected err.panic='test panic', got %v", event["err.panic"])
	}
	if _, ok := event["err.stack"]; !ok {
		t.Error("expected err.stack to be set")
	}
	if event["level"] != "ERROR" {
		t.Errorf("expected level=ERROR, got %v", event["level"])
	}
}

func TestMiddleware_500SetsErrorLevel(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

	handler := Middleware().ModifyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Set(r.Context(), "err.message", "something broke")
		w.WriteHeader(500)
	}))

	req := httptest.NewRequest("GET", "/error", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var event map[string]any
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("failed to parse log event: %v", err)
	}
	if event["level"] != "ERROR" {
		t.Errorf("expected level=ERROR, got %v", event["level"])
	}
	if event["err.message"] != "something broke" {
		t.Errorf("expected err.message='something broke', got %v", event["err.message"])
	}
}

func TestMiddleware_RequestIDGenerated(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

	handler := Middleware().ModifyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := RequestID(r.Context())
		if id == "" {
			t.Error("expected non-empty request ID")
		}
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get(RequestIDHeader) == "" {
		t.Error("expected X-Request-ID response header to be set")
	}
}

func TestSetupLogging_StdLogDoesNotReachSlog(t *testing.T) {
	// After SetupLogging, standard library log.Printf calls should go to
	// stderr only, not through slog to the logs service.
	var buf bytes.Buffer
	oldClient := logsClient
	logsClient = nil
	defer func() { logsClient = oldClient }()

	// Simulate what SetupLogging does: set up slog writing to buf,
	// then call SetupStdLog to redirect stdlib log to stderr.
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	SetupStdLog()

	log.Printf("should not appear in slog output")

	if strings.Contains(buf.String(), "should not appear") {
		t.Error("log.Printf output appeared in slog output; it should only go to stderr")
	}
}

func TestMiddleware_RequestIDPreserved(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

	handler := Middleware().ModifyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := RequestID(r.Context()); got != "upstream-id-123" {
			t.Errorf("expected request ID 'upstream-id-123', got %q", got)
		}
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(RequestIDHeader, "upstream-id-123")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get(RequestIDHeader) != "upstream-id-123" {
		t.Errorf("expected preserved request ID header, got %q", rr.Header().Get(RequestIDHeader))
	}
}
