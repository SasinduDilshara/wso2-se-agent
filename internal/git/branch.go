package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func CreateBranch(repoPath, branchName, startPoint string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "checkout", "-b", branchName, startPoint)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout -b %s failed: %w\n%s", branchName, err, string(output))
	}
	return nil
}

func CurrentBranch(repoPath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func TagExists(repoPath, tag string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "refs/tags/"+tag)
	cmd.Dir = repoPath
	return cmd.Run() == nil
}
