package main

import (
	"testing"

	"github.com/amonks/incrementum/internal/testsupport"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestTodoScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/todo",
		Setup: func(env *testscript.Env) error {
			return testsupport.SetupScriptEnv(t, env)
		},
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"envset": testsupport.CmdEnvSet,
			"todoid": testsupport.CmdTodoID,
		},
	})
}
