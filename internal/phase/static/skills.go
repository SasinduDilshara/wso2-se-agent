package static

import (
	"archive/tar"
	"compress/gzip"
	b64 "encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	configpkg "github.com/Tharsanan1/wso2-se-agent/internal/config"
	"github.com/Tharsanan1/wso2-se-agent/internal/phase"
	"github.com/Tharsanan1/wso2-se-agent/internal/state"
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

	dstSkillsDir := filepath.Join(ctx.Workspace, ".claude", "skills")

	// Copy generic skills from global config (separate repo)
	gcfg := ctx.GlobalConfig
	if gcfg.GenericSkillsRepo != "" {
		genericLocalPath, err := downloadSkillsRepo(gcfg.GenericSkillsRepo, gcfg.GenericSkillsBranch)
		if err != nil {
			result.Status = state.StatusFailed
			result.Error = fmt.Sprintf("failed to download generic skills repo: %v", err)
			return result, fmt.Errorf("%s", result.Error)
		}
		genericRef := gcfg.GenericSkillsRef
		if genericRef == "" {
			genericRef = "skills"
		}
		genericSkillsDir := filepath.Join(genericLocalPath, genericRef)
		if _, err := os.Stat(genericSkillsDir); err == nil {
			if err := copyDir(genericSkillsDir, dstSkillsDir); err != nil {
				result.Status = state.StatusFailed
				result.Error = fmt.Sprintf("failed to copy generic skills: %v", err)
				return result, fmt.Errorf("%s", result.Error)
			}
			ctx.Printer.Info("  Installed generic skills")
		}
	}

	// Copy product-specific skills (override generic ones if same name)
	productLocalPath, err := downloadSkillsRepo(ctx.ProductConfig.SkillsRepo, ctx.ProductConfig.SkillsBranch)
	if err != nil {
		result.Status = state.StatusFailed
		result.Error = fmt.Sprintf("failed to download product skills repo: %v", err)
		return result, fmt.Errorf("%s", result.Error)
	}

	skillsRef := ctx.ProductConfig.SkillsRef
	srcDir := filepath.Join(productLocalPath, skillsRef)
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

	// Install CLAUDE.md
	dstClaude := filepath.Join(ctx.Workspace, "CLAUDE.md")
	claudeMDInstalled := false

	// Option 1: Download from custom URL if configured
	if ctx.ProductConfig.ClaudeMDURL != "" {
		ctx.Printer.Info("  Downloading CLAUDE.md from custom URL...")
		if err := downloadClaudeMD(ctx.ProductConfig.ClaudeMDURL, dstClaude); err != nil {
			ctx.Printer.Info(fmt.Sprintf("  Warning: failed to download CLAUDE.md: %v", err))
		} else {
			ctx.Printer.Info("  Downloaded CLAUDE.md from custom URL")
			claudeMDInstalled = true
		}
	}

	// Option 2: Copy from skills repo
	if !claudeMDInstalled {
		srcClaude := filepath.Join(srcDir, "CLAUDE.md")
		if _, err := os.Stat(srcClaude); err == nil {
			if err := copyFile(srcClaude, dstClaude); err != nil {
				result.Status = state.StatusFailed
				result.Error = fmt.Sprintf("failed to copy CLAUDE.md: %v", err)
				return result, fmt.Errorf("%s", result.Error)
			}
			ctx.Printer.Info("  Copied CLAUDE.md from skills repo")
			claudeMDInstalled = true
		}
	}

	// Option 3: Generate minimal fallback
	if !claudeMDInstalled {
		ctx.Printer.Info("  Warning: CLAUDE.md not found from URL or skills repo")
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

// downloadClaudeMD fetches a CLAUDE.md from a GitHub URL using gh api.
// Accepts URLs like https://github.com/org/repo/blob/branch/path/CLAUDE.md
func downloadClaudeMD(ghURL, destPath string) error {
	// Convert GitHub blob URL to API path: org/repo and path
	// e.g., https://github.com/org/repo/blob/main/CLAUDE.md
	//     → repos/org/repo/contents/CLAUDE.md?ref=main
	ghURL = strings.TrimSuffix(ghURL, "/")
	parts := strings.SplitN(ghURL, "github.com/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid GitHub URL: %s", ghURL)
	}

	// org/repo/blob/branch/path...
	segments := strings.SplitN(parts[1], "/blob/", 2)
	if len(segments) != 2 {
		return fmt.Errorf("expected GitHub blob URL: %s", ghURL)
	}

	orgRepo := segments[0] // org/repo
	branchAndPath := segments[1]
	slashIdx := strings.Index(branchAndPath, "/")
	if slashIdx < 0 {
		return fmt.Errorf("no file path in URL: %s", ghURL)
	}
	branch := branchAndPath[:slashIdx]
	filePath := branchAndPath[slashIdx+1:]

	apiPath := fmt.Sprintf("repos/%s/contents/%s?ref=%s", orgRepo, filePath, branch)

	cmd := exec.Command("gh", "api", apiPath, "--jq", ".content")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("gh api failed: %w", err)
	}

	// GitHub API returns base64-encoded content
	import64 := strings.TrimSpace(string(out))
	decoded, err := base64Decode(import64)
	if err != nil {
		return fmt.Errorf("failed to decode content: %w", err)
	}

	return os.WriteFile(destPath, decoded, 0644)
}

func base64Decode(s string) ([]byte, error) {
	// GitHub API returns base64 with newlines
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\\n", "")
	return b64.StdEncoding.DecodeString(s)
}

func (p *SkillsPhase) ExpectedArtifacts() []string {
	return []string{}
}

func downloadSkillsRepo(repo, branch string) (string, error) {
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
