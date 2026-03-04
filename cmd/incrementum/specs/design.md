# Design Todos

## Overview

Design todos are work items that produce specifications, documentation, or other
design artifacts. They run interactively rather than headless, allowing for
collaborative exploration with the agent.

## Todo Type

Design is a todo type alongside `task`, `bug`, and `feature`:

```
type: task | bug | feature | design
```

## Lifecycle

Design todos follow the same lifecycle as regular todos:

- `open` → `in_progress` → `done`
- Can be `waiting` when blocked on external factors
- Can be `proposed` when created by agents

If an error occurs after the todo is marked `in_progress`, we attempt to revert
it to `open`. This is best-effort: if the reopen itself fails (e.g., due to the
same underlying issue that caused the original error), both errors are returned.
Error paths that trigger reopening include:
- Store release failures after marking the todo started
- Interactive session errors
- Non-zero exit codes from the agent

This matches headless job behavior and prevents todos from getting stuck in
`in_progress` with no active job.

## Job Integration

When `ii job do` runs a todo with `type: design`:

1. Mark the todo `in_progress`
2. Launch an interactive agent session instead of headless
3. The user collaborates with the agent to produce the design
4. On successful completion (exit code 0), the todo is marked `done`
5. On failure (non-zero exit or error), the todo is reopened to `open`

Design todos are excluded from `ii job do-all` since they require interaction.

## Use Cases

- Writing new specifications
- Exploring solution approaches before implementation
- Architectural decision records
- API design sessions

## Differences from Regular Todos

| Aspect | Regular Todo | Design Todo |
| ------ | ------------ | ----------- |
| Execution mode | Headless | Interactive |
| Included in do-all | Yes | No |
| Output | Code commits | Specs, docs, decisions |
