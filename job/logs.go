package job

import (
	"encoding/json"
	"fmt"
	"strings"

	internalstrings "github.com/amonks/incrementum/internal/strings"
)

// LogSnapshot returns the stored job event log.
func LogSnapshot(jobID string, opts EventLogOptions) (string, error) {
	entries, err := readEventLog(jobID, opts, false)
	if err != nil {
		return "", err
	}
	writer := &logSnapshotWriter{repoPath: opts.RepoPath}
	for _, event := range entries {
		if appendErr := writer.Append(event); appendErr != nil {
			return "", appendErr
		}
	}
	return internalstrings.TrimTrailingNewlines(writer.String()), nil
}

type logSnapshotWriter struct {
	builder      strings.Builder
	started      bool
	skipSpacing  bool
	lastCategory string
	opencode     *opencodeEventInterpreter
	agent        *agentEventInterpreter
	repoPath     string
}

func (writer *logSnapshotWriter) Append(event Event) error {
	if strings.HasPrefix(event.Name, "job.") {
		switch event.Name {
		case jobEventStage:
			data, err := decodeEventData[stageEventData](event.Data)
			if err != nil {
				return err
			}
			writer.writeStage(StageMessage(data.Stage))
		case jobEventPrompt:
			data, err := decodeEventData[promptEventData](event.Data)
			if err != nil {
				return err
			}
			writer.writeBlock(
				formatLogLabel(promptLabel(data.Purpose), documentIndent),
				formatPromptBody(data.Prompt, subdocumentIndent),
			)
		case jobEventCommitMessage:
			data, err := decodeEventData[commitMessageEventData](event.Data)
			if err != nil {
				return err
			}
			label := commitMessageLabel(data.Label)
			writer.writeBlock(
				formatLogLabel(label, documentIndent),
				formatCommitMessageBody(data.Message, subdocumentIndent, data.Preformatted),
			)
		case jobEventTranscript:
			data, err := decodeEventData[transcriptEventData](event.Data)
			if err != nil {
				return err
			}
			writer.skipSpacing = true
			writer.writeBlock(
				formatLogLabel("LLM transcript:", documentIndent),
				formatTranscriptBody(data.Transcript, subdocumentIndent),
			)
		case jobEventReview:
			data, err := decodeEventData[reviewEventData](event.Data)
			if err != nil {
				return err
			}
			writer.writeBlock(
				formatLogLabel(reviewLabel(data.Purpose), documentIndent),
				formatLogBody(data.Details, subdocumentIndent, true),
			)
		case jobEventTests:
			data, err := decodeEventData[testsEventData](event.Data)
			if err != nil {
				return err
			}
			writer.writeTests(data.Results)
		case jobEventOpencodeError:
			data, err := decodeEventData[opencodeErrorEventData](event.Data)
			if err != nil {
				return err
			}
			writer.writeBlock(
				formatLogLabel(legacyErrorLabel(data.Purpose), documentIndent),
				formatLogBody(data.Error, subdocumentIndent, false),
			)
		case jobEventAgentError:
			data, err := decodeEventData[agentErrorEventData](event.Data)
			if err != nil {
				return err
			}
			writer.writeBlock(
				formatLogLabel(agentErrorLabel(data.Purpose), documentIndent),
				formatLogBody(data.Error, subdocumentIndent, false),
			)
		case jobEventOpencodeStart, jobEventOpencodeEnd:
			return nil
		case jobEventAgentStart, jobEventAgentEnd:
			return nil
		default:
			return nil
		}
		writer.lastCategory = "job"
		return nil
	}

	if internalstrings.IsBlank(event.Name) && internalstrings.IsBlank(event.Data) {
		return nil
	}

	// Check if this is an agent event
	if IsAgentEvent(event) {
		return writer.appendAgentEvent(event)
	}

	return writer.appendOpencodeEvent(event)
}

func legacyEventLabel(name string) string {
	trimmed, ok := trimmedLabelValue(name)
	if !ok {
		return "LLM event:"
	}
	return fmt.Sprintf("LLM event (%s):", trimmed)
}

