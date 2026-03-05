package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppsFrom(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "apps.toml")
	if err := os.WriteFile(path, []byte(`
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

  [[apps.dogs.routes]]
    path = "dogs"
    host = "thor"
    access = "ajm@passkey"

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

	cfg, err := LoadAppsFrom(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Defaults.Region != "ord" {
		t.Errorf("expected region ord, got %s", cfg.Defaults.Region)
	}

	dogs, ok := cfg.Apps["dogs"]
	if !ok {
		t.Fatal("expected dogs app")
	}
	if dogs.VMSize != "shared-cpu-2x" {
		t.Errorf("expected dogs vm_size shared-cpu-2x, got %s", dogs.VMSize)
	}
	if len(dogs.Routes) != 2 {
		t.Fatalf("expected 2 routes for dogs, got %d", len(dogs.Routes))
	}
	if dogs.Routes[0].Host != "fly" {
		t.Errorf("expected first dogs route host fly, got %s", dogs.Routes[0].Host)
	}
	if dogs.Routes[0].Access != "autogroup:danger-all" {
		t.Errorf("expected first dogs route access autogroup:danger-all, got %s", dogs.Routes[0].Access)
	}
}

func TestListHosts(t *testing.T) {
	cfg := &AppsConfig{
		Apps: map[string]AppEntry{
			"dogs": {Routes: []Route{
				{Host: "fly"},
				{Host: "thor"},
			}},
			"logs":     {Routes: []Route{{Host: "fly"}}},
			"calendar": {Routes: []Route{{Host: "brigid"}}},
		},
	}

	hosts := cfg.ListHosts()
	expected := []string{"brigid", "fly", "thor"}
	if len(hosts) != len(expected) {
		t.Fatalf("expected %d hosts, got %d: %v", len(expected), len(hosts), hosts)
	}
	for i, h := range hosts {
		if h != expected[i] {
			t.Errorf("host[%d] = %q, want %q", i, h, expected[i])
		}
	}
}

func TestAppsForHost(t *testing.T) {
	cfg := &AppsConfig{
		Apps: map[string]AppEntry{
			"dogs":     {Routes: []Route{{Host: "fly"}, {Host: "thor"}}},
			"logs":     {Routes: []Route{{Host: "fly"}}},
			"calendar": {Routes: []Route{{Host: "brigid"}}},
		},
	}

	flyApps := cfg.AppsForHost("fly")
	if len(flyApps) != 2 || flyApps[0] != "dogs" || flyApps[1] != "logs" {
		t.Errorf("expected [dogs logs], got %v", flyApps)
	}

	thorApps := cfg.AppsForHost("thor")
	if len(thorApps) != 1 || thorApps[0] != "dogs" {
		t.Errorf("expected [dogs], got %v", thorApps)
	}

	brigidApps := cfg.AppsForHost("brigid")
	if len(brigidApps) != 1 || brigidApps[0] != "calendar" {
		t.Errorf("expected [calendar], got %v", brigidApps)
	}
}

func TestFlyApps(t *testing.T) {
	cfg := &AppsConfig{
		Apps: map[string]AppEntry{
			"dogs":     {Routes: []Route{{Host: "fly"}, {Host: "thor"}}},
			"logs":     {Routes: []Route{{Host: "fly"}}},
			"calendar": {Routes: []Route{{Host: "brigid"}}},
		},
	}

	flyApps := cfg.FlyApps()
	if len(flyApps) != 2 || flyApps[0] != "dogs" || flyApps[1] != "logs" {
		t.Errorf("expected [dogs logs], got %v", flyApps)
	}
}
