package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/amonks/creamery"
)

func TestDashboardTemplateExecutes(t *testing.T) {
	tmpl, err := loadDashboardTemplate()
	if err != nil {
		t.Fatalf("loadDashboardTemplate: %v", err)
	}

	data := pageData{
		GeneratedAt: time.Now(),
		SourcePath:  "batchlog",
		Analytics:   creamery.AnalyzeBatchLog(nil, creamery.DefaultIngredientCatalog()),
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, dashboardTemplate, data); err != nil {
		t.Fatalf("execute template: %v", err)
	}
	if !strings.Contains(buf.String(), "Batch Log") {
		t.Fatalf("expected rendered output to mention Batch Log, got: %s", buf.String())
	}
}

func TestServeURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{":8080", "http://localhost:8080"},
		{"0.0.0.0:9000", "http://0.0.0.0:9000"},
		{"127.0.0.1:80", "http://127.0.0.1:80"},
		{"example.com:8443", "http://example.com:8443"},
	}
	for _, tc := range cases {
		if got := serveURL(tc.in); got != tc.want {
			t.Fatalf("serveURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
