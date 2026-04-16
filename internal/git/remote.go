package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func GetRemoteURL(repoPath, remoteName string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "remote", "get-url", remoteName)
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("remote '%s' not found in %s", remoteName, repoPath)
	}
	return strings.TrimSpace(string(output)), nil
}

func HasRemote(repoPath, remoteName string) bool {
	_, err := GetRemoteURL(repoPath, remoteName)
	return err == nil
}

func AddRemote(repoPath, remoteName, url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "remote", "add", remoteName, url)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git remote add %s failed: %w\n%s", remoteName, err, string(output))
	}
	return nil
}

// ValidateRemote checks that a remote URL contains the expected org/repo
func ValidateRemote(repoPath, remoteName, expectedOrgRepo string) error {
	url, err := GetRemoteURL(repoPath, remoteName)
	if err != nil {
		return err
	}

	// Normalize: extract org/repo from various URL formats
	// ssh: git@github.com:wso2/carbon-apimgt.git
	// https: https://github.com/wso2/carbon-apimgt.git
	normalized := url
	normalized = strings.TrimSuffix(normalized, ".git")
	parts := strings.Split(normalized, "/")
	if len(parts) >= 2 {
		actual := parts[len(parts)-2] + "/" + parts[len(parts)-1]
		// Handle SSH format: git@github.com:org/repo
		if strings.Contains(actual, ":") {
			actual = strings.Split(actual, ":")[1]
		}
		if actual != expectedOrgRepo {
			return fmt.Errorf("remote '%s' points to %s, expected %s", remoteName, actual, expectedOrgRepo)
		}
	}

	return nil
}
