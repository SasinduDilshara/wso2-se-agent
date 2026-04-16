package script

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

type EnvVars struct {
	Workspace   string
	IssueNumber string
	IssueURL    string
	Product     string
	Version     string
	PortOffset  string
	StateFile   string
}

func Run(scriptPath string, env EnvVars, timeout time.Duration) error {
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return nil // script doesn't exist, skip
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", scriptPath)
	cmd.Dir = env.Workspace
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"WSE_WORKSPACE="+env.Workspace,
		"WSE_ISSUE_NUMBER="+env.IssueNumber,
		"WSE_ISSUE_URL="+env.IssueURL,
		"WSE_PRODUCT="+env.Product,
		"WSE_VERSION="+env.Version,
		"WSE_PORT_OFFSET="+env.PortOffset,
		"WSE_STATE_FILE="+env.StateFile,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("script %s failed: %w", scriptPath, err)
	}
	return nil
}

func Exists(scriptPath string) bool {
	_, err := os.Stat(scriptPath)
	return err == nil
}
