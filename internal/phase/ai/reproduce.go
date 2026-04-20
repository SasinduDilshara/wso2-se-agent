package ai

import (
	"fmt"

	"github.com/SasinduDilshara/wso2-se-agent/internal/phase"
	"github.com/SasinduDilshara/wso2-se-agent/internal/state"
)

type ReproducePhase struct {
	runner *AIPhaseRunner
}

func NewReproducePhase() *ReproducePhase {
	return &ReproducePhase{runner: NewAIPhaseRunner()}
}

func (p *ReproducePhase) Name() string        { return "reproduce" }
func (p *ReproducePhase) Type() phase.PhaseType { return phase.PhaseTypeAI }

func (p *ReproducePhase) Preconditions(ctx *phase.PhaseContext) error {
	return nil // first AI phase, no preconditions
}

func (p *ReproducePhase) Execute(ctx *phase.PhaseContext) (*state.PhaseResult, error) {
	prompt := fmt.Sprintf("/reproduce %s", ctx.IssueURL)
	return p.runner.RunAIPhase(ctx, "reproduce", prompt, 0)
}

func (p *ReproducePhase) ExpectedArtifacts() []string {
	return []string{} // artifact name varies by issue number, checked in post-work
}
