package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/amonks/incrementum/agent"
	"github.com/amonks/incrementum/internal/listflags"
	"github.com/amonks/incrementum/internal/ui"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agent sessions",
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agent sessions",
	RunE:  runAgentList,
}

var agentLogsCmd = &cobra.Command{
	Use:   "logs <session-id>",
	Short: "Show agent session logs snapshot",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentLogs,
}

var agentTailCmd = &cobra.Command{
	Use:   "tail <session-id>",
	Short: "Stream transcript from agent session until it ends",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentTail,
}

var agentLogsJSON bool
var agentTailJSON bool

var agentListJSON bool
var agentListAll bool

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentListCmd, agentLogsCmd, agentTailCmd)

	agentListCmd.Flags().BoolVar(&agentListJSON, "json", false, "Output as JSON")
	listflags.AddAllFlag(agentListCmd, &agentListAll)

	agentLogsCmd.Flags().BoolVar(&agentLogsJSON, "json", false, "Output as JSONL")
	agentTailCmd.Flags().BoolVar(&agentTailJSON, "json", false, "Output as JSONL")
}

func runAgentList(cmd *cobra.Command, args []string) error {
	store, repoPath, err := openAgentStoreAndRepoPath()
	if err != nil {
		return err
	}

	sessions, err := store.ListSessions(repoPath)
	if err != nil {
		return fmt.Errorf("list agent sessions: %w", err)
	}

	allSessions := sessions
	sessions = filterAgentSessionsForList(sessions, agentListAll)

	if agentListJSON {
		return encodeJSONToStdout(sessions)
	}

	if len(sessions) == 0 {
		fmt.Println(agentEmptyListMessage(len(allSessions), agentListAll))
		return nil
	}

	prefixLengths := agentSessionPrefixLengths(allSessions)
	fmt.Print(formatAgentTable(sessions, ui.HighlightID, time.Now(), prefixLengths))
	return nil
}

func runAgentLogs(cmd *cobra.Command, args []string) error {
	store, repoPath, err := openAgentStoreAndRepoPath()
	if err != nil {
		return err
	}

	if agentLogsJSON {
		logContent, err := store.Logs(repoPath, args[0])
		if err != nil {
			return err
		}
		fmt.Print(logContent)
		return nil
	}

	transcript, err := store.Transcript(repoPath, args[0])
	if err != nil {
		return err
	}

	fmt.Print(transcript)
	return nil
}

func runAgentTail(cmd *cobra.Command, args []string) error {
	store, repoPath, err := openAgentStoreAndRepoPath()
	if err != nil {
		return err
	}

	if agentTailJSON {
		// Snapshot JSONL output for now; real tailing of JSONL will come with event log streaming.
		logContent, err := store.Logs(repoPath, args[0])
		if err != nil {
			return err
		}
		fmt.Print(logContent)
		return nil
	}

	// Tail currently polls until the session ends and prints only newly appended
	// transcript content each poll (placeholder until true event streaming lands).
	var last string
	for {
		session, err := store.FindSession(repoPath, args[0])
		if err != nil {
			return err
		}

		transcript, err := store.Transcript(repoPath, session.ID)
		if err != nil {
			return err
		}

		fmt.Print(agent.TranscriptTailDiff(last, transcript))
		last = transcript

		if session.Status != agent.SessionActive {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func filterAgentSessionsForList(sessions []agent.Session, includeAll bool) []agent.Session {
	if includeAll {
		return sessions
	}

	// Filter to only active sessions
	var active []agent.Session
	for _, s := range sessions {
		if s.Status == agent.SessionActive {
			active = append(active, s)
		}
	}
	return active
}

func formatAgentTable(sessions []agent.Session, highlight func(string, int) string, now time.Time, prefixLengths map[string]int) string {
	rows := make([][]string, 0, len(sessions))
	highlight, prefixLengths = normalizeAgentTableInputs(sessions, highlight, prefixLengths)

	for _, session := range sessions {
		age := formatAgentAge(session, now)
		duration := formatAgentDuration(session, now)
		tokens := "-"
		if session.TokensUsed > 0 {
			tokens = strconv.Itoa(session.TokensUsed)
		}
		cost := "-"
		if session.Cost > 0 {
			cost = fmt.Sprintf("$%.4f", session.Cost)
		}
		prefixLen := ui.PrefixLength(prefixLengths, session.ID)

		rows = append(rows, []string{
			highlight(session.ID, prefixLen),
			string(session.Status),
			session.Model,
			age,
			duration,
			tokens,
			cost,
		})
	}

	return ui.FormatTable([]string{"SESSION", "STATUS", "MODEL", "AGE", "DURATION", "TOKENS", "COST"}, rows)
}

func agentSessionPrefixLengths(sessions []agent.Session) map[string]int {
	sessionIDs := make([]string, 0, len(sessions))
	for _, session := range sessions {
		sessionIDs = append(sessionIDs, session.ID)
	}
	return ui.UniqueIDPrefixLengths(sessionIDs)
}

func normalizeAgentTableInputs(sessions []agent.Session, highlight func(string, int) string, prefixLengths map[string]int) (func(string, int) string, map[string]int) {
	if highlight == nil {
		highlight = func(value string, prefix int) string { return value }
	}
	if prefixLengths == nil {
		prefixLengths = agentSessionPrefixLengths(sessions)
	}
	return highlight, prefixLengths
}

func formatAgentAge(session agent.Session, now time.Time) string {
	if session.CreatedAt.IsZero() {
		return "-"
	}
	return ui.FormatDurationShort(now.Sub(session.CreatedAt))
}

func formatAgentDuration(session agent.Session, now time.Time) string {
	if session.CreatedAt.IsZero() {
		return "-"
	}

	// For active sessions, use now - created_at
	// For completed/failed sessions, use completed_at - created_at (or updated_at as fallback)
	if session.Status == agent.SessionActive {
		return ui.FormatDurationShort(now.Sub(session.CreatedAt))
	}

	if !session.CompletedAt.IsZero() {
		return ui.FormatDurationShort(session.CompletedAt.Sub(session.CreatedAt))
	}

	if !session.UpdatedAt.IsZero() {
		return ui.FormatDurationShort(session.UpdatedAt.Sub(session.CreatedAt))
	}

	return "-"
}
