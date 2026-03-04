package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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
