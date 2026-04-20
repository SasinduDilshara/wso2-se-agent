package phase

import (
	"time"

	"github.com/SasinduDilshara/wso2-se-agent/internal/config"
	"github.com/SasinduDilshara/wso2-se-agent/internal/state"
	"github.com/SasinduDilshara/wso2-se-agent/internal/ui"
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
	RiskThreshold int
	PackPath      string
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
	riskThreshold int,
	packPath string,
	verbose bool,
) *PhaseContext {
	return &PhaseContext{
		Workspace:     workspace,
		IssueURL:      issueURL,
		IssueNumber:   issueNumber,
		ProductConfig: productCfg,
		GlobalConfig:  globalCfg,
		RepoRegistry:  repoReg,
		State:         ws,
		AutoApprove:   autoApprove,
		MaxBudgetUSD:  maxBudget,
		RiskThreshold: riskThreshold,
		PackPath:      packPath,
		Printer:       ui.NewPrinter(verbose),
		RunTimestamp:  time.Now().Format("20060102-150405"),
		Verbose:       verbose,
	}
}
