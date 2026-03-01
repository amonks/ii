# Todo Store

## Overview

The todo subsystem provides a lightweight, Jira-like tracker scoped to a
jujutsu repository. Todos live in a dedicated store so they can be shared
across workspaces without polluting the code history.

## Architecture

- Storage lives in a special orphan change parented at `root()` and
  referenced by the jj bookmark `incr/tasks`.
- Writes access the store through a background workspace from the workspace
  pool; operations never mutate the user's working copy.
- Read-only access does not require a workspace. Reads use
  `jj file show -r incr/tasks <file>` to fetch JSONL data directly.
- Data is stored as JSONL files in the store workspace:
  - `todos.jsonl` holds one JSON object per todo.
  - `dependencies.jsonl` holds one JSON object per dependency.
- All writes are guarded by exclusive file locks, written to a temp file
  and atomically renamed. Each write snapshots the jj workspace to persist
  the change.
- Cross-process coordination uses an exclusive `flock(2)` on a lock file
  in the state directory (`~/.local/state/incrementum/todo-<repo-name>.lock`).
  The lock is acquired before opening the workspace and held for the entire
  `Store` lifetime, serializing all todo operations across processes.
  Lock files are intentionally NOT removed on release; removing the path
  would allow a new opener to create a different inode, breaking
  inode-based mutual exclusion.
- `todo.Open` can create the store when missing, optionally prompting the
  user before creating the bookmark.
- `todo.Open` acquires a workspace with a purpose string from
  `OpenOptions.Purpose`, defaulting to `todo store`.
- `OpenOptions.ReadOnly` skips workspace acquisition and opens the store
  for read-only access.
- Prompting via stdin only happens when stdin is a TTY; non-interactive calls
  skip the prompt and proceed with creation unless a custom prompter is used.
- `StdioPrompter` writes prompts to stderr for consistency with error messages
  in interactive error-recovery scenarios, and reads responses from stdin.

## Data Model

### Todo

Fields (JSON keys):

- `id`: 8-character lowercase base32 identifier.
- `title`: required; must include non-whitespace characters; max length 500 characters.
- `description`: optional free text.
- `status`: `open`, `proposed`, `queued`, `in_progress`, `queued_for_merge`, `merging`, `merge_failed`, `closed`, `done`, `waiting`, `stuck`, or `tombstone`.
- `priority`: integer 0..4 (0 = critical, 4 = backlog).
- `type`: `task`, `bug`, or `feature`.
- `job_id`: optional job identifier populated after implementation when queued for merge.
- `implementation_model`: optional model override for implementation.
- `code_review_model`: optional model override for commit review.
- `project_review_model`: optional model override for project review.
- `created_at`, `updated_at`: timestamps.
- `closed_at`: timestamp if closed or done.
- `started_at`: timestamp when entering `in_progress`.
- `completed_at`: timestamp when finishing work (set when transitioning from `in_progress` to `done` or `queued_for_merge`).
- `deleted_at`: timestamp if tombstoned.
- `delete_reason`: optional reason when tombstoned.
- `source`: optional origin tracker; empty means user-created, `habit:<name>` means created by a habit.

### Dependency

Fields (JSON keys):

- `todo_id`: todo that owns the dependency.
- `depends_on_id`: todo that must be resolved first.
- `created_at`: timestamp.

## Semantics

### ID Generation

- IDs are derived from `title + RFC3339Nano timestamp`, hashed with SHA-256,
  then base32-encoded and lowercased.
- The store resolves user-provided IDs by case-insensitive prefix matching.
  Prefixes must be unambiguous; otherwise operations fail.

### Status + Timestamp Rules

- `open`/`proposed`/`queued`/`in_progress`/`waiting`/`stuck`: `closed_at` must be empty; `completed_at` must be empty; `deleted_at` must be empty.
- `queued_for_merge`/`merging`/`merge_failed`: `closed_at` must be empty; `completed_at` must be set; `deleted_at` must be empty.
- `closed`/`done`: `closed_at` must be set; `deleted_at` must be empty.
- `tombstone`: `deleted_at` must be set; `closed_at` must be empty;
  `delete_reason` is allowed only when tombstoned.
