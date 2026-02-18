package libraryserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCanManageLibrary_FromContext(t *testing.T) {
	t.Run("returns false with empty context", func(t *testing.T) {
		ctx := context.Background()
		if CanManageLibrary(ctx) {
			t.Error("expected CanManageLibrary to return false for empty context")
		}
	})

	t.Run("returns true when set to true", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), canManageKey{}, true)
		if !CanManageLibrary(ctx) {
			t.Error("expected CanManageLibrary to return true")
		}
	})

	t.Run("returns false when set to false", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), canManageKey{}, false)
		if CanManageLibrary(ctx) {
			t.Error("expected CanManageLibrary to return false")
		}
	})
}

func TestRequireManage_Forbidden(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	handler := requireManage(inner)

	// Request without the capability in context => 403
	req := httptest.NewRequest("GET", "/import/", nil)
	ctx := context.WithValue(req.Context(), canManageKey{}, false)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", rec.Code)
	}
}

func TestRequireManage_Allowed(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	handler := requireManage(inner)

	// Request with the capability in context => passes through
	req := httptest.NewRequest("GET", "/import/", nil)
	ctx := context.WithValue(req.Context(), canManageKey{}, true)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got %q", rec.Body.String())
	}
}

func TestCanManageMiddleware(t *testing.T) {
	t.Run("sets canManage true when header present", func(t *testing.T) {
		var got bool
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = CanManageLibrary(r.Context())
		})

		handler := canManageMiddleware(inner)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Tailscale-Cap-Movies-Write", "true")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if !got {
			t.Error("expected CanManageLibrary to be true when header is set")
		}
	})

	t.Run("sets canManage false when header absent", func(t *testing.T) {
		got := true // start true to verify it gets set to false
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = CanManageLibrary(r.Context())
		})

		handler := canManageMiddleware(inner)

		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if got {
			t.Error("expected CanManageLibrary to be false when header is absent")
		}
	})
}
