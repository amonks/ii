package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	proxyproto "github.com/pires/go-proxyproto"

	"monks.co/pkg/config"
	"monks.co/pkg/ports"
	"monks.co/pkg/tls"
	"monks.co/pkg/traffic"
	"tailscale.com/tsnet"
)

var (
	machine = flag.String("machine", "", "machine name; must have a corresponding toml file in config/.")
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	flag.Parse()

	config, err := config.Load(*machine)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancelCause(context.Background())
	for _, service := range config.Services {
		routes := map[string]int{}
		for _, app := range service.Apps {
			routes[app] = ports.Apps[app]
		}
		fmt.Println("routes", routes)

		app := &app{routes, service}
		var server *server
		switch service.Type {
		case "redirect-to-https":
			server = app.redirectServer(ctx, service.Addr)
		case "https":
			server, err = app.httpsServer(ctx, service.Addr, config.ACME)
			if err != nil {
				return err
			}
		case "tsnet":
			server, err = app.tsnetServer(ctx, service.Addr)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported service type '%s'", service.Type)
		}

		wg.Add(1)
		go func() {
			if err := server.serve(); err != nil {
				fmt.Println("error:", err)
			}
			server.stop()
			cancel(err)
			wg.Done()
		}()
	}

	wg.Wait()
	return nil
}

type app struct {
	routes  map[string]int
	service config.Service
}

func (a *app) redirectServer(ctx context.Context, addr string) *server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		u := req.URL
		u.Host = net.JoinHostPort(req.Host, "443")
		u.Scheme = "https"
		http.Redirect(w, req, u.String(), 301)
	})
	srv := &http.Server{
		ConnContext: connContext,
		Addr:        addr,
		Handler:     handler,
	}
	listen := func() (net.Listener, error) {
		return net.Listen("tcp", addr)
	}
	return &server{srv, listen, false, func() {}}
}

func connContext(ctx context.Context, conn net.Conn) context.Context {
	if conn, ok := conn.(*proxyproto.Conn); ok {
		if conn.LocalAddr() == nil {
			fmt.Printf("couldn't retrieve local address")
		}
		if conn.RemoteAddr() == nil {
			fmt.Printf("couldn't retrieve remote address")
		}

		return context.WithValue(ctx, traffic.RemoteAddrKey, conn.RemoteAddr().String())
	}
	return context.WithValue(ctx, traffic.RemoteAddrKey, conn.RemoteAddr().String())
}

func (a *app) httpsServer(ctx context.Context, addr string, acme tls.ACME) (*server, error) {
	p, err := traffic.New(addr, &proxy{a.routes})
	if err != nil {
		return nil, err
	}
	tlsConfig, stop, err := tls.NewTLSConfig(ctx, acme)
	if err != nil {
		return nil, err
	}
	srv := &http.Server{
		ConnContext: connContext,
		Addr:        addr,
		Handler:     BMRRedirectorHandler(p),
		TLSConfig:   tlsConfig,
	}

	listen := func() (net.Listener, error) {
		return net.Listen("tcp", addr)
	}

	return &server{srv, listen, true, stop}, nil
}

func (a *app) tsnetServer(ctx context.Context, addr string) (*server, error) {
	srv := &tsnet.Server{
		Hostname:  "monksgo",
		Dir:       a.service.StoragePath,
		Ephemeral: true,
		AuthKey:   os.Getenv("TS_AUTHKEY"),
	}
	p, err := traffic.New(addr, &proxy{a.routes})
	if err != nil {
		return nil, fmt.Errorf("error starting traffic logger: %w", err)
	}
	httpSrv := &http.Server{
		ConnContext: connContext,
		Addr:        addr,
		Handler:     BMRRedirectorHandler(p),
	}

	listen := func() (net.Listener, error) {
		ln, err := srv.Listen("tcp", ":80")
		if err != nil {
			return nil, fmt.Errorf("error listening on tsnet: %w", err)
		}
		return ln, nil
	}

	stop := func() {}

	return &server{httpSrv, listen, false, stop}, nil
}

type server struct {
	*http.Server
	listen func() (net.Listener, error)
	tls    bool
	stop   func()
}

func (s *server) serve() error {
	fmt.Println("will listen on", s.Addr)
	ln, err := s.listen()
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
		err = s.ServeTLS(proxyListener, "", "")
	} else {
		err = s.Serve(proxyListener)
	}
	if err != nil {
		return fmt.Errorf("error in serve: %w", err)
	}
	return nil
}
