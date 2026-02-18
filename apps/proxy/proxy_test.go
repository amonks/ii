package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxyRequestSetsXForwardedPrefix(t *testing.T) {
	// Backend that echoes back the X-Forwarded-Prefix it receives.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Got-Prefix", r.Header.Get("X-Forwarded-Prefix"))
		w.WriteHeader(200)
	}))
	defer backend.Close()

	p := &proxy{transport: http.DefaultTransport}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/map/some/path", nil)

	p.proxyRequest("map", backend.Listener.Addr().String(), w, req)

	got := w.Header().Get("X-Got-Prefix")
	if got != "/map" {
		t.Errorf("X-Forwarded-Prefix = %q, want %q", got, "/map")
	}
}

func TestProxyRequestRewritesLocationHeader(t *testing.T) {
	tests := []struct {
		name         string
		prefix       string
		location     string
		wantLocation string
	}{
		{
			name:         "root-relative path gets prefix",
			prefix:       "map",
			location:     "/",
			wantLocation: "/map/",
		},
		{
			name:         "root-relative deeper path gets prefix",
			prefix:       "map",
			location:     "/some/path",
			wantLocation: "/map/some/path",
		},
		{
			name:         "absolute URL passes through",
			prefix:       "map",
			location:     "https://example.com/path",
			wantLocation: "https://example.com/path",
		},
		{
			name:         "protocol-relative URL passes through",
			prefix:       "map",
			location:     "//example.com/path",
			wantLocation: "//example.com/path",
		},
		{
			name:         "relative path passes through",
			prefix:       "map",
			location:     "../other",
			wantLocation: "../other",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", tt.location)
				w.WriteHeader(302)
			}))
			defer backend.Close()

			p := &proxy{transport: http.DefaultTransport}
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/"+tt.prefix+"/", nil)

			p.proxyRequest(tt.prefix, backend.Listener.Addr().String(), w, req)

			got := w.Header().Get("Location")
			if got != tt.wantLocation {
				t.Errorf("Location = %q, want %q", got, tt.wantLocation)
			}
		})
	}
}
