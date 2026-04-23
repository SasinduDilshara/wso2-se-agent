package static

import (
	"fmt"
	"io"
	"net"
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

		worktreePath := filepath.Join(ctx.Workspace, repo.WorktreeName())

		// Skip if worktree already exists
		if _, err := os.Stat(worktreePath); err == nil {
			ctx.Printer.Info(fmt.Sprintf("  Worktree already exists: %s", repo.WorktreeName()))
			continue
		}

		// Fetch upstream
		ctx.Printer.Info(fmt.Sprintf("  Fetching upstream for %s...", repo.Name))
		if err := git.FetchUpstream(entry.LocalPath); err != nil {
			ctx.Printer.Info(fmt.Sprintf("  Warning: fetch upstream failed for %s: %v", repo.Name, err))
		}

		// Clean up any existing branch/worktree from a prior run.
		// Prune first so stale worktree refs are removed before deleting the branch;
		// otherwise git refuses to delete a branch "checked out" in a stale worktree.
		branch := fmt.Sprintf("fix/issue-%s", ctx.IssueNumber)
		git.PruneWorktrees(entry.LocalPath)
		git.DeleteBranch(entry.LocalPath, branch)

		// Create worktree from upstream/<branch>
		startPoint := fmt.Sprintf("upstream/%s", repo.Branch)

		ctx.Printer.Info(fmt.Sprintf("  Creating worktree: %s (branch: %s from %s)", repo.Name, branch, startPoint))
		if err := git.CreateWorktree(entry.LocalPath, worktreePath, branch, startPoint); err != nil {
			// Ask user before falling back
			fallback := fmt.Sprintf("origin/%s", repo.Branch)
			ctx.Printer.Info(fmt.Sprintf("  Failed to create from %s: %v", startPoint, err))
			if ctx.Printer.ConfirmProceed(fmt.Sprintf("  Fall back to %s instead?", fallback)) {
				if err2 := git.CreateWorktree(entry.LocalPath, worktreePath, branch, fallback); err2 != nil {
					result.Status = state.StatusFailed
					result.Error = fmt.Sprintf("failed to create worktree for %s from %s: %v", repo.Name, fallback, err2)
					return result, fmt.Errorf("%s", result.Error)
				}
			} else {
				result.Status = state.StatusFailed
				result.Error = fmt.Sprintf("failed to create worktree for %s from %s", repo.Name, startPoint)
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

	// Allocate a free port offset for this workspace. The offset is added to every
	// default port the product listens on so concurrent runs don't collide, and is
	// exposed to pre/post hooks as WSE_PORT_OFFSET by the engine. Skipped when
	// pick_port_offset is false in product config.
	if ctx.ProductConfig.Runtime.ShouldPickPortOffset() {
		offset := findFreePortOffset(ctx.ProductConfig.Runtime.DefaultPorts)
		result.Metadata["port_offset"] = offset
		ctx.Printer.Info(fmt.Sprintf("  Allocated port offset: %d", offset))
	}

	result.Status = state.StatusSuccess
	return result, nil
}

func findFreePortOffset(defaultPorts []int) int {
	for offset := 0; offset <= 200; offset += 10 {
		allFree := true
		for _, port := range defaultPorts {
			addr := fmt.Sprintf(":%d", port+offset)
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				allFree = false
				break
			}
			ln.Close()
		}
		if allFree {
			return offset
		}
	}
	return 0
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
