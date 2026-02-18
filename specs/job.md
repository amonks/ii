# Job Subcommand

## Overview

The job package and subcommand automate todo completion via LLM-driven agents. A job
runs from the current working directory and executes a work loop: the agent is
asked to complete the next highest-priority step and write a commit message,
tests run, the agent reviews the change, and the result is committed. The loop
continues until the agent makes no changes, then the job skips tests and moves
directly to the final project review before completing. Jobs retry on test
failure or review rejection until the agent decides to abandon.

Jobs emit a merged stream of agent and job events to the JSONL event log and
optionally to a caller-provided Go channel via `RunOptions.EventStream`, which
is closed when the run completes.

Jobs run from the repo root by default but can optionally use a separate
workspace path.

## Architecture

The job implementation lives in a Go package with clean exports; the `cmd/ii`
subcommand stays a thin wrapper that wires flags and delegates to the package.

## Testing

Follow our usual testing practice:

- Prefer lots of focused unit tests in `job/`.

## Storage

- Job state stored in `~/.local/state/incrementum/state.json` alongside other
  state.
- Jobs are scoped per repo using the same repo slug as other state.
- Jobs do not create sessions or workspaces.
- Job records track LLM sessions created during the job.
- Job event logs are stored as JSONL at
  `~/.local/share/incrementum/jobs/events/<job-id>.jsonl`.
- Job event entries use a unified event shape (`id`, `name`, `data`) and include
  both agent events and job-specific events (stage changes, prompts, transcripts,
  test results, review feedback, commit messages, session boundaries, errors).

## Job Model

Fields (JSON keys):

- `id`: 8-character job id (hash of todo_id + timestamp).
- `repo`: repo slug.
- `todo_id`: full resolved todo id.
- `agent`: model name (empty string when unset).
- `stage`: `implementing`, `testing`, `reviewing`, `committing`.
- `feedback`: feedback from last failed stage (test results list or review
  feedback).
- `agent_sessions`: list of `{"purpose": string, "id": string}` tracking
  LLM sessions created during this job.
- `changes`: list of changes created during this job (see
  [job-changes.md](./job-changes.md)).
- `project_review`: final project review outcome (see
  [job-changes.md](./job-changes.md)).
- `status`: `active`, `completed`, `failed`, `abandoned`.
- `created_at`: timestamp.
- `started_at`: timestamp.
- `updated_at`: timestamp.
- `completed_at`: timestamp.

## Model Selection

- The model is resolved in this order: `RunOptions.Model` override (API only) ->
  todo-level model for the stage -> config stage model -> config default model.
- Todo-level fields map to stages: `implementation_model` for implementing,
  `code_review_model` for step review, `project_review_model` for project review.

## Feedback File

The agent communicates review outcomes by writing to `.incrementum-feedback` in the
job workspace root (`WorkspacePath`).

Format:

```
<OUTCOME>

<details>
```

Where `<OUTCOME>` (first line, trimmed) is one of:

- `ACCEPT` - changes look good, proceed. Optionally followed by blank line and
  review comments noting what looks good or any observations. Comments are
  included in the commit message when present.
- `ABANDON` - task is impossible or misguided, give up. Must be followed by blank
  line and reason text explaining why the task is being abandoned.
- `REQUEST_CHANGES` - followed by blank line and feedback text.

If the file doesn't exist after review, treat as `ACCEPT` with no comments.

## Commit Message File

The agent writes the generated commit message to `.incrementum-commit-message` in the
job workspace root (`WorkspacePath`) during the implementing stage. The commit
message should describe the entire working tree diff created in that stage.

## State Machine

```
implementing -> testing -> reviewing -> committing -> implementing
     ^             |            |           |
     |             |            |           +-> (continue work loop)
     |             |            +--------------> implementing (REQUEST_CHANGES)
     |             +--------------------------> implementing (test failure)
     |
     +-> (no changes) -> reviewing -> completed

reviewing -> abandoned (ABANDON)
any stage -> failed (unrecoverable error)
```

### implementing

1. Re-read the todo from the store. This allows edits made from another process
   to be picked up between implementation runs. The re-read todo is used for
   the remainder of the implement→test→review→commit cycle.
2. Best-effort `jj workspace update-stale` in the repo working directory.
3. Delete `.incrementum-feedback` from the workspace root if it exists.
4. Record the current working copy commit id.
5. Run the agent with `prompt-implementation.tmpl` when no feedback is present,
   or `prompt-feedback.tmpl` when responding to feedback (PWD set to the
   workspace root).
