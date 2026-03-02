# Bringing an External Repo into the Monorepo

Lab notes for importing `~/git/amonks/beetman` → `./cmd/beetman`.

## System inventory

### How publishing works

Two modes in `config/publish.toml`:

1. **Default mirror** (git-filter-repo): packages without an explicit
   `mirror` go to the shared `github.com/amonks/go` repo. Currently
   only `pkg/set` uses this.

2. **Explicit mirror** (git subtree split): packages with a `mirror`
   field get their own GitHub repo. `git subtree split --prefix=<dir>`
   extracts the subtree, then pushes to the mirror. Tags are stripped
   of the directory prefix. This is what we want for beetman.

Config example from the spec:
```toml
[[package]]
dir = "cmd/run"
module_path = "github.com/amonks/run"
mirror = "github.com/amonks/run"
```

### How go.work is managed

`cmd/taskmaker` auto-discovers modules by scanning `{apps,pkg,cmd}/*`
for directories containing `.go` files. It generates `go.work` and
creates `go.mod` for any module that doesn't have one. If
`publish.toml` specifies a `module_path` override, taskmaker uses
that instead of the default `monks.co/<dir>`.

### Validation (runs in CI via `go test`)

- Every public package's transitive `monks.co/*` deps must also be public
- Every public package must have a LICENSE file
- Every public package's `go.mod` must have the expected module path

### What beetman looks like

- Module: `github.com/amonks/beetman`, Go 1.24.0
- Deps: `github.com/mattn/go-sqlite3`
- Structure: root package is the library (`package beetman`),
  `beetman/main.go` is the CLI, `internal/` has sub-packages
- 31 commits, has tests, no CI
- No internal `monks.co/*` dependencies (standalone)

## Plan

### Step 1: git subtree add

```sh
git subtree add --prefix=cmd/beetman ~/git/amonks/beetman main
```

This creates a merge commit that brings in beetman's full history
under `cmd/beetman/`. The history is preserved (as separate commits
visible in `git log`).

Note: hashes in the monorepo will differ from the original repo. The
user confirmed this is fine.

### Step 2: Update go.mod

Change module path and Go version:
- `github.com/amonks/beetman` → `monks.co/beetman` (vanity path)
- Go version: `1.24.0` → `1.26.0`
- Update all internal imports from `github.com/amonks/beetman` to
  `monks.co/beetman`

### Step 3: Add to publish.toml

```toml
[[package]]
dir = "cmd/beetman"
module_path = "monks.co/beetman"
mirror = "github.com/amonks/beetman"
```

The vanity module path (`monks.co/beetman`) will be served by the
proxy via go-import meta tags, pointing at the GitHub mirror.

### Step 4: Add LICENSE file

MIT license, Andrew Monks <a@monks.co> 2026.

### Step 5: Run taskmaker

```sh
go run ./cmd/taskmaker
```

This regenerates `go.work` to include `./cmd/beetman`.

### Step 6: Verify

Run the full CI check suite and publish dry-run:

```sh
go tool run test
go run ./cmd/publish --dry-run
```

The `test` task runs `go-test`, `gofix`, and `staticcheck`. The
imported code may need fixes for newer Go idioms (e.g.,
`strings.SplitSeq`, `min` builtin).

## Log

### Step 1: git subtree add

Used `jj new` to get a clean git working tree (jj colocated repo),
then ran:

```
git subtree add --prefix=cmd/beetman ~/git/amonks/beetman main
```

Brought in 31 commits under `cmd/beetman/`.

### Step 2–4: Module path, publish config, LICENSE

- Changed `go.mod` module path to `monks.co/beetman`, Go 1.26.0
- `sed` across all .go files to swap `github.com/amonks/beetman` →
  `monks.co/beetman`
- Added `cmd/beetman` entry to `config/publish.toml` with explicit
  mirror `github.com/amonks/beetman`
- Created `cmd/beetman/LICENSE` (MIT, Andrew Monks, 2026)

### Step 5: taskmaker

`go run ./cmd/taskmaker` — regenerated `go.work`, updated
`go-sqlite3` dep from v1.14.24 to v1.14.34.

### Step 6: Verify

Beetman tests: all 8 packages pass.

Publish validation: **initially failed**. `BuildDepGraph` uses a
2-component heuristic to map import paths to module directories
(`monks.co/X/Y/...` → `X/Y`). For `monks.co/beetman/internal/foo`,
this produces `beetman/internal` — a nonexistent module.

**First attempt**: added a filtering step that drops deps pointing to
directories not present in the graph. This worked but was lossy — if
another module imported `monks.co/beetman`, that dep would be
silently dropped.

**Proper fix**: replaced the 2-component heuristic entirely.
`BuildDepGraph` now reads `go.mod` files from all module directories
to build a `module_path → directory` map, then resolves imports via
longest prefix match (same algorithm `go` itself uses). This
correctly maps `monks.co/beetman/internal/foo` → `cmd/beetman`
(self-reference, excluded by `TransitiveDeps`), and would also
correctly resolve cross-module deps like `monks.co/beetman` →
`cmd/beetman` from other modules.

New functions: `readModulePath`, `buildModuleMap`, `resolveImportDir`.
New tests: `TestResolveImportDir`, `TestBuildDepGraphNonStandardModulePath`.

Dry-run output confirms beetman publishes via subtree split:
```
publishing cmd/beetman -> github.com/amonks/beetman (subtree split)
```

### Remaining concerns

- The proxy needs to serve vanity import meta tags for
  `monks.co/beetman`. The proxy loads `publish.toml` and serves
  go-import tags for explicit-mirror packages. This should work
  automatically once deployed, but should be verified.
