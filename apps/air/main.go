package main

import (
	"flag"
	"fmt"
)

var (
	port = flag.Int("port", 3000, "port")
	mode = flag.String("mode", "fetch", "mode: fetch, serve")
)

func main() {
	flag.Parse()

	db, err := NewDB()
	if err != nil {
		panic(err)
	}

	switch *mode {
	case "fetch":
		if err := fetch(db); err != nil {
			panic(err)
		}
		fmt.Println("done")
	case "serve":
		addr := fmt.Sprintf("0.0.0.0:%d", *port)
		if err := serve(db, addr); err != nil {
			panic(err)
		}
	}
	fmt.Println("done")
}
