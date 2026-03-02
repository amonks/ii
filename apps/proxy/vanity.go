package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"monks.co/pkg/env"
)

type vanityModule struct {
	modulePath   string // e.g., "monks.co/pkg/serve"
	mirror       string // e.g., "github.com/amonks/go" or "github.com/amonks/run"
	importPrefix string // go-import prefix: "monks.co" for default mirror, modulePath for explicit
	dir          string // directory in monorepo, e.g., "pkg/serve"
}

// loadVanityModules loads the public package list from config/publish.toml
// and returns vanity module entries and the default mirror (if any).
func loadVanityModules() ([]vanityModule, string, error) {
	path := env.InMonksRoot("config", "publish.toml")

	var cfg struct {
		DefaultMirror string `toml:"default_mirror"`
		Package       []struct {
			Dir        string `toml:"dir"`
			ModulePath string `toml:"module_path"`
			Mirror     string `toml:"mirror"`
		} `toml:"package"`
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, "", fmt.Errorf("loading publish config: %w", err)
	}

	var mods []vanityModule
	for _, pkg := range cfg.Package {
		modulePath := pkg.ModulePath
		if modulePath == "" {
			modulePath = "monks.co/" + pkg.Dir
		}

		mirror := pkg.Mirror
		importPrefix := modulePath
		if mirror == "" {
			// Default mirror: go-import prefix is "monks.co" so Go
			// knows the VCS root and can find modules in subdirectories.
			mirror = cfg.DefaultMirror
			importPrefix = "monks.co"
		}

		if mirror == "" {
			// No explicit mirror and no default mirror configured.
			mirror = "github.com/amonks/" + filepath.Base(pkg.Dir)
			importPrefix = modulePath
		}

		mods = append(mods, vanityModule{
			modulePath:   modulePath,
			mirror:       mirror,
			importPrefix: importPrefix,
			dir:          pkg.Dir,
		})
	}
	return mods, cfg.DefaultMirror, nil
}

// vanityHandler returns a function that handles go-import meta tag requests
// and redirects human visitors to the GitHub mirror. It returns true if the
// request was handled.
func vanityHandler(modules []vanityModule, defaultMirror string) func(http.ResponseWriter, *http.Request) bool {
	return func(w http.ResponseWriter, r *http.Request) bool {
		// Handle root ?go-get=1 for Go's verification of the VCS root.
		if r.URL.Path == "/" && r.URL.Query().Get("go-get") == "1" && defaultMirror != "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
<meta name="go-import" content="monks.co git https://%s">
</head>
<body>
</body>
</html>`, defaultMirror)
			return true
		}

		mod := matchVanityModule(r.URL.Path, modules)
		if mod == nil {
			return false
		}

		if r.URL.Query().Get("go-get") == "1" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
<meta name="go-import" content="%s git https://%s">
<meta http-equiv="refresh" content="0; url=https://pkg.go.dev/%s">
</head>
<body>
Redirecting to <a href="https://pkg.go.dev/%s">pkg.go.dev</a>...
</body>
</html>`, mod.importPrefix, mod.mirror, mod.modulePath, mod.modulePath)
			return true
		}

		// Human visitor: redirect to GitHub.
		if mod.importPrefix == "monks.co" {
			// Default mirror: link to the specific directory.
			http.Redirect(w, r, "https://"+mod.mirror+"/tree/main/"+mod.dir, http.StatusTemporaryRedirect)
		} else {
			http.Redirect(w, r, "https://"+mod.mirror, http.StatusTemporaryRedirect)
		}
		return true
	}
}

// matchVanityModule finds the module that matches the request path.
// The path "/pkg/serve" matches "monks.co/pkg/serve".
// The path "/cmd/run/runner" matches "monks.co/cmd/run" (subpackage).
func matchVanityModule(path string, modules []vanityModule) *vanityModule {
	// Remove leading slash.
	path = strings.TrimPrefix(path, "/")

	// Only match multi-segment paths (single-segment = app routes).
	if !strings.Contains(path, "/") {
		return nil
	}

	for i := range modules {
		prefix := strings.TrimPrefix(modules[i].modulePath, "monks.co/")
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return &modules[i]
		}
	}
	return nil
}
