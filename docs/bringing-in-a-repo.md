# Bringing an External Repo into the Monorepo

Guide and lab notes from importing beetman, gods-teeth, creamery, and incrementum.

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
- Remove `tool` directives from the imported go.mod — they can
  conflict with the monorepo's own tool directives (e.g., two
  different paths resolving to the same binary name). Run
  `go mod tidy` after removing them.
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
  → `for i := range n`, manual map copy → `maps.Copy`,
  `pointerHelper(x)` → `new(x)`, etc.)
  Apply fixes with `go fix ./path/...`.
- **staticcheck**: unused variables, etc. Note that gofix may
  inline helper functions (e.g., `func boolPointer(b bool) *bool`)
  at call sites but leave the now-unused function definitions
  behind — staticcheck will then flag them as unused. Delete the
  dead functions after applying gofix.
- **cgo dependencies**: if the imported code uses cgo with
  pkg-config (like nlopt), ensure the system library is available
  both locally (via `.envrc`) and in CI (via `apt-get install` in
  the GitHub Actions workflow).

### 9. If publishing: verify

```sh
go run ./cmd/publish --dry-run
```

## Gotchas

### Source repo must be git-colocated

`git subtree add` needs a real `.git` directory in the source
repo. If the source repo is jj-only (non-colocated), enable
colocation first:

```sh
cd /path/to/source-repo
jj git colocation enable
```

This creates a proper `.git` directory that `git subtree add` can
read from. The jj internal git store (`.jj/repo/store/git`) is
**not** guaranteed to be consistent and must not be used directly.

### jj colocation (monorepo side)

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

### Cross-module dependencies within the workspace

If other monorepo modules import the newly imported module, you
must update their imports and go.mod files. Before the module
is published, the vanity path won't resolve via the network, so
you must **remove** the `require` directive for it from
consuming modules' go.mod files. The Go workspace resolves it
via the `use` directive; a versioned `require` line triggers a
network fetch that will fail pre-publish. After the first
publish, `go mod tidy` should work normally. Similarly,
Dockerfiles that used `go install <old-path>@latest` should
be changed to `go install ./<dir>` to build from local source
(avoids network dependency and version skew).

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

### incrementum (cmd/incrementum)

Published to `github.com/amonks/incrementum` with vanity path
`monks.co/incrementum`. Source repo was jj-only (non-colocated) —
required `jj git colocation enable` before `git subtree add`
could read from it. Had a `tool` directive for
`github.com/amonks/run/cmd/run` that conflicted with the
monorepo's `github.com/amonks/run` (both resolve to the `run`
binary), causing "tool is ambiguous" errors — removed the tool
block and ran `go mod tidy`. gofix inlined pointer helper
functions, leaving dead code that staticcheck flagged.

### run (cmd/run)

Published to `github.com/amonks/run` with vanity path
`monks.co/run`. Had 34 tags (v1.0.0-beta.1 through
v1.0.0-beta.35) that needed remapping to monorepo format
(`cmd/run/vX.Y.Z`). `git subtree add` does not carry tags and
rewrites commit SHAs, so tags were remapped by matching commit
subject lines between the source repo and the imported history.
Had a `tool` directive for `go.abhg.dev/doc2go` — removed it
and ran `go mod tidy`.

This was the first import where other monorepo modules
(`cmd/taskmaker`, `apps/ci`) depend on the imported module.
After changing imports from `github.com/amonks/run` to
`monks.co/run`, the `require` directives in their go.mod files
had to be removed — the vanity path wasn't published yet, so
Go couldn't verify the version from the network. The workspace
`use` directive handles resolution locally. After first publish,
`go mod tidy` should add the `require` lines back normally.

The Dockerfiles also needed updating: `go install
github.com/amonks/run@latest` was changed to
`go install ./cmd/run` (builds from the local source already
copied into the Docker context, avoiding network dependency).

The vanity handler (`apps/proxy/vanity.go`) previously rejected
all single-segment paths (like `/run`) to avoid intercepting
app routes (like `/dogs`). This was changed to match against
known published modules instead, so `monks.co/run` resolves
correctly while app routes are still unaffected.

### backupd (cmd/backupd)

Published to `github.com/amonks/backupd` with vanity path
`monks.co/backupd`. Source repo was jj-only (non-colocated) —
required `jj git colocation enable` before `git subtree add`
could read from it. Module path was already `monks.co/backupd`
so no import rewrites needed. Had a `tools.go` with templ,
govulncheck, and staticcheck tool imports — removed it and ran
`go mod tidy`. The `tasks.toml` was adapted to use `go tool`
commands instead of `go run` for tools. gofix applied two
changes: `fmt.Sprintf` → `fmt.Appendf` in logger, manual map
copy → `maps.Copy` in sync status. First staticcheck run failed
due to stale cache; passed on retry.