- `started_at` is only set for `in_progress`, `queued_for_merge`, `merging`, `merge_failed`, or `done` todos.
- `completed_at` is only set for `queued_for_merge`, `merging`, `merge_failed`, or `done` todos.
- `job_id` is allowed on `queued_for_merge`, `merging`, `merge_failed`, or `done` todos; it is cleared when finishing from a merge status unless explicitly set.
- `waiting` represents todos blocked on external factors (upstream PRs, API
  availability, etc.). Unlike dependency blocking (for internal task ordering),
  waiting is for external factors. The reason for waiting lives in the
  description field (unstructured).

### Create

- Title is required and validated.
- CLI `todo create` expects the title via `--title`; it is not positional.
- Defaults: `type=task`, `priority=medium` (2), `status=open`.
- If `INCREMENTUM_TODO_PROPOSER=true` is set in the CLI environment, the default
  status is `proposed` instead.
- Agent runs set `INCREMENTUM_TODO_PROPOSER=true` for tool commands, so todos
  created by the agent default to `proposed`.
- Type inputs are case-insensitive and stored as lowercase.
- Editor mode is used by default only when no create fields are supplied; use `--edit` to force it or `--no-edit` to skip it.
- CLI description input via `--description -` / `--desc -` trims trailing CR/LF characters.
- Dependencies may be supplied as IDs; each dependency must reference an
  existing todo.
- Dependency IDs accept the same case-insensitive prefix matching as other
  commands.
- Optional per-todo model overrides (`implementation_model`, `code_review_model`,
  `project_review_model`) default to empty and override project/global settings
  when set.

### Update

- Only fields explicitly provided are changed.
- When `todo update` runs in editor mode for multiple IDs, the CLI opens one editor session per todo.
- Editor mode is used by default only when no update fields are supplied; if update fields are provided, the editor opens only with `--edit`.
- `todo edit` is an alias for `todo update`.
- CLI description input via `--description -` / `--desc -` trims trailing CR/LF characters.
- Status transitions automatically adjust timestamps:
  - `closed`/`done` sets `closed_at` and clears delete markers.
- `open`/`proposed`/`queued`/`in_progress`: clears `closed_at`, `completed_at`, and delete markers.
  - `in_progress` sets `started_at` when the status changes.
- `queued_for_merge`/`merging`/`merge_failed`: clears `closed_at` and delete markers, preserving `started_at`/`completed_at`.
  - `queued_for_merge` preserves `started_at` and sets `completed_at` when moving from `in_progress`.
  - `done` preserves `started_at` and sets `completed_at` only when moving from `in_progress`.
  - `tombstone` clears `closed_at`; `deleted_at` must be set.
- Status and type inputs are case-insensitive and stored as lowercase.
- Updating `deleted_at` without `delete_reason` preserves any existing delete reason; clear it explicitly when needed.
- Reapplying the current status does not reset timestamps unless explicitly provided.
- `job_id` is cleared automatically when transitioning out of merge-related statuses unless explicitly set.
- `updated_at` always changes when a todo is updated.

### Close / Reopen / Start / Queue / Delete

- `close` sets status to `closed` and updates `closed_at`.
- `reopen` sets status to `open` and clears `closed_at`.
- `start` sets status to `in_progress`, clears `closed_at`, and sets `started_at`.
- `queue` sets status to `queued` and clears `closed_at`.
- `queue_for_merge` sets status to `queued_for_merge`, preserves `started_at`, sets `completed_at`, and stores `job_id`.
- `merge` sets status to `merging` (preserves `started_at`, `completed_at`, `job_id`).
- `merge_failed` sets status to `merge_failed` (preserves `started_at`, `completed_at`, `job_id`).
- `finish` sets status to `done` and sets `completed_at` when transitioning from `in_progress`.
- `delete` sets status to `tombstone`, sets `deleted_at`, clears `closed_at`,
  and optionally records a delete reason.
