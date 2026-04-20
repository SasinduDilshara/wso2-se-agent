package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	gitutil "github.com/SasinduDilshara/wso2-se-agent/internal/git"
	"github.com/SasinduDilshara/wso2-se-agent/internal/state"
)

var cleanAll bool

var cleanCmd = &cobra.Command{
	Use:   "clean [workspace-path]",
	Short: "Remove worktrees and clean up a workspace",
	Args:  cobra.ExactArgs(1),
	RunE:  runClean,
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanAll, "all", false, "Also delete the workspace directory")
}

func runClean(cmd *cobra.Command, args []string) error {
	wsPath := args[0]

	ws, err := state.Load(wsPath)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Remove worktrees
	for _, wt := range ws.Worktrees {
		fmt.Printf("Removing worktree: %s\n", wt.RepoName)
		if err := gitutil.RemoveWorktree(wt.BasePath, wt.LocalPath); err != nil {
			fmt.Printf("  Warning: %v\n", err)
		}

		// Clean up branch
		gitutil.DeleteBranch(wt.BasePath, wt.Branch)

		// Prune
		gitutil.PruneWorktrees(wt.BasePath)
	}

	if cleanAll {
		fmt.Printf("Deleting workspace: %s\n", wsPath)
		if err := os.RemoveAll(wsPath); err != nil {
			return fmt.Errorf("failed to delete workspace: %w", err)
		}
	}

	fmt.Println("Clean complete.")
	return nil
}
