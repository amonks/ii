package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"co.monks.monks.co/dbserver/promises"
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
)

func main() {
	http.Handle("/promises", http.StripPrefix("/promises/", promises.Server()))
	http.Handle("/", http.FileServer(http.Dir("./static")))

	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatal(err)
	}
}