6. Before running the agent with feedback, write the previous draft commit
   message (if any) to `.incrementum-commit-message` in the workspace root.
7. Template receives: `Todo`, `Feedback`, and `Message` (previous commit message
   when responding to feedback).
8. Best-effort `jj debug snapshot` in the repo working directory immediately
   before the agent runs.
9. Run agent to completion.
10. Record session in job record with purpose `implement`.
11. If agent returns an error before completion, record a `job.agent.error`
    event with the purpose and error message, then mark the job `failed`.
12. If agent fails (nonzero exit): mark job `failed` with an error that
    includes the exit code, error reason (if available), session id, model, prompt
    template, repo/workspace paths, and before/after commit ids. If the exit code is negative and the working
    copy commit changed, best-effort restore the workspace to the pre-agent
    commit and retry once. If the retry still fails, best-effort restore before
    failing and include the retry attempt in the error details. Context overflow
    errors are handled specially: retry once within the stage invocation, and if
    the retry also fails with context overflow, stay in the implementing stage
    with feedback instead of failing the job. This allows the agent to continue
    from where it left off with a fresh context window; the working tree is
    preserved with any partial progress. Context overflow is detected by
    case-insensitive substring matching for: "context overflow",
    "maximum context length", "context_length_exceeded", "prompt is too long",
    or "request too large".
13. Run `jj log -r @ -T empty --no-graph` to check if the current change has
    uncommitted work. Treat `true` (empty) as no work to commit, `false` (not
    empty) as work to commit. This check uses the `empty` template rather than
    comparing commit IDs because a previous job run may have left uncommitted
    work in `@` if it failed after making changes.
14. If the change is empty:
    - Delete `.incrementum-commit-message` from the workspace root if it exists.
    - Flag the next review cycle as the final project review.
15. If the change is not empty:
    - Read `.incrementum-commit-message` from the workspace root, trimming trailing
      newlines, trailing whitespace on each line, and any leading blank lines.
    - If the file is missing, set feedback instructing the agent to write a commit
      message describing the changes, stay in `implementing` stage, and restart
      the work loop. This handles cases where the LLM made changes but forgot to
      write the commit message file.
    - Store the message for the committing stage.
16. Transition to `testing` when changes were detected (change not empty) and
    commit message was provided, stay in `implementing` when commit message was
    missing, otherwise transition to `reviewing`.

### testing

1. Run each test command from config sequentially (only when changes were
   detected in the implementing stage).
2. Capture combined stdout/stderr output and exit code for each command.
3. Store the command, exit code, and output in the job test event log.
4. If any command fails (nonzero exit):
   - Build feedback as a markdown list with one entry per test command, using
     `- <command> is passing` or `- <command> is failing`.
   - Transition to `implementing`.
5. If all pass: transition to `reviewing`.
6. If the job was in final project review when tests failed, the next implementing
   stage restarts the work loop.

### reviewing

1. Best-effort `jj workspace update-stale` in the repo working directory.
2. Delete `.incrementum-feedback` from the workspace root if it exists.
3. Best-effort `jj debug snapshot` in the repo working directory immediately
   before the agent runs.
4. Run the agent with:
   - `prompt-commit-review.tmpl` during the work loop, or
   - `prompt-project-review.tmpl` during the final project review.
5. Template receives: `Todo`, `Message` (commit message from the implementing stage).
   If the review template does not reference `Message` or `CommitMessageBlock`,
   the job appends a `Commit message` block with heading-and-indent formatting
   before rendering.
   - If the commit message is required for the step review and missing, fail with
     a descriptive error that calls out the implementation prompt and expected
     `.incrementum-commit-message` location.
6. Template instructs the agent to inspect changes and write outcome to
   `.incrementum-feedback`.
7. Run agent to completion.
8. Record session in job record with purpose `review` or `project-review`.
9. If agent returns an error before completion, record a `job.agent.error`
   event with the purpose and error message, then mark the job `failed`.
10. If agent fails (nonzero exit): mark job `failed` with an error that
    includes the exit code, error reason (if available), session id, and model.
11. Read `.incrementum-feedback` from the workspace root:
   - Delete `.incrementum-feedback` after reading.
   - Missing or first line is `ACCEPT`:
     - During the work loop: transition to `committing`.
     - During project review: mark job `completed`.
   - First line is `ABANDON`: extract reason (lines after first blank line),
     mark job `abandoned`, and return an error with the reason attached.
   - First line is `REQUEST_CHANGES`: extract feedback (lines after first blank
     line), transition to `implementing` and restart the work loop if needed.
   - Other first line: treat as invalid format, mark job `failed`.

### committing

