package main

import "testing"

func TestAgentEmptyListMessageNoSessions(t *testing.T) {
	message := agentEmptyListMessage(0, false)
	if message != "No agent sessions found." {
		t.Fatalf("expected agent empty message, got %q", message)
	}
}

func TestAgentEmptyListMessageSuggestsAll(t *testing.T) {
	message := agentEmptyListMessage(2, false)
	if message != "No active agent sessions found. Use --all to include completed/failed sessions." {
		t.Fatalf("expected agent --all hint, got %q", message)
	}
}

func TestAgentEmptyListMessageWithAll(t *testing.T) {
	message := agentEmptyListMessage(0, true)
	if message != "No agent sessions found." {
		t.Fatalf("expected agent empty message with --all, got %q", message)
	}
}

func TestJobEmptyListMessageNoJobs(t *testing.T) {
	message := jobEmptyListMessage(0, "", false)
	if message != "No jobs found." {
		t.Fatalf("expected no jobs message, got %q", message)
	}
}

func TestJobEmptyListMessageForStatusFilter(t *testing.T) {
	message := jobEmptyListMessage(2, "Completed", false)
	if message != "No jobs found with status completed." {
		t.Fatalf("expected status message, got %q", message)
	}
}

func TestJobEmptyListMessageSuggestsAll(t *testing.T) {
	message := jobEmptyListMessage(3, "", false)
	if message != "No active jobs found. Use --all to include completed/failed/abandoned jobs." {
		t.Fatalf("expected --all hint, got %q", message)
	}
}

func TestTodoEmptyListMessageNoTodos(t *testing.T) {
	message := todoEmptyListMessage(0, "", false, false, false, false)
	if message != "No todos found." {
		t.Fatalf("expected no todos message, got %q", message)
	}
}

func TestTodoEmptyListMessageForStatusFilter(t *testing.T) {
	message := todoEmptyListMessage(2, "Done", false, false, false, false)
	if message != "No todos found with status done." {
		t.Fatalf("expected status message, got %q", message)
	}
}

func TestTodoEmptyListMessageSuggestsAll(t *testing.T) {
	message := todoEmptyListMessage(2, "", false, false, true, false)
	if message != "No todos found. Use --all to include done todos." {
		t.Fatalf("expected --all hint, got %q", message)
	}
}

func TestTodoEmptyListMessageSuggestsTombstones(t *testing.T) {
	message := todoEmptyListMessage(2, "", true, false, false, true)
	if message != "No todos found. Use --tombstones to include deleted todos." {
		t.Fatalf("expected --tombstones hint, got %q", message)
	}
}

func TestTodoEmptyListMessageSuggestsAllAndTombstones(t *testing.T) {
	message := todoEmptyListMessage(3, "", false, false, true, true)
	if message != "No todos found. Use --all to include done todos. Use --tombstones to include deleted todos." {
		t.Fatalf("expected combined hint, got %q", message)
	}
}
