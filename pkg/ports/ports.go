package ports

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

var Apps = map[string]int{}

func init() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	bs, err := os.ReadFile("config/ports")
	if err != nil {
		return err
	}
	str := string(bs)
	str = strings.TrimSpace(str)
	lines := strings.Split(str, "\n")
	for _, line := range lines {
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			return fmt.Errorf("weird line in config/ports: '%s'", line)
		}
		port, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return err
		}
		Apps[parts[1]] = int(port)
	}
	return nil
}
