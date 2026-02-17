# Incrementum

Incrementum is a CLI for managing focused work with todos, sessions, and
Jujutsu-backed workspaces. The main entrypoint is the `ii` command, which wraps
the public Go packages defined in this repo.

## Core concepts

- Workspace: a pooled, isolated working copy managed via Jujutsu.
- Todo: a task record stored in a dedicated branch for tracking.
- Session: a unit of work that acquires a workspace to complete one todo.
- Job: orchestration for running LLM agent work on todos.

## Repository layout

- `cmd/ii`: CLI entrypoint and subcommands.
- `workspace/`: workspace pool implementation.
- `todo/`: todo storage and operations.
- `session/`: session lifecycle and helpers.
- `internal/`: shared internal helpers and infrastructure.
- `specs/`: behavioral specifications for each package.

## Development

- Specs live in `specs/README.md` and describe intended behavior.
- Run `go tool run test` to execute the test suite (unit/integration tests and
  `go fix` conformance). See `tasks.toml` for individual tasks.
