package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
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

	// Auto-detect GitHub username
	detected, _ := config.LoadGlobalConfig()
	if detected != nil && detected.GitHubUsername != "" {
		cfg.GitHubUsername = detected.GitHubUsername
	}

	// Prompt for GitHub username (pre-filled if detected)
	reader := bufio.NewReader(os.Stdin)
	if cfg.GitHubUsername != "" {
		fmt.Printf("GitHub username [%s]: ", cfg.GitHubUsername)
	} else {
		fmt.Print("GitHub username: ")
	}
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

	// Prompt for Claude model (empty lets claude pick its own default)
	modelDisplay := cfg.ClaudeModel
	if modelDisplay == "" {
		modelDisplay = "empty = claude default"
	}
	fmt.Printf("Claude model [%s]: ", modelDisplay)
	model, _ := reader.ReadString('\n')
	model = strings.TrimSpace(model)
	if model != "" {
		cfg.ClaudeModel = model
	}

	// Prompt for risk threshold (0-10)
	fmt.Printf("Risk threshold (0-10) [%d]: ", cfg.RiskThreshold)
	rt, _ := reader.ReadString('\n')
	rt = strings.TrimSpace(rt)
	if rt != "" {
		if n, err := strconv.Atoi(rt); err == nil && n >= 0 && n <= 10 {
			cfg.RiskThreshold = n
		} else {
			fmt.Printf("  (invalid; keeping %d)\n", cfg.RiskThreshold)
		}
	}

	// Prompt for per-phase max budget USD
	fmt.Printf("Max budget per phase USD [%.2f]: ", cfg.MaxBudgetUSD)
	mb, _ := reader.ReadString('\n')
	mb = strings.TrimSpace(mb)
	if mb != "" {
		if f, err := strconv.ParseFloat(mb, 64); err == nil && f > 0 {
			cfg.MaxBudgetUSD = f
		} else {
			fmt.Printf("  (invalid; keeping %.2f)\n", cfg.MaxBudgetUSD)
		}
	}

	// Prompt for generic skills repo
	fmt.Printf("Generic skills repo (org/repo, empty to skip) [%s]: ", cfg.GenericSkillsRepo)
	gsRepo, _ := reader.ReadString('\n')
	gsRepo = strings.TrimSpace(gsRepo)
	if gsRepo != "" {
		cfg.GenericSkillsRepo = gsRepo
	}

	if cfg.GenericSkillsRepo != "" {
		fmt.Printf("Generic skills branch [%s]: ", cfg.GenericSkillsBranch)
		gsBranch, _ := reader.ReadString('\n')
		gsBranch = strings.TrimSpace(gsBranch)
		if gsBranch != "" {
			cfg.GenericSkillsBranch = gsBranch
		}

		fmt.Printf("Generic skills path in repo [%s]: ", cfg.GenericSkillsRef)
		gsRef, _ := reader.ReadString('\n')
		gsRef = strings.TrimSpace(gsRef)
		if gsRef != "" {
			cfg.GenericSkillsRef = gsRef
		}
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
	if len(cfg.PhaseModels) > 0 {
		fmt.Printf("phase_models:\n")
		for _, phase := range []string{"reproduce", "plan", "risk-assessment", "fix", "verify", "test-coverage", "pr"} {
			if m, ok := cfg.PhaseModels[phase]; ok && m != "" {
				fmt.Printf("  %-15s %s\n", phase+":", m)
			}
		}
	}
	fmt.Printf("workspace_root:   %s\n", cfg.WorkspaceRoot)
	fmt.Printf("\nGeneric skills:\n")
	if cfg.GenericSkillsRepo != "" {
		fmt.Printf("  repo:           %s\n", cfg.GenericSkillsRepo)
		fmt.Printf("  branch:         %s\n", cfg.GenericSkillsBranch)
		fmt.Printf("  ref:            %s\n", cfg.GenericSkillsRef)
	} else {
		fmt.Printf("  (not configured)\n")
	}

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
