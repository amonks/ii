package job

import (
	"strings"
	"testing"
)

func TestIsAgentEvent(t *testing.T) {
	tests := []struct {
		name   string
		event  Event
		expect bool
	}{
		{"agent.start", Event{Name: "agent.start"}, true},
		{"agent.end", Event{Name: "agent.end"}, true},
		{"turn.start", Event{Name: "turn.start"}, true},
		{"turn.end", Event{Name: "turn.end"}, true},
		{"message.start", Event{Name: "message.start"}, true},
		{"message.update", Event{Name: "message.update"}, true},
		{"message.end", Event{Name: "message.end"}, true},
		{"tool.start", Event{Name: "tool.start"}, true},
		{"tool.end", Event{Name: "tool.end"}, true},
		{"opencode message.updated", Event{Name: "", Data: `{"type":"message.updated"}`}, false},
		{"job.stage", Event{Name: "job.stage"}, false},
		{"empty", Event{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAgentEvent(tt.event)
			if result != tt.expect {
				t.Errorf("IsAgentEvent(%v) = %v, want %v", tt.event, result, tt.expect)
			}
		})
	}
}

func TestAgentEventInterpreterToolStart(t *testing.T) {
	interp := newAgentEventInterpreter("/test/repo")

	// Test bash tool
	bashEvent := Event{
		Name: "tool.start",
		Data: `{"TurnIndex":0,"ToolCallID":"tool-1","ToolName":"bash","Arguments":{"command":"go test ./..."}}`,
	}
	results, err := interp.Handle(bashEvent)
	if err != nil {
		t.Fatalf("Handle bash tool.start: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0].Inline, "bash 'go test ./...'") {
		t.Errorf("expected bash command in output, got %q", results[0].Inline)
	}

	// Test read tool with relative path
	readEvent := Event{
		Name: "tool.start",
		Data: `{"TurnIndex":0,"ToolCallID":"tool-2","ToolName":"read","Arguments":{"path":"/test/repo/src/main.go"}}`,
	}
	results, err = interp.Handle(readEvent)
	if err != nil {
		t.Fatalf("Handle read tool.start: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0].Inline, "read file 'src/main.go'") {
		t.Errorf("expected relative path in output, got %q", results[0].Inline)
	}
}

func TestAgentEventInterpreterToolEnd(t *testing.T) {
	interp := newAgentEventInterpreter("")

	// Start then end tool
	startEvent := Event{
		Name: "tool.start",
		Data: `{"TurnIndex":0,"ToolCallID":"tool-1","ToolName":"read","Arguments":{"path":"/tmp/test.txt"}}`,
	}
	if _, err := interp.Handle(startEvent); err != nil {
		t.Fatalf("Handle tool.start: %v", err)
	}

	endEvent := Event{
		Name: "tool.end",
		Data: `{"TurnIndex":0,"ToolCallID":"tool-1","ToolName":"read","Result":{"IsError":false,"Content":[{"Type":"text","Text":"file contents"}]}}`,
	}
	results, err := interp.Handle(endEvent)
	if err != nil {
		t.Fatalf("Handle tool.end: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0].Inline, "Tool end: read file") {
		t.Errorf("expected tool end output, got %q", results[0].Inline)
	}
	if strings.Contains(results[0].Inline, "failed") {
		t.Errorf("expected no failed status, got %q", results[0].Inline)
	}
}

func TestAgentEventInterpreterToolFailed(t *testing.T) {
	interp := newAgentEventInterpreter("")

	endEvent := Event{
		Name: "tool.end",
		Data: `{"TurnIndex":0,"ToolCallID":"tool-1","ToolName":"read","Result":{"IsError":true,"Content":[{"Type":"text","Text":"file not found"}]}}`,
	}
	results, err := interp.Handle(endEvent)
	if err != nil {
		t.Fatalf("Handle tool.end: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0].Inline, "(failed)") {
		t.Errorf("expected failed status, got %q", results[0].Inline)
	}
}

func TestAgentEventInterpreterMessageEnd(t *testing.T) {
	interp := newAgentEventInterpreter("")

	// Message with thinking and text
	msgEvent := Event{
		Name: "message.end",
		Data: `{
			"TurnIndex": 0,
			"Message": {
				"Role": "assistant",
				"Content": [
					{"Type": "thinking", "Thinking": "Let me analyze this..."},
					{"Type": "text", "Text": "Here is the answer."}
				],
				"StopReason": "end"
			}
		}`,
	}
	results, err := interp.Handle(msgEvent)
	if err != nil {
		t.Fatalf("Handle message.end: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (thinking + response), got %d", len(results))
	}

	// Check thinking result
	if results[0].Kind != "thinking" {
		t.Errorf("expected thinking kind, got %q", results[0].Kind)
	}
	if !strings.Contains(results[0].Body, "Let me analyze this") {
		t.Errorf("expected thinking body, got %q", results[0].Body)
	}

	// Check response result
	if results[1].Kind != "response" {
		t.Errorf("expected response kind, got %q", results[1].Kind)
	}
	if !strings.Contains(results[1].Body, "Here is the answer") {
		t.Errorf("expected response body, got %q", results[1].Body)
	}
}

func TestAgentEventInterpreterAgentStartEnd(t *testing.T) {
	interp := newAgentEventInterpreter("")

	startEvent := Event{
		Name: "agent.start",
		Data: `{"Config":{"Model":{"ID":"claude-sonnet-4"},"WorkDir":"/work"}}`,
	}
	results, err := interp.Handle(startEvent)
	if err != nil {
		t.Fatalf("Handle agent.start: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0].Inline, "Agent started") {
		t.Errorf("expected agent started, got %q", results[0].Inline)
	}
	if !strings.Contains(results[0].Inline, "claude-sonnet-4") {
		t.Errorf("expected model in output, got %q", results[0].Inline)
	}

	endEvent := Event{
		Name: "agent.end",
		Data: `{"Usage":{"Input":100,"Output":200,"Total":300,"Cost":{"Total":0.005}}}`,
	}
	results, err = interp.Handle(endEvent)
	if err != nil {
		t.Fatalf("Handle agent.end: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0].Inline, "Agent finished") {
		t.Errorf("expected agent finished, got %q", results[0].Inline)
	}
	if !strings.Contains(results[0].Inline, "300") {
		t.Errorf("expected token count in output, got %q", results[0].Inline)
	}
}

func TestEventFormatterRendersAgentEvents(t *testing.T) {
	formatter := NewEventFormatter()

	// Tool start
	toolStart := Event{
		Name: "tool.start",
		Data: `{"TurnIndex":0,"ToolCallID":"tool-1","ToolName":"read","Arguments":{"path":"/tmp/example.txt"}}`,
	}
	chunk, err := formatter.Append(toolStart)
	if err != nil {
		t.Fatalf("append tool.start: %v", err)
	}
	if !strings.Contains(chunk, "Tool start: read file '/tmp/example.txt'") {
		t.Errorf("expected tool start output, got %q", chunk)
	}

	// Tool end
	toolEnd := Event{
		Name: "tool.end",
		Data: `{"TurnIndex":0,"ToolCallID":"tool-1","ToolName":"read","Result":{"IsError":false,"Content":[{"Type":"text","Text":"contents"}]}}`,
	}
	chunk, err = formatter.Append(toolEnd)
	if err != nil {
		t.Fatalf("append tool.end: %v", err)
	}
	if !strings.Contains(chunk, "Tool end: read file") {
		t.Errorf("expected tool end output, got %q", chunk)
	}
}

func TestAgentEventInterpreterSuppressesBashWithoutCommand(t *testing.T) {
	interp := newAgentEventInterpreter("")

	// Bash without command should be suppressed
	bashEvent := Event{
		Name: "tool.start",
		Data: `{"TurnIndex":0,"ToolCallID":"tool-1","ToolName":"bash","Arguments":{}}`,
	}
	results, err := interp.Handle(bashEvent)
	if err != nil {
		t.Fatalf("Handle bash tool.start: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected no results for bash without command, got %d", len(results))
	}
}

func TestAgentEventInterpreterGlobGrep(t *testing.T) {
	interp := newAgentEventInterpreter("/test/repo")

	// Glob with pattern and path
	globEvent := Event{
		Name: "tool.start",
		Data: `{"TurnIndex":0,"ToolCallID":"tool-1","ToolName":"glob","Arguments":{"pattern":"**/*.go","path":"/test/repo/src"}}`,
	}
	results, err := interp.Handle(globEvent)
	if err != nil {
		t.Fatalf("Handle glob tool.start: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0].Inline, "glob '**/*.go' in 'src'") {
		t.Errorf("expected glob output, got %q", results[0].Inline)
	}

	// Grep with pattern
	grepEvent := Event{
		Name: "tool.start",
		Data: `{"TurnIndex":0,"ToolCallID":"tool-2","ToolName":"grep","Arguments":{"pattern":"func main"}}`,
	}
	results, err = interp.Handle(grepEvent)
	if err != nil {
		t.Fatalf("Handle grep tool.start: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0].Inline, "grep 'func main'") {
		t.Errorf("expected grep output, got %q", results[0].Inline)
	}
}

func TestLogSnapshotFormatsAgentEvents(t *testing.T) {
	eventsDir := t.TempDir()
	jobID := "job-agent"
	log, err := OpenEventLog(jobID, EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	defer func() {
		if err := log.Close(); err != nil {
			t.Fatalf("close log: %v", err)
		}
	}()

	// Add stage event
	if err := appendJobEvent(log, jobEventStage, stageEventData{Stage: StageImplementing}); err != nil {
		t.Fatalf("append stage event: %v", err)
	}

	// Add agent tool events
	agentToolStart := Event{
		Name: "tool.start",
		Data: `{"TurnIndex":0,"ToolCallID":"tool-1","ToolName":"read","Arguments":{"path":"/tmp/example.txt"}}`,
	}
	if err := log.Append(agentToolStart); err != nil {
		t.Fatalf("append agent tool start event: %v", err)
	}

	agentToolEnd := Event{
		Name: "tool.end",
		Data: `{"TurnIndex":0,"ToolCallID":"tool-1","ToolName":"read","Result":{"IsError":false,"Content":[{"Type":"text","Text":"file contents"}]}}`,
	}
	if err := log.Append(agentToolEnd); err != nil {
		t.Fatalf("append agent tool end event: %v", err)
	}

	snapshot, err := LogSnapshot(jobID, EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	checks := []string{
		"Running implementation prompt:",
		"Tool start: read file '/tmp/example.txt'",
		"Tool end: read file",
	}
	for _, check := range checks {
		if !strings.Contains(snapshot, check) {
			t.Errorf("expected snapshot to include %q, got %q", check, snapshot)
		}
	}
}
