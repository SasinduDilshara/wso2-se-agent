package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/Tharsanan1/wso2-se-agent/internal/config"
	"github.com/Tharsanan1/wso2-se-agent/internal/issue"
	"github.com/Tharsanan1/wso2-se-agent/internal/phase"
	"github.com/Tharsanan1/wso2-se-agent/internal/phase/ai"
	"github.com/Tharsanan1/wso2-se-agent/internal/phase/static"
	"github.com/Tharsanan1/wso2-se-agent/internal/state"
	"github.com/Tharsanan1/wso2-se-agent/internal/ui"
)

var (
	product       string
	version       string
	issueURL      string
	workspace     string
	phaseOnly     string
	fromPhase     string
	toPhase       string
	yes           bool
	dryRun        bool
	maxBudgetUSD  float64
	packPath      string
	modelOverride string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the issue-fixing pipeline",
	Long:  "Run the phased pipeline to reproduce, assess, fix, verify, and PR a GitHub issue.",
	RunE:  runPipeline,
}

func init() {
	runCmd.Flags().StringVar(&product, "product", "", "Product name (e.g., apim, is, mi)")
	runCmd.Flags().StringVar(&version, "version", "", "Product version (e.g., 4.3.0)")
	runCmd.Flags().StringVar(&issueURL, "issue", "", "GitHub issue URL")
	runCmd.Flags().StringVar(&workspace, "workspace", "", "Workspace path (default: auto-generated)")
	runCmd.Flags().StringVar(&phaseOnly, "phase", "", "Run a single phase only")
	runCmd.Flags().StringVar(&fromPhase, "from", "", "Start from this phase")
	runCmd.Flags().StringVar(&toPhase, "to", "", "Stop after this phase")
	runCmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmations (risk gate still applies)")
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the plan without executing")
	runCmd.Flags().Float64Var(&maxBudgetUSD, "max-budget-usd", 0, "Max budget per phase in USD")
	runCmd.Flags().StringVar(&packPath, "pack", "", "Path to product pack zip file")
	runCmd.Flags().StringVar(&modelOverride, "model", "", "Claude model to use for every AI phase (overrides config; empty = use config)")

	runCmd.MarkFlagRequired("product")
	runCmd.MarkFlagRequired("version")
	runCmd.MarkFlagRequired("issue")
	runCmd.MarkFlagRequired("pack")
}

func runPipeline(cmd *cobra.Command, args []string) error {
	// Load configs
	globalCfg, err := config.LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w\nRun: wso2-se-agent config init", err)
	}

	repoReg, err := config.LoadRepoRegistry()
	if err != nil {
		return fmt.Errorf("failed to load repo registry: %w\nRun: wso2-se-agent setup-repos", err)
	}

	productCfg, err := config.LoadProductConfig(product, version)
	if err != nil {
		return fmt.Errorf("failed to load product config: %w", err)
	}

	// Parse issue
	iss, err := issue.ParseURL(issueURL)
	if err != nil {
		return err
	}

	// Resolve workspace path
	wsPath := workspace
	if wsPath == "" {
		wsPath = filepath.Join(globalCfg.WorkspaceRoot,
			fmt.Sprintf("%s-issues-%s", product, iss.Number))
	}
	if err := os.MkdirAll(wsPath, 0755); err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Load or create state
	ws, err := state.Load(wsPath)
	if err != nil {
		return fmt.Errorf("failed to load workspace state: %w", err)
	}

	// Resolve max budget
	budget := globalCfg.MaxBudgetUSD
	if maxBudgetUSD > 0 {
		budget = maxBudgetUSD
	}

	// Build phase registry
	registry := phase.NewRegistry()
	registry.Register(static.NewPrereqPhase())
	registry.Register(static.NewWorkspacePhase())
	registry.Register(static.NewSkillsPhase())
	registry.Register(ai.NewReproducePhase())
	registry.Register(ai.NewPlanPhase())
	registry.Register(ai.NewRiskAssessmentPhase())
	registry.Register(ai.NewFixPhase())
	registry.Register(ai.NewVerifyPhase())
	registry.Register(ai.NewTestCoveragePhase())
	registry.Register(ai.NewPRPhase())

	// Resolve pipeline
	phases, err := registry.Pipeline(fromPhase, toPhase, phaseOnly, productCfg.SkipPhases)
	if err != nil {
		return err
	}

	// Build context
	ctx := phase.NewPhaseContext(
		wsPath, issueURL, iss.Number,
		productCfg, globalCfg, repoReg, ws,
		yes, budget, packPath, modelOverride, verbose,
	)

	// Collect phase names for display
	phaseNames := make([]string, len(phases))
	for i, p := range phases {
		phaseNames[i] = p.Name()
	}

	printer := ui.NewPrinter(verbose)
	printer.PipelineSummary(product, version, issueURL, phaseNames)

	// Dry run — just show the plan
	if dryRun {
		fmt.Println("Dry run — phases that would execute:")
		for i, p := range phases {
			fmt.Printf("  [%d/%d] %s (%s)\n", i+1, len(phases), p.Name(), p.Type())
		}
		fmt.Printf("\nWorkspace: %s\n", wsPath)
		fmt.Printf("Max budget/phase: $%.2f\n", budget)
		return nil
	}

	// Run the pipeline
	engine := phase.NewEngine(registry)
	if err := engine.Run(ctx, phases); err != nil {
		// Check if it's a resumable error
		if strings.Contains(err.Error(), "Resume with:") || strings.Contains(err.Error(), "pipeline halted") {
			return err
		}
		return fmt.Errorf("%w\n\nResume with: wso2-se-agent run --product %s --version %s --issue %s --from <phase>",
			err, product, version, issueURL)
	}

	fmt.Printf("\n%s=== Pipeline complete ===%s\n", ui.BoldGreen, ui.Reset)
	if ws.PRURL != "" {
		fmt.Printf("PR: %s\n", ws.PRURL)
	}
	return nil
}
