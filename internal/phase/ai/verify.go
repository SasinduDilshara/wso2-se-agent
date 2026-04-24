package ai

import (
	"fmt"

	"github.com/Tharsanan1/wso2-se-agent/internal/phase"
	"github.com/Tharsanan1/wso2-se-agent/internal/state"
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
	if !ctx.State.PhaseSucceeded("fix") {
		return fmt.Errorf("fix phase must succeed first. Run: --phase fix")
	}
	return nil
}

func (p *VerifyPhase) Execute(ctx *phase.PhaseContext) (*state.PhaseResult, error) {
	prompt := fmt.Sprintf("/verify-fix %s", ctx.IssueURL)
	return p.runner.RunAIPhase(ctx, "verify", prompt, 0, nil)
}

func (p *VerifyPhase) ExpectedArtifacts() []string {
	return []string{}
}
