package main

import (
	"testing"

	"monks.co/ii/internal/testsupport"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestHabitScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/habit",
		Setup: func(env *testscript.Env) error {
			return testsupport.SetupScriptEnv(t, env)
		},
	})
}
