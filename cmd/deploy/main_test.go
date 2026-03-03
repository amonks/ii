package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAffectedApps(t *testing.T) {
	flyApps := []string{"dogs", "homepage", "logs", "proxy", "writing"}

	graph := map[string][]string{
		"apps/dogs":     {"pkg/dogs", "pkg/serve"},
		"apps/homepage": {"pkg/serve", "pkg/letterboxd"},
		"apps/logs":     {"pkg/logs", "pkg/serve"},
		"apps/proxy":    {"pkg/serve", "pkg/tls", "pkg/tailnet"},
		"apps/writing":  {"pkg/posts", "pkg/serve"},
		"pkg/dogs":      {},
		"pkg/serve":     {"pkg/middleware"},
		"pkg/middleware": {"pkg/reqlog"},
		"pkg/reqlog":    {},
		"pkg/logs":      {},
		"pkg/letterboxd": {},
		"pkg/tls":       {},
		"pkg/tailnet":   {},
		"pkg/posts":     {"pkg/markdown"},
		"pkg/markdown":  {},
	}

	t.Run("app dir change", func(t *testing.T) {
		changed := []string{"apps/dogs/main.go"}
		got := affectedApps(flyApps, changed, graph)
		assertApps(t, got, []string{"dogs"})
	})

	t.Run("direct pkg dependency", func(t *testing.T) {
		changed := []string{"pkg/dogs/handler.go"}
		got := affectedApps(flyApps, changed, graph)
		assertApps(t, got, []string{"dogs"})
	})

	t.Run("transitive pkg dependency", func(t *testing.T) {
		// pkg/reqlog is used by pkg/middleware, which is used by pkg/serve,
		// which is used by all apps.
		changed := []string{"pkg/reqlog/logger.go"}
		got := affectedApps(flyApps, changed, graph)
		assertApps(t, got, flyApps)
	})

	t.Run("shared pkg dependency", func(t *testing.T) {
		// pkg/serve is used by all apps.
		changed := []string{"pkg/serve/mux.go"}
		got := affectedApps(flyApps, changed, graph)
		assertApps(t, got, flyApps)
	})

	t.Run("root go.mod deploys all", func(t *testing.T) {
		changed := []string{"go.mod"}
		got := affectedApps(flyApps, changed, graph)
		assertApps(t, got, flyApps)
	})

	t.Run("root go.sum deploys all", func(t *testing.T) {
		changed := []string{"go.sum"}
		got := affectedApps(flyApps, changed, graph)
		assertApps(t, got, flyApps)
	})

	t.Run("config fly-apps.toml deploys all", func(t *testing.T) {
		changed := []string{filepath.Join("config", "fly-apps.toml")}
		got := affectedApps(flyApps, changed, graph)
		assertApps(t, got, flyApps)
	})

	t.Run("nil changed files (initial push) deploys all", func(t *testing.T) {
		got := affectedApps(flyApps, nil, graph)
		assertApps(t, got, flyApps)
	})

	t.Run("unrelated files deploy nothing", func(t *testing.T) {
		changed := []string{"README.md", "specs/deploy.md", ".github/workflows/ci.yml"}
		got := affectedApps(flyApps, changed, graph)
		assertApps(t, got, nil)
	})

	t.Run("non-fly app change deploys nothing", func(t *testing.T) {
		changed := []string{"apps/calendar/main.go"}
		got := affectedApps(flyApps, changed, graph)
		assertApps(t, got, nil)
	})

	t.Run("multiple changes deduplicate", func(t *testing.T) {
		changed := []string{
			"apps/dogs/main.go",
			"apps/dogs/handler.go",
			"pkg/dogs/model.go",
		}
		got := affectedApps(flyApps, changed, graph)
		assertApps(t, got, []string{"dogs"})
	})

	t.Run("mixed app and pkg changes", func(t *testing.T) {
		changed := []string{
			"apps/dogs/main.go",
			"pkg/posts/render.go", // affects writing
		}
		got := affectedApps(flyApps, changed, graph)
		assertApps(t, got, []string{"dogs", "writing"})
	})

	t.Run("empty changed files deploys nothing", func(t *testing.T) {
		changed := []string{}
		got := affectedApps(flyApps, changed, graph)
		assertApps(t, got, nil)
	})
}

func TestLoadFlyApps(t *testing.T) {
	root := t.TempDir()

	// Create a minimal fly-apps.toml.
	configDir := filepath.Join(root, "config")
	if err := mkdirAll(configDir); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(filepath.Join(configDir, "fly-apps.toml"), `
[apps.proxy]
  vm_size = "shared-cpu-8x"

[apps.dogs]
  vm_size = "shared-cpu-2x"

[apps.logs]
  vm_size = "shared-cpu-4x"
`); err != nil {
		t.Fatal(err)
	}

	apps, err := loadFlyApps(root)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"dogs", "logs", "proxy"}
	assertApps(t, apps, expected)
}

func TestSortStrings(t *testing.T) {
	s := []string{"c", "a", "b"}
	sortStrings(s)
	if s[0] != "a" || s[1] != "b" || s[2] != "c" {
		t.Errorf("expected [a b c], got %v", s)
	}
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

func mkdirAll(path string) error {
	return mkdirAllPerm(path, 0755)
}

func mkdirAllPerm(path string, perm uint32) error {
	return os.MkdirAll(path, os.FileMode(perm))
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
