package ai

import (
	"fmt"

	"github.com/Tharsanan1/wso2-se-agent/internal/phase"
	"github.com/Tharsanan1/wso2-se-agent/internal/state"
)

type PlanPhase struct {
	runner *AIPhaseRunner
}

func NewPlanPhase() *PlanPhase {
	return &PlanPhase{runner: NewAIPhaseRunner()}
}

func (p *PlanPhase) Name() string          { return "plan" }
func (p *PlanPhase) Type() phase.PhaseType { return phase.PhaseTypeAI }

func (p *PlanPhase) Preconditions(ctx *phase.PhaseContext) error {
	if !ctx.State.PhaseSucceeded("reproduce") {
		return fmt.Errorf("reproduce phase must succeed first. Run: --phase reproduce")
	}
	return nil
}

func (p *PlanPhase) Execute(ctx *phase.PhaseContext) (*state.PhaseResult, error) {
	prompt := fmt.Sprintf("/plan %s", ctx.IssueNumber)
	return p.runner.RunAIPhase(ctx, "plan", prompt, 0)
}

func (p *PlanPhase) ExpectedArtifacts() []string {
	return []string{}
}
