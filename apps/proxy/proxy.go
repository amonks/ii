package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"monks.co/pkg/env"
	"monks.co/pkg/gzipserver"
	"monks.co/pkg/prometh"
)

type proxy struct {
	routes    map[string]string
	rewrites  map[string]string
	transport http.RoundTripper
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if to, hasRewrite := p.rewrites[req.URL.Path]; hasRewrite {
		req.URL.Path = to
	}

	firstSegment := strings.Split(req.URL.Path, "/")[1]
	if backend, hasRoute := p.routes[firstSegment]; hasRoute {
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

		p.proxyRequest(firstSegment, backend, w, req)
		return
	}

	srv := gzipserver.FileServer(gzipserver.Dir(env.InMonksRoot("static")))
	srv.ServeHTTP(w, req)
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
			log.Println("proxy", req.In.URL.String(), req.Out.URL.String())
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
