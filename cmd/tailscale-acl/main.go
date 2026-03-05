// Command tailscale-acl generates a complete Tailscale ACL JSON by
// merging config/tailscale-acl-base.jsonc with routing grants derived
// from config/apps.toml.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"monks.co/pkg/config"
	"monks.co/pkg/env"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadApps()
	if err != nil {
		return fmt.Errorf("loading apps config: %w", err)
	}

	basePath := env.InMonksRoot("config", "tailscale-acl-base.jsonc")
	baseBytes, err := os.ReadFile(basePath)
	if err != nil {
		return fmt.Errorf("reading base ACL: %w", err)
	}

	// Strip JSONC comments.
	baseBytes = stripJSONCComments(baseBytes)

	var base map[string]any
	if err := json.Unmarshal(baseBytes, &base); err != nil {
		return fmt.Errorf("parsing base ACL: %w", err)
	}

	grants := generateGrants(cfg)

	// Append to existing grants.
	existingGrants, _ := base["grants"].([]any)
	for _, g := range grants {
		existingGrants = append(existingGrants, g)
	}
	base["grants"] = existingGrants

	out, err := json.MarshalIndent(base, "", "    ")
	if err != nil {
		return fmt.Errorf("marshaling ACL: %w", err)
	}

	fmt.Println(string(out))
	return nil
}

type routeEntry struct {
	Path    string `json:"path"`
	Backend string `json:"backend"`
}

type grant struct {
	Src []string       `json:"src"`
	Dst []string       `json:"dst"`
	App map[string]any `json:"app"`
}

func generateGrants(cfg *config.AppsConfig) []grant {
	// Group routes by access value.
	type routeInfo struct {
		entry        routeEntry
		capabilities []string
	}
	accessRoutes := map[string][]routeInfo{}

	for name, app := range cfg.Apps {
		for _, r := range app.Routes {
			backend := deriveBackend(name, r.Host, cfg.Defaults.Region)
			ri := routeInfo{
				entry:        routeEntry{Path: r.Path, Backend: backend},
				capabilities: r.Capabilities,
			}
			accessRoutes[r.Access] = append(accessRoutes[r.Access], ri)
		}
	}

	// Sort access values for deterministic output.
	accessValues := make([]string, 0, len(accessRoutes))
	for a := range accessRoutes {
		accessValues = append(accessValues, a)
	}
	sort.Strings(accessValues)

	var grants []grant

	// Track capability grants to deduplicate.
	type capKey struct {
		access string
		cap    string
	}
	capGrants := map[capKey]bool{}

	for _, access := range accessValues {
		routes := accessRoutes[access]

		// Sort routes for deterministic output.
		sort.Slice(routes, func(i, j int) bool {
			if routes[i].entry.Path != routes[j].entry.Path {
				return routes[i].entry.Path < routes[j].entry.Path
			}
			return routes[i].entry.Backend < routes[j].entry.Backend
		})

		// Build the public routing grant.
		entries := make([]routeEntry, len(routes))
		for i, ri := range routes {
			entries[i] = ri.entry
		}

		grants = append(grants, grant{
			Src: []string{access},
			Dst: []string{"tag:monks-co"},
			App: map[string]any{
				"monks.co/cap/public": entries,
			},
		})

		// Build capability grants.
		for _, ri := range routes {
			for _, cap := range ri.capabilities {
				key := capKey{access: access, cap: cap}
				if capGrants[key] {
					continue
				}
				capGrants[key] = true

				grants = append(grants, grant{
					Src: []string{access},
					Dst: []string{"tag:monks-co"},
					App: map[string]any{
						"monks.co/cap/" + cap: []map[string]any{{}},
					},
				})
			}
		}
	}

	return grants
}

func deriveBackend(app, host, defaultRegion string) string {
	if host == "fly" {
		return fmt.Sprintf("monks-%s-fly-%s", app, defaultRegion)
	}
	return fmt.Sprintf("monks-%s-%s", app, host)
}

// stripJSONCComments removes // line comments from JSONC.
func stripJSONCComments(b []byte) []byte {
	// Simple state machine: track whether we're inside a string.
	out := make([]byte, 0, len(b))
	i := 0
	for i < len(b) {
		if b[i] == '"' {
			// Copy the entire string literal.
			out = append(out, b[i])
			i++
			for i < len(b) && b[i] != '"' {
				if b[i] == '\\' && i+1 < len(b) {
					out = append(out, b[i], b[i+1])
					i += 2
					continue
				}
				out = append(out, b[i])
				i++
			}
			if i < len(b) {
				out = append(out, b[i])
				i++
			}
		} else if i+1 < len(b) && b[i] == '/' && b[i+1] == '/' {
			// Skip until end of line.
			for i < len(b) && b[i] != '\n' {
				i++
			}
		} else {
			out = append(out, b[i])
			i++
		}
	}
	return out
}
