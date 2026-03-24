# Internal Test Support

## Overview
The testsupport package provides helpers for integration tests and testscript scenarios.

## Build Helpers
- `BuildII` builds the `ii` binary once per test run and returns its path.
- Build errors fail the calling test.

## Testscript Helpers
- `EnsureHomeDirs` creates the default state/workspace directories under a given home path.
- `SetupTestHome` creates a temp home, ensures state/workspace dirs, and sets `HOME`.
- `SetupScriptEnv` provisions test home/state/workspace directories and sets `II` and `HOME`.
- `CmdEnvSet` captures a file's trimmed contents into an env var.
- `CmdTodoID` looks up a todo by title in a JSON list and exports its ID.
- `CmdAgentSession` inserts or updates an agent session in SQLite for testscript fixtures.
