package serve

import (
	"context"
	"net/http/httptest"
	"testing"
)

func TestBasePath(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{
			name:   "with forwarded prefix",
			header: "/map",
			want:   "/map/",
		},
		{
			name:   "without forwarded prefix",
			header: "",
			want:   "/",
		},
		{
			name:   "with longer prefix",
			header: "/map-dev-on-brigid",
			want:   "/map-dev-on-brigid/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("X-Forwarded-Prefix", tt.header)
			}
			got := BasePath(req)
			if got != tt.want {
				t.Errorf("BasePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBasePathFromContext(t *testing.T) {
	t.Run("with basepath in context", func(t *testing.T) {
		ctx := WithBasePath(context.Background(), "/map/")
		got := BasePathFromContext(ctx)
		if got != "/map/" {
			t.Errorf("BasePathFromContext() = %q, want %q", got, "/map/")
		}
	})

	t.Run("without basepath in context", func(t *testing.T) {
		got := BasePathFromContext(context.Background())
		if got != "/" {
			t.Errorf("BasePathFromContext() = %q, want %q", got, "/")
		}
	})
}
