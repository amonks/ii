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

## Job Integration

When `ii job do` runs a todo with `type: design`:

1. Launch an interactive agent session instead of headless
2. The user collaborates with the agent to produce the design
3. On completion, the todo is marked `done`

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