1. Best-effort `jj workspace update-stale` in the repo working directory.
2. If the working copy diff (`jj diff --stat --from @- --to @`) is empty, skip
   committing and transition back to `implementing` (the next loop will detect
   no changes and move to project review). An output with no file stat lines or
   non-zero summary counts as empty.
3. Format final message with a fixed commit message layout (not templated). The
   format uses the agent-generated summary/body plus a todo block, reflowed via
   the markdown renderer to 80/76/72 columns with 0/4/8-space indentation. Todo
   descriptions are rendered via the markdown renderer to preserve lists and code
   blocks.
4. Normalize the formatted message by trimming leading blank lines and trailing
   whitespace on each line. Left-trim the first non-blank line so the summary
   line starts at column 0 even if the markdown renderer indents paragraphs.
5. Best-effort `jj workspace update-stale` in the repo working directory.
6. Run `jj commit -m "<formatted message>"` in the repo working directory.
7. If commit fails: mark job `failed`.
8. Transition back to `implementing` to continue the work loop.

Commit message format:

```
<summary line>

Here is a generated commit message:

    <reflowed body>

Review comments:

    <review comments if present>

This commit is a step towards implementing this todo:

    ID: <id>
    Title: <title>
    Type: <type>
    Priority: <priority> (<name>)
    Description:
        <markdown-rendered description>
```

The "Review comments" section is only included when the reviewer provided comments
with their ACCEPT verdict.

## Failure Handling

- `failed`: unrecoverable error (commit fails, invalid feedback format).
- `abandoned`: agent decided the task is impossible.

Both reopen the todo.

On interrupt (SIGINT), mark job `failed` and reopen the todo.

### Stale Job Detection

Active jobs that haven't been updated within 10 minutes are considered stale
(orphaned). When `ii job list` runs, it automatically marks stale active jobs
as `failed`. This handles cases where a job process crashed or was killed
without proper cleanup.

## Todo Status Updates

- Before running, mark the todo `in_progress`.
- When a job completes successfully, mark the todo `done`.
- When a job fails or is abandoned, reopen the todo (`open`).

## Config

```toml
[job]
model = "claude-sonnet-4-20250514"
implementation-model = "claude-sonnet-4-20250514"
code-review-model = "claude-sonnet-4-20250514"
project-review-model = "claude-sonnet-4-20250514"
test-commands = [
  "go test ./...",
  "golangci-lint run",
]
```

`test-commands` must be configured with at least one entry; jobs fail in the
testing stage if it is missing or empty.

Config is loaded from `incrementum.toml` or `.incrementum/config.toml` and
`~/.config/incrementum/config.toml`; project values override global values.

Callers can supply a preloaded config via `RunOptions.Config` to avoid
filesystem reads; when set, the job runner does not call `LoadConfig`.

`model` is an optional default model.

`implementation-model`, `code-review-model`, and `project-review-model` override
`model` for their respective stages.

## Templates

Bundled defaults via `//go:embed`, overridable by placing files in
`.incrementum/templates/` unless noted. Use `ii help templates` to print the
default template contents, override paths, and variable types for prompt
templates.

| File                             | Stage        | Mode   |
| -------------------------------- | ------------ | ------ |
| `prompt-implementation.tmpl`     | implementing | todo   |
| `prompt-feedback.tmpl`           | implementing | both   |
| `prompt-commit-review.tmpl`      | reviewing    | todo   |
| `prompt-project-review.tmpl`     | reviewing    | todo   |
| `prompt-habit-implementation.tmpl` | implementing | habit  |
| `prompt-habit-review.tmpl`       | reviewing    | habit  |

Templates use Go `text/template` syntax (commit messages are generated in code).

All prompt templates receive the same data:

- `Todo` (`todo.Todo`): `ID`, `Title`, `Description`, `Type`, `Priority`, `Status`,
  `CreatedAt`, `UpdatedAt`, `ClosedAt`, `DeletedAt`, `DeleteReason`.
- `Feedback` (`string`)
- `Message` (`string`)
- `AgentTranscripts` (`[]AgentTranscript`): list of transcripts from prior agent
  sessions in this job.
- `WorkspacePath` (`string`): absolute path to the job's workspace root.
- `ReviewInstructions` (`string`): standard review output instructions block.
- `TodoBlock` (`string`): formatted heading-and-indent block that includes ID, title,
  type, priority, and description; each field is on its own indented line and the
  description text is reflowed and indented one level deeper.
- `FeedbackBlock` (`string`): formatted heading-and-indent block for the feedback text.
- `CommitMessageBlock` (`string`): formatted heading-and-indent block for the commit
  message text.
