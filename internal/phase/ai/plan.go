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
	// Pass the full URL (not just the number) so the agent knows which org/repo
	// the issue lives in. With only the number, `gh issue view` has to guess
	// the repo and burns turns trying every WSO2 org until it finds the right
	// one — or never, on a private repo. The skill extracts the number from
	// the URL when it needs it for filenames like `.ai/plan-<n>.md`.
	prompt := fmt.Sprintf("/plan %s", ctx.IssueURL)
	return p.runner.RunAIPhase(ctx, "plan", prompt, 0, nil)
}

func (p *PlanPhase) ExpectedArtifacts() []string {
	return []string{}
}
