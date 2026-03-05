package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/spf13/cobra"
	"monks.co/incrementum/internal/db"
	"monks.co/incrementum/internal/listflags"
	"monks.co/incrementum/internal/paths"
	"monks.co/incrementum/internal/ui"
	"monks.co/incrementum/workspace"
	"golang.org/x/term"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage a pool of jujutsu workspaces",
}

var workspaceAcquireCmd = &cobra.Command{
	Use:   "acquire",
	Short: "Acquire an available workspace or create a new one",
	RunE:  runWorkspaceAcquire,
}

var workspaceReleaseCmd = &cobra.Command{
	Use:   "release [name...]",
	Short: "Release one or more acquired workspaces back to the pool",
	RunE:  runWorkspaceRelease,
}

var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all workspaces for the current repo",
	RunE:  runWorkspaceList,
}

var workspaceExecCmd = &cobra.Command{
	Use:           "exec [flags] -- <command> [args...]",
	Short:         "Run a command in an acquired workspace",
	Args:          cobra.MinimumNArgs(1),
	SilenceUsage: true,
	RunE:          runWorkspaceExec,
}

var workspaceDestroyAllCmd = &cobra.Command{
	Use:   "destroy-all",
	Short: "Destroy all workspaces for the current repository",
	RunE:  runWorkspaceDestroyAll,
}

var (
	workspaceAcquireRev     string
	workspaceAcquirePurpose string
	workspaceExecRev        string
	workspaceExecPurpose    string
	workspaceListJSON       bool
	workspaceListAll        bool
)

func init() {
	rootCmd.AddCommand(workspaceCmd)
	workspaceCmd.AddCommand(workspaceAcquireCmd, workspaceReleaseCmd, workspaceListCmd, workspaceExecCmd, workspaceDestroyAllCmd)

	workspaceAcquireCmd.Flags().StringVar(&workspaceAcquireRev, "rev", "@", "Revision to base the new change on")
	workspaceAcquireCmd.Flags().StringVar(&workspaceAcquirePurpose, "purpose", "", "Purpose for acquiring the workspace")
	workspaceExecCmd.Flags().StringVar(&workspaceExecRev, "rev", "@", "Revision to base the new change on")
	workspaceExecCmd.Flags().StringVar(&workspaceExecPurpose, "purpose", "", "Purpose for acquiring the workspace (defaults to \"exec: <command>\")")
	workspaceListCmd.Flags().BoolVar(&workspaceListJSON, "json", false, "Output as JSON")
	listflags.AddAllFlag(workspaceListCmd, &workspaceListAll)
}

func openWorkspacePoolAndRepoPath() (*workspace.Pool, string, error) {
	repoPath, err := getRepoPath()
	if err != nil {
		return nil, "", err
	}

	stateDir := os.Getenv("INCREMENTUM_STATE_DIR")
	resolvedStateDir, err := paths.ResolveWithDefault(stateDir, paths.DefaultStateDir)
	if err != nil {
		return nil, "", err
	}

	workspacesDir, err := paths.ResolveWithDefault("", paths.DefaultWorkspacesDir)
	if err != nil {
		return nil, "", err
	}

	dbPath := filepath.Join(resolvedStateDir, "state.db")
	dbStore, err := db.Open(dbPath, db.OpenOptions{LegacyJSONPath: filepath.Join(resolvedStateDir, "state.json")})
	if err != nil {
		return nil, "", err
	}

	pool := workspace.NewPool(dbStore.SqlDB(), workspacesDir)
	pool.SetCloseFunc(dbStore.Close)
	return pool, repoPath, nil
}

func runWorkspaceAcquire(cmd *cobra.Command, args []string) error {
	if err := workspace.ValidateAcquirePurpose(workspaceAcquirePurpose); err != nil {
		return err
	}

	pool, repoPath, err := openWorkspacePoolAndRepoPath()
	if err != nil {
		return err
	}
	defer pool.Close()

	wsPath, err := pool.Acquire(repoPath, workspace.AcquireOptions{
		Rev:     workspaceAcquireRev,
		Purpose: workspaceAcquirePurpose,
	})
	if err != nil {
		return fmt.Errorf("acquire workspace: %w", err)
	}

	fmt.Println(wsPath)
	return nil
}

