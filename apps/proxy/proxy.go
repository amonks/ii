package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"monks.co/pkg/env"
	"monks.co/pkg/gzipserver"
	"monks.co/pkg/prometh"
)

type proxy struct {
	rewrites  map[string]string
	transport http.RoundTripper
}

// routesFromCaps builds a route table from Tailscale-Cap-* headers.
// Each cap header contains a JSON array of {path, backend} entries.
func routesFromCaps(req *http.Request) map[string]string {
	routes := map[string]string{}
	for key, values := range req.Header {
		if !strings.HasPrefix(key, "Tailscale-Cap-") {
			continue
		}
		var entries []struct {
			Path    string `json:"path"`
			Backend string `json:"backend"`
		}
		if err := json.Unmarshal([]byte(values[0]), &entries); err != nil {
			slog.Warn("route: failed to parse cap header",
				"header", key,
				"value", values[0],
				"error", err,
			)
			continue
		}
		for _, e := range entries {
			if e.Path != "" && e.Backend != "" {
				routes[e.Path] = e.Backend
			}
		}
	}
	return routes
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if to, hasRewrite := p.rewrites[req.URL.Path]; hasRewrite {
		req.URL.Path = to
	}

	user := req.Header.Get("Tailscale-User")
	firstSegment := strings.Split(req.URL.Path, "/")[1]
	routes := routesFromCaps(req)

	if backend, hasRoute := routes[firstSegment]; hasRoute {
		// We need to visit the subsites at a url that ends in a "/",
		// otherwise relative links within the subsite won't use the
		// subsite's prefix.
		//
		// That is, if you click a link with href="page", there are two
		// places you might go. If you're currently on `/map/`, the
		// link takes you to `/map/page`. If you're on `/map`, without
		// the trailing slash, it'll take you to '/page'.
		//
		// TODO: I'm getting redirect loops from this sometimes.
		if req.URL.Path == "/"+firstSegment {
			http.Redirect(w, req, req.URL.String()+"/", http.StatusMovedPermanently)
			return
		}

		slog.Info("route: proxying",
			"user", user,
			"host", req.Host,
			"method", req.Method,
			"path", req.URL.Path,
			"app", firstSegment,
			"backend", backend,
			"routes", routeKeys(routes),
		)
		p.proxyRequest(firstSegment, backend, w, req)
		return
	}

	slog.Info("route: static",
		"user", user,
		"host", req.Host,
		"method", req.Method,
		"path", req.URL.Path,
		"routes", routeKeys(routes),
	)
	srv := gzipserver.FileServer(gzipserver.Dir(env.InMonksRoot("apps", "proxy", "static")))
	srv.ServeHTTP(w, req)
}

func routeKeys(routes map[string]string) []string {
	keys := make([]string, 0, len(routes))
	for k := range routes {
		keys = append(keys, k)
	}
	return keys
}

type StatusCodeWriter struct {
	http.ResponseWriter
	code  int
	route string
}

func (w *StatusCodeWriter) WriteHeader(code int) {
	w.code = code
	w.route = w.Header().Get("x-mux-route")
	w.Header().Del("x-mux-route")
	w.ResponseWriter.WriteHeader(code)
}

func (p *proxy) proxyRequest(prefix string, backend string, w http.ResponseWriter, req *http.Request) {
	proxy := &httputil.ReverseProxy{
		Transport: p.transport,
		Rewrite: func(req *httputil.ProxyRequest) {
			req.Out.URL.Scheme = "http"
			req.Out.URL.Host = backend
			req.Out.URL.Path = strings.TrimPrefix(req.Out.URL.Path, "/"+prefix)
			req.Out.Host = backend
		},
	}
	startAt := time.Now()
	scw := &StatusCodeWriter{ResponseWriter: w}
	proxy.ServeHTTP(scw, req)
	dur := time.Since(startAt)

	labels := prometh.SanitizeLabels(
		req.Host,
		strings.Split(req.URL.Path, "/")[1],
		scw.route,
		fmt.Sprintf("%d", scw.code),
		req.Header.Get("user-agent"),
	)
	requestDurationsMetric.WithLabelValues(labels...).Observe(float64(dur.Milliseconds()))
	requestsMetric.WithLabelValues(labels...).Inc()
}
