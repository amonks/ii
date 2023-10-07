package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"monks.co/pkg/tls"
	"monks.co/pkg/traffic"
)

var (
	httpRedirectAddress = flag.String("httpRedirectAddress", "0.0.0.0:80", "address to listen at for http->https redirect, default: 0.0.0.0:80")
	httpsAddress        = flag.String("httpsAddress", "0.0.0.0:443", "address to listen at (https), default: 0.0.0.0:443")
	acmeConfig          = flag.String("acmeConfig", "", "acme config file path")
)

func main() {
	flag.Parse()
	ctx, cancel := context.WithCancel(context.Background())

	routes, err := parseRoutes(flag.Args())
	if err != nil {
		panic(err)
	}

	httpsServer, err := newHTTPSServer(ctx, routes)
	if err != nil {
		panic(err)
	}

	httpServer, err := newHTTPServer(ctx)
	if err != nil {
		panic(err)
	}

	exit := make(chan error)

	go func() {
		fmt.Println("listening at " + *httpsAddress)
		exit <- httpsServer.ListenAndServeTLS("", "")
	}()

	go func() {
		fmt.Println("listening at " + *httpRedirectAddress)
		exit <- httpServer.ListenAndServe()
	}()

	err = <-exit
	cancel()
	<-exit
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func newHTTPServer(ctx context.Context) (*http.Server, error) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		u := req.URL
		u.Host = net.JoinHostPort(req.Host, "443")
		u.Scheme = "https"
		http.Redirect(w, req, u.String(), 301)
	})
	srv := &http.Server{
		Addr:    *httpRedirectAddress,
		Handler: handler,
	}
	return srv, nil
}

func newHTTPSServer(ctx context.Context, routes map[string]int) (*http.Server, error) {
	p, err := traffic.New(*httpsAddress, &proxy{routes})
	if err != nil {
		return nil, err
	}

	tlsConfig, _, err := tls.ReadTLSConfig(context.Background(), *acmeConfig)
	if err != nil {
		return nil, err
	}
	srv := &http.Server{
		Addr:      *httpsAddress,
		Handler:   p,
		TLSConfig: tlsConfig,
	}

	return srv, nil
}

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
