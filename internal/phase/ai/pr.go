package ai

import (
	"fmt"

	"github.com/Tharsanan1/wso2-se-agent/internal/phase"
	"github.com/Tharsanan1/wso2-se-agent/internal/state"
)

type PRPhase struct {
	runner *AIPhaseRunner
}

func NewPRPhase() *PRPhase {
	return &PRPhase{runner: NewAIPhaseRunner()}
}

func (p *PRPhase) Name() string        { return "pr" }
func (p *PRPhase) Type() phase.PhaseType { return phase.PhaseTypeAI }

func (p *PRPhase) Preconditions(ctx *phase.PhaseContext) error {
	if !ctx.State.PhaseSucceeded("verify") {
		return fmt.Errorf("verify phase must succeed first. Run: --phase verify")
	}
	return nil
}

func (p *PRPhase) Execute(ctx *phase.PhaseContext) (*state.PhaseResult, error) {
	prompt := fmt.Sprintf("/submit-fix %s", ctx.IssueNumber)
	return p.runner.RunAIPhase(ctx, "pr", prompt, 0)
}

func (p *PRPhase) ExpectedArtifacts() []string {
	return []string{}
}