func legacyErrorLabel(purpose string) string {
	trimmed, ok := trimmedLabelValue(purpose)
	if !ok {
		return "LLM error:"
	}
	label := strings.ReplaceAll(trimmed, "-", " ")
	return fmt.Sprintf("LLM %s error:", label)
}

func agentErrorLabel(purpose string) string {
	trimmed, ok := trimmedLabelValue(purpose)
	if !ok {
		return "Agent error:"
	}
	label := strings.ReplaceAll(trimmed, "-", " ")
	return fmt.Sprintf("Agent %s error:", label)
}

func trimmedLabelValue(value string) (string, bool) {
	trimmed := internalstrings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}
	return trimmed, true
}

func (writer *logSnapshotWriter) writeStage(value string) {
	if internalstrings.IsBlank(value) {
		return
	}
	if writer.started {
		writer.builder.WriteString("\n")
	}
	writer.builder.WriteString(value)
	writer.builder.WriteString("\n")
	writer.started = true
	writer.skipSpacing = true
}

func (writer *logSnapshotWriter) writeBlock(lines ...string) {
	if len(lines) == 0 {
		return
	}
	if writer.started && !writer.skipSpacing {
		writer.builder.WriteString("\n")
	}
	writer.skipSpacing = false
	writer.started = true
	for _, line := range lines {
		writer.builder.WriteString(line)
		writer.builder.WriteString("\n")
	}
}

func (writer *logSnapshotWriter) appendOpencodeEvent(event Event) error {
	if writer.opencode == nil {
		writer.opencode = newOpencodeEventInterpreter(nil, writer.repoPath)
	}
	outputs, err := writer.opencode.Handle(event)
	if err != nil {
		return err
	}
	for _, output := range outputs {
		lines := formatOpencodeText(output)
		if len(lines) == 0 {
			continue
		}
		if writer.lastCategory == "opencode" {
			writer.skipSpacing = true
		}
		writer.writeBlock(lines...)
		writer.lastCategory = "opencode"
	}
	return nil
}

func (writer *logSnapshotWriter) appendAgentEvent(event Event) error {
	if writer.agent == nil {
		writer.agent = newAgentEventInterpreter(writer.repoPath)
	}
	outputs, err := writer.agent.Handle(event)
	if err != nil {
		return err
	}
	for _, output := range outputs {
		lines := formatAgentText(output)
		if len(lines) == 0 {
			continue
		}
		if writer.lastCategory == "agent" {
			writer.skipSpacing = true
		}
		writer.writeBlock(lines...)
		writer.lastCategory = "agent"
	}
	return nil
}

func (writer *logSnapshotWriter) writeTests(results []testResultEventData) {
	writer.writeBlock(formatTestLogBody(testResultLogsFromEventData(results)))
}

func (writer *logSnapshotWriter) String() string {
	return writer.builder.String()
}

func (writer *logSnapshotWriter) Len() int {
	return writer.builder.Len()
}

// EventFormatter formats job events incrementally.
type EventFormatter struct {
	writer logSnapshotWriter
}

// NewEventFormatter creates a new EventFormatter.
func NewEventFormatter() *EventFormatter {
	return NewEventFormatterWithRepoPath("")
}

// NewEventFormatterWithRepoPath creates a new EventFormatter anchored to a repo path.
func NewEventFormatterWithRepoPath(repoPath string) *EventFormatter {
	return &EventFormatter{writer: logSnapshotWriter{repoPath: repoPath}}
}

// Append formats a job event and returns the newly added output.
func (formatter *EventFormatter) Append(event Event) (string, error) {
	if formatter == nil {
		return "", nil
	}
	return appendEventOutput(&formatter.writer, event)
}

func decodeEventData[T any](payload string) (T, error) {
	var data T
	if internalstrings.IsBlank(payload) {
		return data, nil
	}
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return data, fmt.Errorf("decode job event data: %w", err)
	}
	return data, nil
}
