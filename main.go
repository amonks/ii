package main

import (
	"fmt"
	"log"
	"net/http"

	"co.monks.monks.co/ping"
	"co.monks.monks.co/promises"
)

func main() {
	mux := http.NewServeMux()

	mux.Handle("/promises/", promises.Server())
	mux.Handle("/ping/", ping.Server())

	mux.Handle("/", http.FileServer(http.Dir("./static")))

	fmt.Println("on 3000")
	if err := http.ListenAndServe(":3000", mux); err != nil {
		log.Fatal(err)
	}
}
