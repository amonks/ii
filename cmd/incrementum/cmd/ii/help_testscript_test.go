package main

import (
	"testing"

	"github.com/amonks/incrementum/internal/testsupport"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestHelpScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/help",
		Setup: func(env *testscript.Env) error {
			return testsupport.SetupScriptEnv(t, env)
		},
	})
}
