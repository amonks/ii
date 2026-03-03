package changedetect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAffectedApps(t *testing.T) {
	flyApps := []string{"dogs", "homepage", "logs", "proxy", "writing"}

	graph := map[string][]string{
		"apps/dogs":      {"pkg/dogs", "pkg/serve"},
		"apps/homepage":  {"pkg/serve", "pkg/letterboxd"},
		"apps/logs":      {"pkg/logs", "pkg/serve"},
		"apps/proxy":     {"pkg/serve", "pkg/tls", "pkg/tailnet"},
		"apps/writing":   {"pkg/posts", "pkg/serve"},
		"pkg/dogs":       {},
		"pkg/serve":      {"pkg/middleware"},
		"pkg/middleware":  {"pkg/reqlog"},
		"pkg/reqlog":     {},
		"pkg/logs":       {},
		"pkg/letterboxd": {},
		"pkg/tls":        {},
		"pkg/tailnet":    {},
		"pkg/posts":      {"pkg/markdown"},
		"pkg/markdown":   {},
	}

	t.Run("app dir change", func(t *testing.T) {
		changed := []string{"apps/dogs/main.go"}
		got := AffectedApps(flyApps, changed, graph)
		assertApps(t, got, []string{"dogs"})
	})

	t.Run("direct pkg dependency", func(t *testing.T) {
		changed := []string{"pkg/dogs/handler.go"}
		got := AffectedApps(flyApps, changed, graph)
		assertApps(t, got, []string{"dogs"})
	})

	t.Run("transitive pkg dependency", func(t *testing.T) {
		// pkg/reqlog is used by pkg/middleware, which is used by pkg/serve,
		// which is used by all apps.
		changed := []string{"pkg/reqlog/logger.go"}
		got := AffectedApps(flyApps, changed, graph)
		assertApps(t, got, flyApps)
	})

	t.Run("shared pkg dependency", func(t *testing.T) {
		// pkg/serve is used by all apps.
		changed := []string{"pkg/serve/mux.go"}
		got := AffectedApps(flyApps, changed, graph)
		assertApps(t, got, flyApps)
	})

	t.Run("root go.mod deploys all", func(t *testing.T) {
		changed := []string{"go.mod"}
		got := AffectedApps(flyApps, changed, graph)
		assertApps(t, got, flyApps)
	})

	t.Run("root go.sum deploys all", func(t *testing.T) {
		changed := []string{"go.sum"}
		got := AffectedApps(flyApps, changed, graph)
		assertApps(t, got, flyApps)
	})

	t.Run("config fly-apps.toml deploys all", func(t *testing.T) {
		changed := []string{filepath.Join("config", "fly-apps.toml")}
		got := AffectedApps(flyApps, changed, graph)
		assertApps(t, got, flyApps)
	})

	t.Run("nil changed files (initial push) deploys all", func(t *testing.T) {
		got := AffectedApps(flyApps, nil, graph)
		assertApps(t, got, flyApps)
	})

	t.Run("unrelated files deploy nothing", func(t *testing.T) {
		changed := []string{"README.md", "specs/deploy.md", ".github/workflows/ci.yml"}
		got := AffectedApps(flyApps, changed, graph)
		assertApps(t, got, nil)
	})

	t.Run("non-fly app change deploys nothing", func(t *testing.T) {
		changed := []string{"apps/calendar/main.go"}
		got := AffectedApps(flyApps, changed, graph)
		assertApps(t, got, nil)
	})

	t.Run("multiple changes deduplicate", func(t *testing.T) {
		changed := []string{
			"apps/dogs/main.go",
			"apps/dogs/handler.go",
			"pkg/dogs/model.go",
		}
		got := AffectedApps(flyApps, changed, graph)
		assertApps(t, got, []string{"dogs"})
	})

	t.Run("mixed app and pkg changes", func(t *testing.T) {
		changed := []string{
			"apps/dogs/main.go",
			"pkg/posts/render.go", // affects writing
		}
		got := AffectedApps(flyApps, changed, graph)
		assertApps(t, got, []string{"dogs", "writing"})
	})

	t.Run("empty changed files deploys nothing", func(t *testing.T) {
		changed := []string{}
		got := AffectedApps(flyApps, changed, graph)
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

	apps, err := LoadFlyApps(root)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"dogs", "logs", "proxy"}
	assertApps(t, apps, expected)
}

func TestLoadFlyAppsConfig(t *testing.T) {
	root := t.TempDir()

	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "fly-apps.toml"), []byte(`
[defaults]
  region = "ord"
  vm_size = "shared-cpu-1x"
  vm_memory = "256"

[apps.dogs]
  vm_size = "shared-cpu-2x"
  vm_memory = "1gb"
  volume = "monks_dogs_data"
  packages = ["sqlite"]
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFlyAppsConfig(root)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Defaults.Region != "ord" {
		t.Errorf("expected region ord, got %s", cfg.Defaults.Region)
	}
	if cfg.Defaults.VMSize != "shared-cpu-1x" {
		t.Errorf("expected vm_size shared-cpu-1x, got %s", cfg.Defaults.VMSize)
	}

	dogs, ok := cfg.Apps["dogs"]
	if !ok {
		t.Fatal("expected dogs app in config")
	}
	if dogs.VMSize != "shared-cpu-2x" {
		t.Errorf("expected dogs vm_size shared-cpu-2x, got %s", dogs.VMSize)
	}
	if dogs.Volume != "monks_dogs_data" {
		t.Errorf("expected dogs volume monks_dogs_data, got %s", dogs.Volume)
	}
	if len(dogs.Packages) != 1 || dogs.Packages[0] != "sqlite" {
		t.Errorf("expected dogs packages [sqlite], got %v", dogs.Packages)
	}
}

func TestChangedFiles(t *testing.T) {
	t.Run("all zeros returns nil (initial push)", func(t *testing.T) {
		files, err := ChangedFiles("/tmp", "0000000000000000000000000000000000000000")
		if err != nil {
			t.Fatal(err)
		}
		if files != nil {
			t.Errorf("expected nil for initial push, got %v", files)
		}
	})
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
