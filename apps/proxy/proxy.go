package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"monks.co/pkg/env"
	"monks.co/pkg/gzipserver"
	"monks.co/pkg/prometh"
	"monks.co/pkg/reqlog"
)

type proxy struct {
	rewrites  map[string]string
	transport http.RoundTripper
}

// routesFromCaps builds a route table from Tailscale-Cap-* headers.
// Each cap header contains a JSON array of {path, backend} entries.
func routesFromCaps(req *http.Request) map[string]string {
	log := reqlog.Logger(req.Context())
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
			log.Warn("failed to parse cap header",
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

	ctx := req.Context()
	user := req.Header.Get("Tailscale-User")
	firstSegment := strings.Split(req.URL.Path, "/")[1]
	routes := routesFromCaps(req)

	reqlog.Set(ctx, "proxy.user", user)
	reqlog.Set(ctx, "proxy.routes", routeKeys(routes))

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

		reqlog.Set(ctx, "proxy.action", "proxy")
		reqlog.Set(ctx, "proxy.upstream", firstSegment)
		reqlog.Set(ctx, "proxy.backend", backend)
		p.proxyRequest(firstSegment, backend, w, req)
		return
	}

	reqlog.Set(ctx, "proxy.action", "static")
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

func (w *StatusCodeWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (p *proxy) proxyRequest(prefix string, backend string, w http.ResponseWriter, req *http.Request) {
	proxy := &httputil.ReverseProxy{
		Transport: p.transport,
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.Out.URL.Scheme = "http"
			pr.Out.URL.Host = backend
			pr.Out.URL.Path = strings.TrimPrefix(pr.Out.URL.Path, "/"+prefix)
			pr.Out.Host = backend
			// Forward request ID to downstream.
			if id := reqlog.RequestID(pr.In.Context()); id != "" {
				pr.Out.Header.Set(reqlog.RequestIDHeader, id)
			}
		},
	}
	startAt := time.Now()
	scw := &StatusCodeWriter{ResponseWriter: w}
	proxy.ServeHTTP(scw, req)
	dur := time.Since(startAt)

	reqlog.Set(req.Context(), "proxy.upstream_ms", dur.Milliseconds())
	reqlog.Set(req.Context(), "proxy.upstream_status", scw.code)
	reqlog.Set(req.Context(), "proxy.upstream_route", scw.route)

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
