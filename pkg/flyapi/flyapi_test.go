package flyapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCreateMachine(t *testing.T) {
	want := MachineInfo{
		ID:        "machine123",
		Name:      "test-machine",
		State:     "started",
		Region:    "ord",
		CreatedAt: "2026-03-02T00:00:00Z",
		Config: MachineConfig{
			Image: "registry.fly.io/myapp:latest",
		},
	}

	var gotBody MachineCreateInput
	var gotAuth string
	var gotContentType string
	var gotMethod string
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("reading request body: %v", err)
		}
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Fatalf("unmarshaling request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	c := NewClient("test-token", "myapp")
	c.BaseURL = srv.URL

	input := MachineCreateInput{
		Name:   "test-machine",
		Region: "ord",
		Config: MachineConfig{
			Image:       "registry.fly.io/myapp:latest",
			AutoDestroy: true,
		},
	}

	got, err := c.CreateMachine(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateMachine: %v", err)
	}

	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/apps/myapp/machines" {
		t.Errorf("path = %q, want /apps/myapp/machines", gotPath)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("authorization = %q, want %q", gotAuth, "Bearer test-token")
	}
	if gotContentType != "application/json" {
		t.Errorf("content-type = %q, want application/json", gotContentType)
	}
	if gotBody.Name != "test-machine" {
		t.Errorf("request body name = %q, want %q", gotBody.Name, "test-machine")
	}
	if gotBody.Region != "ord" {
		t.Errorf("request body region = %q, want %q", gotBody.Region, "ord")
	}
	if gotBody.Config.Image != "registry.fly.io/myapp:latest" {
		t.Errorf("request body image = %q, want %q", gotBody.Config.Image, "registry.fly.io/myapp:latest")
	}
	if !gotBody.Config.AutoDestroy {
		t.Errorf("request body auto_destroy = false, want true")
	}

	if got.ID != want.ID {
		t.Errorf("response ID = %q, want %q", got.ID, want.ID)
	}
	if got.Name != want.Name {
		t.Errorf("response Name = %q, want %q", got.Name, want.Name)
	}
	if got.State != want.State {
		t.Errorf("response State = %q, want %q", got.State, want.State)
	}
	if got.Region != want.Region {
		t.Errorf("response Region = %q, want %q", got.Region, want.Region)
	}
}

func TestCreateMachineError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		w.Write([]byte(`{"error":"invalid config"}`))
	}))
	defer srv.Close()

	c := NewClient("test-token", "myapp")
	c.BaseURL = srv.URL

	_, err := c.CreateMachine(context.Background(), MachineCreateInput{
		Config: MachineConfig{Image: "bad"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 422 {
		t.Errorf("status code = %d, want 422", apiErr.StatusCode)
	}
	if apiErr.Message == "" {
		t.Error("expected non-empty error message")
	}
}

func TestWaitForState(t *testing.T) {
	var gotAuth string
	var gotMethod string
	var gotPath string
	var gotState string
	var gotTimeout string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotState = r.URL.Query().Get("state")
		gotTimeout = r.URL.Query().Get("timeout")

		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := NewClient("wait-token", "myapp")
	c.BaseURL = srv.URL

	err := c.WaitForState(context.Background(), "machine456", "started", 30*time.Second)
	if err != nil {
		t.Fatalf("WaitForState: %v", err)
	}

	if gotMethod != "GET" {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/apps/myapp/machines/machine456/wait" {
		t.Errorf("path = %q, want /apps/myapp/machines/machine456/wait", gotPath)
	}
	if gotAuth != "Bearer wait-token" {
		t.Errorf("authorization = %q, want %q", gotAuth, "Bearer wait-token")
	}
	if gotState != "started" {
		t.Errorf("state = %q, want %q", gotState, "started")
	}
	if gotTimeout != "30" {
		t.Errorf("timeout = %q, want %q", gotTimeout, "30")
	}
}

func TestGetMachine(t *testing.T) {
	want := MachineInfo{
		ID:     "machine789",
		Name:   "get-test",
		State:  "stopped",
		Region: "iad",
		Events: []MachineEvent{
			{Type: "start", Status: "started", Timestamp: 1000},
		},
	}

	var gotAuth string
	var gotMethod string
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	c := NewClient("get-token", "myapp")
	c.BaseURL = srv.URL

	got, err := c.GetMachine(context.Background(), "machine789")
	if err != nil {
		t.Fatalf("GetMachine: %v", err)
	}

	if gotMethod != "GET" {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/apps/myapp/machines/machine789" {
		t.Errorf("path = %q, want /apps/myapp/machines/machine789", gotPath)
	}
	if gotAuth != "Bearer get-token" {
		t.Errorf("authorization = %q, want %q", gotAuth, "Bearer get-token")
	}

	if got.ID != want.ID {
		t.Errorf("response ID = %q, want %q", got.ID, want.ID)
	}
	if got.State != want.State {
		t.Errorf("response State = %q, want %q", got.State, want.State)
	}
	if len(got.Events) != 1 {
		t.Fatalf("response events len = %d, want 1", len(got.Events))
	}
	if got.Events[0].Type != "start" {
		t.Errorf("response event type = %q, want %q", got.Events[0].Type, "start")
	}
}

func TestStopMachine(t *testing.T) {
	var gotAuth string
	var gotMethod string
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")

		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := NewClient("stop-token", "myapp")
	c.BaseURL = srv.URL

	err := c.StopMachine(context.Background(), "machinestop")
	if err != nil {
		t.Fatalf("StopMachine: %v", err)
	}

	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/apps/myapp/machines/machinestop/stop" {
		t.Errorf("path = %q, want /apps/myapp/machines/machinestop/stop", gotPath)
	}
	if gotAuth != "Bearer stop-token" {
		t.Errorf("authorization = %q, want %q", gotAuth, "Bearer stop-token")
	}
}

func TestListMachines(t *testing.T) {
	machines := []MachineInfo{
		{ID: "m1", Name: "ci-builder-1", State: "stopped", Config: MachineConfig{Image: "registry.fly.io/monks-ci-builder:deploy-abc"}},
		{ID: "m2", Name: "ci-builder-2", State: "started", Config: MachineConfig{Image: "registry.fly.io/monks-ci-builder:deploy-def"}},
	}

	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Header.Get("Authorization") != "Bearer list-token" {
			t.Errorf("unexpected auth: %s", r.Header.Get("Authorization"))
		}
		json.NewEncoder(w).Encode(machines)
	}))
	defer srv.Close()

	c := NewClient("list-token", "monks-ci-builder")
	c.BaseURL = srv.URL

	got, err := c.ListMachines(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/apps/monks-ci-builder/machines" {
		t.Errorf("path = %q, want /apps/monks-ci-builder/machines", gotPath)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 machines, got %d", len(got))
	}
	if got[0].Config.Image != "registry.fly.io/monks-ci-builder:deploy-abc" {
		t.Errorf("unexpected image: %s", got[0].Config.Image)
	}
}

func TestStopMachineError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer srv.Close()

	c := NewClient("stop-token", "myapp")
	c.BaseURL = srv.URL

	err := c.StopMachine(context.Background(), "machinestop")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 500 {
		t.Errorf("status code = %d, want 500", apiErr.StatusCode)
	}
}
