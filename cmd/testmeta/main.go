package main

import (
	"fmt"

	"monks.co/pkg/meta"
)

func main() {
	fmt.Println(meta.MachineName())
	fmt.Println(meta.AppName())
}
