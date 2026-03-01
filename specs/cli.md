# CLI Conventions

The CLI architecture and testing expectations are defined in the Architecture section of `specs/README.md`.
Use this document for additional CLI-specific guidance when it is not already covered there.

## Init Command

- `ii init` writes `incrementum.toml` in the current jj repo root.
- If `incrementum.toml` or `.incrementum/config.toml` already exists, the command fails with an error.
- The generated file sets `job.test-commands` when a test command is detected.
- If no test command is detected, the file contains a commented `job.test-commands` hint instead.

### Test Command Heuristic

`ii init` checks for test commands in order:

1. `tasks.toml` and `go.mod` present -> `go tool run test`
2. `tasks.toml` present -> `run test`
3. `./bin/test` (or `.sh`, `.bash`, `.fish`, `.zsh`) present -> `./bin/test`
4. `go.mod` present -> `go test ./...`
5. `package.json` present -> `npm test`
6. none -> leave `job.test-commands` unset

## Merge Command

- `ii merge <change-id> [--onto <bookmark>]` rebases a change onto the target bookmark and resolves conflicts with an agent.
- `--onto` defaults to `[merge].target` from config; falls back to `main` when unset.
- The command runs in the current jj workspace and updates the target bookmark on success.
- On success, the command prints `Merged <change-id> onto <target>`.

## Version Flag

- `ii -version` prints the build identifiers instead of a semantic version.
- Output format:

```text
change_id <6-char-jj-change-id>
commit_id <6-char-jj-commit-id>
```

- The identifiers are embedded at build time via `-ldflags`.
- Only the first 6 characters of each identifier are used for brevity.

### Build Metadata Injection

The version variables (`buildChangeID`, `buildCommitID`) are defined in `cmd/ii/version.go` within the `main` package. Because they are in package `main`, the linker requires the `main.` prefix regardless of import path:

```bash
ldflags="-X main.buildChangeID=${change_id:0:6} -X main.buildCommitID=${commit_id:0:6}"
go run -ldflags "$ldflags" ./cmd/ii "$@"
```

Two scripts embed build metadata:

- `./bin/ii` -> runs the CLI during development with metadata from the current jujutsu revision
- `./bin/install` -> installs the CLI to `$GOPATH/bin` with metadata from the current jujutsu revision

Both scripts use `jj log -r @ -T change_id` and `jj log -r @ -T commit_id` to fetch the current revision identifiers, then truncate each to the first 6 characters using bash substring expansion (`${var:0:6}`).
