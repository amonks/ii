package requireenv

import (
	"fmt"
	"os"
	"sync"
)

// Require reads an environment variable and panics if it is not set.
func Require(env string) string {
	v := os.Getenv(env)
	if v == "" {
		panic(fmt.Errorf("env '%s' not set", env))
	}
	return v
}

// Lazy returns a function that calls Require on first invocation,
// caching the result. Safe to assign at package level without
// panicking at init time.
func Lazy(env string) func() string {
	var once sync.Once
	var val string
	return func() string {
		once.Do(func() { val = Require(env) })
		return val
	}
}
