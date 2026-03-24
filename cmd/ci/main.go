// Command ci is a CLI client for the monks.co CI system.
//
// It queries the CI server's JSON API to display run status, job details,
// and build logs. Designed for use by both humans and AI agents debugging
// CI failures.
//
// Usage:
//
//	go tool ci <command> [flags] [args]
//
// Commands:
//
//	runs          List recent CI runs
//	show <id>     Show details of a specific run
//	log <id>      Show failure logs for a run (smart mode)
//	log <id> <job> <stream>
//	              Show logs for a specific stream
//	deployments   List current deployments
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

const defaultBaseURL = "https://monks.co/ci"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ci: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		printUsage()
		return nil
	}

	cmd := os.Args[1]
	if cmd == "-help" || cmd == "--help" || cmd == "help" {
		printUsage()
		return nil
	}

	baseURL := os.Getenv("CI_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	switch cmd {
	case "runs":
		return cmdRuns(baseURL, os.Args[2:])
	case "show":
		return cmdShow(baseURL, os.Args[2:])
	case "log":
		return cmdLog(baseURL, os.Args[2:])
	case "deployments":
		return cmdDeployments(baseURL, os.Args[2:])
	default:
		return fmt.Errorf("unknown command %q; run 'go tool ci -help' for usage", cmd)
	}
}

func printUsage() {
	fmt.Fprint(os.Stderr, `Usage: go tool ci <command> [flags] [args]

Commands:
  runs          List recent CI runs
  show <id>     Show run details (jobs and streams)
  log <id>      Show failure logs (first failed stream, last N lines)
  deployments   List current app deployments

Run 'go tool ci <command> -help' for command-specific help.

Environment:
  CI_URL    Base URL of the CI server (default: https://monks.co/ci)
`)
}

// --- runs command ---

func cmdRuns(baseURL string, args []string) error {
	fs := flag.NewFlagSet("runs", flag.ExitOnError)
	limit := fs.Int("n", 10, "number of runs to show")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: go tool ci runs [flags]

List recent CI runs with their status, commit SHA, and timing.

Flags:
  -n int    Number of runs to show (default 10, max 100)
`)
	}
	fs.Parse(args)

	var runs []apiRun
	if err := getJSON(fmt.Sprintf("%s/api/runs?limit=%d", baseURL, *limit), &runs); err != nil {
		return err
	}

	if len(runs) == 0 {
		fmt.Println("No runs found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tSHA\tTRIGGER\tSTARTED\tDURATION")
	for _, r := range runs {
		sha := r.HeadSHA
		if len(sha) > 8 {
			sha = sha[:8]
		}
		started := formatTime(r.StartedAt)
		dur := ""
		if r.FinishedAt != nil {
			dur = formatDuration(r.StartedAt, *r.FinishedAt)
		} else if r.Status == "running" {
			dur = "running"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
			r.ID, statusStr(r.Status), sha, r.Trigger, started, dur)
	}
	w.Flush()
	return nil
}

// --- show command ---

func cmdShow(baseURL string, args []string) error {
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: go tool ci show <run-id>

Show detailed information about a CI run including all jobs and their
streams with status, duration, and error messages.

If <run-id> is "latest", shows the most recent run.
`)
	}
	fs.Parse(args)
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("expected exactly one argument")
	}

	runID, err := resolveRunID(baseURL, fs.Arg(0))
	if err != nil {
		return err
	}

	var state runState
	if err := getJSON(fmt.Sprintf("%s/api/runs/%d", baseURL, runID), &state); err != nil {
		return err
	}

	printRunDetail(&state)
	return nil
}

// --- log command ---

