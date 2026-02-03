package main

import (
	"fmt"
	"os"
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
	Short: "Show agent session logs",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentLogs,
}

var agentTranscriptCmd = &cobra.Command{
	Use:   "transcript <session-id>",
	Short: "Show readable transcript of agent session",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentTranscript,
}

var agentTailCmd = &cobra.Command{
	Use:   "tail <session-id>",
	Short: "Stream events from agent session (or show logs for completed sessions)",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentTail,
}

var agentListJSON bool
var agentListAll bool

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentListCmd, agentLogsCmd, agentTranscriptCmd, agentTailCmd)

	agentListCmd.Flags().BoolVar(&agentListJSON, "json", false, "Output as JSON")
	listflags.AddAllFlag(agentListCmd, &agentListAll)
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

	logs, err := store.Logs(repoPath, args[0])
	if err != nil {
		return err
	}

	fmt.Print(logs)
	return nil
}

func runAgentTranscript(cmd *cobra.Command, args []string) error {
	store, repoPath, err := openAgentStoreAndRepoPath()
	if err != nil {
		return err
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

	// Find the session to check its status
	session, err := store.FindSession(repoPath, args[0])
	if err != nil {
		return err
	}

	// For completed sessions, just show logs
	if session.Status != agent.SessionActive {
		logs, err := store.Logs(repoPath, session.ID)
		if err != nil {
			return err
		}
		fmt.Print(logs)
		return nil
	}

	// For active sessions, we would ideally stream events in real-time.
	// However, this requires additional infrastructure (watching the event log file).
	// For now, we print the current logs with a note that the session is still active.
	logs, err := store.Logs(repoPath, session.ID)
	if err != nil {
		return err
	}

	fmt.Print(logs)
	fmt.Fprintln(os.Stderr, "\n[Session is still active - showing current logs]")
	return nil
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
