# Bringing an External Repo into the Monorepo

Guide and lab notes from importing beetman, gods-teeth, and creamery.

## Procedure

### 1. Clean working tree

`git subtree add` requires a clean git working tree. In a jj
colocated repo, run `jj new` to achieve this.

### 2. git subtree add

```sh
git subtree add --prefix=<target-dir> <source-repo-path> main
```

This creates a merge commit bringing in the full history. Commit
hashes will differ from the source repo.

### 3. Update go.mod and imports

- Change the module path to `monks.co/<dir>` (or a vanity path if
  publishing with an explicit mirror).
- Bump Go version to match the monorepo (currently 1.26.0).
- Update all internal imports: `find <dir> -name '*.go' -exec sed -i '' 's|old/path|new/path|g' {} +`

Taskmaker will create go.mod for modules that don't have one, but
it **won't overwrite existing go.mod files** — you must update
the module path manually.

### 4. If publishing: update publish.toml and add LICENSE

For packages published to their own GitHub mirror:

```toml
[[package]]
dir = "cmd/example"
module_path = "monks.co/example"
mirror = "github.com/amonks/example"
```

Every public package needs a LICENSE file (enforced by validation).

### 5. If the app runs on a machine: update config/<machine>.toml

Add the app name to the `apps` list for the appropriate machine.
This controls which apps appear in the machine's dev task.

### 6. App tasks.toml

The monorepo expects at least `build` and `dev` tasks for apps that
appear in a machine config. Check existing apps (e.g.,
`apps/ping/tasks.toml`) for the standard pattern:

```toml
[[task]]
  id = "build"
  type = "short"
  watch = ["**/*.go", "*.go"]
  cmd = "go build -o ../../bin/<app> ."

[[task]]
  id = "dev"
  type = "long"
  dependencies = ["build"]
  cmd = "../../bin/<app>"
```

If the imported repo has a `tasks.toml`, adapt it to include at
least `build` (with `go build -o ../../bin/`).

### 7. Run taskmaker

```sh
go run ./cmd/taskmaker
```

Regenerates `go.work` (auto-discovers `{apps,pkg,cmd}/*`) and
`tasks.toml` (machine dev tasks, build aggregation).

### 8. Run the full CI check suite

```sh
go tool run test
```

This runs `go-test`, `gofix`, and `staticcheck` across the entire
workspace. Common issues with imported code:

- **gofix**: older Go idioms get auto-fixed (`strings.Split` →
  `strings.SplitSeq`, manual min → `min` builtin, `for i := 0; i < n`
  → `for i := range n`, manual map copy → `maps.Copy`, etc.)
  Apply fixes with `go fix ./path/...`.
- **staticcheck**: unused variables, etc.
- **cgo dependencies**: if the imported code uses cgo with
  pkg-config (like nlopt), ensure the system library is available
  both locally (via `.envrc`) and in CI (via `apt-get install` in
  the GitHub Actions workflow).

### 9. If publishing: verify

```sh
go run ./cmd/publish --dry-run
```

## Gotchas

### jj colocation

`git subtree add` requires a clean git working tree. Don't use
`git stash` in a jj colocated repo — it doesn't work well. Use
`jj new` instead. Avoid other direct git operations (checkout,
reset, etc.) in jj repos.

### Module paths that don't match directory layout

If a module's import path doesn't follow the `monks.co/<dir>`
convention (e.g., `monks.co/beetman` at `cmd/beetman`), the
publish validation dep graph still works correctly — it reads
go.mod files and resolves imports via longest prefix match.

### direnv / .envrc

External repos may have `.envrc` files with environment variables
needed for cgo builds. These don't automatically apply in the
monorepo. Add needed variables to the monorepo's root `.envrc`
and to `.github/workflows/ci.yml`.

### Stale tests

Imported repos may have tests that were already failing (stale
counts, outdated expectations). These weren't caught because the
original repo may not have had CI. Fix or update them.

### taskmaker won't overwrite existing go.mod

If the imported repo already has a `go.mod`, taskmaker preserves
it. You must manually update the module path. If there's no
`go.mod`, taskmaker creates one with the default path
(`monks.co/<dir>`).

## History

### beetman (cmd/beetman)

Published to `github.com/amonks/beetman` with vanity path
`monks.co/beetman`. Required fixing the publish dep graph to use
module-path-aware import resolution instead of a 2-component
directory heuristic.

### gods-teeth (apps/gods-teeth)

Simple import from `~/git/amonks/delta-green`. No publishing, no
machine config. Clean import with no issues.

### creamery (apps/creamery)

Runs on brigid. Required adding nlopt system dependency to
`.envrc` and CI. Needed a `build` task added to its `tasks.toml`.
gofix had many changes (maps.Copy, range-over-int, etc.). Had a
pre-existing test failure (stale batch count).
