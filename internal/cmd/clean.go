package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	gitutil "github.com/Tharsanan1/wso2-se-agent/internal/git"
	"github.com/Tharsanan1/wso2-se-agent/internal/state"
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

		// Prune stale worktree refs first, then delete branch
		gitutil.PruneWorktrees(wt.BasePath)
		gitutil.DeleteBranch(wt.BasePath, wt.Branch)
	}

	if cleanAll {
		fmt.Printf("Deleting workspace: %s\n", wsPath)
		// Ensure all files are writable before removal; extracted product
		// packs may contain read-only directories from server runtime.
		forceRemoveAll(wsPath)
	}

	fmt.Println("Clean complete.")
	return nil
}

// forceRemoveAll makes all directories writable then removes the tree.
// WSO2 product packs create read-only dirs that cause os.RemoveAll to fail.
func forceRemoveAll(root string) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			os.Chmod(path, 0755)
		}
		return nil
	})
	if err := os.RemoveAll(root); err != nil {
		fmt.Printf("  Warning: %v\n", err)
	}
}
