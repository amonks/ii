package job

import (
	"regexp"
	"strings"
	"testing"
)

func TestLogSnapshotFormatsJobEvents(t *testing.T) {
	eventsDir := t.TempDir()
	jobID := "job-logs"
	log, err := OpenEventLog(jobID, EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	defer func() {
		if err := log.Close(); err != nil {
			t.Fatalf("close log: %v", err)
		}
	}()

	if err := appendJobEvent(log, jobEventStage, stageEventData{Stage: StageImplementing}); err != nil {
		t.Fatalf("append stage event: %v", err)
	}
	if err := appendJobEvent(log, jobEventPrompt, promptEventData{Purpose: "implement", Prompt: "First line.\nSecond line."}); err != nil {
		t.Fatalf("append prompt event: %v", err)
	}
	if err := appendJobEvent(log, jobEventTranscript, transcriptEventData{Purpose: "implement", Transcript: "Transcript line."}); err != nil {
		t.Fatalf("append transcript event: %v", err)
	}
	if err := appendJobEvent(log, jobEventCommitMessage, commitMessageEventData{Label: "Draft", Message: "feat: add logs"}); err != nil {
		t.Fatalf("append commit message event: %v", err)
	}
	if err := appendJobEvent(log, jobEventStage, stageEventData{Stage: StageTesting}); err != nil {
		t.Fatalf("append test stage event: %v", err)
	}
	if err := appendJobEvent(log, jobEventTests, buildTestsEventData([]TestCommandResult{{Command: "go test ./...", ExitCode: 1, Output: "tests failed"}})); err != nil {
		t.Fatalf("append tests event: %v", err)
	}
	if err := appendJobEvent(log, jobEventStage, stageEventData{Stage: StageReviewing}); err != nil {
		t.Fatalf("append review stage event: %v", err)
	}
	if err := appendJobEvent(log, jobEventTranscript, transcriptEventData{Purpose: "review", Transcript: "Review transcript line."}); err != nil {
		t.Fatalf("append review transcript event: %v", err)
	}
	if err := appendJobEvent(log, jobEventReview, reviewEventData{Purpose: "review", Outcome: ReviewOutcomeRequestChanges, Details: "Add tests."}); err != nil {
		t.Fatalf("append review event: %v", err)
	}

	snapshot, err := LogSnapshot(jobID, EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	checks := []string{
		"Running implementation prompt:",
		"    Implementation prompt:",
		"        First line. Second line.",
		"    LLM transcript:",
		"        Transcript line.",
		"    Draft change description:",
		"        feat: add logs",
		"Implementation prompt complete; running tests:",
		"Command: go test ./...",
		"Exit Code: 1",
		"Output:",
		"tests failed",
		"Starting review:",
		"Review transcript line.",
		"    Code review result:",
		"        Add tests.",
	}
	for _, check := range checks {
		if !strings.Contains(snapshot, check) {
			t.Fatalf("expected snapshot to include %q, got %q", check, snapshot)
		}
	}
}

func TestLogSnapshotHandlesLargeEvent(t *testing.T) {
	eventsDir := t.TempDir()
	jobID := "job-logs-large"
	log, err := OpenEventLog(jobID, EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	defer func() {
		if err := log.Close(); err != nil {
			t.Fatalf("close log: %v", err)
		}
	}()

	largePrompt := strings.Repeat("word ", 20000)
	if err := appendJobEvent(log, jobEventStage, stageEventData{Stage: StageImplementing}); err != nil {
		t.Fatalf("append stage event: %v", err)
	}
	if err := appendJobEvent(log, jobEventPrompt, promptEventData{Purpose: "implement", Prompt: largePrompt}); err != nil {
		t.Fatalf("append prompt event: %v", err)
	}

	snapshot, err := LogSnapshot(jobID, EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if !strings.Contains(snapshot, "    Implementation prompt:") {
		t.Fatalf("expected prompt label, got %q", snapshot)
	}
	if !strings.Contains(snapshot, "word word word") {
		t.Fatalf("expected large prompt content, got %q", snapshot)
	}
}

func TestLogSnapshotRendersMarkdownCommitMessage(t *testing.T) {
	eventsDir := t.TempDir()
	jobID := "job-logs-markdown"
	log, err := OpenEventLog(jobID, EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	defer func() {
		if err := log.Close(); err != nil {
			t.Fatalf("close log: %v", err)
		}
	}()

	message := "Summary:\n\n- First item\n- Second item\n\n```bash\necho first\necho second\n```"
	if err := appendJobEvent(log, jobEventCommitMessage, commitMessageEventData{Label: "Draft", Message: message}); err != nil {
		t.Fatalf("append commit message event: %v", err)
	}

	snapshot, err := LogSnapshot(jobID, EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	checks := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^\s+Summary:$`),
		regexp.MustCompile(`(?m)^\s+.*First item$`),
		regexp.MustCompile(`(?m)^\s+.*Second item$`),
		regexp.MustCompile(`(?m)^\s+echo first$`),
		regexp.MustCompile(`(?m)^\s+echo second$`),
	}
	for _, check := range checks {
		if !check.MatchString(snapshot) {
			t.Fatalf("expected markdown commit message output to match %q, got %q", check.String(), snapshot)
		}
	}
}

func TestEventFormatterAppendsOutput(t *testing.T) {
	formatter := NewEventFormatter()

	chunk, err := formatter.Append(Event{Name: "job.stage", Data: "{\"stage\":\"implementing\"}"})
	if err != nil {
		t.Fatalf("append stage event: %v", err)
	}
	if !strings.Contains(chunk, "Running implementation prompt:") {
		t.Fatalf("expected stage output, got %q", chunk)
	}

	chunk, err = formatter.Append(Event{Name: "job.prompt", Data: "{\"purpose\":\"implement\",\"prompt\":\"Hello\"}"})
	if err != nil {
		t.Fatalf("append prompt event: %v", err)
	}
	if !strings.Contains(chunk, "Implementation prompt:") {
		t.Fatalf("expected prompt output, got %q", chunk)
	}

	opencodeToolStart := `{"type":"message.part.updated","properties":{"part":{"id":"prt-tool","messageID":"msg-tool","type":"tool","tool":"glob","state":{"status":"running","input":{"pattern":"**/*.go","path":"/tmp"}}}}}`
	chunk, err = formatter.Append(Event{Data: opencodeToolStart})
	if err != nil {
		t.Fatalf("append opencode tool start event: %v", err)
	}
	if !strings.Contains(chunk, "Tool (running): glob '") {
		t.Fatalf("expected tool status output, got %q", chunk)
	}
	opencodeToolEnd := `{"type":"message.part.updated","properties":{"part":{"id":"prt-tool","messageID":"msg-tool","type":"tool","tool":"glob","state":{"status":"completed","input":{"pattern":"**/*.go","path":"/tmp"}}}}}`
	chunk, err = formatter.Append(Event{Data: opencodeToolEnd})
	if err != nil {
		t.Fatalf("append opencode tool end event: %v", err)
	}
	if !strings.Contains(chunk, "Tool (completed): glob '") {
		t.Fatalf("expected tool completed output, got %q", chunk)
	}

	chunk, err = formatter.Append(Event{Name: "job.agent.error", Data: "{\"purpose\":\"implement\",\"error\":\"agent session not found\"}"})
	if err != nil {
		t.Fatalf("append agent error: %v", err)
	}
	if !strings.Contains(chunk, "Agent implement error:") {
		t.Fatalf("expected agent error label, got %q", chunk)
	}
	if !strings.Contains(chunk, "agent session not found") {
		t.Fatalf("expected agent error details, got %q", chunk)
	}
}

func TestEventFormatterRendersLLMMessages(t *testing.T) {
	formatter := NewEventFormatter()

	userMessage := `{"type":"message.updated","properties":{"info":{"id":"msg-user","role":"user"}}}`
	userPrompt := `{"type":"message.part.updated","properties":{"part":{"id":"prt-user","messageID":"msg-user","type":"text","text":"Prompt line."}}}`
	if _, err := formatter.Append(Event{Data: userMessage}); err != nil {
		t.Fatalf("append user message event: %v", err)
	}
	chunk, err := formatter.Append(Event{Data: userPrompt})
	if err != nil {
		t.Fatalf("append user prompt event: %v", err)
	}
	if !strings.Contains(chunk, "LLM prompt:") {
		t.Fatalf("expected LLM prompt label, got %q", chunk)
	}
	if !strings.Contains(chunk, "Prompt line.") {
		t.Fatalf("expected opencode prompt text, got %q", chunk)
	}

	assistantMessage := `{"type":"message.updated","properties":{"info":{"id":"msg-assistant","role":"assistant"}}}`
	assistantText := `{"type":"message.part.updated","properties":{"part":{"id":"prt-assistant","messageID":"msg-assistant","type":"text","text":"Response line."}}}`
	assistantThinking := `{"type":"message.part.updated","properties":{"part":{"id":"prt-reason","messageID":"msg-assistant","type":"reasoning","text":"Thinking line."}}}`
	assistantComplete := `{"type":"message.updated","properties":{"info":{"id":"msg-assistant","role":"assistant","time":{"completed":1}}}}`
	if _, err := formatter.Append(Event{Data: assistantMessage}); err != nil {
		t.Fatalf("append assistant message event: %v", err)
	}
	if _, err := formatter.Append(Event{Data: assistantText}); err != nil {
		t.Fatalf("append assistant text event: %v", err)
	}
	if _, err := formatter.Append(Event{Data: assistantThinking}); err != nil {
		t.Fatalf("append assistant thinking event: %v", err)
	}
	chunk, err = formatter.Append(Event{Data: assistantComplete})
	if err != nil {
		t.Fatalf("append assistant complete event: %v", err)
	}
	if !strings.Contains(chunk, "LLM thinking:") {
		t.Fatalf("expected LLM thinking label, got %q", chunk)
	}
	if !strings.Contains(chunk, "Thinking line.") {
		t.Fatalf("expected LLM thinking text, got %q", chunk)
	}
	if !strings.Contains(chunk, "LLM response:") {
		t.Fatalf("expected LLM response label, got %q", chunk)
	}
	if !strings.Contains(chunk, "Response line.") {
		t.Fatalf("expected opencode response text, got %q", chunk)
	}
}

func TestEventFormatterRendersLLMPromptMarkdown(t *testing.T) {
	formatter := NewEventFormatter()

	userMessage := `{"type":"message.updated","properties":{"info":{"id":"msg-user","role":"user"}}}`
	userPrompt := `{"type":"message.part.updated","properties":{"part":{"id":"prt-user","messageID":"msg-user","type":"text","text":"Checklist:\n\n- First item\n- Second item"}}}`
	if _, err := formatter.Append(Event{Data: userMessage}); err != nil {
		t.Fatalf("append user message event: %v", err)
	}
	chunk, err := formatter.Append(Event{Data: userPrompt})
	if err != nil {
		t.Fatalf("append user prompt event: %v", err)
	}

	checks := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^\s+Checklist:$`),
		regexp.MustCompile(`(?m)^\s+.*First item$`),
		regexp.MustCompile(`(?m)^\s+.*Second item$`),
	}
	for _, check := range checks {
		if !check.MatchString(chunk) {
			t.Fatalf("expected opencode markdown output to match %q, got %q", check.String(), chunk)
		}
	}
}

func TestEventFormatterFallsBackOnMalformedEvent(t *testing.T) {
	formatter := NewEventFormatter()

	malformed := `{"type":"message.updated","properties":"nope"}`
	chunk, err := formatter.Append(Event{Data: malformed})
	if err != nil {
		t.Fatalf("append malformed event: %v", err)
	}
	if !strings.Contains(chunk, "LLM event (message.updated):") {
		t.Fatalf("expected fallback LLM label, got %q", chunk)
	}
	if !strings.Contains(chunk, `"type":"message.updated"`) {
		t.Fatalf("expected raw opencode payload, got %q", chunk)
	}
}

func TestEventFormatterRendersApplyPatchWithFiles(t *testing.T) {
	formatter := NewEventFormatter()

	patchJSON := `{"type":"message.part.updated","properties":{"part":{"id":"prt-patch","messageID":"msg-patch","type":"tool","tool":"apply_patch","state":{"status":"running","input":{"patch":"--- a/src/main.go\n+++ b/src/main.go\n@@ -1,3 +1,4 @@\n package main\n+import \"fmt\"\n"}}}}}`
	chunk, err := formatter.Append(Event{Data: patchJSON})
	if err != nil {
		t.Fatalf("append apply_patch event: %v", err)
	}
	if !strings.Contains(chunk, "patch file 'src/main.go'") {
		t.Fatalf("expected patch file summary, got %q", chunk)
	}
}

func TestEventFormatterRendersApplyPatchWithMultipleFiles(t *testing.T) {
	formatter := NewEventFormatter()

	patchJSON := `{"type":"message.part.updated","properties":{"part":{"id":"prt-multi","messageID":"msg-multi","type":"tool","tool":"apply_patch","state":{"status":"running","input":{"patch":"--- a/src/main.go\n+++ b/src/main.go\n@@ -1,3 +1,4 @@\n package main\n+import \"fmt\"\n--- a/src/util.go\n+++ b/src/util.go\n@@ -1 +1,2 @@\n package main\n+func helper() {}\n"}}}}}`
	chunk, err := formatter.Append(Event{Data: patchJSON})
	if err != nil {
		t.Fatalf("append apply_patch event: %v", err)
	}
	if !strings.Contains(chunk, "patch files 'src/main.go, src/util.go'") {
		t.Fatalf("expected patch files summary, got %q", chunk)
	}
}

func TestEventFormatterRendersApplyPatchWithoutPatchContent(t *testing.T) {
	formatter := NewEventFormatter()

	// When patch content is missing, fall back to "apply patch"
	patchJSON := `{"type":"message.part.updated","properties":{"part":{"id":"prt-empty","messageID":"msg-empty","type":"tool","tool":"apply_patch","state":{"status":"running","input":{}}}}}`
	chunk, err := formatter.Append(Event{Data: patchJSON})
	if err != nil {
		t.Fatalf("append apply_patch event: %v", err)
	}
	if !strings.Contains(chunk, "Tool (running): apply patch") {
		t.Fatalf("expected fallback apply patch summary, got %q", chunk)
	}
}

func TestEventFormatterSuppressesBashWithoutCommand(t *testing.T) {
	formatter := NewEventFormatter()

	// When bash command is empty, no log should be emitted
	bashJSON := `{"type":"message.part.updated","properties":{"part":{"id":"prt-bash","messageID":"msg-bash","type":"tool","tool":"bash","state":{"status":"running","input":{}}}}}`
	chunk, err := formatter.Append(Event{Data: bashJSON})
	if err != nil {
		t.Fatalf("append bash event: %v", err)
	}
	if strings.Contains(chunk, "Tool start") {
		t.Fatalf("expected no tool start log for bash without command, got %q", chunk)
	}
}

func TestEventFormatterRendersToolFailure(t *testing.T) {
	formatter := NewEventFormatter()

	// Start the tool first
	startJSON := `{"type":"message.part.updated","properties":{"part":{"id":"prt-fail","messageID":"msg-fail","type":"tool","tool":"read","state":{"status":"running","input":{"filePath":"/tmp/missing.txt"}}}}}`
	if _, err := formatter.Append(Event{Data: startJSON}); err != nil {
		t.Fatalf("append tool start event: %v", err)
	}

	// End with failed status
	endJSON := `{"type":"message.part.updated","properties":{"part":{"id":"prt-fail","messageID":"msg-fail","type":"tool","tool":"read","state":{"status":"failed","input":{"filePath":"/tmp/missing.txt"}}}}}`
	chunk, err := formatter.Append(Event{Data: endJSON})
	if err != nil {
		t.Fatalf("append tool end event: %v", err)
	}
	if !strings.Contains(chunk, "(failed)") {
		t.Fatalf("expected failed status in output, got %q", chunk)
	}
}

func TestEventFormatterLongBashCommand(t *testing.T) {
	formatter := NewEventFormatter()

	// Long command similar to real-world usage (full workspace path)
	longCommand := "cd /Users/ajm/.local/share/incrementum/workspaces/users-ajm-git-amonks-incrementum/ws-005 && go test ./job -v"

	bashJSON := `{"type":"message.part.updated","properties":{"part":{"id":"prt-bash","messageID":"msg-bash","type":"tool","tool":"bash","state":{"status":"running","input":{"command":"` + longCommand + `"}}}}}`
	chunk, err := formatter.Append(Event{Data: bashJSON})
	if err != nil {
		t.Fatalf("append bash event: %v", err)
	}

	// The full command should be in the output without truncation
	if !strings.Contains(chunk, longCommand) {
		t.Errorf("expected full command in output (no truncation), got %q", chunk)
	}

	// Should end with the command, not truncated with ellipsis
	// Note: we avoid '...' in the command itself to reliably detect truncation
	if strings.HasSuffix(strings.TrimSpace(chunk), "...") {
		t.Errorf("bash command should not be truncated with trailing ellipsis, got %q", chunk)
	}
}

func TestEventFormatterLegacyToolStatus(t *testing.T) {
	// Verify that each legacy tool status update is rendered independently.
	// Unlike agent events which emit explicit tool.start/tool.end, legacy
	// opencode events only have status updates. Each status is shown as-is.
	formatter := NewEventFormatter()

	makeEvent := func(status string) Event {
		return Event{Data: `{"type":"message.part.updated","properties":{"part":{"id":"prt-status","messageID":"msg-status","type":"tool","tool":"read","state":{"status":"` + status + `","input":{"filePath":"/tmp/test.txt"}}}}}`}
	}

	// First pending status
	chunk, err := formatter.Append(makeEvent("pending"))
	if err != nil {
		t.Fatalf("append pending event: %v", err)
	}
	if !strings.Contains(chunk, "Tool (pending):") {
		t.Fatalf("expected Tool (pending) on first pending, got %q", chunk)
	}

	// Second pending status - should still emit (no deduplication)
	chunk, err = formatter.Append(makeEvent("pending"))
	if err != nil {
		t.Fatalf("append second pending event: %v", err)
	}
	if !strings.Contains(chunk, "Tool (pending):") {
		t.Fatalf("expected Tool (pending) on duplicate pending, got %q", chunk)
	}

	// Running status
	chunk, err = formatter.Append(makeEvent("running"))
	if err != nil {
		t.Fatalf("append running event: %v", err)
	}
	if !strings.Contains(chunk, "Tool (running):") {
		t.Fatalf("expected Tool (running), got %q", chunk)
	}

	// Completed status
	chunk, err = formatter.Append(makeEvent("completed"))
	if err != nil {
		t.Fatalf("append completed event: %v", err)
	}
	if !strings.Contains(chunk, "Tool (completed):") {
		t.Fatalf("expected Tool (completed), got %q", chunk)
	}

	// Another completed status - should still emit (no deduplication)
	chunk, err = formatter.Append(makeEvent("completed"))
	if err != nil {
		t.Fatalf("append second completed event: %v", err)
	}
	if !strings.Contains(chunk, "Tool (completed):") {
		t.Fatalf("expected Tool (completed) on duplicate completed, got %q", chunk)
	}
}
