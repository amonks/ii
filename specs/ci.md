# CI

## Overview

Self-hosted CI/CD service replacing GitHub Actions with two Fly apps.
The orchestrator receives webhook triggers and manages builder machines.
The builder runs tests, builds OCI images, deploys apps, publishes
mirrors, and applies terraform.

Orchestrator code: [apps/ci/](../apps/ci/)

Builder code: [apps/ci/cmd/builder/](../apps/ci/cmd/builder/)

## Architecture

**monks-ci** (orchestrator): Always-on Fly app (`shared-cpu-1x`, 256MB).
Receives triggers, creates builder machines, tracks runs in SQLite,
serves dashboard, sends SMS on failure. Stores build output on its
persistent volume.

**monks-ci-builder** (builder): Ephemeral Fly machine
(`performance-4x`, 4GB, `auto_destroy: true`). Fat image with all
build deps. Persistent volume at `/data` for caches and repo clone.
Joins tailnet as `monks-ci-builder-fly-ord`. Communicates with
orchestrator via `tailnet.Client()`. Streams output in real time.

## Trigger

POST `/trigger` with `{"sha":"abc123"}`. Not publicly accessible —
only reachable over tailnet. Called by a minimal GitHub Actions
workflow that joins the tailnet and curls the endpoint.

Behavior:
- Rejects if a run is already in progress (returns 409)
- Looks up base SHA from last successful run (all-zeros if none)
- Creates run row in SQLite
- Resolves current builder image by listing `monks-ci-builder` machines
  via the Fly API (falls back to a hardcoded default if none found)
- Creates builder machine via `pkg/flyapi`
- Returns 202 with run ID, head SHA, base SHA

## Builder Callback API

The builder reports progress back over tailnet:

- `PUT /api/runs/{id}/jobs/{name}/start` — mark job in_progress,
  set output_path to the output directory for this job
- `PUT /api/runs/{id}/jobs/{name}/done` — store result, duration,
  kind-specific data (deploy details, terraform resource counts)
- `POST /api/runs/{id}/jobs/{name}/output/{stream}` — append raw
  bytes to a named output stream file on disk
- `PUT /api/runs/{id}/done` — mark run complete, SMS on failure
- `GET /api/runs/{id}/base-sha` — return base SHA for this run
- `POST /api/runs/{id}/deployments` — record deployment
- `POST /runs/{id}/mark-dead` — mark a running run as dead
  (dashboard action, not builder API)

## Output Streaming

Each job has one or more named output streams stored as append-only
files at `/data/output/runs/{runID}/{jobName}/{stream}.log` on the
orchestrator's volume.

**Builder side**: A `StreamWriter` implements `io.Writer`, buffers
writes, and flushes them to the orchestrator's output endpoint on a
cadence (every 500ms or 8KB, whichever comes first). The `Reporter`
has a `StreamWriter(jobName, stream)` method to create writers.

**Test jobs**: Use the `run` library (github.com/amonks/run)
programmatically via `taskfile.Load` + `runner.New`. A custom
`MultiWriter` implementation returns a `StreamWriter` per task ID,
giving separate output streams for each task (go-test, staticcheck,
templ, etc).

**Other jobs**: Single stream per job. Shellout stdout/stderr and
progress messages both write to the same `StreamWriter`. Deploy
jobs log progress ("compiling X", "pushing image", "deploying").

## Live Output Hub

An in-memory pub/sub (`OutputHub` in `notify.go`) enables live-tailing
of build output. Keys are `"runID/jobName/stream"`.

- `appendOutput` API handler publishes bytes to the hub after writing
  to disk
- `finishJob` API handler calls `CloseAll(prefix)` to close all
  subscriber channels for that job, signaling EOF
- `serveStream` with `?stream=1` query param writes existing file
  content then subscribes to the hub for live updates, flushing each
  chunk as it arrives. The connection stays open until the channel
  closes (job finished) or the client disconnects.

## Dashboard

- `GET /` — recent runs, current deployments per app
- `GET /runs/{id}` — jobs for this run with inline output viewers.
  Each stream is a collapsible `<details>` showing a status dot,
  stream name, and last line preview. Expanding loads the stream
  content. For running runs, JS uses `fetch()` with `getReader()`
  to live-tail the `?stream=1` endpoint, auto-scrolling and updating
  the last-line preview. For finished runs, a simple fetch loads the
  full content on expand.
- `GET /deployments` — deployment history
- `GET /output/{runID}/{jobName}` — redirects to single stream or
  lists available streams for multi-stream jobs
- `GET /output/{runID}/{jobName}/{stream}` — stream log content.
  Without `?stream=1`, returns current file content. With `?stream=1`,
  returns current content then live-tails new data until the job
  finishes.

## Database

SQLite with WAL mode. Tables: `runs`, `jobs`, `deploy_jobs`,
`terraform_jobs`, `deployments`. See
[migrations/001_initial.sql](../apps/ci/migrations/001_initial.sql).

## Task Event

When a CI run finishes (via the `PUT /api/runs/{id}/done` endpoint), the
orchestrator emits a single wide `slog.Info("task", ...)` event with all
run metadata flattened into dotted keys:

- `task.name` = `"ci-run"`
- `task.status` = success/failed/dead
- `task.duration_ms` = run wall-clock time
- `task.error` = error message (if any)
- `run.id`, `run.head_sha`, `run.base_sha`, `run.trigger`
- `job.<name>.status`, `job.<name>.duration_ms` — per finished job
- `deploy.<app>.image_ref`, `deploy.<app>.compile_ms`,
  `deploy.<app>.push_ms`, `deploy.<app>.deploy_ms`,
  `deploy.<app>.image_bytes` — per deploy job
- `terraform.resources_added`, `terraform.resources_changed`,
  `terraform.resources_destroyed` — if terraform ran

The run detail page displays this event as a key-value table in the
"Task Event" section, fetched from the logs service with
`q=group:app,app:ci,msg:task` and filtered by `run.id`.

## SMS on Failure

Uses `tailnet.Client()` to POST to
`http://monks-sms-brigid/?message=CI+run+{id}+failed:+{error}`

## Builder Pipeline

1. Join tailnet (via `tailnet.WaitReady`; uses `TS_AUTHKEY` from env)
2. Clone or fetch repo onto persistent volume at `/data/repo`
   (`jj git clone --colocate` on first run, `jj git fetch` on
   subsequent runs, then `jj new` the head SHA)
3. Get base SHA from orchestrator
4. Diff changed files
5. Run generate + test (per-task output streams via run library)
6. Concurrently:
   - Deploy affected apps (each app deploys in parallel; streams compile/push/deploy progress)
   - Publish subtrees (streams output)
   - Terraform apply (streams output)
   All three run as goroutines; errors are collected (not fail-fast) and joined.
7. Report run complete
10. Exit → machine self-destructs

## Dependencies

- `pkg/flyapi` — create builder machines
- `pkg/ci/changedetect` — change detection
- `pkg/ci/publish` — mirror publishing (accepts io.Writer for output)
- `pkg/oci` — OCI image building
- `pkg/database` — SQLite
- `pkg/serve` — HTTP mux
- `pkg/tailnet` — tailnet membership
- `pkg/reqlog` — structured logging
- `github.com/amonks/run` — task runner (programmatic API for test jobs)
