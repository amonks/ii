package gzipserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFileServer(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		acceptEncoding string
		wantStatus     int
		wantEncoding   string
	}{
		{
			name:       "normal file",
			path:       "/test.txt",
			wantStatus: http.StatusOK,
		},
		{
			name:           "gzipped file with accept",
			path:           "/test.txt",
			acceptEncoding: "gzip",
			wantStatus:     http.StatusOK,
			wantEncoding:   "gzip",
		},
		{
			name:           "brotli file with accept",
			path:           "/test.txt",
			acceptEncoding: "br",
			wantStatus:     http.StatusOK,
			wantEncoding:   "br",
		},
		{
			name:       "missing file",
			path:       "/notfound.txt",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "directory redirect",
			path:       "/dir",
			wantStatus: http.StatusMovedPermanently,
		},
		{
			name:       "directory index",
			path:       "/dir/",
			wantStatus: http.StatusOK,
		},
	}

	fs := newTestFS()
	handler := FileServer(fs)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status code = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantEncoding != "" {
				if got := w.Header().Get("Content-Encoding"); got != tt.wantEncoding {
					t.Errorf("Content-Encoding = %q, want %q", got, tt.wantEncoding)
				}
				if got := w.Header().Get("Vary"); got != "Accept-Encoding" {
					t.Errorf("Vary = %q, want \"Accept-Encoding\"", got)
				}
			}
		})
	}
}