- `SeriesLogBlock` (`string`): formatted block showing the jj log for the patch series
  (commits from `fork_point(@|main)` to `@-`) with descriptions and diff stats. Empty when there
  are no commits in the series. Included in `prompt-implementation.tmpl` and
  `prompt-feedback.tmpl` to give the agent context about work already completed.
- `HabitName` (`string`): name of the habit (filename without extension). Empty for
  regular todo jobs.
- `HabitInstructions` (`string`): full text of the habit instruction document,
  formatted as an indented block. Empty for regular todo jobs.

Shared templates:

- `review-questions.tmpl`: defines `review_questions`, the default review
  question list. Overrides live at `.incrementum/templates/review-questions.tmpl`.
- `review-instructions.tmpl`: embedded review output instructions block. This is
  part of the internal API and is not overrideable.

## Commands

### `ii job do [todo-id... | creation-flags | --habit [name]]`

Create and run a job to completion (blocking).

- If one or more todo-ids provided: run each existing todo in sequence.
- If creation flags provided: create todo first (same flags as `ii todo create`:
  `--title`, `--type`, `--priority`, `--description/--desc`, `--deps`,
  `--edit/--no-edit`).
- `--habit <name>` runs the named habit from `.incrementum/habits/<name>.md`.
  Accepts habit name or unique prefix.
- `--habit` (no name) runs the alphabetically first habit.
- `--habit` cannot be combined with todo-ids or todo creation flags.
- If no args and interactive: open $EDITOR to create todo.
- If `--rev` is omitted, default to `trunk()`.

#### Multi-Todo Queueing

When multiple todo IDs are specified (e.g., `ii job do abc def ghi`), the CLI
marks all specified todos as `queued` before starting work. This shows in the
todo list that the work is accounted for. As each todo is processed, it
transitions from `queued` to `in_progress` to `done`.

If the job exits before processing all queued todos (due to error, interrupt, or
early completion), any todos that were never started are reset to `open`. The
currently-running todo that errored is left in its current state (likely
`in_progress`) since it may have legitimately transitioned before failing.
This ensures no todos are left in a stale `queued` state while preserving
accurate state for todos that were actually worked on.

Note: This queueing behavior only applies to explicit multi-todo `job do`
invocations. `job do-all` does not use queueing because it dynamically selects
the next todo from the ready list after each job completes.

Behavior:

1. Resolve or create todo(s).
2. Release the todo store workspace once the todo is loaded.
3. Mark the todo `in_progress`.
4. Run the job from the workspace root (no session/workspace or new change is created).
5. Output job context: workdir and full todo details.
6. Create job record with status `active`, stage `implementing`.
7. Run state machine to completion.
8. Output progress: stage transitions and formatted logs (agent event stream
   entries labeled and indented, tool start/end entries surfaced separately,
   prompts and commit messages rendered via the markdown renderer, test
   results, review feedback) with 80-column wrapping where formatting applies
   and 0/4/8-space indentation for document hierarchy.
9. On success: mark todo done and print final commit info with 80-column
   wrapping and 0/4/8-space indentation (todo descriptions are
   markdown-rendered).
10. On failure/abandon: reopen todo and print reason. For abandoned jobs, print
    the abandon reason with the same 80-column wrapping and indentation used for
    commit messages.

Exit codes:

- 0: completed.
- 1: failed or abandoned.

#### Habit Workflow

When `--habit` is provided, the workflow differs from regular todos:

1. Load instructions from `.incrementum/habits/<name>.md`.
2. Run implementation stage with `prompt-habit-implementation.tmpl` (or
   `prompt-feedback.tmpl` when responding to feedback).
3. Run testing stage (same as regular todos).
4. Run step review with `prompt-habit-review.tmpl`.
5. On ACCEPT: commit and create an artifact todo with `source: habit:<name>`.
6. On REQUEST_CHANGES: loop back to implementation with feedback.
7. On ABANDON: job completes successfully with no artifact (nothing worth doing
   right now is a valid outcome for habits).

Habits skip the project review stage. The commit message includes the full habit
instructions text.

### `ii job do-all [--priority <n>] [--type <type>]`

Run jobs for all ready todos that match the provided filters.

- `--priority` filters by maximum priority; `--priority=1` includes priority 0
  and 1 todos (priority 0 first).
- `--type` filters by exact todo type (`task`, `bug`, `feature`).

Behavior:

1. Read the ready todo list (open, unblocked), sorted by priority.
2. Select the first todo matching the filters.
3. Run `ii job do` for that todo.
4. Repeat from step 1 until no matching todos remain.
5. Print `nothing left to do` when the run finishes without a match.

