package meta

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	appName     = newCache(getAppName)
	machineName = newCache(getMachineName)
)

func AppName() string     { return appName.get() }
func MachineName() string { return machineName.get() }
func IsFly() bool         { return strings.HasPrefix(MachineName(), "fly-") }

func getAppName() string {
	if name := os.Getenv("MONKS_APP_NAME"); name != "" {
		return name
	}
	if path, err := os.Executable(); err != nil {
		return "unknown"
	} else {
		return filepath.Base(path)
	}
}

func getMachineName() string {
	if region := os.Getenv("FLY_REGION"); region != "" {
		return fmt.Sprintf("fly-%s", region)
	}

	bs, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), "locals.fish"))
	if err != nil {
		return "unknown"
	}

	fields := strings.Fields(string(bs))
	for i := range fields {
		if fields[i] == "machine_name" {
			return fields[i+1]
		}
	}

	return "unknown"
}

type cache struct {
	val string
	mu  sync.Mutex
	op  func() string
}

func newCache(op func() string) *cache {
	return &cache{op: op}
}

func (c *cache) get() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.val != "" {
		return c.val
	}
	c.val = c.op()
	return c.val
}
