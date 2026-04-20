package ai

import (
	"fmt"

	"github.com/SasinduDilshara/wso2-se-agent/internal/phase"
	"github.com/SasinduDilshara/wso2-se-agent/internal/state"
)

type RiskAssessmentPhase struct {
	runner *AIPhaseRunner
}

func NewRiskAssessmentPhase() *RiskAssessmentPhase {
	return &RiskAssessmentPhase{runner: NewAIPhaseRunner()}
}

func (p *RiskAssessmentPhase) Name() string        { return "risk-assessment" }
func (p *RiskAssessmentPhase) Type() phase.PhaseType { return phase.PhaseTypeAI }

func (p *RiskAssessmentPhase) Preconditions(ctx *phase.PhaseContext) error {
	if !ctx.State.PhaseSucceeded("reproduce") {
		return fmt.Errorf("reproduce phase must succeed first. Run: --phase reproduce")
	}
	return nil
}

func (p *RiskAssessmentPhase) Execute(ctx *phase.PhaseContext) (*state.PhaseResult, error) {
	prompt := fmt.Sprintf("/risk-assessment %s", ctx.IssueNumber)
	return p.runner.RunAIPhase(ctx, "risk-assessment", prompt, 0)
}

func (p *RiskAssessmentPhase) ExpectedArtifacts() []string {
	return []string{}
}
