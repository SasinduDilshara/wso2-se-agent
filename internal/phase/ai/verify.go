package ai

import (
	"fmt"

	"github.com/SasinduDilshara/wso2-se-agent/internal/phase"
	"github.com/SasinduDilshara/wso2-se-agent/internal/state"
)

type VerifyPhase struct {
	runner *AIPhaseRunner
}

func NewVerifyPhase() *VerifyPhase {
	return &VerifyPhase{runner: NewAIPhaseRunner()}
}

func (p *VerifyPhase) Name() string        { return "verify" }
func (p *VerifyPhase) Type() phase.PhaseType { return phase.PhaseTypeAI }

func (p *VerifyPhase) Preconditions(ctx *phase.PhaseContext) error {
	if !ctx.State.PhaseSucceeded("plan-and-fix") {
		return fmt.Errorf("plan-and-fix phase must succeed first. Run: --phase plan-and-fix")
	}
	return nil
}

func (p *VerifyPhase) Execute(ctx *phase.PhaseContext) (*state.PhaseResult, error) {
	prompt := fmt.Sprintf("/verify-fix %s", ctx.IssueURL)
	return p.runner.RunAIPhase(ctx, "verify", prompt, 0)
}

func (p *VerifyPhase) ExpectedArtifacts() []string {
	return []string{}
}
