# Testing

## Overview
Incrementum tests are organized into tiers that exercise real behavior instead of mocks. Tests should model the way the CLI and jobs interact with jj repositories, filesystem state, and LLM agents.

## No Test Skipping

Tests must never silently skip. All tests must either pass or fail explicitly.

For integration tests that need LLM provider access, tests **must not** reach directly for provider-specific env vars like `ANTHROPIC_API_KEY`. Instead, they should load `./incrementum.toml` and assert that it includes provider configuration for any models used by the test. If a required model is not configured, the test should `t.Fatal`.

- **Never use `t.Skip()`** for missing dependencies like external binaries, or configuration files
- **Use `t.Fatal()` instead** when a required dependency is missing
- This ensures CI reports accurate pass/fail status rather than hiding untested code paths

Note: internal integration tests are not guaranteed to run from the repo root. They should locate the repo root (by walking up to find `incrementum.toml`) and load config via that path while keeping a temp `HOME` so global config does not bleed in. Tests must only exercise models that are configured in the repo's `incrementum.toml`; do not require unconfigured models.

Example - correct pattern (require model via repo config):
```go
func requireModel(t *testing.T, modelID string) {
    t.Helper()

    cfg, err := config.Load(".")
    if err != nil {
        t.Fatalf("failed to load repo config: %v", err)
    }
    if _, err := cfg.LLM.ProviderForModel(modelID); err != nil {
        t.Fatalf("test requires model %q to be configured in ./incrementum.toml: %v", modelID, err)
    }
}
```

Example - incorrect pattern (hard-coded env var):
```go
func getAnthropicKey(t *testing.T) string {
    key := os.Getenv("ANTHROPIC_API_KEY")
    if key == "" {
        t.Fatal("ANTHROPIC_API_KEY not set")
    }
    return key
}
```

## Test Tiers

### Unit Tests
- Pure Go logic stays in package-local tests next to the code.
- Focus on core domain logic, formatting helpers, and validation rules.
- Snapshot tests for text formatting live under `job/testdata/snapshots` and compare rendered prompts, commit messages, and log output against curated fixtures. Update these files manually when formatting rules change.

### Integration Tests
- Integration tests use real binaries and on-disk state (jj, workspaces).
- Use helpers in `internal/testsupport` to set up temp home/state directories.

### Realistic End-to-End Tests
- End-to-end tests create real jj repositories, seed commits, and set a plausible `main` bookmark.
- They run the `ii` CLI, create todos, and complete them via job flows.
- Scripts verify todo state, filesystem changes, and jj history rather than mocked results.

## Testscript and txtar Suites
- CLI e2e suites live under `cmd/ii/testdata` and are executed from `cmd/ii/*_test.go`.
- Each test is a txtar archive with a phase-oriented script plus supporting files.
- Testscript `exec` runs the real `ii` binary built via `BuildII`.

## Supporting Utilities
- `internal/testsupport.BuildII` builds the CLI once per test run.
- `internal/testsupport.SetupScriptEnv` prepares `HOME`, state, and workspace roots for testscript.
- `internal/testsupport.CmdEnvSet` and `internal/testsupport.CmdTodoID` help plumb script output into later steps.
- Job tests inject mock LLM callbacks to keep runs deterministic.

## Agent CLI Tests

Agent CLI tests in `cmd/ii/testdata/agent/*.txtar` exercise the `ii agent run` command against real LLM providers. These tests:

- Require `./incrementum.toml` to exist with a valid provider configuration supporting `claude-haiku-4-5`
- Test each built-in tool (bash, read, write, edit) via actual LLM API calls
- Verify that tool call arguments are correctly parsed and executed
- Use testscript to run the CLI binary and verify file system outcomes

Example test structure:
```
# Setup: create repo and copy the real config
mkdir repo
cd repo
exec jj git init
cp ../incrementum.toml incrementum.toml

# Run agent with specific tool request
exec $II agent run --model claude-haiku-4-5 'Use the write tool to create "./test.txt" with content "hello"'

# Verify the tool executed correctly
exists test.txt
exec cat test.txt
stdout 'hello'
```
