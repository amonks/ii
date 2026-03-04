package main

import (
	"fmt"

	"github.com/amonks/incrementum/job"
)

func appendAndPrintEvent(formatter *job.EventFormatter, event job.Event) error {
	chunk, err := formatter.Append(event)
	if err != nil {
		return err
	}
	if chunk != "" {
		fmt.Print(chunk)
	}
	return nil
}
