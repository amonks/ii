package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"monks.co/pkg/ci/changedetect"
)

// fakeDeps returns a resolveDeps function backed by a map of package dir
// to transitive dependencies.
func fakeDeps(deps map[string][]string) func(string) ([]string, error) {
	return func(pkgPath string) ([]string, error) {
		d, ok := deps[pkgPath]
		if !ok {
			return nil, fmt.Errorf("unknown package: %s", pkgPath)
		}
		return d, nil
	}
}

func TestAffectedApps(t *testing.T) {
	flyApps := []string{"dogs", "homepage", "logs", "proxy", "writing"}

	// Package-level transitive deps for each app.
	deps := map[string][]string{
		"apps/dogs":     {"pkg/dogs", "pkg/serve", "pkg/middleware", "pkg/reqlog"},
		"apps/homepage": {"pkg/serve", "pkg/letterboxd", "pkg/middleware", "pkg/reqlog"},
		"apps/logs":     {"pkg/logs", "pkg/serve", "pkg/middleware", "pkg/reqlog"},
		"apps/proxy":    {"pkg/serve", "pkg/tls", "pkg/tailnet", "pkg/middleware", "pkg/reqlog"},
		"apps/writing":  {"pkg/posts", "pkg/serve", "pkg/markdown", "pkg/middleware", "pkg/reqlog"},
	}
	resolve := fakeDeps(deps)

	t.Run("app dir change", func(t *testing.T) {
		changed := []string{"apps/dogs/main.go"}
		got, err := changedetect.AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, []string{"dogs"})
	})

	t.Run("direct pkg dependency", func(t *testing.T) {
		changed := []string{"pkg/dogs/handler.go"}
		got, err := changedetect.AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, []string{"dogs"})
	})

	t.Run("transitive pkg dependency", func(t *testing.T) {
		changed := []string{"pkg/reqlog/logger.go"}
		got, err := changedetect.AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, flyApps)
	})

	t.Run("shared pkg dependency", func(t *testing.T) {
		changed := []string{"pkg/serve/mux.go"}
		got, err := changedetect.AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, flyApps)
	})

	t.Run("root go.mod deploys all", func(t *testing.T) {
		changed := []string{"go.mod"}
		got, err := changedetect.AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, flyApps)
	})

	t.Run("root go.sum deploys all", func(t *testing.T) {
		changed := []string{"go.sum"}
		got, err := changedetect.AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, flyApps)
	})

	t.Run("config fly-apps.toml deploys all", func(t *testing.T) {
		changed := []string{filepath.Join("config", "fly-apps.toml")}
		got, err := changedetect.AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, flyApps)
	})

	t.Run("nil changed files (initial push) deploys all", func(t *testing.T) {
		got, err := changedetect.AffectedApps(flyApps, nil, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, flyApps)
	})

	t.Run("unrelated files deploy nothing", func(t *testing.T) {
		changed := []string{"README.md", "specs/deploy.md", ".github/workflows/ci.yml"}
		got, err := changedetect.AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, nil)
	})

	t.Run("non-fly app change deploys nothing", func(t *testing.T) {
		changed := []string{"apps/calendar/main.go"}
		got, err := changedetect.AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, nil)
	})

	t.Run("multiple changes deduplicate", func(t *testing.T) {
		changed := []string{
			"apps/dogs/main.go",
			"apps/dogs/handler.go",
			"pkg/dogs/model.go",
		}
		got, err := changedetect.AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, []string{"dogs"})
	})

	t.Run("mixed app and pkg changes", func(t *testing.T) {
		changed := []string{
			"apps/dogs/main.go",
			"pkg/posts/render.go",
		}
		got, err := changedetect.AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, []string{"dogs", "writing"})
	})

	t.Run("empty changed files deploys nothing", func(t *testing.T) {
		changed := []string{}
		got, err := changedetect.AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, nil)
	})
}

func TestLoadFlyApps(t *testing.T) {
	root := t.TempDir()

	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "fly-apps.toml"), []byte(`
[apps.proxy]
  vm_size = "shared-cpu-8x"

[apps.dogs]
  vm_size = "shared-cpu-2x"

[apps.logs]
  vm_size = "shared-cpu-4x"
`), 0644); err != nil {
		t.Fatal(err)
	}

	apps, err := changedetect.LoadFlyApps(root)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"dogs", "logs", "proxy"}
	assertApps(t, apps, expected)
}

func assertApps(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("expected %d apps %v, got %d apps %v", len(want), want, len(got), got)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("app[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
