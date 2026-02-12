package tailnet

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"monks.co/pkg/meta"
	"tailscale.com/tsnet"
)

func hostname() string {
	return "monks-" + meta.AppName() + "-" + meta.MachineName()
}

// ListenAndServe starts a tsnet server with hostname
// monks-{app}-{machine}, listens on :80, and serves HTTP.
func ListenAndServe(ctx context.Context, handler http.Handler) error {
	h := hostname()
	srv := &tsnet.Server{
		Hostname:  h,
		Dir:       filepath.Join(os.TempDir(), "tsnet-"+h),
		Ephemeral: true,
		AuthKey:   tailscaleAuthKey,
	}
	if err := srv.Start(); err != nil {
		return fmt.Errorf("tsnet start: %w", err)
	}
	defer srv.Close()

	ln, err := srv.Listen("tcp", ":80")
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

var (
	clientOnce sync.Once
	clientNode *tsnet.Server
)

// Client returns an HTTP client that routes through tailscale.
// On non-Fly machines, returns http.DefaultClient.
// Lazily starts a client-only tsnet node on first call.
func Client() *http.Client {
	if !meta.IsFly() {
		return http.DefaultClient
	}
	clientOnce.Do(func() {
		h := hostname() + "-client"
		clientNode = &tsnet.Server{
			Hostname:  h,
			Dir:       filepath.Join(os.TempDir(), "tsnet-"+h),
			Ephemeral: true,
			AuthKey:   tailscaleAuthKey,
		}
		if err := clientNode.Start(); err != nil {
			panic(fmt.Errorf("failed to start tailnet client: %w", err))
		}
	})
	return clientNode.HTTPClient()
}