func cmdLog(baseURL string, args []string) error {
	fs := flag.NewFlagSet("log", flag.ExitOnError)
	lines := fs.Int("n", 80, "number of log lines to show")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: go tool ci log [flags] <run-id> [<job> <stream>]

Show build logs for a CI run.

Without <job> and <stream> arguments, operates in "smart" mode:
  - Finds the first stream in the run that failed
  - Shows the last N lines of that stream's log
  - Skips subsequent jobs/streams (which typically just show
    "context canceled" after the first real failure)
  - Includes a header identifying which stream is shown

With <job> and <stream> arguments, shows the full log for that
specific stream (stream names use ~ instead of / in URLs, but
you can use either separator here).

If <run-id> is "latest", uses the most recent run.

Flags:
  -n int    Number of trailing log lines to show in smart mode (default 80)

Examples:
  go tool ci log latest           Show why the latest run failed
  go tool ci log 42               Show why run 42 failed
  go tool ci log 42 test monks.co   Show full test log for monks.co
`)
	}
	fs.Parse(args)

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("expected at least one argument")
	}

	runID, err := resolveRunID(baseURL, fs.Arg(0))
	if err != nil {
		return err
	}

	// Specific stream mode.
	if fs.NArg() == 3 {
		job := fs.Arg(1)
		stream := strings.ReplaceAll(fs.Arg(2), "/", "~")
		return fetchAndPrintLog(baseURL, runID, job, stream, 0)
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("expected 1 or 3 arguments, got %d", fs.NArg())
	}

	// Smart mode: find the first failure.
	var state runState
	if err := getJSON(fmt.Sprintf("%s/api/runs/%d", baseURL, runID), &state); err != nil {
		return err
	}

	if state.Run.Status == "running" {
		return fmt.Errorf("run %d is still running", runID)
	}
	if state.Run.Status == "success" {
		fmt.Printf("Run %d succeeded.\n", runID)
		return nil
	}

	// Find first failed stream.
	job, stream, found := findFirstFailure(&state)
	if !found {
		// No individual stream failed — show run-level error.
		if state.Run.Error != nil {
			fmt.Printf("Run %d %s: %s\n", runID, state.Run.Status, *state.Run.Error)
		} else {
			fmt.Printf("Run %d %s (no failed streams found)\n", runID, state.Run.Status)
		}
		return nil
	}

	fmt.Fprintf(os.Stderr, "--- run %d | job: %s | stream: %s ---\n", runID, job, strings.ReplaceAll(stream, "~", "/"))
	return fetchAndPrintLog(baseURL, runID, job, stream, *lines)
}

// --- deployments command ---

func cmdDeployments(baseURL string, args []string) error {
	fs := flag.NewFlagSet("deployments", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: go tool ci deployments

List current app deployments showing the app name, deployed commit,
and deployment time.
`)
	}
	fs.Parse(args)

	var deployments []apiDeployment
	if err := getJSON(fmt.Sprintf("%s/api/deployments", baseURL), &deployments); err != nil {
		return err
	}

	if len(deployments) == 0 {
		fmt.Println("No deployments found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "APP\tSHA\tDEPLOYED")
	for _, d := range deployments {
		sha := d.CommitSHA
		if len(sha) > 8 {
			sha = sha[:8]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", d.App, sha, formatTime(d.DeployedAt))
	}
	w.Flush()
	return nil
}

// --- API types ---

type apiRun struct {
	ID         int64   `json:"ID"`
	HeadSHA    string  `json:"HeadSHA"`
	BaseSHA    string  `json:"BaseSHA"`
	StartedAt  string  `json:"StartedAt"`
	FinishedAt *string `json:"FinishedAt"`
	Status     string  `json:"Status"`
	Trigger    string  `json:"Trigger"`
	Error      *string `json:"Error"`
}

type runState struct {
	Run     runJSON                 `json:"run"`
	Jobs    []jobJSON               `json:"jobs"`
	Streams map[string][]streamJSON `json:"streams"`
}

type runJSON struct {
	ID         int64   `json:"id"`
	Status     string  `json:"status"`
	HeadSHA    string  `json:"head_sha"`
	BaseSHA    string  `json:"base_sha"`
	Trigger    string  `json:"trigger"`
	StartedAt  string  `json:"started_at"`
	FinishedAt *string `json:"finished_at,omitempty"`
	Error      *string `json:"error,omitempty"`
}

type jobJSON struct {
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	Status     string  `json:"status"`
	DurationMs *int64  `json:"duration_ms,omitempty"`
	Error      *string `json:"error,omitempty"`
}

type streamJSON struct {
	Name        string  `json:"name"`
	DisplayName string  `json:"display_name"`
	Status      string  `json:"status"`
	DurationMs  *int64  `json:"duration_ms,omitempty"`
	Error       *string `json:"error,omitempty"`
}

type apiDeployment struct {
	App        string `json:"App"`
	CommitSHA  string `json:"CommitSHA"`
	ImageRef   string `json:"ImageRef"`
	DeployedAt string `json:"DeployedAt"`
}

// --- helpers ---

func getJSON(url string, v any) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, url, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func resolveRunID(baseURL, arg string) (int64, error) {
	if arg == "latest" {
		var runs []apiRun
		if err := getJSON(fmt.Sprintf("%s/api/runs?limit=1", baseURL), &runs); err != nil {
			return 0, err
		}
		if len(runs) == 0 {
			return 0, fmt.Errorf("no runs found")
		}
		return runs[0].ID, nil
	}
	id, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid run ID %q (use a number or \"latest\")", arg)
	}
	return id, nil
}

func findFirstFailure(state *runState) (job, stream string, found bool) {
	for _, j := range state.Jobs {
		if j.Status != "failed" {
			continue
		}
		streams := state.Streams[j.Name]
		for _, s := range streams {
			if s.Status == "failed" {
				return j.Name, s.Name, true
			}
		}
		// Job failed but no stream marked failed — return first stream.
		if len(streams) > 0 {
			return j.Name, streams[0].Name, true
		}
		return j.Name, "", true
	}
	return "", "", false
}

func fetchAndPrintLog(baseURL string, runID int64, job, stream string, tailLines int) error {
	if stream == "" {
		return fmt.Errorf("no output stream found for job %q", job)
	}
	url := fmt.Sprintf("%s/output/%d/%s/%s", baseURL, runID, job, stream)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("fetching log: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d fetching log", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading log: %w", err)
	}

	output := string(body)
	if tailLines > 0 {
		output = lastNLines(output, tailLines)
	}
	fmt.Print(output)
	if len(output) > 0 && !strings.HasSuffix(output, "\n") {
		fmt.Println()
	}
	return nil
}

func lastNLines(s string, n int) string {
	s = strings.TrimRight(s, "\n")
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s + "\n"
	}
	return strings.Join(lines[len(lines)-n:], "\n") + "\n"
}

