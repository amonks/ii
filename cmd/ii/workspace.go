package main

import (
	"fmt"
	"time"

	"github.com/amonks/incrementum/internal/listflags"
	"github.com/amonks/incrementum/internal/ui"
	"github.com/amonks/incrementum/workspace"
	"github.com/spf13/cobra"
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
	Use:   "release [name]",
	Short: "Release an acquired workspace back to the pool",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runWorkspaceRelease,
}

var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all workspaces for the current repo",
	RunE:  runWorkspaceList,
}

var workspaceDestroyAllCmd = &cobra.Command{
	Use:   "destroy-all",
	Short: "Destroy all workspaces for the current repository",
	RunE:  runWorkspaceDestroyAll,
}

var (
	workspaceAcquireRev     string
	workspaceAcquirePurpose string
	workspaceListJSON       bool
	workspaceListAll        bool
)

func init() {
	rootCmd.AddCommand(workspaceCmd)
	workspaceCmd.AddCommand(workspaceAcquireCmd, workspaceReleaseCmd, workspaceListCmd, workspaceDestroyAllCmd)

	workspaceAcquireCmd.Flags().StringVar(&workspaceAcquireRev, "rev", "@", "Revision to base the new change on")
	workspaceAcquireCmd.Flags().StringVar(&workspaceAcquirePurpose, "purpose", "", "Purpose for acquiring the workspace")
	workspaceListCmd.Flags().BoolVar(&workspaceListJSON, "json", false, "Output as JSON")
	listflags.AddAllFlag(workspaceListCmd, &workspaceListAll)
}

func openWorkspacePoolAndRepoPath() (*workspace.Pool, string, error) {
	pool, err := workspace.Open()
	if err != nil {
		return nil, "", err
	}

	repoPath, err := getRepoPath()
	if err != nil {
		return nil, "", err
	}

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

func runWorkspaceRelease(cmd *cobra.Command, args []string) error {
	pool, repoPath, err := openWorkspacePoolAndRepoPath()
	if err != nil {
		return err
	}

	wsName, err := resolveWorkspaceName(args, pool)
	if err != nil {
		return err
	}

	if err := pool.ReleaseByName(repoPath, wsName); err != nil {
		return err
	}

	fmt.Printf("released workspace %s\n", wsName)
	return nil
}

func runWorkspaceList(cmd *cobra.Command, args []string) error {
	pool, repoPath, err := openWorkspacePoolAndRepoPath()
	if err != nil {
		return err
	}

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
