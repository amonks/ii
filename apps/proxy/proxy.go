package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type proxy struct {
	routes map[string]int
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	for prefix, port := range p.routes {
		if !strings.HasPrefix(req.URL.Path, prefix) {
			continue
		}
		if req.URL.Path == prefix {
			http.Redirect(w, req, req.URL.String()+"/", 301)
			return
		}
		p.proxyRequest(prefix, port, w, req)
		return
	}

	path := filepath.Join(os.Getenv("MONKS_ROOT"), "static", req.URL.Path)
	http.ServeFile(w, req, path)
}

func (p *proxy) proxyRequest(prefix string, port int, w http.ResponseWriter, req *http.Request) {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(req *httputil.ProxyRequest) {
			req.Out.URL.Scheme = "http"
			req.Out.URL.Host = fmt.Sprintf("0.0.0.0:%d", port)
			req.Out.URL.Path = strings.TrimPrefix(req.Out.URL.Path, prefix)
			req.Out.Host = req.Out.URL.Host
		},
	}
	proxy.ServeHTTP(w, req)
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