func printRunDetail(state *runState) {
	r := state.Run
	fmt.Printf("Run %d  %s  sha:%s  trigger:%s\n", r.ID, statusStr(r.Status), shortSHA(r.HeadSHA), r.Trigger)
	fmt.Printf("Started: %s", formatTime(r.StartedAt))
	if r.FinishedAt != nil {
		fmt.Printf("  Duration: %s", formatDuration(r.StartedAt, *r.FinishedAt))
	}
	fmt.Println()
	if r.Error != nil {
		fmt.Printf("Error: %s\n", *r.Error)
	}
	fmt.Println()

	for _, j := range state.Jobs {
		dur := ""
		if j.DurationMs != nil {
			dur = fmt.Sprintf(" (%s)", fmtMs(*j.DurationMs))
		}
		fmt.Printf("  %s %s%s\n", statusStr(j.Status), j.Name, dur)
		if j.Error != nil {
			fmt.Printf("    error: %s\n", *j.Error)
		}

		streams := state.Streams[j.Name]
		for _, s := range streams {
			sdur := ""
			if s.DurationMs != nil {
				sdur = fmt.Sprintf(" (%s)", fmtMs(*s.DurationMs))
			}
			fmt.Printf("    %s %s%s\n", statusStr(s.Status), s.DisplayName, sdur)
			if s.Error != nil {
				fmt.Printf("      error: %s\n", *s.Error)
			}
		}
	}
}

func statusStr(s string) string {
	switch s {
	case "success":
		return "ok"
	case "failed":
		return "FAIL"
	case "running", "in_progress":
		return "..."
	case "pending":
		return "   "
	case "skipped":
		return "skip"
	case "cancelled":
		return "cancel"
	case "superseded":
		return "super"
	default:
		return s
	}
}

func shortSHA(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func formatTime(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Local().Format("Jan 02 15:04")
}

func formatDuration(start, end string) string {
	s, err := time.Parse(time.RFC3339, start)
	if err != nil {
		return ""
	}
	e, err := time.Parse(time.RFC3339, end)
	if err != nil {
		return ""
	}
	return fmtMs(e.Sub(s).Milliseconds())
}

func fmtMs(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	if d < time.Second {
		return fmt.Sprintf("%dms", ms)
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}
