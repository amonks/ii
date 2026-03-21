// Package mobile provides a gomobile-compatible binding for the breadcrumbs
// node. It starts a node and an HTTP server on localhost, returning the port.
// All interaction with the node goes through HTTP.
package mobile

import (
	"context"
	"net"
	"net/http"
	"sync"

	"monks.co/apps/breadcrumbs/node"
)

var (
	mu       sync.Mutex
	cancel   context.CancelFunc
	nodeInst *node.Node
	srv      *http.Server
	port     int
)

// Start starts a breadcrumbs node and an HTTP server on localhost.
// configJSON is JSON matching the node.Config format.
// Returns the port number the server is listening on.
func Start(configJSON []byte) (int, error) {
	mu.Lock()
	defer mu.Unlock()

	if nodeInst != nil {
		return port, nil
	}

	config, err := node.ParseConfig(configJSON)
	if err != nil {
		return 0, err
	}

	ctx, c := context.WithCancel(context.Background())
	cancel = c

	n, err := node.NewNode(ctx, config)
	if err != nil {
		cancel()
		return 0, err
	}
	nodeInst = n

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		nodeInst.Close()
		nodeInst = nil
		cancel()
		return 0, err
	}
	port = ln.Addr().(*net.TCPAddr).Port

	srv = &http.Server{Handler: n.Handler()}
	go srv.Serve(ln)

	return port, nil
}

// Stop shuts down the HTTP server and closes the node.
func Stop() {
	mu.Lock()
	defer mu.Unlock()

	if nodeInst == nil {
		return
	}

	srv.Close()
	nodeInst.Close()
	cancel()

	nodeInst = nil
	srv = nil
}
