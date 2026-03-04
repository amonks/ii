package main

import (
	"testing"

	"monks.co/incrementum/internal/testsupport"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestInitScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/init",
		Setup: func(env *testscript.Env) error {
			return testsupport.SetupScriptEnv(t, env)
		},
	})
}
