package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVanityGoGet(t *testing.T) {
	modules := []vanityModule{
		{modulePath: "monks.co/pkg/serve", mirror: "github.com/amonks/go", importPrefix: "monks.co", dir: "pkg/serve"},
		{modulePath: "monks.co/cmd/run", mirror: "github.com/amonks/run", importPrefix: "monks.co/cmd/run", dir: "cmd/run"},
	}

	handler := vanityHandler(modules, "github.com/amonks/go")

	t.Run("go-get for default mirror module uses monks.co prefix", func(t *testing.T) {
		req := httptest.NewRequest("GET", "https://monks.co/pkg/serve?go-get=1", nil)
		w := httptest.NewRecorder()

		handled := handler(w, req)
		if !handled {
			t.Fatal("expected handler to handle the request")
		}

		body := w.Body.String()
		if !strings.Contains(body, `monks.co git https://github.com/amonks/go`) {
			t.Errorf("expected monks.co prefix for default mirror, got: %s", body)
		}
		if !strings.Contains(body, `pkg.go.dev/monks.co/pkg/serve`) {
			t.Errorf("expected pkg.go.dev link to module path, got: %s", body)
		}
	})

	t.Run("go-get for explicit mirror module uses module-specific prefix", func(t *testing.T) {
		req := httptest.NewRequest("GET", "https://monks.co/cmd/run?go-get=1", nil)
		w := httptest.NewRecorder()

		handled := handler(w, req)
		if !handled {
			t.Fatal("expected handler to handle the request")
		}

		body := w.Body.String()
		if !strings.Contains(body, `monks.co/cmd/run git https://github.com/amonks/run`) {
			t.Errorf("expected module-specific prefix for explicit mirror, got: %s", body)
		}
	})

	t.Run("go-get for subpackage resolves to module root", func(t *testing.T) {
		req := httptest.NewRequest("GET", "https://monks.co/cmd/run/runner?go-get=1", nil)
		w := httptest.NewRecorder()

		handled := handler(w, req)
		if !handled {
			t.Fatal("expected handler to handle the request")
		}

		body := w.Body.String()
		if !strings.Contains(body, `monks.co/cmd/run git https://github.com/amonks/run`) {
			t.Errorf("expected cmd/run module root, got: %s", body)
		}
	})

	t.Run("root go-get returns default mirror meta tag", func(t *testing.T) {
		req := httptest.NewRequest("GET", "https://monks.co/?go-get=1", nil)
		w := httptest.NewRecorder()

		handled := handler(w, req)
		if !handled {
			t.Fatal("expected handler to handle root go-get")
		}

		body := w.Body.String()
		if !strings.Contains(body, `monks.co git https://github.com/amonks/go`) {
			t.Errorf("expected root go-import for default mirror, got: %s", body)
		}
	})

	t.Run("root without go-get is not handled", func(t *testing.T) {
		req := httptest.NewRequest("GET", "https://monks.co/", nil)
		w := httptest.NewRecorder()

		handled := handler(w, req)
		if handled {
			t.Error("root without go-get should not be handled")
		}
	})

	t.Run("human redirect for default mirror goes to tree", func(t *testing.T) {
		req := httptest.NewRequest("GET", "https://monks.co/pkg/serve", nil)
		w := httptest.NewRecorder()

		handled := handler(w, req)
		if !handled {
			t.Fatal("expected handler to handle the request")
		}

		if w.Code != http.StatusTemporaryRedirect {
			t.Errorf("expected 307 redirect, got %d", w.Code)
		}
		loc := w.Header().Get("Location")
		if loc != "https://github.com/amonks/go/tree/main/pkg/serve" {
			t.Errorf("expected redirect to tree URL, got: %s", loc)
		}
	})

	t.Run("human redirect for explicit mirror goes to repo root", func(t *testing.T) {
		req := httptest.NewRequest("GET", "https://monks.co/cmd/run", nil)
		w := httptest.NewRecorder()

		handled := handler(w, req)
		if !handled {
			t.Fatal("expected handler to handle the request")
		}

		if w.Code != http.StatusTemporaryRedirect {
			t.Errorf("expected 307 redirect, got %d", w.Code)
		}
		loc := w.Header().Get("Location")
		if loc != "https://github.com/amonks/run" {
			t.Errorf("expected redirect to repo root, got: %s", loc)
		}
	})

	t.Run("unknown path not handled", func(t *testing.T) {
		req := httptest.NewRequest("GET", "https://monks.co/map/?go-get=1", nil)
		w := httptest.NewRecorder()

		handled := handler(w, req)
		if handled {
			t.Error("should not handle unknown paths")
		}
	})

	t.Run("single-segment app path not handled", func(t *testing.T) {
		req := httptest.NewRequest("GET", "https://monks.co/dogs?go-get=1", nil)
		w := httptest.NewRecorder()

		handled := handler(w, req)
		if handled {
			t.Error("should not handle unknown single-segment paths (these are app routes)")
		}
	})

	t.Run("single-segment module path is handled", func(t *testing.T) {
		mods := []vanityModule{
			{modulePath: "monks.co/run", mirror: "github.com/amonks/run", importPrefix: "monks.co/run", dir: "cmd/run"},
		}
		h := vanityHandler(mods, "github.com/amonks/go")

		req := httptest.NewRequest("GET", "https://monks.co/run?go-get=1", nil)
		w := httptest.NewRecorder()

		handled := h(w, req)
		if !handled {
			t.Fatal("expected handler to handle single-segment module path")
		}

		body := w.Body.String()
		if !strings.Contains(body, `monks.co/run git https://github.com/amonks/run`) {
			t.Errorf("expected go-import meta tag, got: %s", body)
		}
	})

	t.Run("single-segment module subpackage is handled", func(t *testing.T) {
		mods := []vanityModule{
			{modulePath: "monks.co/run", mirror: "github.com/amonks/run", importPrefix: "monks.co/run", dir: "cmd/run"},
		}
		h := vanityHandler(mods, "github.com/amonks/go")

		req := httptest.NewRequest("GET", "https://monks.co/run/taskfile?go-get=1", nil)
		w := httptest.NewRecorder()

		handled := h(w, req)
		if !handled {
			t.Fatal("expected handler to handle subpackage of single-segment module")
		}

		body := w.Body.String()
		if !strings.Contains(body, `monks.co/run git https://github.com/amonks/run`) {
			t.Errorf("expected go-import meta tag for module root, got: %s", body)
		}
	})
}
