package changedetect

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
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
		got, err := AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, []string{"dogs"})
	})

	t.Run("direct pkg dependency", func(t *testing.T) {
		changed := []string{"pkg/dogs/handler.go"}
		got, err := AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, []string{"dogs"})
	})

	t.Run("transitive pkg dependency", func(t *testing.T) {
		// pkg/reqlog is a transitive dep of all apps.
		changed := []string{"pkg/reqlog/logger.go"}
		got, err := AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, flyApps)
	})

	t.Run("shared pkg dependency", func(t *testing.T) {
		// pkg/serve is used by all apps.
		changed := []string{"pkg/serve/mux.go"}
		got, err := AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, flyApps)
	})

	t.Run("root go.mod deploys all", func(t *testing.T) {
		changed := []string{"go.mod"}
		got, err := AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, flyApps)
	})

	t.Run("root go.sum deploys all", func(t *testing.T) {
		changed := []string{"go.sum"}
		got, err := AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, flyApps)
	})

	t.Run("config apps.toml deploys all", func(t *testing.T) {
		changed := []string{filepath.Join("config", "apps.toml")}
		got, err := AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, flyApps)
	})

	t.Run("nil changed files (initial push) deploys all", func(t *testing.T) {
		got, err := AffectedApps(flyApps, nil, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, flyApps)
	})

	t.Run("unrelated files deploy nothing", func(t *testing.T) {
		changed := []string{"README.md", "specs/deploy.md", ".github/workflows/ci.yml"}
		got, err := AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, nil)
	})

	t.Run("non-fly app change deploys nothing", func(t *testing.T) {
		changed := []string{"apps/calendar/main.go"}
		got, err := AffectedApps(flyApps, changed, resolve)
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
		got, err := AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, []string{"dogs"})
	})

	t.Run("mixed app and pkg changes", func(t *testing.T) {
		changed := []string{
			"apps/dogs/main.go",
			"pkg/posts/render.go", // affects writing
		}
		got, err := AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, []string{"dogs", "writing"})
	})

	t.Run("empty changed files deploys nothing", func(t *testing.T) {
		changed := []string{}
		got, err := AffectedApps(flyApps, changed, resolve)
		if err != nil {
			t.Fatal(err)
		}
		assertApps(t, got, nil)
	})
}

func TestIsImageAffected(t *testing.T) {
	resolve := fakeDeps(map[string][]string{
		"apps/ci/cmd/builder": {"pkg/ci/changedetect", "pkg/depgraph", "pkg/oci"},
	})

	t.Run("dockerfile changed", func(t *testing.T) {
		changed := []string{"apps/ci/builder.Dockerfile"}
		got, err := IsImageAffected(changed, "apps/ci/builder.Dockerfile", resolve, "apps/ci/cmd/builder")
		if err != nil {
			t.Fatal(err)
		}
		if !got {
			t.Error("expected image to be affected when Dockerfile changed")
		}
	})

	t.Run("package source changed", func(t *testing.T) {
		changed := []string{"apps/ci/cmd/builder/deployer.go"}
		got, err := IsImageAffected(changed, "apps/ci/builder.Dockerfile", resolve, "apps/ci/cmd/builder")
		if err != nil {
			t.Fatal(err)
		}
		if !got {
			t.Error("expected image to be affected when package source changed")
		}
	})

	t.Run("dep changed", func(t *testing.T) {
		changed := []string{"pkg/ci/changedetect/changedetect.go"}
		got, err := IsImageAffected(changed, "apps/ci/builder.Dockerfile", resolve, "apps/ci/cmd/builder")
		if err != nil {
			t.Fatal(err)
		}
		if !got {
			t.Error("expected image to be affected when dep changed")
		}
	})

	t.Run("unrelated change", func(t *testing.T) {
		changed := []string{"apps/dogs/main.go"}
		got, err := IsImageAffected(changed, "apps/ci/builder.Dockerfile", resolve, "apps/ci/cmd/builder")
		if err != nil {
			t.Fatal(err)
		}
		if got {
			t.Error("expected image not to be affected by unrelated change")
		}
	})

	t.Run("base image no pkg path", func(t *testing.T) {
		changed := []string{"apps/ci/base.Dockerfile"}
		got, err := IsImageAffected(changed, "apps/ci/base.Dockerfile", nil, "")
		if err != nil {
			t.Fatal(err)
		}
		if !got {
			t.Error("expected base image to be affected when Dockerfile changed")
		}
	})

	t.Run("base image unrelated change", func(t *testing.T) {
		changed := []string{"apps/dogs/main.go"}
		got, err := IsImageAffected(changed, "apps/ci/base.Dockerfile", nil, "")
		if err != nil {
			t.Fatal(err)
		}
		if got {
			t.Error("expected base image not to be affected by unrelated change")
		}
	})
}

func TestLoadFlyApps(t *testing.T) {
	root := t.TempDir()

	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "apps.toml"), []byte(`
[apps.proxy]
  vm_size = "shared-cpu-8x"
  [[apps.proxy.routes]]
    path = "proxy"
    host = "fly"
    access = "tag:service"

[apps.dogs]
  vm_size = "shared-cpu-2x"
  [[apps.dogs.routes]]
    path = "dogs"
    host = "fly"
    access = "autogroup:danger-all"

[apps.logs]
  vm_size = "shared-cpu-4x"
  [[apps.logs.routes]]
    path = "logs"
    host = "fly"
    access = "tag:service"

[apps.calendar]
  [[apps.calendar.routes]]
    path = "calendar"
    host = "brigid"
    access = "ajm@passkey"
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
	if err := os.WriteFile(filepath.Join(configDir, "apps.toml"), []byte(`
[defaults]
  region = "ord"
  vm_size = "shared-cpu-1x"
  vm_memory = "256"

[apps.dogs]
  vm_size = "shared-cpu-2x"
  vm_memory = "1gb"
  packages = ["sqlite"]
  [[apps.dogs.routes]]
    path = "dogs"
    host = "fly"
    access = "autogroup:danger-all"
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
