package config

import (
	"os"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"monks.co/pkg/env"
	"monks.co/pkg/tls"
)

type Config struct {
	Mode      string            `toml:"mode"`
	AppList   []string          `toml:"apps"`
	Services  []Service         `toml:"service"`
	ACME      tls.ACME          `toml:"acme"`
	Redirects map[string]string `toml:"redirects"`
}

func (c *Config) Apps() []string {
	if len(c.AppList) > 0 {
		return c.AppList
	}
	apps := map[string]struct{}{}
	for _, service := range c.Services {
		for app := range service.Apps {
			apps[app] = struct{}{}
		}
	}
	var list []string
	for app := range apps {
		list = append(list, app)
	}
	return list
}

type Service struct {
	Type        string            `toml:"type"`
	Addr        string            `toml:"addr"`
	Apps        map[string]string `toml:"apps"`
	ExtraRoutes map[string]int    `toml:"extra_routes"`
	StoragePath string            `toml:"storage_path"`
	Rewrites    map[string]string `toml:"rewrites"`
}

var variables = map[string]func() (string, error){
	"$TAILSCALE_IP": func() (string, error) {
		// TODO: shell out to `tailscale ip | head -n1`
		return "100.77.26.146", nil
	},
}

func ListMachines() ([]string, error) {
	listing, err := os.ReadDir(env.InMonksRoot("config"))
	if err != nil {
		return nil, err
	}
	var machines []string
	for _, entry := range listing {
		if entry.IsDir() {
			continue
		}
		if name := entry.Name(); strings.HasSuffix(name, ".toml") {
			machines = append(machines, strings.TrimSuffix(name, ".toml"))
		}
	}

	sort.Strings(machines)
	return machines, nil
}

func Load(machine string) (*Config, error) {
	path := env.InMonksRoot("config", machine+".toml")
	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	bs, err = resolveVariables(bs)
	if err != nil {
		return nil, err
	}
	var config Config
	if err := toml.Unmarshal(bs, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func resolveVariables(bs []byte) ([]byte, error) {
	var foundKeys []string
	for k := range variables {
		if strings.Contains(string(bs), k) {
			foundKeys = append(foundKeys, k)
		}
	}
	for _, k := range foundKeys {
		resolve := variables[k]
		val, err := resolve()
		if err != nil {
			return nil, err
		}
		bs = []byte(strings.ReplaceAll(string(bs), k, val))
	}
	return bs, nil
}
