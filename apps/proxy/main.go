package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	proxyproto "github.com/pires/go-proxyproto"

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

	a := &app{routes}

	exit := make(chan error)

	go func() { exit <- a.httpsServer(ctx).serve() }()
	go func() { exit <- a.redirectServer(ctx).serve() }()

	err = <-exit
	cancel()
	<-exit
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type app struct {
	routes map[string]int
}

func (a *app) redirectServer(ctx context.Context) *server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		u := req.URL
		u.Host = net.JoinHostPort(req.Host, "443")
		u.Scheme = "https"
		http.Redirect(w, req, u.String(), 301)
	})
	srv := &http.Server{
		ConnContext: connContext,
		Addr:        *httpRedirectAddress,
		Handler:     handler,
	}
	return &server{srv, false}
}

func connContext(ctx context.Context, conn net.Conn) context.Context {
	if conn, ok := conn.(*proxyproto.Conn); ok {
		if conn.LocalAddr() == nil {
			fmt.Printf("couldn't retrieve local address")
		}
		fmt.Printf("local address: %q", conn.LocalAddr().String())
		if conn.RemoteAddr() == nil {
			fmt.Printf("couldn't retrieve remote address")
		}
		fmt.Printf("remote address: %q", conn.RemoteAddr().String())

		return context.WithValue(ctx, traffic.RemoteAddrKey, conn.RemoteAddr().String())
	}
	return context.WithValue(ctx, traffic.RemoteAddrKey, conn.RemoteAddr().String())
}

func (a *app) httpsServer(ctx context.Context) *server {
	p, err := traffic.New(*httpsAddress, &proxy{a.routes})
	if err != nil {
		panic(err)
	}

	tlsConfig, _, err := tls.ReadTLSConfig(context.Background(), *acmeConfig)
	if err != nil {
		panic(err)
	}
	srv := &http.Server{
		ConnContext: connContext,
		Addr:        *httpsAddress,
		Handler:     BMRRedirectorHandler(p),
		TLSConfig:   tlsConfig,
	}

	return &server{srv, true}
}

type server struct {
	*http.Server
	tls bool
}

func (s *server) serve() error {
	fmt.Println("will listen on", s.Addr)
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	proxyListener := &proxyproto.Listener{
		Listener:          ln,
		ReadHeaderTimeout: 10 * time.Second,
	}
	defer proxyListener.Close()
	fmt.Println("listening on", s.Addr)
	if s.tls {
		return s.ServeTLS(proxyListener, "", "")
	} else {
		return s.Serve(proxyListener)
	}
}
