package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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
	Long:  "Interactive setup to register existing fork clones and validate remotes.",
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

	reg, err := config.LoadRepoRegistry()
	if err != nil {
		return err
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Setting up repos for %s %s\n", setupProduct, setupVersion)
	fmt.Printf("GitHub username: %s\n\n", globalCfg.GitHubUsername)

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

		// Ask for local path
		fmt.Printf("  Local clone path for %s: ", repo.Name)
		localPath, _ := reader.ReadString('\n')
		localPath = strings.TrimSpace(localPath)

		if localPath == "" {
			fmt.Println("  Skipped (no path provided).")
			continue
		}

		// Expand ~
		if strings.HasPrefix(localPath, "~") {
			home, _ := os.UserHomeDir()
			localPath = home + localPath[1:]
		}

		// Validate path exists
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			fmt.Printf("  Warning: path does not exist: %s\n", localPath)
			continue
		}

		// Derive fork and upstream
		forkOrg := globalCfg.GitHubUsername
		if forkOrg == "" {
			fmt.Print("  Fork org/user: ")
			text, _ := reader.ReadString('\n')
			forkOrg = strings.TrimSpace(text)
		}

		upstream := fmt.Sprintf("wso2/%s", repo.Name)
		fork := fmt.Sprintf("%s/%s", forkOrg, repo.Name)

		// Validate remotes
		if gitutil.HasRemote(localPath, "origin") {
			fmt.Printf("  origin remote: OK\n")
		} else {
			fmt.Printf("  Warning: no 'origin' remote found\n")
		}

		if gitutil.HasRemote(localPath, "upstream") {
			fmt.Printf("  upstream remote: OK\n")
		} else {
			fmt.Printf("  No 'upstream' remote. Add it? [Y/n] ")
			text, _ := reader.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(text)) != "n" {
				upstreamURL := fmt.Sprintf("https://github.com/%s.git", upstream)
				if err := gitutil.AddRemote(localPath, "upstream", upstreamURL); err != nil {
					fmt.Printf("  Failed to add upstream: %v\n", err)
				} else {
					fmt.Printf("  Added upstream: %s\n", upstreamURL)
				}
			}
		}

		reg.Repos[repo.Name] = config.RepoEntry{
			LocalPath: localPath,
			Fork:      fork,
			Upstream:  upstream,
		}
		fmt.Printf("  Registered: %s -> %s\n\n", repo.Name, localPath)
	}

	if err := config.SaveRepoRegistry(reg); err != nil {
		return fmt.Errorf("failed to save repos.yaml: %w", err)
	}

	reposPath, _ := config.GetReposFilePath()
	fmt.Printf("Repo registry saved to %s\n", reposPath)
	return nil
}
