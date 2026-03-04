package main

import (
	"encoding/json"
	"os"
)

func encodeJSONToStdout(value any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
