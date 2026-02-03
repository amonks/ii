package job

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/amonks/incrementum/internal/paths"
	internalstrings "github.com/amonks/incrementum/internal/strings"
)

const (
	jobEventStage         = "job.stage"
	jobEventPrompt        = "job.prompt"
	jobEventTranscript    = "job.transcript"
	jobEventCommitMessage = "job.commit_message"
	jobEventReview        = "job.review"
	jobEventTests         = "job.tests"
	jobEventAgentStart    = "job.agent.start"
	jobEventAgentEnd      = "job.agent.end"
	jobEventAgentError    = "job.agent.error"

	// Legacy event constants (for backward compatibility parsing old logs)
	jobEventOpencodeStart = "job.opencode.start"
	jobEventOpencodeEnd   = "job.opencode.end"
	jobEventOpencodeError = "job.opencode.error"
)

// Event captures a job log event.
type Event struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
	Data string `json:"data,omitempty"`
}

// EventLogOptions configures job event logs.
type EventLogOptions struct {
	EventsDir string
	RepoPath  string
}

// EventLog writes job events to a JSONL log.
type EventLog struct {
	path    string
	file    *os.File
	encoder *json.Encoder
	stream  chan<- Event
	mu      sync.Mutex
}

// OpenEventLog creates a job event log.
func OpenEventLog(jobID string, opts EventLogOptions) (*EventLog, error) {
	path, err := eventLogPath(jobID, opts)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create job events dir: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create job event log: %w", err)
	}
	return &EventLog{path: path, file: file, encoder: json.NewEncoder(file)}, nil
}

// SetStream attaches an event channel for streaming events.
func (log *EventLog) SetStream(stream chan<- Event) {
	if log == nil {
		return
	}
	log.mu.Lock()
	defer log.mu.Unlock()
	log.stream = stream
}

// Append writes a new event to the log.
func (log *EventLog) Append(event Event) error {
	if log == nil {
		return nil
	}
	log.mu.Lock()
	defer log.mu.Unlock()
	if log.encoder == nil {
		return fmt.Errorf("job event log is closed")
	}
	if err := log.encoder.Encode(event); err != nil {
		return err
	}
	if log.stream != nil {
		log.stream <- event
	}
	return nil
}

// Close flushes and closes the event log.
func (log *EventLog) Close() error {
	if log == nil {
		return nil
	}
	log.mu.Lock()
	defer log.mu.Unlock()
	if log.file == nil {
		return nil
	}
	err := log.file.Close()
	log.file = nil
	log.encoder = nil
	return err
}

func eventLogPath(jobID string, opts EventLogOptions) (string, error) {
	if jobID == "" {
		return "", fmt.Errorf("job id is required")
	}
	root, err := paths.ResolveWithDefault(opts.EventsDir, paths.DefaultJobEventsDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, jobID+".jsonl"), nil
}

func openEventLogFile(jobID string, opts EventLogOptions) (*os.File, error) {
	path, err := eventLogPath(jobID, opts)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// EventLogPath returns the path to the job event log.
func EventLogPath(jobID string, opts EventLogOptions) (string, error) {
	return eventLogPath(jobID, opts)
}

// ReadEvents reads job events from a JSONL reader.
func ReadEvents(reader io.Reader) ([]Event, error) {
	events := make([]Event, 0)
	if reader == nil {
		return events, nil
	}
	buffer := bufio.NewReader(reader)
	for {
		line, err := buffer.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		line = internalstrings.TrimTrailingNewlines(line)
		if !internalstrings.IsBlank(line) {
			var event Event
			if unmarshalErr := json.Unmarshal([]byte(line), &event); unmarshalErr != nil {
				return nil, fmt.Errorf("decode job event: %w", unmarshalErr)
			}
			events = append(events, event)
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}
	return events, nil
}

func readEventLog(jobID string, opts EventLogOptions, allowMissing bool) ([]Event, error) {
	file, err := openEventLogFile(jobID, opts)
	if err != nil {
		if allowMissing && os.IsNotExist(err) {
			return []Event{}, nil
		}
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()
	return ReadEvents(file)
}

// EventSnapshot returns the stored job events.
func EventSnapshot(jobID string, opts EventLogOptions) ([]Event, error) {
	return readEventLog(jobID, opts, true)
}

func appendJobEvent(log *EventLog, name string, payload any) error {
	if log == nil {
		return nil
	}
	data, err := marshalJobEventData(payload)
	if err != nil {
		return err
	}
	return log.Append(Event{Name: name, Data: data})
}

func marshalJobEventData(payload any) (string, error) {
	if payload == nil {
		return "", nil
	}
	if value, ok := payload.(string); ok {
		return value, nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type stageEventData struct {
	Stage Stage `json:"stage"`
}

type promptEventData struct {
	Purpose  string `json:"purpose"`
	Template string `json:"template"`
	Prompt   string `json:"prompt"`
}

type transcriptEventData struct {
	Purpose    string `json:"purpose"`
	Transcript string `json:"transcript"`
}

type commitMessageEventData struct {
	Label        string `json:"label"`
	Message      string `json:"message"`
	Preformatted bool   `json:"preformatted,omitempty"`
}

type reviewEventData struct {
	Purpose string        `json:"purpose"`
	Outcome ReviewOutcome `json:"outcome"`
	Details string        `json:"details,omitempty"`
}

type testResultEventData struct {
	Command  string `json:"command"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output,omitempty"`
}

type testsEventData struct {
	Results []testResultEventData `json:"results"`
}

// Legacy event data types (for backward compatibility parsing old logs)
type opencodeErrorEventData struct {
	Purpose string `json:"purpose"`
	Error   string `json:"error"`
}

func buildTestsEventData(results []TestCommandResult) testsEventData {
	data := testsEventData{Results: make([]testResultEventData, 0, len(results))}
	for _, result := range results {
		data.Results = append(data.Results, testResultEventData{Command: result.Command, ExitCode: result.ExitCode, Output: result.Output})
	}
	return data
}
