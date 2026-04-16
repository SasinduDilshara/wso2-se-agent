package git

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

const defaultTimeout = 5 * time.Minute

func FetchUpstream(basePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "fetch", "upstream")
	cmd.Dir = basePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch upstream in %s failed: %w\n%s", basePath, err, string(output))
	}
	return nil
}

func CreateWorktree(basePath, worktreePath, branch, startPoint string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "worktree", "add", worktreePath, "-b", branch, startPoint)
	cmd.Dir = basePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add failed: %w\n%s", err, string(output))
	}
	return nil
}

func RemoveWorktree(basePath, worktreePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = basePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove failed: %w\n%s", err, string(output))
	}
	return nil
}

func PruneWorktrees(basePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "worktree", "prune")
	cmd.Dir = basePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree prune failed: %w\n%s", err, string(output))
	}
	return nil
}

func DeleteBranch(basePath, branch string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "branch", "-D", branch)
	cmd.Dir = basePath
	cmd.CombinedOutput() // ignore error — branch may not exist
	return nil
}
