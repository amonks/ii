# Incrementum

Incrementum is a command-line todo tracker for Jujutsu repositories. Its todos
live in an orphan jj change rather than in a database, so they travel with the
repo and are shared across every workspace of it without touching code history.

The entrypoint is the `ii` command; every subcommand is a todo verb.

```
ii create --title 'Fix the login bug' --type bug
ii list
ii start <id>
ii finish <id>
```

## Core concepts

- **Todo**: a task record with a title, description, type, priority, status,
  and dependencies.
- **Todo store**: the orphan jj change holding those records, locked across
  processes so concurrent `ii` invocations cannot interleave writes.

## Repository layout

- `.`: CLI entrypoint and subcommands.
- `todo/`: todo storage and operations.
- `internal/`: shared internal helpers.

## Development

- Specs live in [`specs/ii/`](../../specs/ii/) and describe intended behavior.
- Run `go tool run test` to execute the test suite. See `tasks.toml` for
  individual tasks.