### `ii job list [--status <s>] [--all] [--json]`

List jobs for current repo.

- Default: active jobs only.
- `--status`: filter by status (case-insensitive).
- `--all`: show all statuses.
- `--json`: structured output.

Columns: `JOB`, `TODO`, `STAGE`, `STATUS`, `IMPL`, `REVIEW`, `PROJECT`, `AGE`, `DURATION`, `TITLE`.

`IMPL`, `REVIEW`, and `PROJECT` show the models used for implementation,
commit review, and project review.

`AGE` uses `now - created_at`.

`DURATION` uses `now - created_at` for active jobs, otherwise
`updated_at - created_at`.

`JOB` highlights the shortest unique prefix across all jobs in the repo.

`TODO` uses the same prefix highlighting as other todo output.

`SESSION` uses the shortest unique prefix across job session IDs in the repo.

When list is empty but jobs exist, print hint about `--all`.

### `ii job show <job-id>`

Show detailed job info.

Output includes:

- Job ID, status, stage.
- Todo ID and title.
- Feedback (if any).
- LLM sessions with purposes.

### `ii job logs <job-id>` (alias: `log`)

Show the combined job event stream.

Reads the job's JSONL event log and prints entries in the order they were
recorded, formatting stage transitions and logs with the same 80-column reflow
and 0/4/8-space indentation used during `ii job do` output.
Agent events are rendered with tool summaries (e.g., `Tool start: read file 'path'`)
and message content (thinking/response).

## Agent Event Rendering

The job event log renderer supports agent events (from the `agent` package).
Agent events are identified by their event names (`agent.`, `turn.`, `message.`,
`tool.` prefixes) and rendered with:

- Tool summaries showing tool name and key arguments (bash command, file path)
- Message content showing thinking blocks and assistant responses
- Agent start/end showing model and token usage

Agent tool events (`tool.start`, `tool.end`) emit exactly one "Tool start" log
when a tool begins executing and exactly one "Tool end" log when the tool
completes (with success or failure). This ensures that every start is paired
with an end in the rendered output.

Legacy tool events (from opencode `message.part.updated`) are rendered as
individual status updates without pairing: `Tool (<status>): <summary>`. The
same tool may appear multiple times with different statuses (pending, running,
completed, failed). See `specs/events.md` for details.

## LLM Integration

The job runner uses the `agent` package to run LLM sessions. The key types are:

### Types

```go
type AgentRunOptions struct {
    RepoPath      string
    WorkspacePath string
    Prompt        string
    Model         string
    StartedAt     time.Time
    EventLog      *EventLog
    Env           []string
}

type AgentRunResult struct {
    SessionID     string
    ExitCode      int
    Error         string // Optional: error message when ExitCode is non-zero
    InputTokens   int    // Input tokens consumed
    OutputTokens  int    // Output tokens generated
    TotalTokens   int    // Total tokens consumed
    ContextWindow int    // Model's context window size, for diagnostics
}
```

`ExitCode` is a result code indicating success (0) or failure (non-zero). It is
a synthetic code set to 1 when an error occurs. The term "exit code" is used
for consistency even though the agent does not spawn a subprocess.

`Error` is optional and may be empty even when `ExitCode` is non-zero.

The token usage fields (`InputTokens`, `OutputTokens`, `TotalTokens`) and
`ContextWindow` are populated by the CLI layer from the agent's `RunResult.Usage`
and model configuration. These are logged in `job.agent.end` events for
diagnostics—particularly to identify when agents exit because they approach the
model's context limit.

```go
type AgentSession struct {
    Purpose string // "implement", "review", or "project-review"
    ID      string // Agent session ID
}

type AgentTranscript struct {
    Purpose    string
    Transcript string
}
```

### RunOptions Fields

- `RunLLM`: Callback to run an LLM session. CLI wiring uses the `agent` package.
- `Transcripts`: Retrieves transcripts for sessions. Defaults to using
  `agent.Store.TranscriptSnapshot`.
- `Model`: Overrides model selection for all stages when set.

### Job Events

The job runner emits these job-level events:

- `job.agent.start`: Records the purpose and model when an agent run starts.
- `job.agent.end`: Records the purpose, session ID, exit code, token usage
  (input/output/total tokens), context window size, and error message (if any).
  Rendered in logs showing context utilization (e.g., "Agent implement ended:
  48945/128000 input tokens (38%), 160 output tokens").
- `job.agent.error`: Records the purpose and error message when an agent run fails.
