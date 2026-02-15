package serve

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"monks.co/pkg/reqlog"
)

func TestErrorf_SetsErrMessage(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

	handler := reqlog.Middleware().ModifyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Errorf(w, r, 500, "db error: %s", "connection refused")
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != 500 {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}

	var event map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("failed to parse log event: %v\nraw: %s", err, buf.String())
	}
	if event["err.message"] != "db error: connection refused" {
		t.Errorf("expected err.message='db error: connection refused', got %v", event["err.message"])
	}
	if event["level"] != "ERROR" {
		t.Errorf("expected level=ERROR for 500, got %v", event["level"])
	}
}

func TestErrorf_4xxNoErrorLevel(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

	handler := reqlog.Middleware().ModifyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Errorf(w, r, 404, "not found: %s", "/missing")
	}))

	req := httptest.NewRequest("GET", "/missing", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != 404 {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}

	var event map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("failed to parse log event: %v", err)
	}
	// 4xx should be INFO level, not ERROR
	if event["level"] == "ERROR" {
		t.Errorf("expected non-ERROR level for 404, got ERROR")
	}
	// But err.message should still be set
	if event["err.message"] != "not found: /missing" {
		t.Errorf("expected err.message='not found: /missing', got %v", event["err.message"])
	}
}

func TestInternalServerError(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

	handler := reqlog.Middleware().ModifyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		InternalServerError(w, r, http.ErrServerClosed)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != 500 {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}

	var event map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("failed to parse log event: %v", err)
	}
	if event["err.message"] != "http: Server closed" {
		t.Errorf("expected err.message='http: Server closed', got %v", event["err.message"])
	}
}
