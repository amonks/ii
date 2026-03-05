# Publish

## Overview

Tooling for publishing selected subtrees of the private monorepo as
read-only public GitHub mirror repos. The monorepo remains the single
source of truth; mirrors are one-directional copies.

By default, all public packages go into a single shared mirror repo
(`default_mirror`). Packages can override this with an explicit
`mirror` to get their own repo.

Code: [cmd/publish/](../cmd/publish/)

Library: [pkg/ci/publish/](../pkg/ci/publish/)

Config: [config/publish.toml](../config/publish.toml)

## Config

`config/publish.toml` declares which packages are public:

```toml
default_mirror = "github.com/amonks/go"

[[package]]
dir = "pkg/set"
# no mirror → goes into default_mirror, preserving directory structure

[[package]]
dir = "cmd/run"
module_path = "monks.co/run"
mirror = "github.com/amonks/run"  # explicit → gets its own repo
version = "1.0.0-beta"             # auto-publishes as v1.0.0-beta.N
```

Fields:

- `default_mirror` (top-level): shared mirror repo for packages without
  an explicit `mirror`. Directory structure is preserved.
- `dir` (required): directory relative to monorepo root.
- `module_path` (optional): Go module path. Defaults to `monks.co/<dir>`.
- `mirror` (optional): explicit GitHub mirror repo. When set, the
  package gets its own repo via `git subtree split`.
- `version` (optional): version track for auto-versioning. Defaults to
  `"0.0"`. The tool appends an incrementing number. Examples:
  - `"0.0"` → `v0.0.1`, `v0.0.2`, ...
  - `"1.0"` → `v1.0.1`, `v1.0.2`, ...
  - `"1.0.0-beta"` → `v1.0.0-beta.1`, `v1.0.0-beta.2`, ...
  - `"2.0.0-rc"` → `v2.0.0-rc.1`, `v2.0.0-rc.2`, ...
  Changing the track starts fresh at 1. Moving from a pre-release
  track to a release track (e.g., `"1.0.0-beta"` → `"1.0"`) is how
  you graduate to a stable release.

## Validation

`go run ./cmd/publish -validate` enforces publish invariants.
This runs as a task (`publish-validate`) during `go tool run test`,
outside the `go test` cache:

- Every public package's transitive `monks.co/*` deps are also public.
- Every public package has a `LICENSE` file.
- Every public package's `go.mod` has the correct module path.

Note: go.mod completeness for `monks.co/*` cross-dependencies is NOT
validated because the publish flow rewrites go.mods at publish time
(see below).

The dependency graph resolves `monks.co/*` import paths to module
directories by reading `go.mod` files and using longest prefix match.
This handles modules whose path doesn't match their directory (e.g.,
`monks.co/beetman` at `cmd/beetman`).

## Publish Flow

`go run ./cmd/publish [--dry-run]`

### Phase 1: Change Detection

For each public module (topo-sorted in dependency order):

1. Find the latest monorepo tag (`<dir>/v<version>`).
2. Check if any files changed since that tag (`git diff`).
3. A module needs publishing if its source changed, or if any of its
   public dependencies were re-tagged this run.

### Phase 2: Versioning

Each package has a human-managed version track (defaulting to
`"0.0"`). The publish tool appends an incrementing number. Examples:

- Track `"0.0"`, no prior tag → `v0.0.1`
- Track `"0.0"`, latest tag `v0.0.3` → `v0.0.4`
- Track `"1.0.0-beta"`, latest `v1.0.0-beta.37` → `v1.0.0-beta.38`
- Track changed to `"1.0"`, latest `v1.0.0-beta.37` → `v1.0.1`

Modules that haven't changed keep their existing latest tag version.

### Phase 3: go.mod Rewriting

The monorepo uses a Go workspace (`go.work`), so `monks.co/*`
cross-dependencies resolve locally without go.mod require directives.
Published mirrors don't have the workspace, so their go.mods must be
self-contained.

The publish flow:

1. Clones the monorepo to a temp directory.
2. For each module being published, rewrites its go.mod to add
   `require` directives for `monks.co/*` dependencies, pinned to the
   version tags determined in Phase 2.
3. Commits the go.mod changes in the temp clone.
4. Creates version tags in the temp clone.

### Phase 4: Publishing

From the temp clone (with rewritten go.mods):

**Default mirror (git-filter-repo):** Packages without an explicit
`mirror` are published together to the `default_mirror` repo:

1. Clone from the temp dir, run `git filter-repo --path ...` to keep
   only public package directories.
2. Push to the default mirror. Tags use the standard Go subdirectory
   module format (`<dir>/v<version>`).

All default-mirror packages are included if any of them changed
(filter-repo rebuilds the full history).

**Explicit mirrors (git subtree split):** Packages with an explicit
`mirror` get their own repo:

1. `git subtree split --prefix=<dir>` to extract the subtree.
2. Create the mirror repo if it doesn't exist (`gh repo create`).
3. Push to the mirror's `main` branch.
4. Push tags with the directory prefix stripped
   (`cmd/run/v1.0.0` → `v1.0.0`).

### Phase 5: Tagging the Monorepo

After successful publishing, version tags are created in the real
monorepo so future runs can detect what changed.

### jj Compatibility

All git commands detect jj's git backend (`.jj/repo/store/git`) and
set `GIT_DIR`/`GIT_WORK_TREE` accordingly.

### Dry Run

`--dry-run` shows which modules need publishing and what versions
they'd get, without cloning, rewriting, or pushing.

## Vanity Import Paths

The proxy handles `go get monks.co/pkg/set` by serving
`<meta name="go-import">` tags. See [proxy spec](proxy.md#vanity-import-paths).

For default mirror packages, the go-import prefix is `monks.co` (the
VCS root), so Go clones the shared repo and finds modules in
subdirectories. The root path `/?go-get=1` also serves this meta tag
for Go's VCS root verification step.

For explicit mirror packages, the go-import prefix is the module path
itself, pointing at the package's own repo.

## Tagging

Monorepo tags use the format `<dir>/v<version>` (e.g.,
`pkg/set/v0.0.1`, `cmd/run/v1.0.3`).

The publish tool auto-generates tags by incrementing the patch version
from the package's configured `version` base. Tags are standard
canonical semver, compatible with all Go tooling.

- Default mirror: tags are pushed as-is (standard Go subdirectory
  module format).
- Explicit mirrors: tags have the directory prefix stripped
  (e.g., `cmd/run/v1.0.3` → `v1.0.3`).

## CI

GitHub Actions workflow (`.github/workflows/ci.yml`) runs on pushes to
`main`:

1. Checks out with full history (`fetch-depth: 0`).
2. Sets up Go 1.26.0.
3. Installs `git-filter-repo` via pip.
4. Runs `go tool run test`.
5. Runs `go run ./cmd/publish` to detect changes, rewrite go.mods,
   tag, and push mirrors.

`GH_TOKEN` is set from the repo's `GITHUB_TOKEN` secret for
`gh repo create`.

## Dependencies

- `gh` CLI (for creating mirror repos)
- `git-filter-repo` (for default mirror publishing; install via
  `pip install git-filter-repo`)
