package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/Tharsanan1/wso2-se-agent/internal/config"
	gitutil "github.com/Tharsanan1/wso2-se-agent/internal/git"
)

var (
	setupProduct string
	setupVersion string
)

var setupReposCmd = &cobra.Command{
	Use:   "setup-repos",
	Short: "Register local repo clones for a product",
	Long:  "Detects forks, offers to clone them, and registers repo paths.",
	RunE:  runSetupRepos,
}

func init() {
	setupReposCmd.Flags().StringVar(&setupProduct, "product", "", "Product name")
	setupReposCmd.Flags().StringVar(&setupVersion, "version", "", "Product version")
	setupReposCmd.MarkFlagRequired("product")
	setupReposCmd.MarkFlagRequired("version")
}

func runSetupRepos(cmd *cobra.Command, args []string) error {
	productCfg, err := config.LoadProductConfig(setupProduct, setupVersion)
	if err != nil {
		return err
	}

	globalCfg, err := config.LoadGlobalConfig()
	if err != nil {
		return err
	}

	if globalCfg.GitHubUsername == "" {
		return fmt.Errorf("github_username not set and could not auto-detect from gh. Run: gh auth login")
	}

	reg, err := config.LoadRepoRegistry()
	if err != nil {
		return err
	}

	reader := bufio.NewReader(os.Stdin)

	// Default clone location
	configDir, _ := config.GetConfigDir()
	reposDir := filepath.Join(configDir, "repos")

	fmt.Printf("Setting up repos for %s %s\n", setupProduct, setupVersion)
	fmt.Printf("GitHub username: %s\n", globalCfg.GitHubUsername)
	fmt.Printf("Default clone location: %s\n\n", reposDir)

	for _, repo := range productCfg.Repos {
		fmt.Printf("--- %s ---\n", repo.Name)

		// Check if already registered
		if existing, ok := reg.Repos[repo.Name]; ok {
			fmt.Printf("  Already registered at: %s\n", existing.LocalPath)
			fmt.Print("  Re-configure? [y/N] ")
			text, _ := reader.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(text)) != "y" {
				fmt.Println("  Skipped.")
				continue
			}
		}

		upstream := repo.Upstream
		if upstream == "" {
			upstream = "wso2/" + repo.Name
		}

		// Step 1: Ask if they have an existing local clone
		fmt.Print("  Do you have an existing local clone? [y/N] ")
		text, _ := reader.ReadString('\n')

		if strings.TrimSpace(strings.ToLower(text)) == "y" {
			localPath := askForPath(reader, repo.Name)
			if localPath == "" {
				fmt.Println("  Skipped.")
				continue
			}

			// Detect fork name from origin remote
			forkFullName := ""
			if originURL, err := gitutil.GetRemoteURL(localPath, "origin"); err == nil {
				forkFullName = extractOrgRepo(originURL)
			}

			// Add upstream if missing
			if !gitutil.HasRemote(localPath, "upstream") {
				upstreamURL := fmt.Sprintf("https://github.com/%s.git", upstream)
				fmt.Printf("  No upstream remote. Adding: %s\n", upstreamURL)
				if err := gitutil.AddRemote(localPath, "upstream", upstreamURL); err != nil {
					fmt.Printf("  Warning: failed to add upstream: %v\n", err)
				}
			}

			if gitutil.HasRemote(localPath, "origin") {
				fmt.Println("  origin remote: OK")
			}
			if gitutil.HasRemote(localPath, "upstream") {
				fmt.Println("  upstream remote: OK")
			}

			reg.Repos[repo.Name] = config.RepoEntry{
				LocalPath: localPath,
				Fork:      forkFullName,
				Upstream:  upstream,
			}
			fmt.Printf("  Registered: %s -> %s\n\n", repo.Name, localPath)
			continue
		}

		// Step 2: No existing clone — find fork via GitHub API
		fmt.Printf("  Looking for your fork of %s...\n", upstream)
		forkFullName, err := findUserFork(upstream, globalCfg.GitHubUsername)

		if err != nil || forkFullName == "" {
			// No fork found — offer to fork
			fmt.Printf("  No fork found for %s.\n", upstream)
			fmt.Print("  Fork it now? [Y/n] ")
			text, _ := reader.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(text)) == "n" {
				fmt.Println("  Skipped.")
				continue
			}

			fmt.Printf("  Forking %s...\n", upstream)
			forkFullName, err = forkRepo(upstream)
			if err != nil {
				fmt.Printf("  Failed to fork: %v\n", err)
				fmt.Println("  Skipping this repo.")
				continue
			}
			fmt.Printf("  Forked: %s\n", forkFullName)
		} else {
			fmt.Printf("  Found fork: %s\n", forkFullName)
		}

		// Step 3: Clone the fork
		forkRepoName := forkFullName[strings.Index(forkFullName, "/")+1:]
		localPath := filepath.Join(reposDir, forkRepoName)

		if _, err := os.Stat(localPath); err == nil {
			fmt.Printf("  Clone already exists at %s\n", localPath)
		} else {
			fmt.Printf("  Cloning %s to %s...\n", forkFullName, localPath)
			if err := cloneRepo(forkFullName, localPath); err != nil {
				fmt.Printf("  Failed to clone: %v\n", err)
				fmt.Println("  Skipping this repo.")
				continue
			}
			fmt.Println("  Cloned successfully.")
		}

		// Add upstream remote if not present
		if !gitutil.HasRemote(localPath, "upstream") {
			upstreamURL := fmt.Sprintf("https://github.com/%s.git", upstream)
			fmt.Printf("  Adding upstream remote: %s\n", upstreamURL)
			if err := gitutil.AddRemote(localPath, "upstream", upstreamURL); err != nil {
				fmt.Printf("  Warning: failed to add upstream: %v\n", err)
			}
		} else {
			fmt.Println("  upstream remote: OK")
		}

		if gitutil.HasRemote(localPath, "origin") {
			fmt.Println("  origin remote: OK")
		}

		reg.Repos[repo.Name] = config.RepoEntry{
			LocalPath: localPath,
			Fork:      forkFullName,
			Upstream:  upstream,
		}
		fmt.Printf("  Registered: %s -> %s\n\n", repo.Name, localPath)
	}

	if err := config.SaveRepoRegistry(reg); err != nil {
		return fmt.Errorf("failed to save repos.yaml: %w", err)
	}

	reposPath, _ := config.GetReposFilePath()
	fmt.Printf("Repo registry saved to %s\n", reposPath)
	fmt.Println("\nNext step: wso2-se-agent run --product " + setupProduct + " --version " + setupVersion + " --issue <github-issue-url> --pack <path-to-product-zip>")
	return nil
}

