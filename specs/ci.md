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
serves dashboard, sends SMS on failure.

**monks-ci-builder** (builder): Ephemeral Fly machine
(`performance-4x`, 4GB, `auto_destroy: true`). Fat image with all
build deps. Persistent volume for caches. Joins tailnet. Reports
progress to orchestrator.

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

- `PUT /api/runs/{id}/jobs/{name}/start` — mark job in_progress
- `PUT /api/runs/{id}/jobs/{name}/done` — store result, duration,
  kind-specific data (deploy details, terraform resource counts)
- `PUT /api/runs/{id}/done` — mark run complete, SMS on failure
- `GET /api/runs/{id}/base-sha` — return base SHA for this run
- `POST /api/runs/{id}/deployments` — record deployment

## Dashboard

- `GET /` — recent runs, current deployments per app
- `GET /runs/{id}` — jobs for this run, output links
- `GET /deployments` — deployment history
- `GET /output/{runID}/{jobName}` — raw output file

## Database

SQLite with WAL mode. Tables: `runs`, `jobs`, `deploy_jobs`,
`terraform_jobs`, `deployments`. See
[migrations/001_initial.sql](../apps/ci/migrations/001_initial.sql).

## SMS on Failure

Uses `tailnet.Client()` to POST to
`http://monks-sms-brigid/?message=CI+run+{id}+failed:+{error}`

## Builder Pipeline

1. Join tailnet
2. Fetch latest code
3. Get base SHA from orchestrator
4. Diff changed files
5. Run generate + test (via run library)
6. Deploy affected apps (depgraph → build → OCI → push → deploy)
7. Publish subtrees
8. Terraform apply
9. Report run complete
10. Exit → machine self-destructs

## Dependencies

- `pkg/flyapi` — create builder machines
- `pkg/ci/changedetect` — change detection
- `pkg/ci/publish` — mirror publishing
- `pkg/oci` — OCI image building
- `pkg/database` — SQLite
- `pkg/serve` — HTTP mux
- `pkg/tailnet` — tailnet membership
- `pkg/reqlog` — structured logging
