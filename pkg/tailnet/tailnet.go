package tailnet

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"monks.co/pkg/meta"
	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/tailcfg"
	"tailscale.com/tsnet"
)

func hostname() string {
	return "monks-" + meta.AppName() + "-" + meta.MachineName()
}

func tsnetDir() string {
	if data := os.Getenv("MONKS_DATA"); data != "" {
		return filepath.Join(data, "tsnet-"+hostname())
	}
	return filepath.Join(os.TempDir(), "tsnet-"+hostname())
}

var server = &tsnet.Server{
	Hostname: hostname(),
	Dir:      tsnetDir(),
	AuthKey:  tailscaleAuthKey,
}

// WaitReady blocks until the tailscale node is fully authenticated
// and connected to the tailnet. Apps should call this early in startup,
// before making any outbound tailnet connections.
func WaitReady(ctx context.Context) error {
	_, err := server.Up(ctx)
	return err
}

// ListenAndServe starts a tsnet server with hostname
// monks-{app}-{machine}, listens on :80, and serves HTTP.
func ListenAndServe(ctx context.Context, handler http.Handler) error {
	ln, err := server.Listen("tcp", ":80")
	if err != nil {
		return fmt.Errorf("tsnet listen: %w", err)
	}
	defer ln.Close()

	slog.Info("started")

	httpSrv := &http.Server{Handler: handler}
	errs := make(chan error, 1)
	go func() {
		errs <- httpSrv.Serve(ln)
	}()
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		return httpSrv.Shutdown(context.Background())
	}
}

// Client returns an HTTP client that routes through tailscale.
// On non-Fly machines, returns http.DefaultClient.
// Lazily starts a client-only tsnet node on first call.
func Client() *http.Client {
	return server.HTTPClient()
}

// Listen creates a listener on the tsnet server.
func Listen(network, addr string) (net.Listener, error) {
	return server.Listen(network, addr)
}

// WhoIs identifies a peer by remote address.
func WhoIs(ctx context.Context, remoteAddr string) (*apitype.WhoIsResponse, error) {
	lc, err := server.LocalClient()
	if err != nil {
		return nil, fmt.Errorf("tailnet local client: %w", err)
	}
	return lc.WhoIs(ctx, remoteAddr)
}

// AnonCaps returns capabilities from filter rules with SrcIPs: ["*"]
// (i.e. autogroup:danger-all grants). Called once at startup, cached by caller.
func AnonCaps(ctx context.Context) (tailcfg.PeerCapMap, error) {
	// Ensure the server is fully connected and has its netmap before
	// querying filter rules. Start() is non-blocking and Listen()
	// doesn't guarantee the netmap is ready.
	if _, err := server.Up(ctx); err != nil {
		return nil, fmt.Errorf("tailnet up: %w", err)
	}
	lc, err := server.LocalClient()
	if err != nil {
		return nil, fmt.Errorf("tailnet local client: %w", err)
	}
	rules, err := lc.DebugPacketFilterRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("tailnet filter rules: %w", err)
	}
	caps := make(tailcfg.PeerCapMap)
	for _, rule := range rules {
		if !srcIPsContainsStar(rule.SrcIPs) {
			continue
		}
		for _, grant := range rule.CapGrant {
			for cap, vals := range grant.CapMap {
				caps[cap] = append(caps[cap], vals...)
			}
		}
	}
	return caps, nil
}

func srcIPsContainsStar(srcIPs []string) bool {
	return slices.Contains(srcIPs, "*")
}
