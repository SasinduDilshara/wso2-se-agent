package static

import (
	"archive/tar"
	"compress/gzip"
	b64 "encoding/base64"
	"fmt"
	"io"
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
		genericLocalPath, err := downloadSkillsRepo(gcfg.GenericSkillsRepo, gcfg.GenericSkillsBranch, gcfg.SkillsCache)
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
	productLocalPath, err := downloadSkillsRepo(ctx.ProductConfig.SkillsRepo, ctx.ProductConfig.SkillsBranch, ctx.GlobalConfig.SkillsCache)
	if err != nil {
		result.Status = state.StatusFailed
		result.Error = fmt.Sprintf("failed to download product skills repo: %v", err)
		return result, fmt.Errorf("%s", result.Error)
	}

	// skills_ref is the direct path (relative to the skills repo) of the dir
	// that holds the product skills — same convention as GenericSkillsRef on
	// lines 50–54. Example: skills_ref: "skills" → <repo>/skills/.
	skillsRef := ctx.ProductConfig.SkillsRef
	srcDir := filepath.Join(productLocalPath, skillsRef)
	srcSkillsDir := srcDir
	if _, err := os.Stat(srcSkillsDir); err == nil {
		if err := copyDir(srcSkillsDir, dstSkillsDir); err != nil {
			result.Status = state.StatusFailed
			result.Error = fmt.Sprintf("failed to copy product skills: %v", err)
			return result, fmt.Errorf("%s", result.Error)
		}
		ctx.Printer.Info(fmt.Sprintf("  Installed product skills from %s", skillsRef))
	}

	// Copy references directory from the product skills repo so CLAUDE.md links resolve.
	// Path is configurable per product via `references_ref` in product-config.yaml.
	if refsRef := ctx.ProductConfig.ReferencesRef; refsRef != "" {
		srcRefsDir := filepath.Join(productLocalPath, refsRef)
		if _, err := os.Stat(srcRefsDir); err == nil {
			dstRefsDir := filepath.Join(ctx.Workspace, "references")
			if err := copyDir(srcRefsDir, dstRefsDir); err != nil {
				result.Status = state.StatusFailed
				result.Error = fmt.Sprintf("failed to copy references: %v", err)
				return result, fmt.Errorf("%s", result.Error)
			}
			ctx.Printer.Info(fmt.Sprintf("  Installed references from %s", refsRef))
		} else {
			ctx.Printer.Info(fmt.Sprintf("  Warning: references_ref %q not found in skills repo", refsRef))
		}
	}

	// Read the port offset allocated by the workspace phase (used below if we
	// fall back to generating a minimal CLAUDE.md). hasOffset is false when the
	// workspace phase didn't allocate one (e.g. pick_port_offset: false).
	offset, hasOffset := portOffsetFromState(ctx.State)

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
			content := fmt.Sprintf("# %s %s — Issue #%s\n\nIssue: %s\n",
				ctx.ProductConfig.Product, ctx.ProductConfig.Version,
				ctx.IssueNumber, ctx.IssueURL)
			if hasOffset && offset > 0 {
				content += renderPortOffsetBlock(offset, ctx.ProductConfig.Runtime.DefaultPorts)
			}
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

func downloadSkillsRepo(repo, branch string, useCache bool) (string, error) {
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

	if useCache {
		if _, err := os.Stat(cachedPath); err == nil {
			return cachedPath, nil
		}
	} else {
		// Force a fresh download: remove any stale cached copy.
		if err := os.RemoveAll(cachedPath); err != nil {
			return "", fmt.Errorf("failed to clear skills cache: %w", err)
		}
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

// renderPortOffsetBlock returns a Markdown block describing the allocated port
// offset in enough detail for Claude to apply it correctly — what value was
// picked, how to pass it to the server, and what the shifted ports are.
func renderPortOffsetBlock(offset int, defaultPorts []int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n## Port Offset\n\n")
	fmt.Fprintf(&b, "A port offset of **%d** has been allocated for this workspace to avoid conflicts with other running WSO2 servers on this machine.\n\n", offset)
	fmt.Fprintf(&b, "- Pass `-DportOffset=%d` when starting the server (or set `Ports.Offset` in `repository/conf/carbon.xml`).\n", offset)
	if len(defaultPorts) > 0 {
		pairs := make([]string, 0, len(defaultPorts))
		for _, p := range defaultPorts {
			pairs = append(pairs, fmt.Sprintf("%d → %d", p, p+offset))
		}
		fmt.Fprintf(&b, "- All default ports are shifted by %d: %s.\n", offset, strings.Join(pairs, ", "))
	}
	fmt.Fprintf(&b, "- Use the shifted ports for curl, health checks, API calls, and any docs or test code you generate. Do NOT use the default ports.\n")
	return b.String()
}

// portOffsetFromState returns the port offset allocated by the workspace phase.
// The second return is false when no offset was allocated (e.g. when
// pick_port_offset is false or the workspace phase hasn't run yet).
func portOffsetFromState(s *state.WorkspaceState) (int, bool) {
	if s == nil {
		return 0, false
	}
	ws, ok := s.Phases["workspace"]
	if !ok || ws == nil || ws.Metadata == nil {
		return 0, false
	}
	v, ok := ws.Metadata["port_offset"]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
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
