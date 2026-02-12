package tailnet

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"monks.co/pkg/meta"
	"tailscale.com/tsnet"
)

func hostname() string {
	return "monks-" + meta.AppName() + "-" + meta.MachineName()
}

var server = &tsnet.Server{
	Hostname:  hostname(),
	Dir:       filepath.Join(os.TempDir(), "tsnet-"+hostname()),
	Ephemeral: true,
	AuthKey:   tailscaleAuthKey,
}

func init() {
	if meta.IsFly() {
		server.Start()
	}
}

// ListenAndServe starts a tsnet server with hostname
// monks-{app}-{machine}, listens on :80, and serves HTTP.
func ListenAndServe(ctx context.Context, handler http.Handler) error {
	ln, err := server.Listen("tcp", ":80")
	if err != nil {
		return fmt.Errorf("tsnet listen: %w", err)
	}
	defer ln.Close()

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