// findUserFork uses the GitHub API to find the user's fork of an upstream repo.
// Returns the full name (e.g., "Tharsanan1/carbon-apimgt-wso2") or "" if not found.
func findUserFork(upstreamRepo, username string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use gh api to search forks
	jqFilter := fmt.Sprintf(`.[] | select(.owner.login == "%s") | .full_name`, username)
	cmd := exec.CommandContext(ctx, "gh", "api",
		fmt.Sprintf("repos/%s/forks", upstreamRepo),
		"--paginate",
		"--jq", jqFilter,
	)

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return "", nil
	}

	// Take the first line if multiple results
	lines := strings.Split(result, "\n")
	return lines[0], nil
}

// forkRepo forks an upstream repo to the authenticated user's account.
func forkRepo(upstreamRepo string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", "repo", "fork", upstreamRepo, "--clone=false")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, string(output))
	}

	// Parse the fork name from output or re-query
	// gh repo fork outputs something like "Created fork username/repo"
	outStr := string(output)
	if strings.Contains(outStr, "already exists") || strings.Contains(outStr, "Created fork") {
		// Re-query to get the actual name
		cmd2 := exec.CommandContext(ctx, "gh", "api",
			fmt.Sprintf("repos/%s/forks", upstreamRepo),
			"--jq", ".[0].full_name",
		)
		out2, err := cmd2.Output()
		if err == nil {
			name := strings.TrimSpace(string(out2))
			if name != "" {
				return name, nil
			}
		}
	}

	// Fallback: try to parse from gh output as JSON
	var result struct {
		FullName string `json:"full_name"`
	}
	if json.Unmarshal(output, &result) == nil && result.FullName != "" {
		return result.FullName, nil
	}

	return "", fmt.Errorf("forked but could not determine fork name")
}

// cloneRepo clones a GitHub repo to a local path.
func cloneRepo(fullName, localPath string) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", "repo", "clone", fullName, localPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// extractOrgRepo extracts "org/repo" from a git remote URL.
// Handles both https://github.com/org/repo.git and git@github.com:org/repo.git
func extractOrgRepo(url string) string {
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimSpace(url)

	// SSH format: git@github.com:org/repo
	if strings.Contains(url, ":") && strings.Contains(url, "@") {
		parts := strings.SplitN(url, ":", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}

	// HTTPS format: https://github.com/org/repo
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}

	return ""
}

func askForPath(reader *bufio.Reader, repoName string) string {
	fmt.Printf("  Local clone path for %s: ", repoName)
	localPath, _ := reader.ReadString('\n')
	localPath = strings.TrimSpace(localPath)

	if localPath == "" {
		return ""
	}

	// Expand ~
	if strings.HasPrefix(localPath, "~") {
		home, _ := os.UserHomeDir()
		localPath = home + localPath[1:]
	}

	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		fmt.Printf("  Warning: path does not exist: %s\n", localPath)
		return ""
	}

	return localPath
}
