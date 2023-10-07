package main

import (
	"context"
	"flag"
)

var port = flag.Int("port", 3000, "port")

func main() {
	flag.Parse()
	if err := New().Run(context.Background(), *port); err != nil {
		panic(err)
	}
}

