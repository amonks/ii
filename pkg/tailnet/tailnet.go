package tailnet

import (
	"net/http"
	"os"
	"path/filepath"

	"monks.co/credentials"
	"monks.co/pkg/meta"
	"tailscale.com/tsnet"
)

var server = &tsnet.Server{
	Hostname:  "monks.co-" + os.Getenv("FLY_REGION") + "-" + meta.AppName(),
	Dir:       filepath.Join("/data", meta.AppName()),
	Ephemeral: true,
	AuthKey:   credentials.TailscaleAuthKey,
}

func Server() *tsnet.Server {
	if !meta.IsFly() {
		panic("don't use tailnet outside of fly")
	}
	return server
}

func Client() *http.Client {
	if !meta.IsFly() {
		return http.DefaultClient
	}
	return server.HTTPClient()
}

func init() {
	if meta.IsFly() {
		server.Start()
	}
}
