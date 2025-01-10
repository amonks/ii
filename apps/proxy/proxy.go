package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	"monks.co/pkg/env"
	"monks.co/pkg/gzip"
)

type proxy struct {
	routes   map[string]int
	rewrites map[string]string
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if to, hasRewrite := p.rewrites[req.URL.Path]; hasRewrite {
		req.URL.Path = to
	}

	firstSegment := strings.Split(req.URL.Path, "/")[1]
	if port, hasRoute := p.routes[firstSegment]; hasRoute {
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
			http.Redirect(w, req, req.URL.String()+"/", 301)
			return
		}

		p.proxyRequest(firstSegment, port, w, req)
		return
	}

	staticFilePath := env.InMonksRoot("static", req.URL.Path)
	gzip.Middleware(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.ServeFile(w, req, staticFilePath)
	})).ServeHTTP(w, req)
}

type statusListener struct {
	http.ResponseWriter
	code int
}

func (s *statusListener) WriteHeader(code int) {
	s.code = code
	s.ResponseWriter.WriteHeader(code)
}

func (p *proxy) proxyRequest(prefix string, port int, w http.ResponseWriter, req *http.Request) {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(req *httputil.ProxyRequest) {
			req.Out.URL.Scheme = "http"
			req.Out.URL.Host = fmt.Sprintf("0.0.0.0:%d", port)
			req.Out.URL.Path = strings.TrimPrefix(req.Out.URL.Path, "/"+prefix)
			req.Out.Host = req.Out.URL.Host
			log.Println("proxy", req.In.URL.String(), req.Out.URL.String())
		},
	}
	startAt := time.Now()
	lis := &statusListener{ResponseWriter: w}
	proxy.ServeHTTP(lis, req)
	dur := time.Now().Sub(startAt)

	labels := []string{
		req.Host,
		strings.Split(req.URL.Path, "/")[1],
		req.URL.Path,
		fmt.Sprintf("%d", lis.code),
		req.Header.Get("user-agent"),
	}
	requestDurationsMetric.WithLabelValues(labels...).Observe(float64(dur.Milliseconds()))
	requestsMetric.WithLabelValues(labels...).Inc()
}

func parseRoutes(args []string) (map[string]int, error) {
	routes := make(map[string]int, len(args))
	for _, p := range args {
		parts := strings.Split(p, ":")
		if port, err := strconv.ParseInt(parts[1], 10, 64); err != nil {
			return nil, err
		} else {
			prefix := parts[0]
			if !strings.HasPrefix(prefix, "/") {
				prefix = "/" + prefix
			}
			routes[prefix] = int(port)
		}
	}
	return routes, nil
}
