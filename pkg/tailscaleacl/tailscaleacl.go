// Package tailscaleacl generates a complete Tailscale ACL policy by
// merging config/tailscale-acl-base.jsonc with routing grants derived
// from config/apps.toml.
package tailscaleacl

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	tailscale "tailscale.com/client/tailscale/v2"

	"monks.co/pkg/config"
	"monks.co/pkg/env"
)

// Generate produces the full ACL policy as a typed [tailscale.ACL].
func Generate() (*tailscale.ACL, error) {
	cfg, err := config.LoadApps()
	if err != nil {
		return nil, fmt.Errorf("loading apps config: %w", err)
	}

	basePath := env.InMonksRoot("config", "tailscale-acl-base.jsonc")
	baseBytes, err := os.ReadFile(basePath)
	if err != nil {
		return nil, fmt.Errorf("reading base ACL: %w", err)
	}

	// Strip JSONC comments so we can unmarshal as plain JSON.
	baseBytes = stripJSONCComments(baseBytes)

	var acl tailscale.ACL
	if err := json.Unmarshal(baseBytes, &acl); err != nil {
		return nil, fmt.Errorf("parsing base ACL: %w", err)
	}

	acl.Grants = append(acl.Grants, generateGrants(cfg)...)

	return &acl, nil
}

func generateGrants(cfg *config.AppsConfig) []tailscale.Grant {
	// Group routes by access value.
	type routeInfo struct {
		path         string
		backend      string
		capabilities []string
	}
	accessRoutes := map[string][]routeInfo{}

	for name, app := range cfg.Apps {
		for _, r := range app.Routes {
			backend := deriveBackend(name, r.Host, cfg.Defaults.Region)
			ri := routeInfo{
				path:         r.Path,
				backend:      backend,
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

	var grants []tailscale.Grant

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
			if routes[i].path != routes[j].path {
				return routes[i].path < routes[j].path
			}
			return routes[i].backend < routes[j].backend
		})

		// Build the public routing grant.
		entries := make([]map[string]any, len(routes))
		for i, ri := range routes {
			entries[i] = map[string]any{
				"path":    ri.path,
				"backend": ri.backend,
			}
		}

		grants = append(grants, tailscale.Grant{
			Source:      []string{access},
			Destination: []string{"tag:monks-co"},
			App: map[string][]map[string]any{
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

				grants = append(grants, tailscale.Grant{
					Source:      []string{access},
					Destination: []string{"tag:monks-co"},
					App: map[string][]map[string]any{
						"monks.co/cap/" + cap: {{}},
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
