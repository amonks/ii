# Prompt Cache Hierarchy Refactor

## Goal

Restructure prompt assembly so content is ordered most-stable-first with cache
breakpoints between tiers. The big win: tiers 1+2 are identical across
implement and review sessions within the same project, so the review session
cache-hits on the system prefix written during the preceding implement session.

## Target order

| Tier | Stability | Content | Breakpoint |
|------|-----------|---------|------------|
| 1 | Global (built into incrementum) | Role description, usage guidelines | Yes |
| 2 | Per-project (defaults built-in, some overridable) | Workflow context, review questions, review instructions, AGENTS.md chain, test commands | Yes |
| — | Tools field | bash, read, write, edit, task schemas | (auto) |
| 3 | Per-session | workDir, date, phase description + phase-specific instructions | No |
| 4 | Per-iteration (user message) | Todo block, series log, feedback, commit message | Yes (last user msg) |

Notes:

- Review instructions are built-in and not overridable, but are included in
  tier 2 for all sessions (even implementation) so the prefix is shared across
  phases. The token cost is ~10 lines.
- Tier 2 includes both overridable templates (workflow context, review
  questions) and filesystem content (AGENTS.md). All are per-project stable.

## Implementation steps

### 1. New `SystemBlock` type in `internal/llm`

Add `SystemBlock { Text string; CacheBreakpoint bool }` and change
`Request.SystemPrompt string` to `Request.System []SystemBlock`.

### 2. Update Anthropic provider

Each `SystemBlock` maps to one `anthropicContent` in the system array.
`applyAnthropicCaching` places `cache_control` on blocks where
`CacheBreakpoint: true`, plus the existing breakpoints on last tool and last
user message.

### 3. Update OpenAI provider

Concatenate all `SystemBlock.Text` into a single system message (OpenAI
caching is automatic).

### 4. Remove duplicate tool descriptions from system prompt

The current `BuildSystemPrompt` has ~60 lines of tool parameter docs that
duplicate the `tools` field. Remove them; the model gets parameters from the
tool schemas.

### 5. New structured prompt input for agent

Replace `Run(ctx, prompt string, config)` with structured prompt content. The
agent owns system block assembly (it knows about tiers and breakpoints). The
job provides content for each tier:

```go
type PromptContent struct {
    ProjectContext []string // rendered templates: workflow context, review questions, review instructions
    ContextFiles   []string // AGENTS.md chain
    TestCommands   []string // from config
    PhaseContent   string   // phase description + phase-specific instructions
    UserContent    string   // todo block, series log, feedback, commit message
}
```

### 6. Extract stable content from phase templates

Currently phase templates inline `{{template "workflow_context"}}` and
`{{template "review_questions"}}`. The job system renders these separately and
passes them as `ProjectContext`. Phase templates shrink to just the
phase-specific instructions (the part after the `{{template}}` calls).

### 7. Move AGENTS.md from user message to system blocks

`agentsPrelude()` currently prepends to the first user message. It moves to a
tier-2 system block via `ContextFiles`.

### 8. Agent assembles tiered system blocks

New function replaces `BuildSystemPrompt`:

```go
func BuildSystemBlocks(workDir string, content PromptContent) []llm.SystemBlock
```

Returns blocks in tier order:

- Tier 1: role + guidelines (breakpoint: true)
- Tier 2: project context + context files + test commands (breakpoint: true)
- Tier 3: workDir + date + phase content (breakpoint: false)

### 9. Update job system prompt passing

`AgentRunOptions.Prompt string` is replaced or supplemented with structured
content. The job system loads and renders tier-2 templates separately from the
phase template, passing each to the agent.

## Spec changes required

### specs/llm.md

- `Request` type: `SystemPrompt string` -> `System []SystemBlock`
- New `SystemBlock` type definition
- Caching Behavior section: describe tiered breakpoints placed on system blocks
  with `CacheBreakpoint: true`, in addition to existing breakpoints on last
  tool and last user message
- Anthropic section: each `SystemBlock` maps to one `anthropicContent`; blocks
  with `CacheBreakpoint: true` get `cache_control`
- OpenAI section: system blocks concatenated into single system message

### specs/internal-llm.md

- Note that provider conversion now handles `[]SystemBlock` instead of a single
  string

### specs/agent.md

- System Prompt section: rewrite. Replace
  `BuildSystemPrompt(workDir string) string` with
  `BuildSystemBlocks(workDir string, content PromptContent) []llm.SystemBlock`.
  Document the tier structure and what goes in each tier.
- Remove "Available tools and their parameter schemas" from the system prompt
  contents list (tool docs removed; schemas are in the tools field only)
- AGENTS.md section: content moves from "prepended to first user message" to
  "included in tier-2 system block"
- `Run()` API: `prompt string` parameter replaced with `PromptContent` (or
  `AgentConfig` gains structured prompt fields)
- Internal Package API section: update `Run` signature

### specs/internal-agent.md

- Update `Run` signature to match

### specs/job.md

- Templates section: phase templates no longer inline
  `{{template "workflow_context"}}` or `{{template "review_questions"}}`. Those
  are loaded separately and passed to the agent as project-level content.
  Document which templates are tier-2 (rendered once, shared across phases) vs
  tier-3 (per-phase).
- `AgentRunOptions` type: document structured prompt content fields replacing or
  supplementing `Prompt string`
- Note: `review-instructions.tmpl` moves from "included only in review
  templates" to "always included in tier-2 system blocks for all phases" (for
  cross-session cache sharing)

## Files changed

- `internal/llm/types.go` -- `Request`, new `SystemBlock`
- `internal/llm/anthropic.go` -- system block conversion, breakpoint placement
- `internal/llm/openai.go` -- system block concatenation
- `internal/agent/prompt.go` -- rewrite: `BuildSystemBlocks`
- `internal/agent/run.go` -- request assembly, `Run` signature
- `internal/agent/agents.go` -- AGENTS.md moves to system blocks
- `internal/agent/tools.go` -- no change (tools stay the same)
- `job/prompts.go` -- split template rendering: tier-2 vs phase-specific
- `job/runner.go` -- pass structured content to agent
- `job/templates/*.tmpl` -- extract `{{template}}` calls from phase templates
- `specs/llm.md`, `specs/internal-llm.md`, `specs/agent.md`,
  `specs/internal-agent.md`, `specs/job.md`
- Tests for all of the above

## Not in scope

- Compaction (separate, larger effort)
- Subagent tool set unification (independent improvement)
- Cache hit rate monitoring
