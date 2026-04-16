package static

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/Tharsanan1/wso2-se-agent/internal/phase"
	"github.com/Tharsanan1/wso2-se-agent/internal/state"
)

type SkillsPhase struct{}

func NewSkillsPhase() *SkillsPhase { return &SkillsPhase{} }

func (p *SkillsPhase) Name() string        { return "skills" }
func (p *SkillsPhase) Type() phase.PhaseType { return phase.PhaseTypeStatic }

func (p *SkillsPhase) Preconditions(ctx *phase.PhaseContext) error {
	if ctx.GlobalConfig.SkillsRepoPath == "" {
		return fmt.Errorf("skills_repo_path not set in config. Run: wso2-se-agent config init")
	}
	skillsRef := ctx.ProductConfig.SkillsRef
	srcDir := filepath.Join(ctx.GlobalConfig.SkillsRepoPath, skillsRef)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("skills source not found at %s", srcDir)
	}
	return nil
}

func (p *SkillsPhase) Execute(ctx *phase.PhaseContext) (*state.PhaseResult, error) {
	result := &state.PhaseResult{
		Phase:    "skills",
		Metadata: make(map[string]any),
	}

	skillsRef := ctx.ProductConfig.SkillsRef
	srcDir := filepath.Join(ctx.GlobalConfig.SkillsRepoPath, skillsRef)

	dstSkillsDir := filepath.Join(ctx.Workspace, ".claude", "skills")

	// Copy generic skills first (skills/ at repo root)
	genericSkillsDir := filepath.Join(ctx.GlobalConfig.SkillsRepoPath, "skills")
	if _, err := os.Stat(genericSkillsDir); err == nil {
		if err := copyDir(genericSkillsDir, dstSkillsDir); err != nil {
			result.Status = state.StatusFailed
			result.Error = fmt.Sprintf("failed to copy generic skills: %v", err)
			return result, fmt.Errorf("%s", result.Error)
		}
		ctx.Printer.Info("  Installed generic skills")
	}

	// Copy product-specific skills (override generic ones if same name)
	srcSkillsDir := filepath.Join(srcDir, "skills")
	if _, err := os.Stat(srcSkillsDir); err == nil {
		if err := copyDir(srcSkillsDir, dstSkillsDir); err != nil {
			result.Status = state.StatusFailed
			result.Error = fmt.Sprintf("failed to copy product skills: %v", err)
			return result, fmt.Errorf("%s", result.Error)
		}
		ctx.Printer.Info(fmt.Sprintf("  Installed product skills from %s", skillsRef))
	}

	// Allocate port offset
	offset := findFreePortOffset(ctx.ProductConfig.Runtime.DefaultPorts)
	result.Metadata["port_offset"] = offset
	ctx.Printer.Info(fmt.Sprintf("  Allocated port offset: %d", offset))

	// Copy CLAUDE.md from skills source — this is the product-specific context
	srcClaude := filepath.Join(srcDir, "CLAUDE.md")
	dstClaude := filepath.Join(ctx.Workspace, "CLAUDE.md")
	if _, err := os.Stat(srcClaude); err == nil {
		if err := copyFile(srcClaude, dstClaude); err != nil {
			result.Status = state.StatusFailed
			result.Error = fmt.Sprintf("failed to copy CLAUDE.md: %v", err)
			return result, fmt.Errorf("%s", result.Error)
		}
		ctx.Printer.Info("  Copied CLAUDE.md from skills repo")
	} else {
		// Fallback: generate a minimal CLAUDE.md
		content := fmt.Sprintf("# %s %s — Issue #%s\n\nPort offset: %d\nIssue: %s\n",
			ctx.ProductConfig.Product, ctx.ProductConfig.Version,
			ctx.IssueNumber, offset, ctx.IssueURL)
		if err := os.WriteFile(dstClaude, []byte(content), 0644); err != nil {
			result.Status = state.StatusFailed
			result.Error = fmt.Sprintf("failed to write CLAUDE.md: %v", err)
			return result, fmt.Errorf("%s", result.Error)
		}
		ctx.Printer.Info("  Generated minimal CLAUDE.md")
	}

	// Initialize state
	ctx.State.IssueURL = ctx.IssueURL
	ctx.State.IssueNumber = ctx.IssueNumber
	ctx.State.Product = ctx.ProductConfig.Product
	ctx.State.Version = ctx.ProductConfig.Version

	result.Status = state.StatusSuccess
	return result, nil
}

func (p *SkillsPhase) ExpectedArtifacts() []string {
	return []string{}
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

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		return copyFile(path, dstPath)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

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
