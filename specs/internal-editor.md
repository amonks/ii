# Internal Editor

## Overview
The editor package wraps `$EDITOR` and provides an interactive flow for editing todos.

## Editor Helpers
- `IsInteractive` reports whether stdin is a terminal.
- `Edit` launches `$EDITOR` (defaults to `vi`) and waits for exit.

## Todo Editing
- `TodoData` models the fields used in the editable TOML template.
- `RenderTodoTOML` renders a TOML header and description body separated by `---`.
- `ParseTodoTOML` validates the TOML output and normalizes type/status fields.
- `EditTodo` and `EditTodoWithData` create a temp file, launch the editor, and parse the result.
- `EditTodoWithDataRetry` accepts a `todo.Prompter` and loops on validation errors, prompting the user to re-edit if parsing fails. This prevents losing work when the edited content is invalid. When prompter is nil (non-interactive), validation errors return immediately without preserving the temp file. When the prompter returns an error (e.g., EOF), the temp file is preserved and its path is printed for recovery, and the returned error wraps the prompt error. If the editor itself fails to launch or exits with a non-zero status, the temp file is deleted and the error is returned immediately (no retry prompt); this is reasonable because either the user never saw the content, or they explicitly aborted the edit.
- `ParsedTodo` converts into `todo.CreateOptions` or `todo.UpdateOptions` for persistence.
- The todo template always includes a `status` field; create defaults to `open` unless overridden by the caller.

