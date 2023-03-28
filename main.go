package main

import (
	"fmt"
	"log"
	"net/http"

	"co.monks.monks.co/auth"
	"co.monks.monks.co/golink"
	"co.monks.monks.co/ping"
	"co.monks.monks.co/places"
	"co.monks.monks.co/promises"
	"co.monks.monks.co/weblog"
	// "github.com/caddyserver/certmagic"
	// "github.com/libdns/route53"
)

func main() {
	mux := http.NewServeMux()

	mux.Handle("/promises/", auth.InternalHandler(promises.Server()))
	mux.Handle("/ping/", auth.InternalHandler(ping.Server()))
	mux.Handle("/go/", auth.InternalHandler(golink.Server()))

	mux.Handle("/places/", places.Server())

	mux.Handle("/", weblog.Server())

	// go func() {
	// 	err := serveTLS([]string{"brigid.ss.cx"}, mux)
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// }()

	fmt.Println("listening for HTTP requests on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

// func serveTLS(domains []string, mux http.Handler) error {
// 	certmagic.DefaultACME.Agreed = true
// 	certmagic.DefaultACME.Email = "a@monks.co"
// 	certmagic.DefaultACME.CA = certmagic.LetsEncryptProductionCA

// 	certmagic.DefaultACME.DNS01Solver = &certmagic.DNS01Solver{
// 		DNSProvider: &route53.Provider{},
// 	}

// 	return certmagic.HTTPS(domains, mux)
// }
