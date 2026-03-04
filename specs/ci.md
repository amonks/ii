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
Joins tailnet via kernel tailscale (started by the entrypoint script)
as `monks-ci-builder-fly-{region}`. Using kernel tailscale (rather
than tsnet) puts the whole machine on the tailnet so that child
processes like `go test` can reach tailnet hosts (e.g. the LLM
gateway). Communicates with orchestrator via `http.DefaultClient`.
Streams output in real time.
The `monks-ci-builder` Fly app has no standing machines. When CI
rebuilds the builder image, it runs
`fly deploy --build-only --push` to build via Fly's remote builder
and push to the registry without creating any machines. The
orchestrator creates ephemeral machines on demand.

## Data Model

Every pipeline phase is one **Job** with N **Streams**. Each stream
carries its own status, duration, and error. This is uniform across
all job types:

- **fetch**: 1 job, 1 stream ("output")
- **generate**: 1 job, N streams (one per run-library task)
- **test**: 1 job, N streams (one per run-library task)
- **deploy**: 1 job, N streams — "analysis" stream for change
  detection and skip reporting, per-fly-app streams for affected
  apps, plus "publish" and "terraform" streams, all running in
  parallel after analysis completes

Deploy metadata (compile_ms, push_ms, image_ref, etc.) is sent as
part of the `FinishRun` request payload for task event emission,
not stored in the DB.

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
- `PUT /api/runs/{id}/jobs/{name}/done` — store result and duration
- `PUT /api/runs/{id}/jobs/{name}/streams/{stream}/start` — create
  stream record, mark as in_progress
- `PUT /api/runs/{id}/jobs/{name}/streams/{stream}/done` — store
  stream result, duration, error
- `POST /api/runs/{id}/jobs/{name}/output/{stream}` — append raw
  bytes to a named output stream file on disk
- `PUT /api/runs/{id}/done` — mark run complete, emit task event,
  SMS on failure. Accepts optional `deploys` array with deploy
  metadata for the task event log.
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
has a `StreamWriter(jobName, stream)` method to create writers,
plus `StartStream` and `FinishStream` for lifecycle management.
All HTTP calls from the builder (Reporter and StreamWriter) use
`retryDo` with exponential backoff and jitter to survive transient
failures (e.g. orchestrator restart during self-deploy). Reporter
callbacks retry up to 10 times (500ms–30s backoff, ~2–3 min window).
StreamWriter output sends retry up to 5 times (200ms–5s backoff);
on exhaustion the data is dropped (losing a few output lines is
acceptable). Only connection errors and 5xx are retried; 4xx
responses are not retried.

**Test jobs**: Use the `run` library (github.com/amonks/run)
programmatically via `taskfile.Load` + `runner.New`. A custom
`MultiWriter` implementation returns a `StreamWriter` per task ID,
giving separate output streams for each task (go-test, staticcheck,
templ, etc). After `run.Start()` returns, `run.TaskStatus()` is
called for each task ID to set per-stream status (success, failed,
or skipped).

**Deploy job**: A single "deploy" job with one stream per fly app
plus optional streams for image rebuilds. Unaffected apps get a
"skipped" stream. Affected apps get a stream showing
compile/push/deploy progress with success/failed status. Deploy
metadata is accumulated via `reporter.AddDeployResult()` and sent
with the `FinishRun` call. CI app deploys go through the same
compile→OCI→push→deploy path as all other apps. Builder and base
image rebuilds run as separate streams (`ci-builder`, `ci-base`)
concurrently with app deploys when their Dockerfile or Go
dependencies change. Image rebuilds use
`fly deploy --build-only --push`. The orchestrator restart during
self-deploy is safe because state is on a persistent volume (SQLite,
output files), and the builder's retry logic handles transient
connection failures during the restart window.

**Other jobs**: Single "output" stream per job. Shellout stdout/stderr
and progress messages both write to the same `StreamWriter`.

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
  Each stream is a collapsible `<details>` showing a status dot
  colored by per-stream status (not job status), stream name,
  optional duration, and last line preview. Expanding loads the stream
  content. For running runs, JS uses `fetch()` with `getReader()`
  to live-tail the `?stream=1` endpoint, auto-scrolling and updating
  the last-line preview. On connection error, the frontend reconnects
  with exponential backoff (500ms base, 30s max, 50–100% jitter).
  On reconnect, `pre.innerHTML` is cleared because the server replays
  the entire file. If the run finished while disconnected (checked via
  `data-run-status` on `#run-page`), a one-shot static fetch is used
  instead. For finished runs, a simple fetch loads the full content on
  expand. Skipped streams show in gray.
- `GET /deployments` — deployment history
- `GET /output/{runID}/{jobName}` — redirects to single stream or
  lists available streams for multi-stream jobs
- `GET /output/{runID}/{jobName}/{stream}` — stream log content.
  Without `?stream=1`, returns current file content. With `?stream=1`,
  returns current content then live-tails new data until the job
  finishes.

## Database

SQLite with WAL mode. Tables: `runs`, `jobs`, `streams`,
`deployments`. See
[migrations/](../apps/ci/migrations/).

The `streams` table stores per-stream metadata (status, duration,
error) with a unique constraint on (job_id, name). The former
`deploy_jobs` and `terraform_jobs` tables have been dropped —
deploy metadata is now sent as part of the FinishRun payload.

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
- `stream.<job>.<stream>.status`, `stream.<job>.<stream>.duration_ms`
  — per stream from DB
- `deploy.<app>.image_ref`, `deploy.<app>.compile_ms`,
  `deploy.<app>.push_ms`, `deploy.<app>.deploy_ms`,
  `deploy.<app>.image_bytes` — from FinishRun request payload

The run detail page displays this event as a key-value table in the
"Task Event" section, fetched from the logs service with
`q=group:app,app:ci,msg:task` and filtered by `run.id`.

## SMS on Completion

Uses `tailnet.Client()` to POST to `monks-sms-brigid` on both success
and failure:
- Failure: `CI run {id} failed: {error}`
- Success: `CI run {id} succeeded`

## Builder Pipeline

1. Tailnet is already up (kernel tailscale started by entrypoint script)
2. Clone or fetch repo onto persistent volume at `/data/repo`
   (`jj git clone --colocate` on first run, `jj git fetch` on
   subsequent runs, then `jj new` the head SHA)
3. Get base SHA from orchestrator
4. Diff changed files
5. Run generate + test (per-task output streams via run library,
   per-task status via `runner.TaskStatus()`)
6. Single "deploy" job:
   - "analysis" stream: change detection, reports affected/skipped apps
   - Then concurrently: per-app deploy streams (affected only; deploy
     metadata accumulated for task event), image rebuilds if needed,
     "publish" stream, "terraform" stream
   Errors are collected (not fail-fast) and joined.
7. Report run complete (sends accumulated deploy metadata)
10. Exit → machine self-destructs

## Dependencies

- `pkg/flyapi` — create builder machines
- `pkg/ci/changedetect` — change detection
- `pkg/ci/publish` — mirror publishing (accepts io.Writer for output)
- `pkg/oci` — OCI image building
- `pkg/database` — SQLite
- `pkg/serve` — HTTP mux
- `pkg/tailnet` — tailnet membership (orchestrator only; builder uses kernel tailscale)
- `pkg/reqlog` — structured logging
- `github.com/amonks/run` — task runner (programmatic API for test jobs)