func runWorkspaceExec(cmd *cobra.Command, args []string) error {
	pool, repoPath, err := openWorkspacePoolAndRepoPath()
	if err != nil {
		return err
	}
	defer pool.Close()

	purpose := workspaceExecPurpose
	if purpose == "" {
		purpose = "exec: " + args[0]
	}

	wsPath, err := pool.Acquire(repoPath, workspace.AcquireOptions{
		Rev:     workspaceExecRev,
		Purpose: purpose,
	})
	if err != nil {
		return fmt.Errorf("acquire workspace: %w", err)
	}
	defer pool.Release(wsPath)

	c := exec.Command(args[0], args[1:]...)
	c.Dir = wsPath

	// Use a PTY so interactive programs see a terminal.
	ptmx, err := pty.Start(c)
	if err != nil {
		return fmt.Errorf("start command: %w", err)
	}
	defer ptmx.Close()

	// Handle window size changes.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			_ = pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	// Set initial size.
	_ = pty.InheritSize(os.Stdin, ptmx)

	// Put stdin into raw mode so keystrokes pass through.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err == nil {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}

	// Copy stdin→pty and pty→stdout concurrently.
	outDone := make(chan struct{})
	go func() {
		copyIO(ptmx, os.Stdin)
	}()
	go func() {
		defer close(outDone)
		copyIO(os.Stdout, ptmx)
	}()

	// Wait for the command to finish.
	cmdErr := c.Wait()

	// Wait for all output to be copied before returning.
	<-outDone

	// Stop relaying signals.
	signal.Stop(ch)
	close(ch)

	if cmdErr != nil {
		var exitErr *exec.ExitError
		if errors.As(cmdErr, &exitErr) {
			return exitError{code: exitErr.ExitCode()}
		}
		return cmdErr
	}
	return nil
}

func copyIO(dst *os.File, src *os.File) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			nw, writeErr := dst.Write(buf[:n])
			written += int64(nw)
			if writeErr != nil {
				return written, writeErr
			}
		}
		if readErr != nil {
			return written, readErr
		}
	}
}

func runWorkspaceRelease(cmd *cobra.Command, args []string) error {
	pool, repoPath, err := openWorkspacePoolAndRepoPath()
	if err != nil {
		return err
	}
	defer pool.Close()

	// If no args provided, resolve from current directory
	if len(args) == 0 {
		wsName, err := resolveWorkspaceName(nil, pool)
		if err != nil {
			return err
		}
		args = []string{wsName}
	}

	for _, wsName := range args {
		if err := pool.ReleaseByName(repoPath, wsName); err != nil {
			return err
		}
		fmt.Printf("released workspace %s\n", wsName)
	}
	return nil
}

func runWorkspaceList(cmd *cobra.Command, args []string) error {
	pool, repoPath, err := openWorkspacePoolAndRepoPath()
	if err != nil {
		return err
	}
	defer pool.Close()

	items, err := pool.List(repoPath)
	if err != nil {
		return fmt.Errorf("list workspaces: %w", err)
	}

	items = filterWorkspaceList(items, workspaceListAll)

	if workspaceListJSON {
		return encodeJSONToStdout(items)
	}

	if len(items) == 0 {
		fmt.Println("No workspaces found for this repository.")
		return nil
	}

	fmt.Print(formatWorkspaceTable(items, nil, time.Now()))
	return nil
}

func filterWorkspaceList(items []workspace.Info, includeAll bool) []workspace.Info {
	if includeAll {
		return items
	}

	filtered := make([]workspace.Info, 0, len(items))
	for _, item := range items {
		switch item.Status {
		case workspace.StatusAcquired, workspace.StatusAvailable:
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func runWorkspaceDestroyAll(cmd *cobra.Command, args []string) error {
	pool, repoPath, err := openWorkspacePoolAndRepoPath()
	if err != nil {
		return err
	}
	defer pool.Close()

	return pool.DestroyAll(repoPath)
}

func formatWorkspaceTable(items []workspace.Info, highlight func(string) string, now time.Time) string {
	if highlight == nil {
		highlight = func(value string) string { return value }
	}

	rows := make([][]string, 0, len(items))
	for _, item := range items {
		purpose := item.Purpose
		if purpose == "" {
			purpose = "-"
		}

		rev := item.Rev
		if rev == "" {
			rev = "-"
		}

		age := formatWorkspaceAge(item, now)
		duration := formatWorkspaceDuration(item, now)
		rows = append(rows, []string{
			highlight(item.Name),
			string(item.Status),
			age,
			duration,
			rev,
			ui.TruncateTableCell(purpose),
			ui.TruncateTableCell(item.Path),
		})
	}

	return ui.FormatTable([]string{"NAME", "STATUS", "AGE", "DURATION", "REV", "PURPOSE", "PATH"}, rows)
}

func formatWorkspaceAge(item workspace.Info, now time.Time) string {
	if item.CreatedAt.IsZero() {
		return "-"
	}
	return ui.FormatDurationShort(now.Sub(item.CreatedAt))
}

func formatWorkspaceDuration(item workspace.Info, now time.Time) string {
	if item.CreatedAt.IsZero() {
		return "-"
	}
	if item.Status == workspace.StatusAcquired {
		return ui.FormatDurationShort(now.Sub(item.CreatedAt))
	}
	if item.UpdatedAt.IsZero() {
		return "-"
	}
	return ui.FormatDurationShort(item.UpdatedAt.Sub(item.CreatedAt))
}
