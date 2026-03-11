// Command agent is a standalone CLI agent wrapping pkg/agent.
//
// Usage:
//
//	agent run [--model MODEL] [--workdir DIR] [-p PROMPT | --prompt-file FILE]
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
