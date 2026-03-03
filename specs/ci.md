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
build deps. Persistent volume for caches. Joins tailnet. Streams
output to orchestrator in real time.

## Trigger

POST `/trigger` with `{"sha":"abc123"}`. Not publicly accessible —
only reachable over tailnet. Called by a minimal GitHub Actions
workflow that joins the tailnet and curls the endpoint.

Behavior:
- Rejects if a run is already in progress (returns 409)
- Looks up base SHA from last successful run (all-zeros if none)
- Creates run row in SQLite
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

## Dashboard

- `GET /` — recent runs, current deployments per app
- `GET /runs/{id}` — jobs for this run with inline error messages,
  output links, and a "Mark Dead" button for running runs
- `GET /deployments` — deployment history
- `GET /output/{runID}/{jobName}` — redirects to single stream or
  lists available streams for multi-stream jobs
- `GET /output/{runID}/{jobName}/{stream}` — raw stream log file

## Database

SQLite with WAL mode. Tables: `runs`, `jobs`, `deploy_jobs`,
`terraform_jobs`, `deployments`. See
[migrations/001_initial.sql](../apps/ci/migrations/001_initial.sql).

## SMS on Failure

Uses `tailnet.Client()` to POST to
`http://monks-sms-brigid/?message=CI+run+{id}+failed:+{error}`

## Builder Pipeline

1. Join tailnet
2. Fetch latest code (streams output)
3. Get base SHA from orchestrator
4. Diff changed files
5. Run generate + test (per-task output streams via run library)
6. Deploy affected apps (streams compile/push/deploy progress)
7. Publish subtrees (streams output)
8. Terraform apply (streams output)
9. Report run complete
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
