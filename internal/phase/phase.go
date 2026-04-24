package phase

import (
	"time"

	"github.com/Tharsanan1/wso2-se-agent/internal/config"
	"github.com/Tharsanan1/wso2-se-agent/internal/state"
	"github.com/Tharsanan1/wso2-se-agent/internal/ui"
)

type PhaseType string

const (
	PhaseTypeStatic PhaseType = "static"
	PhaseTypeAI     PhaseType = "ai"
)

type Phase interface {
	Name() string
	Type() PhaseType
	Preconditions(ctx *PhaseContext) error
	Execute(ctx *PhaseContext) (*state.PhaseResult, error)
	ExpectedArtifacts() []string
}

type PhaseContext struct {
	Workspace     string
	IssueURL      string
	IssueNumber   string
	ProductConfig *config.ProductConfig
	GlobalConfig  *config.GlobalConfig
	RepoRegistry  *config.RepoRegistry
	State         *state.WorkspaceState
	AutoApprove   bool
	MaxBudgetUSD  float64
	// MaxTotalBudgetUSD, when > 0, is a cumulative USD cap across every AI
	// phase that has ever run against this workspace (not just the current
	// invocation — prior spend from state.json counts). Set by the run
	// command's --max-total-budget-usd flag. When exceeded, the pipeline
	// halts before starting the next phase.
	MaxTotalBudgetUSD float64
	PackPath          string
	// ModelOverride, when non-empty, forces every AI phase in this run to use
	// this Claude model, overriding both the global claude_model and any
	// per-phase model in phase_models. Set by the run command's --model flag.
	ModelOverride string
	Printer       *ui.Printer
	RunTimestamp  string
	Verbose       bool
}

func NewPhaseContext(
	workspace, issueURL, issueNumber string,
	productCfg *config.ProductConfig,
	globalCfg *config.GlobalConfig,
	repoReg *config.RepoRegistry,
	ws *state.WorkspaceState,
	autoApprove bool,
	maxBudget float64,
	maxTotalBudget float64,
	packPath string,
	modelOverride string,
	verbose bool,
) *PhaseContext {
	return &PhaseContext{
		Workspace:         workspace,
		IssueURL:          issueURL,
		IssueNumber:       issueNumber,
		ProductConfig:     productCfg,
		GlobalConfig:      globalCfg,
		RepoRegistry:      repoReg,
		State:             ws,
		AutoApprove:       autoApprove,
		MaxBudgetUSD:      maxBudget,
		MaxTotalBudgetUSD: maxTotalBudget,
		PackPath:          packPath,
		ModelOverride:     modelOverride,
		Printer:           ui.NewPrinter(verbose),
		RunTimestamp:      time.Now().Format("20060102-150405"),
		Verbose:           verbose,
	}
}
