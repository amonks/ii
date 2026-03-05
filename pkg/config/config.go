package config

import (
	"fmt"
	"os"
	"sort"

	"github.com/BurntSushi/toml"
	"monks.co/pkg/env"
	"monks.co/pkg/tls"
)

// AppsConfig represents the top-level config/apps.toml.
type AppsConfig struct {
	Defaults Defaults            `toml:"defaults"`
	Apps     map[string]AppEntry `toml:"apps"`
}

// Defaults are the default values for fly-deployed apps.
type Defaults struct {
	Region   string `toml:"region"`
	VMSize   string `toml:"vm_size"`
	VMMemory string `toml:"vm_memory"`
}

// AppEntry is the configuration for a single app.
type AppEntry struct {
	VMSize   string   `toml:"vm_size"`
	VMMemory string   `toml:"vm_memory"`
	Public   bool     `toml:"public"`
	Packages []string `toml:"packages"`
	Files    []string `toml:"files"`
	Cmd      []string `toml:"cmd"`
	Routes   []Route  `toml:"routes"`
}

// Route is a single proxy route for an app.
type Route struct {
	Path         string   `toml:"path"`
	Host         string   `toml:"host"`
	Access       string   `toml:"access"`
	Capabilities []string `toml:"capabilities"`
}

// LoadApps reads config/apps.toml.
func LoadApps() (*AppsConfig, error) {
	return LoadAppsFrom(env.InMonksRoot("config", "apps.toml"))
}

// LoadAppsFrom reads an apps.toml file from the given path.
func LoadAppsFrom(path string) (*AppsConfig, error) {
	var cfg AppsConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &cfg, nil
}

// ListHosts returns sorted unique hosts derived from all routes.
func (c *AppsConfig) ListHosts() []string {
	set := map[string]struct{}{}
	for _, app := range c.Apps {
		for _, r := range app.Routes {
			set[r.Host] = struct{}{}
		}
	}
	hosts := make([]string, 0, len(set))
	for h := range set {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	return hosts
}

// AppsForHost returns sorted app names that have a route on the given host.
func (c *AppsConfig) AppsForHost(host string) []string {
	set := map[string]struct{}{}
	for name, app := range c.Apps {
		for _, r := range app.Routes {
			if r.Host == host {
				set[name] = struct{}{}
			}
		}
	}
	apps := make([]string, 0, len(set))
	for name := range set {
		apps = append(apps, name)
	}
	sort.Strings(apps)
	return apps
}

// FlyApps returns sorted app names that have at least one route with host "fly".
func (c *AppsConfig) FlyApps() []string {
	return c.AppsForHost("fly")
}

// FlyAppNames returns the sorted fly app names (convenience for change detection).
func FlyAppNames(cfg *AppsConfig) []string {
	return cfg.FlyApps()
}

// ProxyConfig represents the proxy runtime config (config/proxy.toml).
type ProxyConfig struct {
	Services  []Service         `toml:"service"`
	ACME      tls.ACME          `toml:"acme"`
	Redirects map[string]string `toml:"redirects"`
}

// Service is a listener configuration for the proxy.
type Service struct {
	Type     string            `toml:"type"`
	Addr     string            `toml:"addr"`
	Rewrites map[string]string `toml:"rewrites"`
}

// LoadProxy reads config/proxy.toml.
func LoadProxy() (*ProxyConfig, error) {
	path := env.InMonksRoot("config", "proxy.toml")
	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config ProxyConfig
	if err := toml.Unmarshal(bs, &config); err != nil {
		return nil, err
	}
	return &config, nil
}
