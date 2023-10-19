package main

import (
	"flag"
	"fmt"
)

var (
	mode string
	port int
)

func main() {
	flag.IntVar(&port, "port", 3000, "port")
	flag.StringVar(&mode, "mode", "fetch", "mode: fetch, serve")
	flag.Parse()

	db, err := NewDB()
	if err != nil {
		panic(err)
	}

	switch mode {
	case "fetch":
		if err := fetch(db); err != nil {
			panic(err)
		}
		fmt.Println("done")
	case "serve":
		addr := fmt.Sprintf("0.0.0.0:%d", port)
		if err := serve(db, addr); err != nil {
			panic(err)
		}
	}
	fmt.Println("done")
}
