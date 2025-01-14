package tailnet

import (
	"net/http"
	"os"

	"monks.co/pkg/meta"
	"tailscale.com/tsnet"
)

var server = &tsnet.Server{
	Hostname:  "monks.co-" + os.Getenv("FLY_REGION"),
	Dir:       "/data",
	Ephemeral: true,
	AuthKey:   os.Getenv("TS_AUTHKEY"),
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
