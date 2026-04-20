package static

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	configpkg "github.com/SasinduDilshara/wso2-se-agent/internal/config"
	"github.com/SasinduDilshara/wso2-se-agent/internal/phase"
	"github.com/SasinduDilshara/wso2-se-agent/internal/state"
)

type SkillsPhase struct{}

func NewSkillsPhase() *SkillsPhase { return &SkillsPhase{} }

func (p *SkillsPhase) Name() string        { return "skills" }
func (p *SkillsPhase) Type() phase.PhaseType { return phase.PhaseTypeStatic }

func (p *SkillsPhase) Preconditions(ctx *phase.PhaseContext) error {
	if ctx.ProductConfig.SkillsRepo == "" {
		return fmt.Errorf("skills_repo not set in product config for %s/%s", ctx.ProductConfig.Product, ctx.ProductConfig.Version)
	}
	return nil
}

func (p *SkillsPhase) Execute(ctx *phase.PhaseContext) (*state.PhaseResult, error) {
	result := &state.PhaseResult{
		Phase:    "skills",
		Metadata: make(map[string]any),
	}

	// Resolve the skills source directory
	skillsLocalPath, err := resolveSkillsPath(ctx)
	if err != nil {
		result.Status = state.StatusFailed
		result.Error = err.Error()
		return result, err
	}

	skillsRef := ctx.ProductConfig.SkillsRef
	srcDir := filepath.Join(skillsLocalPath, skillsRef)
	dstSkillsDir := filepath.Join(ctx.Workspace, ".claude", "skills")

	// Copy generic skills first (skills/ at repo root)
	genericRef := ctx.ProductConfig.GenericSkillsRef
	if genericRef == "" {
		genericRef = "skills"
	}
	genericSkillsDir := filepath.Join(skillsLocalPath, genericRef)
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

	// Copy CLAUDE.md from skills source
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
		ctx.Printer.Info(fmt.Sprintf("  Warning: CLAUDE.md not found at %s", srcClaude))
		if ctx.Printer.ConfirmProceed("  Generate a minimal CLAUDE.md instead?") {
			content := fmt.Sprintf("# %s %s — Issue #%s\n\nPort offset: %d\nIssue: %s\n",
				ctx.ProductConfig.Product, ctx.ProductConfig.Version,
				ctx.IssueNumber, offset, ctx.IssueURL)
			if err := os.WriteFile(dstClaude, []byte(content), 0644); err != nil {
				result.Status = state.StatusFailed
				result.Error = fmt.Sprintf("failed to write CLAUDE.md: %v", err)
				return result, fmt.Errorf("%s", result.Error)
			}
			ctx.Printer.Info("  Generated minimal CLAUDE.md")
		} else {
			result.Status = state.StatusFailed
			result.Error = "CLAUDE.md not found and user declined fallback"
			return result, fmt.Errorf("%s", result.Error)
		}
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

// resolveSkillsPath returns the local path to the skills repo contents.
// Downloads the tarball from GitHub and caches it.
func resolveSkillsPath(ctx *phase.PhaseContext) (string, error) {
	return downloadSkillsRepo(ctx.ProductConfig)
}

func downloadSkillsRepo(pcfg *configpkg.ProductConfig) (string, error) {
	repo := pcfg.SkillsRepo
	branch := pcfg.SkillsBranch
	if branch == "" {
		branch = "main"
	}

	// Cache directory
	cacheDir, err := configpkg.GetConfigDir()
	if err != nil {
		return "", err
	}
	cacheDir = filepath.Join(cacheDir, "cache", "skills")
	cacheKey := strings.ReplaceAll(repo, "/", "-") + "-" + branch
	cachedPath := filepath.Join(cacheDir, cacheKey)

	// Use cache if it exists
	if _, err := os.Stat(cachedPath); err == nil {
		return cachedPath, nil
	}

	// Download tarball using gh api
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}

	tarballPath := filepath.Join(cacheDir, cacheKey+".tar.gz")

	// Use shell redirection since older gh versions don't support --output
	tarballFile, err := os.Create(tarballPath)
	if err != nil {
		return "", fmt.Errorf("failed to create tarball file: %w", err)
	}

	cmd := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/tarball/%s", repo, branch))
	cmd.Stdout = tarballFile
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	tarballFile.Close()
	if err != nil {
		os.Remove(tarballPath)
		return "", fmt.Errorf("failed to download skills repo: %w", err)
	}

	// Extract tarball
	if err := extractTarball(tarballPath, cachedPath); err != nil {
		os.RemoveAll(cachedPath)
		return "", fmt.Errorf("failed to extract skills tarball: %w", err)
	}

	// Clean up tarball
	os.Remove(tarballPath)

	return cachedPath, nil
}

func extractTarball(tarballPath, destDir string) error {
	f, err := os.Open(tarballPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	// GitHub tarballs have a top-level directory like "owner-repo-commitsha/"
	// We need to strip that prefix
	stripPrefix := ""

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Detect the top-level directory prefix from the first entry
		if stripPrefix == "" {
			parts := strings.SplitN(header.Name, "/", 2)
			if len(parts) > 1 {
				stripPrefix = parts[0] + "/"
			}
		}

		// Strip the prefix
		name := strings.TrimPrefix(header.Name, stripPrefix)
		if name == "" {
			continue
		}

		target := filepath.Join(destDir, name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
			if err := os.Chmod(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		}
	}

	return nil
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
