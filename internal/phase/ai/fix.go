package ai

import (
	"fmt"

	"github.com/Tharsanan1/wso2-se-agent/internal/phase"
	"github.com/Tharsanan1/wso2-se-agent/internal/state"
)

type FixPhase struct {
	runner *AIPhaseRunner
}

func NewFixPhase() *FixPhase {
	return &FixPhase{runner: NewAIPhaseRunner()}
}

func (p *FixPhase) Name() string          { return "fix" }
func (p *FixPhase) Type() phase.PhaseType { return phase.PhaseTypeAI }

func (p *FixPhase) Preconditions(ctx *phase.PhaseContext) error {
	if !ctx.State.PhaseSucceeded("risk-assessment") {
		return fmt.Errorf("risk-assessment phase must succeed first")
	}
	return nil
}

func (p *FixPhase) Execute(ctx *phase.PhaseContext) (*state.PhaseResult, error) {
	// Pass the full URL — same reason as PlanPhase: skills that hit `gh issue view`
	// need the org/repo from the URL, not just the number.
	prompt := fmt.Sprintf("/fix %s", ctx.IssueURL)
	return p.runner.RunAIPhase(ctx, "fix", prompt, 0, nil)
}

func (p *FixPhase) ExpectedArtifacts() []string {
	return []string{}
}
