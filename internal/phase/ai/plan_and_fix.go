package ai

import (
	"fmt"

	"github.com/SasinduDilshara/wso2-se-agent/internal/phase"
	"github.com/SasinduDilshara/wso2-se-agent/internal/state"
)

type PlanAndFixPhase struct {
	runner *AIPhaseRunner
}

func NewPlanAndFixPhase() *PlanAndFixPhase {
	return &PlanAndFixPhase{runner: NewAIPhaseRunner()}
}

func (p *PlanAndFixPhase) Name() string        { return "plan-and-fix" }
func (p *PlanAndFixPhase) Type() phase.PhaseType { return phase.PhaseTypeAI }

func (p *PlanAndFixPhase) Preconditions(ctx *phase.PhaseContext) error {
	if !ctx.State.PhaseSucceeded("reproduce") {
		return fmt.Errorf("reproduce phase must succeed first. Run: --phase reproduce")
	}
	return nil
}

func (p *PlanAndFixPhase) Execute(ctx *phase.PhaseContext) (*state.PhaseResult, error) {
	prompt := fmt.Sprintf("/plan-fix %s", ctx.IssueNumber)
	return p.runner.RunAIPhase(ctx, "plan-and-fix", prompt, 0)
}

func (p *PlanAndFixPhase) ExpectedArtifacts() []string {
	return []string{}
}
