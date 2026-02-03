# Events

This spec describes the event stream and rendering rules for agent and job logs.

## Sources

- Agent events are emitted as typed events by the `agent` package and recorded as JSONL.
- Job events are recorded as JSONL entries under the job events directory.

## Agent Events

The agent package emits structured events during execution. See `specs/agent.md`
for the full list of event types. Agent events are rendered with:

- Tool summaries showing tool name and key arguments (bash command, file path)
- Message content showing thinking blocks and assistant responses
- Agent start/end showing model and token usage

## Legacy Event Format

Job logs from before the agent migration contain events in an older format
(originally from the opencode tool). The event renderer in
`job/opencode_event_renderer.go` parses and displays these legacy events for
backward compatibility with existing job log files.

### Legacy Event Types

Legacy events use SSE-style JSON payloads with a `type` field:

- `server.connected`
- `server.heartbeat`
- `session.created`
- `session.updated`
- `session.status`
- `session.idle`
- `session.diff`
- `message.updated`
- `message.part.updated`
- `file.edited`
- `file.watcher.updated`
- `lsp.updated`
- `lsp.client.diagnostics`
- `todo.updated`

### Rendering switches

Each legacy event type has a display switch (see `job/opencode_event_renderer.go`).
Default behavior:

- `message.part.updated`: enabled (drives prompt/response/thinking/tool summaries)
- all other listed event types: disabled

Switches control what is shown to users; all events are still recorded in full on disk.

## Text rendering (width-aware)

Only a curated subset of activity is shown in the text logs (CLI/TUI). Output is
formatted to the standard line width and indented like other job log entries.

- Tool calls: one-line summaries with start/end markers. Tool start is emitted when the tool begins; tool end is emitted when the state reaches a terminal status (completed, failed, error, cancelled).
  - Example: `Tool start: read file 'src/file.ts'` and `Tool end: read file 'src/file.ts'` (paths are repo-relative when possible).
  - Failed tools show the status: `Tool end: read file '/missing.txt' (failed)`
  - For `apply_patch` tools, file paths are extracted from the unified diff and shown in the summary.
  - For `bash` tools with empty command input, no log is emitted (the command arrives in a subsequent event).
- Prompt text: emitted for user messages.
  - Label: `LLM prompt:`
- Assistant responses: emitted when an assistant message completes.
  - Label: `LLM response:`
- Assistant thinking: emitted when an assistant message completes and a reasoning part has non-empty text.
  - Label: `LLM thinking:`
- Prompt, response, and thinking bodies are rendered as markdown via `internal/markdown` (glamour) before indentation.

## Raw event display

Raw JSON payloads are not rendered by default. If an event payload cannot be
decoded into a known shape, it falls back to a generic "LLM event" block to
avoid hiding malformed data in logs.
