package job

import (
	"testing"

	"github.com/amonks/incrementum/todo"
)

func TestCommitMessageSnapshot(t *testing.T) {
	item := todo.Todo{
		ID:          "todo-57uzut5r",
		Title:       "Snapshot-test text formatting",
		Description: "Add snapshot tests for commit message formatting and ensure markdown blocks wrap cleanly.",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityHigh,
	}

	message := "feat: snapshot long text\n\n" +
		"This adds snapshot coverage for prompt rendering and log snapshots. " +
		"It includes long paragraphs that should wrap cleanly and a list:\n\n" +
		"- First bullet is long enough to wrap and includes inline `code` to verify rendering.\n" +
		"- Second bullet continues with more words to trigger wrapping within the configured width.\n\n" +
		"```bash\n" +
		"go test ./... -run Snapshot\n" +
		"```\n"

	formatted := formatCommitMessage(item, message, "")
	requireSnapshot(t, "commit-message.txt", formatted)
}
