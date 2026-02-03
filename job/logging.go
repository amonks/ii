package job

import (
	"fmt"
	"io"
	"strings"

	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/charmbracelet/lipgloss"
)

// Logger captures structured job log entries.
type Logger interface {
	Prompt(PromptLog)
	CommitMessage(CommitMessageLog)
	Review(ReviewLog)
	Tests(TestLog)
}

// PromptLog captures prompt details.
type PromptLog struct {
	Purpose    string
	Template   string
	Prompt     string
	Transcript string
}

// CommitMessageLog captures commit message text.
type CommitMessageLog struct {
	Label        string
	Message      string
	Preformatted bool
}

// ReviewLog captures review feedback.
type ReviewLog struct {
	Purpose  string
	Feedback ReviewFeedback
}

// TestLog captures test command results.
type TestLog struct {
	Results []TestCommandResult
}

type noopLogger struct{}

func (noopLogger) Prompt(PromptLog)               {}
func (noopLogger) CommitMessage(CommitMessageLog) {}
func (noopLogger) Review(ReviewLog)               {}
func (noopLogger) Tests(TestLog)                  {}

func resolveLogger(logger Logger) Logger {
	if logger == nil {
		return noopLogger{}
	}
	return logger
}

// ConsoleLogger writes formatted log output.
type ConsoleLogger struct {
	writer      io.Writer
	headerStyle lipgloss.Style
	started     bool
}

// NewConsoleLogger builds a styled logger for interactive output.
func NewConsoleLogger(writer io.Writer) *ConsoleLogger {
	if writer == nil {
		writer = io.Discard
	}
	return &ConsoleLogger{
		writer:      writer,
		headerStyle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33")),
	}
}

// ResetSpacing clears entry spacing so the next log block is adjacent.
func (logger *ConsoleLogger) ResetSpacing() {
	if logger == nil {
		return
	}
	logger.started = false
}

// Prompt logs a prompt entry.
func (logger *ConsoleLogger) Prompt(entry PromptLog) {
	if logger == nil {
		return
	}
	label := promptLabel(entry.Purpose)
	lines := []string{
		formatLogLabel(logger.headerStyle.Render(label), documentIndent),
		formatPromptBody(entry.Prompt, subdocumentIndent),
	}
	if !internalstrings.IsBlank(entry.Transcript) {
		lines = append(lines,
			formatLogLabel(logger.headerStyle.Render("LLM transcript:"), documentIndent),
			formatTranscriptBody(entry.Transcript, subdocumentIndent),
		)
	}
	logger.writeBlock(lines...)
}

// CommitMessage logs a commit message entry.
func (logger *ConsoleLogger) CommitMessage(entry CommitMessageLog) {
	if logger == nil {
		return
	}
	label := commitMessageLabel(entry.Label)
	logger.writeBlock(
		formatLogLabel(logger.headerStyle.Render(label), documentIndent),
		formatCommitMessageBody(entry.Message, subdocumentIndent, entry.Preformatted),
	)
}

// Review logs review feedback.
func (logger *ConsoleLogger) Review(entry ReviewLog) {
	if logger == nil {
		return
	}
	label := reviewLabel(entry.Purpose)
	logger.writeBlock(
		formatLogLabel(logger.headerStyle.Render(label), documentIndent),
		formatLogBody(entry.Feedback.Details, subdocumentIndent, true),
	)
}

// Tests logs test results.
func (logger *ConsoleLogger) Tests(entry TestLog) {
	if logger == nil {
		return
	}
	logger.writeBlock(formatTestLogBody(testResultLogsFromCommandResults(entry.Results)))
}

func (logger *ConsoleLogger) writeBlock(lines ...string) {
	if len(lines) == 0 {
		return
	}
	if logger.started {
		fmt.Fprintln(logger.writer)
	}
	logger.started = true
	for _, line := range lines {
		fmt.Fprintln(logger.writer, line)
	}
}

func normalizeLogBody(value string) string {
	value, empty := trimLogBody(value)
	if empty {
		return "-"
	}
	return value
}

func trimLogBody(value string) (string, bool) {
	value = internalstrings.TrimTrailingNewlines(value)
	return value, internalstrings.IsBlank(value)
}

func isDashBody(body string) bool {
	return internalstrings.TrimSpace(body) == "-"
}

func formatLogLabel(label string, indent int) string {
	if internalstrings.IsBlank(label) {
		return ""
	}
	return IndentBlock(label, indent)
}

func renderMarkdownBlockOrDash(body string, indent int, renderWidth int) string {
	rendered := RenderMarkdown(body, renderWidth)
	if internalstrings.IsBlank(rendered) {
		return IndentBlock("-", indent)
	}
	return IndentBlock(rendered, indent)
}

