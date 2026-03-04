package job

import (
	"testing"
)

func TestLogSnapshotSnapshot(t *testing.T) {
	eventsDir := t.TempDir()
	jobID := "job-log-snapshot"
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
	prompt := "Checklist:\n\n" +
		"This paragraph is deliberately long to confirm that markdown rendering wraps within the log width without losing spacing. " +
		"It should break across multiple lines and still preserve paragraph boundaries.\n\n" +
		"- First bullet item includes additional words to make sure wrapping happens after the bullet prefix.\n" +
		"- Second bullet item is also long enough to wrap on the next line in the snapshot.\n\n" +
		"```bash\n" +
		"export SNAPSHOT_MODE=enabled\n" +
		"./bin/ii job run --preview\n" +
		"```"
	if err := appendJobEvent(log, jobEventPrompt, promptEventData{Purpose: "implement", Prompt: prompt}); err != nil {
		t.Fatalf("append prompt event: %v", err)
	}
	if err := appendJobEvent(log, jobEventTranscript, transcriptEventData{Purpose: "implement", Transcript: "Agent transcript line one.\nAgent transcript line two."}); err != nil {
		t.Fatalf("append transcript event: %v", err)
	}
	message := "feat: snapshot log output\n\n" +
		"Keep this description long enough to wrap and include a short list:\n\n" +
		"- Capture prompt formatting.\n" +
		"- Preserve code fences.\n\n" +
		"```bash\n" +
		"go test ./job -run Snapshot\n" +
		"```"
	if err := appendJobEvent(log, jobEventCommitMessage, commitMessageEventData{Label: "Final", Message: message, Preformatted: true}); err != nil {
		t.Fatalf("append commit message event: %v", err)
	}
	if err := appendJobEvent(log, jobEventStage, stageEventData{Stage: StageTesting}); err != nil {
		t.Fatalf("append stage event: %v", err)
	}
	if err := appendJobEvent(log, jobEventTests, buildTestsEventData([]TestCommandResult{{Command: "go test ./job/...", ExitCode: 1, Output: "--- FAIL: TestSnapshot (0.01s)\n    snapshot mismatch"}})); err != nil {
		t.Fatalf("append tests event: %v", err)
	}
	if err := appendJobEvent(log, jobEventStage, stageEventData{Stage: StageReviewing}); err != nil {
		t.Fatalf("append stage event: %v", err)
	}
	review := "Review notes:\n\n" +
		"- Update the snapshot file to match the current output.\n" +
		"- Keep the description block long enough to wrap across lines in the snapshot output."
	if err := appendJobEvent(log, jobEventReview, reviewEventData{Purpose: "review", Details: review}); err != nil {
		t.Fatalf("append review event: %v", err)
	}

	snapshot, err := LogSnapshot(jobID, EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	requireSnapshot(t, "log-snapshot.txt", snapshot)
}
