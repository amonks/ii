package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"

	"co.monks.monks.co/golink"
	"co.monks.monks.co/ping"
	"co.monks.monks.co/places"
	"co.monks.monks.co/promises"
	"tailscale.com/client/tailscale"
)

func main() {
	mux := http.NewServeMux()

	mux.Handle("/promises/", promises.Server())
	mux.Handle("/ping/", ping.Server())
	mux.Handle("/places/", places.Server())
	mux.Handle("/go/", golink.Server())

	mux.Handle("/", http.FileServer(http.Dir("./static")))

	s := &http.Server{
		TLSConfig: &tls.Config{
			GetCertificate: tailscale.GetCertificate,
		},
		Handler: mux,
	}

	fmt.Println("listening for TLS requests")
	if err := s.ListenAndServeTLS("", ""); err != nil {
		log.Fatal(err)
	}
}
