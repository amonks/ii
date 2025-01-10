package main

import (
	"fmt"

	"monks.co/pkg/errlogger"
)

func main() {
	errlogger.Report(200, "testing errlog")
	fmt.Println("ok")
}
