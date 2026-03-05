# Workspace Pool

## Overview
The workspace pool manages a shared set of jujutsu workspaces for a repository. It hands out leases to callers, reuses released workspaces, and persists workspace state so multiple processes can coordinate safely.

## Architecture
- `workspace.Pool` is the public API for acquiring, releasing, listing, and destroying workspaces.
- The CLI constructs `internal/db.DB` once per command and passes the SQLite handle into `workspace.NewPool`.
- Agent session timing helpers (for age/duration display) live in `workspace` to keep the CLI thin.
- Workspace state is persisted in SQLite via `internal/db` (default `~/.local/state/incrementum/state.db`) and the `workspace.Store` SQL helpers.
- Workspaces live under a shared base directory (`~/.local/share/incrementum/workspaces` by default).
- Jujutsu operations are delegated to `internal/jj` (workspace add/forget, edit, and new change).
- Configuration hooks are loaded from the merged config (`incrementum.toml` or `.incrementum/config.toml` plus `~/.config/incrementum/config.toml`) via `internal/config` and executed on each acquire.

## State Model
- Workspace state is managed by `workspace.Store`, which uses SQLite tables (`repos`, `workspaces`). See [internal-db.md](./internal-db.md) for connection behavior. Legacy `state.json` is imported once on first open.
- Workspace-specific state includes: path, repo name, purpose, revision, status, created/updated timestamps, acquisition PID/time, and provisioning status.
- Workspace names are sequential `ws-###` values allocated per repo.

## Workspace Lifecycle
### Acquire
- Defaults: `Rev` defaults to `@`.
- `Purpose` must be non-empty and single-line; `ValidateAcquirePurpose` enforces this validation.
- On acquire, the SQL store does the following:
  - Atomically selects and marks the first available workspace for the repo (single statement update) to avoid concurrent acquisitions.
  - Otherwise allocate a new `ws-###` name and mark it acquired.
- If a new workspace is allocated, `jj workspace add` is executed and the workspace directory is created.
- Once a workspace is selected, the requested revision is resolved to a change ID in the **source repository** context. This is necessary because symbolic refs like `@` have different meanings in the workspace vs source repo. Then a new change is created with `jj new <resolved-rev>` to ensure the workspace is always checked out to a fresh change based on the expected parent.
- If the requested revision is missing and looks like a change ID, the pool retries with `@` (resolved in the source repo) as the parent.
- When `NewChangeMessage` is provided, it is used as the description for that newly created change.
- `incrementum.toml` or `.incrementum/config.toml` is loaded from the source repo (merged with global config) and the workspace `on-create` hook runs for every acquire (including reuse).
- When `SkipHooks` is set, hook execution and provisioning marking are suppressed entirely. This is used by the todo store, which immediately edits to an orphan bookmark where main-tree hooks would fail.
- A workspace is marked `Provisioned` once the hooks run successfully.

### Release
- Release creates a new change at `root()` to reset the workspace state.
- The workspace remains on disk, but its status is marked `available`, and purpose and acquisition metadata are cleared.

### List
- Listing returns every workspace for a repo when `--all` is provided.
- Default CLI output lists both acquired and available workspaces.
- List output is ordered by status (acquired first), then by workspace name.
- CLI table output includes `AGE` and `DURATION` columns showing how long each workspace has been held, plus the revision each workspace was opened to.
- `AGE` uses `now - created_at`.
- `DURATION` uses `now - created_at` for acquired workspaces; available workspaces use `updated_at - created_at`.

### Destroy All
- Destroy-all removes workspaces for a repo from SQLite, forgets each workspace from jj (best-effort), deletes the workspace directories, and removes the repo workspaces directory if empty.

## Repo Resolution
- `RepoRoot(path)` returns the jj root for any path, normalized via `paths.NormalizePath` to handle macOS symlinks like `/private/var` → `/var`.
- `RepoRootFromPath(path)` resolves a workspace path back to the source repo using the SQLite mapping when possible.
- If the path is inside the workspace pool directory but no repo mapping exists, `ErrRepoPathNotFound` is returned.
- All paths stored in SQLite and returned by these functions are normalized to ensure consistent comparisons regardless of whether macOS-specific prefixes are present.

## CLI Commands
- `ii workspace acquire [--rev <rev>] --purpose <text>`: acquire or create a workspace; prints the workspace path.
- `ii workspace release [name...]`: release one or more workspaces by name (or the current workspace when no names provided); prints "released workspace <name>" for each.
- `ii workspace list [--json] [--all]`: list workspaces for the current repo.
- `ii workspace exec [--rev <rev>] [--purpose <text>] -- <command> [args...]`: acquire a workspace, run a command in it via a PTY, and release it when done. If `--purpose` is omitted, defaults to `"exec: <command>"`. Stdin/stdout/stderr are wired through a PTY so interactive programs (shells, editors) work correctly. The child's exit code is propagated.
- `ii workspace destroy-all`: remove all workspaces for the current repo.
