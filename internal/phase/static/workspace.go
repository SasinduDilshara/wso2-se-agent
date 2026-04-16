package static

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Tharsanan1/wso2-se-agent/internal/git"
	"github.com/Tharsanan1/wso2-se-agent/internal/phase"
	"github.com/Tharsanan1/wso2-se-agent/internal/state"
)

type WorkspacePhase struct{}

func NewWorkspacePhase() *WorkspacePhase { return &WorkspacePhase{} }

func (p *WorkspacePhase) Name() string        { return "workspace" }
func (p *WorkspacePhase) Type() phase.PhaseType { return phase.PhaseTypeStatic }

func (p *WorkspacePhase) Preconditions(ctx *phase.PhaseContext) error {
	return nil
}

func (p *WorkspacePhase) Execute(ctx *phase.PhaseContext) (*state.PhaseResult, error) {
	result := &state.PhaseResult{
		Phase:    "workspace",
		Metadata: make(map[string]any),
	}

	// Create workspace directories
	for _, dir := range []string{".wse", ".ai/logs", ".ai/screenshots"} {
		if err := os.MkdirAll(filepath.Join(ctx.Workspace, dir), 0755); err != nil {
			result.Status = state.StatusFailed
			result.Error = fmt.Sprintf("failed to create directory: %v", err)
			return result, fmt.Errorf("%s", result.Error)
		}
	}

	// Create git worktrees for each repo
	for _, repo := range ctx.ProductConfig.Repos {
		entry, ok := ctx.RepoRegistry.Repos[repo.Name]
		if !ok {
			result.Status = state.StatusFailed
			result.Error = fmt.Sprintf("repo %s not found in registry", repo.Name)
			return result, fmt.Errorf("%s", result.Error)
		}

		worktreePath := filepath.Join(ctx.Workspace, repo.Name)

		// Skip if worktree already exists
		if _, err := os.Stat(worktreePath); err == nil {
			ctx.Printer.Info(fmt.Sprintf("  Worktree already exists: %s", repo.Name))
			continue
		}

		// Fetch upstream
		ctx.Printer.Info(fmt.Sprintf("  Fetching upstream for %s...", repo.Name))
		if err := git.FetchUpstream(entry.LocalPath); err != nil {
			ctx.Printer.Info(fmt.Sprintf("  Warning: fetch upstream failed for %s: %v", repo.Name, err))
		}

		// Clean up any existing branch/worktree from a prior run
		branch := fmt.Sprintf("fix/issue-%s", ctx.IssueNumber)
		git.DeleteBranch(entry.LocalPath, branch)
		git.PruneWorktrees(entry.LocalPath)

		// Create worktree — try upstream/<branch> first, fall back to origin/<branch>
		startPoint := fmt.Sprintf("upstream/%s", repo.Branch)

		ctx.Printer.Info(fmt.Sprintf("  Creating worktree: %s (branch: %s from %s)", repo.Name, branch, startPoint))
		if err := git.CreateWorktree(entry.LocalPath, worktreePath, branch, startPoint); err != nil {
			// Fallback to origin/<branch>
			fallback := fmt.Sprintf("origin/%s", repo.Branch)
			ctx.Printer.Info(fmt.Sprintf("  Falling back to %s...", fallback))
			if err2 := git.CreateWorktree(entry.LocalPath, worktreePath, branch, fallback); err2 != nil {
				result.Status = state.StatusFailed
				result.Error = fmt.Sprintf("failed to create worktree for %s: %v (fallback: %v)", repo.Name, err, err2)
				return result, fmt.Errorf("%s", result.Error)
			}
		}

		ctx.State.Worktrees = append(ctx.State.Worktrees, state.WorktreeEntry{
			RepoName:  repo.Name,
			LocalPath: worktreePath,
			Branch:    branch,
			BasePath:  entry.LocalPath,
		})
	}

	// Copy product pack zip
	packSource := ctx.PackPath
	if packSource != "" {
		if _, err := os.Stat(packSource); err != nil {
			result.Status = state.StatusFailed
			result.Error = fmt.Sprintf("product pack not found at %s", packSource)
			return result, fmt.Errorf("%s", result.Error)
		}

		destPath := filepath.Join(ctx.Workspace, filepath.Base(packSource))
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			ctx.Printer.Info(fmt.Sprintf("  Copying product pack: %s", filepath.Base(packSource)))
			if err := copyFileWS(packSource, destPath); err != nil {
				result.Status = state.StatusFailed
				result.Error = fmt.Sprintf("failed to copy product pack: %v", err)
				return result, fmt.Errorf("%s", result.Error)
			}
		} else {
			ctx.Printer.Info(fmt.Sprintf("  Product pack already exists: %s", filepath.Base(packSource)))
		}
	}

	result.Status = state.StatusSuccess
	return result, nil
}

func copyFileWS(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func (p *WorkspacePhase) ExpectedArtifacts() []string {
	return []string{}
}
