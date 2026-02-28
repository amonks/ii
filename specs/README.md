# Specifications

## Architecture

### System Principles

- The specifications are the source of truth for system behavior; update them alongside code changes.
- Keep the domain model coherent and favor clear factoring over shortcuts.
- Treat the system as single-version: remove or migrate old behavior instead of preserving backward compatibility.
- Practice test-driven development: write a failing test before implementing the behavior.
- Never mock: tests must exercise real integrations, especially when shelling out to third-party binaries.
- When you spot unrelated issues, capture them as new todos instead of piggybacking on the current change.

### Development Practices

- [testing.md](./testing.md) defines test tiers, testscript usage, and determinism rules.
- [perf.md](./perf.md) tracks benchmarks, profiling commands, and improvements.

### CLI Architecture

- The CLI is structured as a single entry point that dispatches subcommands.
- Subcommands validate inputs before performing state changes.
- Errors are reported consistently, using a human-readable message and a non-zero exit code.
- Commands follow the pattern `ii <noun> <verb> [args] [flags]`.
- The CLI layer is a thin wrapper around public packages; command handlers delegate to package APIs with minimal logic.
- Command interfaces should mirror the public APIs they wrap (1:1 when possible).
- Flag aliases should be handled via normalization so help shows a single entry; prefer `-d` and accept `--desc` for description when needed.

### CLI I/O & Testing

- Machine-readable output is only emitted when explicitly requested (for example via `--json`).
- Table output aligns and truncates by visible characters instead of raw byte length.
- Table output normalizes line breaks and tabs to spaces to keep rows single-line.
- Each package wrapped into the CLI has both regular tests and an end-to-end test that builds and executes the CLI binary.
- CLI e2e testscript suites live under `cmd/ii/testdata` and are executed from `cmd/ii/*_test.go`.
- CLI wrapper unit tests live under `cmd/ii`; go package APIs are tested in their respective module directories.

### No-Mock Test Audit

- 2026-01-24: Audited all `*_test.go` files under `cmd/ii`, `job`, `todo`, `workspace`, and `internal`; tests rely on real integrations or local helpers with no mock binaries.

## Public Packages

| Spec                               | Code                        | Purpose                                                                                                      |
| ---------------------------------- | --------------------------- | ------------------------------------------------------------------------------------------------------------ |
| [workspace.md](./workspace.md)     | [workspace/](../workspace/) | Jujutsu workspace pool: acquire a workspace to do some isolated work, then release it                        |
| [todo.md](./todo.md)               | [todo/](../todo/)           | Task tracking: command-line JIRA with TODOs stored in a special branch                                       |
| [habit.md](./habit.md)             | [habit/](../habit/)         | Habit management: ongoing improvement practices stored as version-controlled instruction documents           |
| [cli.md](./cli.md)                 | [cmd/ii/](../cmd/ii/)       | CLI conventions and behavior notes                                                                             |
| [llm.md](./llm.md)                 | [llm/](../llm/)             | LLM abstraction: unified API for Anthropic, OpenAI completions, and OpenAI responses                         |
| [agent.md](./agent.md)             | [agent/](../agent/)         | Agent loop: autonomous task execution with built-in tools (bash, read, write, edit)                          |
| [job.md](./job.md)                 | [job/](../job/)             | Jobs system: workflow management for LLM-based todo completion, with acceptance testing                      |
| [job-changes.md](./job-changes.md) | [job/](../job/)             | Job change tracking: persistent state for changes, commits, and reviews within a job                         |
| [events.md](./events.md)           | [job/](../job/)             | Event logging and rendering for agent and jobs                                                               |

## Internal Packages

| Spec                                                   | Code                                                | Purpose                                              |
| ------------------------------------------------------ | --------------------------------------------------- | ---------------------------------------------------- |
| [internal-agent.md](./internal-agent.md)               | [internal/agent/](../internal/agent/)               | Core agent loop without persistence                  |
| [internal-age.md](./internal-age.md)                   | [internal/age/](../internal/age/)                   | Timing helpers for computed ages and durations       |
| [internal-config.md](./internal-config.md)             | [internal/config/](../internal/config/)             | Load project/global configuration and run hook scripts      |
| [internal-editor.md](./internal-editor.md)             | [internal/editor/](../internal/editor/)             | `$EDITOR` integration and todo TOML editing flow     |
| [internal-ids.md](./internal-ids.md)                   | [internal/ids/](../internal/ids/)                   | Unique prefix length calculation for IDs             |
| [internal-jj.md](./internal-jj.md)                     | [internal/jj/](../internal/jj/)                     | Go wrapper around jj CLI commands                    |
| [internal-listflags.md](./internal-listflags.md)       | [internal/listflags/](../internal/listflags/)       | Shared Cobra list flags                              |
| [internal-llm.md](./internal-llm.md)                   | [internal/llm/](../internal/llm/)                   | Core LLM abstraction without persistence             |
| [internal-markdown.md](./internal-markdown.md)         | [internal/markdown/](../internal/markdown/)         | Markdown rendering helpers for terminal output       |
| [internal-paths.md](./internal-paths.md)               | [internal/paths/](../internal/paths/)               | Default state and workspace paths                    |
| [internal-db.md](./internal-db.md)                     | [internal/db/](../internal/db/)                     | SQLite state database connection and repo/workspace helpers |
| [internal-state.md](./internal-state.md)               | [internal/state/](../internal/state/)               | Legacy JSON state store for agent/job until fully migrated  |
| [internal-strings.md](./internal-strings.md)           | [internal/strings/](../internal/strings/)           | Shared whitespace normalization helpers              |
| [internal-testsupport.md](./internal-testsupport.md)   | [internal/testsupport/](../internal/testsupport/)   | Integration test helpers for ii/testscript           |
| [internal-ui.md](./internal-ui.md)                     | [internal/ui/](../internal/ui/)                     | CLI formatting helpers for durations and IDs         |
| [internal-validation.md](./internal-validation.md)     | [internal/validation/](../internal/validation/)     | Shared validation formatting helpers                 |
