# CLI Conventions

The CLI architecture and testing expectations are defined in the Architecture section of `specs/README.md`.
Use this document for additional CLI-specific guidance when it is not already covered there.

## Version Flag

- `ii -version` prints the build identifiers instead of a semantic version.
- Output format:

```text
change_id <jj-change-id>
commit_id <jj-commit-id>
```

- The identifiers are embedded at build time via `-ldflags`.

### Build Metadata Injection

The version variables (`buildChangeID`, `buildCommitID`) are defined in `cmd/ii/version.go` within the `main` package. Because they are in package `main`, the linker requires the `main.` prefix regardless of import path:

```bash
ldflags="-X main.buildChangeID=${change_id} -X main.buildCommitID=${commit_id}"
go run -ldflags "$ldflags" ./cmd/ii "$@"
```

Two scripts embed build metadata:

- `./bin/ii` — runs the CLI during development with metadata from the current jujutsu revision
- `./bin/install` — installs the CLI to `$GOPATH/bin` with metadata from the current jujutsu revision

Both scripts use `jj log -r @ -T change_id` and `jj log -r @ -T commit_id` to fetch the current revision identifiers.
