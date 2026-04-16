package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/Tharsanan1/wso2-se-agent/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create initial configuration",
	RunE:  runConfigInit,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE:  runConfigShow,
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	cfgPath, err := config.GetConfigFilePath()
	if err != nil {
		return err
	}

	// Check if config already exists
	if _, err := os.Stat(cfgPath); err == nil {
		fmt.Printf("Config already exists at %s\n", cfgPath)
		fmt.Print("Overwrite? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(text)) != "y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	cfg := config.DefaultGlobalConfig()

	// Prompt for GitHub username
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("GitHub username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)
	if username != "" {
		cfg.GitHubUsername = username
	}

	// Prompt for workspace root
	fmt.Printf("Workspace root [%s]: ", cfg.WorkspaceRoot)
	wsRoot, _ := reader.ReadString('\n')
	wsRoot = strings.TrimSpace(wsRoot)
	if wsRoot != "" {
		cfg.WorkspaceRoot = wsRoot
	}

	if err := config.SaveGlobalConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Config saved to %s\n", cfgPath)
	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		return err
	}

	cfgPath, _ := config.GetConfigFilePath()
	fmt.Printf("Config file: %s\n\n", cfgPath)
	fmt.Printf("github_username:  %s\n", cfg.GitHubUsername)
	fmt.Printf("risk_threshold:   %d\n", cfg.RiskThreshold)
	fmt.Printf("max_budget_usd:   %.2f\n", cfg.MaxBudgetUSD)
	fmt.Printf("log_level:        %s\n", cfg.LogLevel)
	fmt.Printf("claude_model:     %s\n", cfg.ClaudeModel)
	fmt.Printf("workspace_root:   %s\n", cfg.WorkspaceRoot)

	// Show available products
	products, err := config.ListProducts()
	if err == nil && len(products) > 0 {
		fmt.Printf("\nAvailable products: %s\n", strings.Join(products, ", "))
		for _, p := range products {
			versions, err := config.ListVersions(p)
			if err == nil {
				fmt.Printf("  %s: %s\n", p, strings.Join(versions, ", "))
			}
		}
	}

	return nil
}
