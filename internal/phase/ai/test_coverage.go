package ai

import (
	"fmt"

	"github.com/Tharsanan1/wso2-se-agent/internal/phase"
	"github.com/Tharsanan1/wso2-se-agent/internal/state"
)

type TestCoveragePhase struct {
	runner *AIPhaseRunner
}

func NewTestCoveragePhase() *TestCoveragePhase {
	return &TestCoveragePhase{runner: NewAIPhaseRunner()}
}

func (p *TestCoveragePhase) Name() string        { return "test-coverage" }
func (p *TestCoveragePhase) Type() phase.PhaseType { return phase.PhaseTypeAI }

func (p *TestCoveragePhase) Preconditions(ctx *phase.PhaseContext) error {
	if !ctx.State.PhaseSucceeded("verify") {
		return fmt.Errorf("verify phase must succeed first. Run: --phase verify")
	}
	return nil
}

func (p *TestCoveragePhase) Execute(ctx *phase.PhaseContext) (*state.PhaseResult, error) {
	// Pass the full URL — same reason as PlanPhase: skills that hit `gh issue view`
	// need the org/repo from the URL, not just the number.
	prompt := fmt.Sprintf("/create-tests %s", ctx.IssueURL)
	return p.runner.RunAIPhase(ctx, "test-coverage", prompt, 0, nil)
}

func (p *TestCoveragePhase) ExpectedArtifacts() []string {
	return []string{}
}