func formatLogBody(body string, indent int, wrap bool) string {
	body = normalizeLogBody(body)
	if wrap {
		if isDashBody(body) {
			return IndentBlock(body, indent)
		}
		return renderMarkdownBlockOrDash(body, indent, wrapWidthFor(lineWidth, indent))
	}
	return IndentBlock(body, indent)
}

func formatTranscriptBody(body string, indent int) string {
	return formatLogBody(body, indent, false)
}

func formatCommitMessageBody(body string, indent int, preformatted bool) string {
	if preformatted {
		return formatLogBody(body, indent, false)
	}
	return formatMarkdownBlock(body, indent, false)
}

// Markdown hard line breaks use two trailing spaces.
const markdownHardBreakPadding = 2

func preserveMarkdownLineBreaks(value string) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		if internalstrings.IsBlank(line) {
			continue
		}
		lines[i] = line + "  "
	}
	return strings.Join(lines, "\n")
}

func formatMarkdownBody(body string, indent int) string {
	return formatMarkdownBlock(body, indent, false)
}

func formatPromptBody(body string, indent int) string {
	body, empty := trimLogBody(body)
	if empty {
		return IndentBlock("-", indent)
	}
	if looksLikeMarkdown(body) {
		return formatMarkdownBlock(body, indent, true)
	}
	if hasIndentedLines(body) {
		return IndentBlock(body, indent)
	}
	reflowed := ReflowParagraphs(body, wrapWidthFor(lineWidth, indent))
	return IndentBlock(reflowed, indent)
}

func looksLikeMarkdown(body string) bool {
	markers := []string{"```", "\n- ", "\n* ", "\n1. ", "\n# "}
	for _, marker := range markers {
		if strings.Contains(body, marker) {
			return true
		}
	}
	return false
}

func hasIndentedLines(body string) bool {
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			return true
		}
	}
	return false
}

func formatMarkdownBlock(body string, indent int, preformatted bool) string {
	body, empty := trimLogBody(body)
	if empty {
		return IndentBlock("-", indent)
	}
	width := wrapWidthFor(lineWidth, indent)
	renderWidth := width
	if preformatted {
		body = preserveMarkdownLineBreaks(body)
		renderWidth = width + markdownHardBreakPadding
	}
	return renderMarkdownBlockOrDash(body, indent, renderWidth)
}

type testResultLog struct {
	Command  string
	ExitCode int
	Output   string
}

func testResultLogsFromEventData(results []testResultEventData) []testResultLog {
	return testResultLogsFrom(results, func(result testResultEventData) testResultLog {
		return testResultLog{
			Command:  result.Command,
			ExitCode: result.ExitCode,
			Output:   result.Output,
		}
	})
}

func testResultLogsFromCommandResults(results []TestCommandResult) []testResultLog {
	return testResultLogsFrom(results, func(result TestCommandResult) testResultLog {
		return testResultLog{
			Command:  result.Command,
			ExitCode: result.ExitCode,
			Output:   result.Output,
		}
	})
}

func testResultLogsFrom[T any](results []T, build func(T) testResultLog) []testResultLog {
	logs := make([]testResultLog, 0, len(results))
	for _, result := range results {
		logs = append(logs, build(result))
	}
	return logs
}

func formatTestLogBody(results []testResultLog) string {
	var builder strings.Builder
	for i, result := range results {
		if i > 0 {
			builder.WriteString("\n\n")
		}
		fmt.Fprintf(&builder, "Command: %s\nExit Code: %d\nOutput:\n", result.Command, result.ExitCode)
		output := normalizeLogBody(result.Output)
		builder.WriteString(IndentBlock(output, subdocumentIndent-documentIndent))
	}
	return formatLogBody(builder.String(), documentIndent, false)
}

func promptLabel(purpose string) string {
	switch purpose {
	case "implement":
		return "Implementation prompt:"
	case "review":
		return "Code review prompt:"
	case "project-review":
		return "Project review prompt:"
	default:
		return "Prompt:"
	}
}

func reviewLabel(purpose string) string {
	switch purpose {
	case "review":
		return "Code review result:"
	case "project-review":
		return "Project review result:"
	default:
		return "Review result:"
	}
}

func commitMessageLabel(label string) string {
	if internalstrings.IsBlank(label) {
		return "Commit message:"
	}
	return fmt.Sprintf("%s commit message:", label)
}

// StageMessage returns the standard log message for a stage transition.
func StageMessage(stage Stage) string {
	switch stage {
	case StageImplementing:
		return "Running implementation prompt:"
	case StageTesting:
		return "Implementation prompt complete; running tests:"
	case StageReviewing:
		return "Starting review:"
	case StageCommitting:
		return "Review complete; committing changes:"
	default:
		return fmt.Sprintf("Stage: %s", stage)
	}
}
