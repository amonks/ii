package oci

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// FlyAuthOption returns a remote.Option that authenticates with the Fly.io
// registry using the provided API token. The Fly registry expects username "x"
// with the FLY_API_TOKEN as the password.
func FlyAuthOption(token string) remote.Option {
	return remote.WithAuth(&authn.Basic{
		Username: "x",
		Password: token,
	})
}
