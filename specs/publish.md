# Publish

## Overview

Tooling for publishing selected subtrees of the private monorepo as
read-only public GitHub mirror repos. The monorepo remains the single
source of truth; mirrors are one-directional copies.

By default, all public packages go into a single shared mirror repo
(`default_mirror`). Packages can override this with an explicit
`mirror` to get their own repo.

Code: [cmd/publish/](../cmd/publish/)

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
module_path = "github.com/amonks/run"
mirror = "github.com/amonks/run"  # explicit → gets its own repo
```

Fields:

- `default_mirror` (top-level): shared mirror repo for packages without
  an explicit `mirror`. Directory structure is preserved.
- `dir` (required): directory relative to monorepo root.
- `module_path` (optional): Go module path. Defaults to `monks.co/<dir>`.
- `mirror` (optional): explicit GitHub mirror repo. When set, the
  package gets its own repo via `git subtree split`.

## Validation

Go tests in `cmd/publish/validate_test.go` enforce publish invariants.
These run as part of `go test monks.co/...`:

- Every public package's transitive `monks.co/*` deps are also public.
- Every public package has a `LICENSE` file.
- Every public package's `go.mod` has the correct module path.

## Publish Flow

`go run ./cmd/publish [--dry-run]`

### Default Mirror (git-filter-repo)

Packages without an explicit `mirror` are published together to the
`default_mirror` repo using `git-filter-repo`:

1. Clone the monorepo to a temp directory.
2. Run `git filter-repo --path pkg/set --path pkg/serve ...` to keep
   only the public package directories. This preserves full file
   history (blame, log).
3. Push the filtered result to the default mirror.
4. Push tags matching public package prefixes (e.g., `pkg/set/v1.0.0`).

The mirror preserves directory structure: `pkg/set/go.mod`,
`pkg/serve/go.mod`, etc. at their natural paths. Tags use the standard
Go subdirectory module format (`<dir>/v<version>`).

Adding a new package to `publish.toml` changes the filtered history
retroactively (pulling in the package's full history), requiring a
force push.

### Explicit Mirrors (git subtree split)

Packages with an explicit `mirror` get their own repo:

1. Run `git subtree split --prefix=<dir>` to extract the subtree.
2. Create the mirror repo if it doesn't exist (`gh repo create`).
3. Push the split SHA to the mirror's `main` branch.
4. Push tags with the directory prefix stripped
   (`pkg/serve/v1.0.0` → `v1.0.0`).

### jj Compatibility

All git commands detect jj's git backend (`.jj/repo/store/git`) and
set `GIT_DIR`/`GIT_WORK_TREE` accordingly.

### Dry Run

`--dry-run` prints what would happen without cloning, filtering,
creating repos, or pushing.

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
`pkg/set/v1.0.0`).

- Default mirror: tags are pushed as-is (standard Go subdirectory
  module format).
- Explicit mirrors: tags have the directory prefix stripped
  (`v1.0.0`).

## CI

GitHub Actions workflow (`.github/workflows/ci.yml`) runs on pushes to
`main`:

1. Checks out with full history (`fetch-depth: 0`).
2. Sets up Go 1.26.0.
3. Installs `git-filter-repo` via pip.
4. Runs `go tool run test`.
5. Runs `go run ./cmd/publish` to filter and push mirrors.

`GH_TOKEN` is set from the repo's `GITHUB_TOKEN` secret for
`gh repo create`.

## Dependencies

- `gh` CLI (for creating mirror repos)
- `git-filter-repo` (for default mirror publishing; install via
  `pip install git-filter-repo`)
