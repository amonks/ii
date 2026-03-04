package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"monks.co/pkg/ci/changedetect"
)

func TestDeployAppsCollectsAllErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	reporter := NewReporter(srv.URL, 1, http.DefaultClient)
	cfg := &changedetect.FlyAppsConfig{}

	original := deployAppFunc
	defer func() { deployAppFunc = original }()

	deployAppFunc = func(root, app, sha, flyToken, baseImageRef string, cfg *changedetect.FlyAppsConfig, reporter *Reporter) error {
		if app == "dogs" || app == "logs" {
			return fmt.Errorf("%s deploy error", app)
		}
		return nil
	}

	apps := []string{"dogs", "proxy", "logs", "homepage"}
	err := deployApps(apps, "/tmp", "abc", "token", "ref", cfg, reporter)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "dogs") {
		t.Errorf("expected error to mention dogs, got: %s", errStr)
	}
	if !strings.Contains(errStr, "logs") {
		t.Errorf("expected error to mention logs, got: %s", errStr)
	}
	if strings.Contains(errStr, "proxy") {
		t.Errorf("expected error not to mention proxy, got: %s", errStr)
	}
	if strings.Contains(errStr, "homepage") {
		t.Errorf("expected error not to mention homepage, got: %s", errStr)
	}
}

func TestDeployAppsAllSucceed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	reporter := NewReporter(srv.URL, 1, http.DefaultClient)
	cfg := &changedetect.FlyAppsConfig{}

	original := deployAppFunc
	defer func() { deployAppFunc = original }()

	deployAppFunc = func(root, app, sha, flyToken, baseImageRef string, cfg *changedetect.FlyAppsConfig, reporter *Reporter) error {
		return nil
	}

	err := deployApps([]string{"dogs", "proxy"}, "/tmp", "abc", "token", "ref", cfg, reporter)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestParseTerraformOutput(t *testing.T) {
	tests := []struct {
		output                          string
		wantAdded, wantChanged, wantDel int
	}{
		{
			output:      "Apply complete! Resources: 3 added, 1 changed, 0 destroyed.",
			wantAdded:   3,
			wantChanged: 1,
			wantDel:     0,
		},
		{
			output:      "Apply complete! Resources: 0 added, 0 changed, 2 destroyed.",
			wantAdded:   0,
			wantChanged: 0,
			wantDel:     2,
		},
		{
			output:    "No changes. Infrastructure is up-to-date.",
			wantAdded: 0,
		},
	}

	for _, tt := range tests {
		added, changed, destroyed := parseTerraformOutput(tt.output)
		if added != tt.wantAdded || changed != tt.wantChanged || destroyed != tt.wantDel {
			t.Errorf("parseTerraformOutput(%q) = (%d, %d, %d), want (%d, %d, %d)",
				tt.output, added, changed, destroyed, tt.wantAdded, tt.wantChanged, tt.wantDel)
		}
	}
}

func TestDeployAppCIBuilderFailureStopsEarly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	reporter := NewReporter(srv.URL, 1, http.DefaultClient)
	cfg := &changedetect.FlyAppsConfig{}

	origDeploy := deployAppFunc
	origBuilder := deployCIBuilderFunc
	defer func() {
		deployAppFunc = origDeploy
		deployCIBuilderFunc = origBuilder
	}()

	deployCIBuilderFunc = func(root string, w io.Writer) error {
		return fmt.Errorf("builder image build failed")
	}

	var compileCalled bool
	deployAppFunc = func(root, app, sha, flyToken, baseImageRef string, cfg *changedetect.FlyAppsConfig, reporter *Reporter) error {
		// This is the real deployApp; we track whether it got past the builder step.
		compileCalled = true
		return nil
	}

	// Call the real deployApp (not the mock) to test the ci branch.
	err := deployApp("/tmp", "ci", "abc", "token", "ref", cfg, reporter)
	if err == nil {
		t.Fatal("expected error from builder failure")
	}
	if !strings.Contains(err.Error(), "builder") {
		t.Errorf("expected error to mention builder, got: %s", err.Error())
	}
	if compileCalled {
		t.Error("compile step should not have run after builder failure")
	}
}

func TestDeployAppCIBuilderSuccessContinues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	reporter := NewReporter(srv.URL, 1, http.DefaultClient)
	cfg := &changedetect.FlyAppsConfig{}

	origBuilder := deployCIBuilderFunc
	defer func() { deployCIBuilderFunc = origBuilder }()

	var builderCalled bool
	deployCIBuilderFunc = func(root string, w io.Writer) error {
		builderCalled = true
		return nil
	}

	// The real deployApp will fail at the compile step (no go build env in tests),
	// but the error should be from compile, not from the builder, proving we
	// got past the builder step.
	err := deployApp("/tmp", "ci", "abc", "token", "ref", cfg, reporter)

	if !builderCalled {
		t.Error("expected builder to be called")
	}
	// We expect an error from the compile step (go build will fail in tests).
	// The important thing is that it's NOT a builder error.
	if err != nil && strings.Contains(err.Error(), "builder") {
		t.Errorf("error should not be from builder: %s", err.Error())
	}
}

func TestDeployAppUsesStreams(t *testing.T) {
	// Track API calls to verify stream lifecycle.
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	reporter := NewReporter(srv.URL, 1, http.DefaultClient)
	cfg := &changedetect.FlyAppsConfig{}

	original := deployAppFunc
	defer func() { deployAppFunc = original }()

	deployAppFunc = func(root, app, sha, flyToken, baseImageRef string, cfg *changedetect.FlyAppsConfig, reporter *Reporter) error {
		// Simulated deploy: just call the stream lifecycle.
		reporter.StartStream("deploy", app)
		w := reporter.StreamWriter("deploy", app)
		fmt.Fprintf(w, "deploying %s\n", app)
		w.Close()
		reporter.FinishStream("deploy", app, FinishStreamResult{
			Status:     "success",
			DurationMs: 100,
		})
		reporter.AddDeployResult(DeployResult{
			App:      app,
			ImageRef: "registry.fly.io/monks-" + app + ":sha1",
		})
		return nil
	}

	err := deployApps([]string{"dogs"}, "/tmp", "abc", "token", "ref", cfg, reporter)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify stream start/finish calls were made.
	streamStartPattern := regexp.MustCompile(`PUT /api/runs/1/jobs/deploy/streams/dogs/start`)
	streamFinishPattern := regexp.MustCompile(`PUT /api/runs/1/jobs/deploy/streams/dogs/done`)

	var hasStart, hasFinish bool
	for _, call := range calls {
		if streamStartPattern.MatchString(call) {
			hasStart = true
		}
		if streamFinishPattern.MatchString(call) {
			hasFinish = true
		}
	}
	if !hasStart {
		t.Error("expected stream start call for dogs")
	}
	if !hasFinish {
		t.Error("expected stream finish call for dogs")
	}

	// Verify deploy result was accumulated.
	reporter.mu.Lock()
	defer reporter.mu.Unlock()
	if len(reporter.deploys) != 1 {
		t.Fatalf("expected 1 deploy result, got %d", len(reporter.deploys))
	}
	if reporter.deploys[0].App != "dogs" {
		t.Errorf("expected deploy app dogs, got %s", reporter.deploys[0].App)
	}
}
