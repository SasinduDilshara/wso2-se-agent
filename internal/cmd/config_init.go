package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/SasinduDilshara/wso2-se-agent/internal/config"
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

	// Save global config
	if err := config.SaveGlobalConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	fmt.Printf("\nConfig saved to %s\n", cfgPath)

	// Copy embedded product configs to local config dir
	fmt.Println("\nCopying product configs...")
	if err := config.CopyEmbeddedProducts(false); err != nil {
		return fmt.Errorf("failed to copy product configs: %w", err)
	}

	configDir, _ := config.GetConfigDir()
	fmt.Printf("Product configs copied to %s/products/\n", configDir)

	// Show what was installed
	products, _ := config.ListProducts()
	for _, p := range products {
		versions, _ := config.ListVersions(p)
		fmt.Printf("  %s: %s\n", p, strings.Join(versions, ", "))
	}

	fmt.Printf("\nEdit product configs at %s/products/<product>/<version>/product-config.yaml\n", configDir)
	fmt.Println("\nNext step: wso2-se-agent setup-repos --product <product> --version <version>")
	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		return err
	}

	cfgPath, _ := config.GetConfigFilePath()
	configDir, _ := config.GetConfigDir()

	fmt.Printf("Config file: %s\n\n", cfgPath)
	fmt.Printf("github_username:  %s\n", cfg.GitHubUsername)
	fmt.Printf("risk_threshold:   %d\n", cfg.RiskThreshold)
	fmt.Printf("max_budget_usd:   %.2f\n", cfg.MaxBudgetUSD)
	fmt.Printf("log_level:        %s\n", cfg.LogLevel)
	fmt.Printf("claude_model:     %s\n", cfg.ClaudeModel)
	fmt.Printf("workspace_root:   %s\n", cfg.WorkspaceRoot)

	// Show available products from local config dir
	fmt.Printf("\nProduct configs: %s/products/\n", configDir)
	products, err := config.ListProducts()
	if err == nil && len(products) > 0 {
		for _, p := range products {
			versions, err := config.ListVersions(p)
			if err == nil {
				fmt.Printf("  %s: %s\n", p, strings.Join(versions, ", "))
			}
		}
	} else {
		fmt.Println("  (none — run: wso2-se-agent config init)")
	}

	return nil
}
