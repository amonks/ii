package requireenv

import (
	"fmt"
	"os"
)

func Require(env string) string {
	v := os.Getenv(env)
	if v == "" {
		panic(fmt.Errorf("env '%s' not set", env))
	}
	return v
}
