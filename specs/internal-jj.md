# Internal JJ

## Overview
The jj package wraps the `jj` CLI to provide Go-friendly helpers.

## Client Operations
- Repository init: `Init` runs `jj git init`.
- Workspace operations: `WorkspaceRoot`, `WorkspaceAdd`, `WorkspaceList`, `WorkspaceForget`, `WorkspaceUpdateStale`.
- Change operations: `Edit`, `NewChange`, `NewChangeWithMessage`, `CurrentChangeID`, `CurrentChangeEmpty`, `ChangeIDAt`, `ChangeIDsForRevset`, `DescriptionAt`, `Snapshot`, `Describe`, `DiffStat`.
- `SeriesLog` returns the jj log for the patch series (`fork_point(@|main)..@-`) with descriptions and diff stats. Returns empty string when there are no commits or jj outputs "No revisions to show".
- `Describe` uses `jj describe --stdin` to avoid long argument lists.
- `Commit` is implemented as `Describe` followed by `NewChange`.
- Bookmark operations: `BookmarkList`, `BookmarkCreate`.

## Error Handling
- CLI output is included in errors to help diagnose failures.
- `FileShow` treats "no such file/path" output as `ErrFileNotFound` so callers can treat missing files as empty state.
