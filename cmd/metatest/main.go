package main

import (
	"fmt"

	"monks.co/pkg/meta"
)

func main() {
	fmt.Println(meta.AppName())
	fmt.Println(meta.MachineName())
}