- CLI `todo delete` accepts the same filter flags as `todo list` (status,
  priority, type, id, title, description), plus positional IDs. At least one
  ID or filter flag must be provided. Filters and positional IDs are combined.
- Close/finish/reopen/start/queue do not store reasons; only delete supports
  `delete_reason`.

### List

- Returns todos matching optional filters: status, priority, type, IDs,
  title substring, description substring.
- Priority filters must be within 0..4; invalid values return an error.
- Status and type filters are case-insensitive.
- Invalid status or type filters return errors listing valid values.
- Tombstones are excluded by default unless `IncludeTombstones` is set.
- Setting `Status=tombstone` implicitly includes tombstones in list results.
- CLI `todo list` includes tombstones when `--tombstones` is provided or when `--status tombstone` is specified.
- CLI `todo list` excludes `done` todos by default unless `--status` or `--all` is provided.
- Proposed and waiting todos are included in the default list output alongside open and in-progress work.
- When `todo list` is empty but matching `done` or `tombstone` todos exist, the CLI prints a hint to use `--all` and/or `--tombstones`.
- CLI ID highlighting uses the shortest unique prefix across all todos,
  including tombstones, so the display matches prefix resolution.
- All CLI outputs that show todo IDs (create/update logs, show/detail views,
  list/ready tables, dependency output) use the same prefix highlighting rules.
- CLI table output includes an `AGE` column formatted as `<count><unit>`, using
  `s`, `m`, `h`, or `d` based on recency.
- `AGE` uses `now - created_at`.
- CLI table output includes an `UPDATED` column formatted as `<count><unit>`.
- `UPDATED` uses `now - updated_at`.
- CLI table output includes a `DURATION` column for active/finished work.
- `DURATION` uses `now - started_at` for `in_progress` todos.
- `DURATION` uses `completed_at - started_at` for `done`, `queued_for_merge`, `merging`, and `merge_failed` todos.
- When the todo store is missing, CLI `todo list` does not prompt to create it
  and returns an empty list.

### Show

- CLI detail output includes deleted timestamps and delete reasons when present.
- CLI detail output renders todo descriptions with the markdown renderer and 80-column wrapping.
- When the todo store is missing, CLI `todo show` does not prompt to create it
  and returns the store missing error.
- `Store.Show` returns todos in the same order as the requested IDs.

### Ready

- Returns `open` todos that have no unresolved dependencies.
- `stuck` todos are not considered ready.
- A dependency is unresolved when the depended-on todo is not `closed`, `done`, `stuck`, or `tombstone`.
- Results are ordered by priority (ascending), then type (bug, task, feature),
  then creation time (oldest first); an optional limit truncates the list.
- When the todo store is missing, CLI `todo ready` does not prompt to create it
  and returns an empty list.

### Dependencies

- Dependencies mean `depends_on_id` must be closed before `todo_id` is ready.
- Self-dependencies and duplicates are rejected.
- Dependency inputs must be IDs.
- Dependency trees are computed by walking dependencies from a root todo;
  cycles are avoided by tracking the current traversal path so shared
  dependencies can appear under each branch.
- When the todo store is missing, CLI dependency tree output does not prompt to
  create it and returns the store missing error.

## CLI Mapping

The CLI mirrors the store API:

- `todo create` -> `Store.Create`
- `todo update` (`todo edit`) -> `Store.Update`
- `todo start` -> `Store.Start`
- `todo close` -> `Store.Close`
- `todo finish` (`todo done`) -> `Store.Finish`
- `todo queue` -> `Store.Queue`
- `todo reopen` -> `Store.Reopen`
- `todo delete` (`todo destroy`) -> `Store.Delete`
- `todo show` -> `Store.Show`
- `todo list` -> `Store.List`
- `todo ready` -> `Store.Ready`
- `todo dep add` -> `Store.DepAdd`
- `todo dep tree` -> `Store.DepTree`
