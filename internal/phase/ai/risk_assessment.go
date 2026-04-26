package ai

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/Tharsanan1/wso2-se-agent/internal/phase"
	"github.com/Tharsanan1/wso2-se-agent/internal/state"
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
	if !ctx.State.PhaseSucceeded("plan") {
		return fmt.Errorf("plan phase must succeed first. Run: --phase plan")
	}
	return nil
}

func (p *RiskAssessmentPhase) Execute(ctx *phase.PhaseContext) (*state.PhaseResult, error) {
	// Pass the full URL — same reason as PlanPhase: skills that hit `gh issue view`
	// need the org/repo from the URL, not just the number.
	prompt := fmt.Sprintf("/risk-assessment %s", ctx.IssueURL)
	return p.runner.RunAIPhase(ctx, "risk-assessment", prompt, 0, extractRiskScore)
}

func (p *RiskAssessmentPhase) ExpectedArtifacts() []string {
	return []string{}
}

// riskScoreRE matches the score line the risk-assessment skill emits.
// Examples it accepts:
//
//	Score: 2/10
//	**Score: 7/10 (Medium) — PROCEED**
//	Risk Score: 5/10
//	## Risk Score: 2/10 (Low)
//
// Case-insensitive on the word "score" to be robust to formatting drift.
var riskScoreRE = regexp.MustCompile(`(?i)(?:risk\s+)?score\s*:?\s*\**\s*(\d{1,2})\s*/\s*10`)

// extractRiskScore pulls the 0-10 risk score out of Claude's final text and
// writes it to metadata as a float64 (matching the type the engine's gate
// asserts on in engine.go:108). The score is clamped to [0, 10]; anything
// outside that range or unparsable is ignored so the gate sees "no score"
// rather than a misleading zero.
func extractRiskScore(resultText string) map[string]any {
	m := riskScoreRE.FindStringSubmatch(resultText)
	if len(m) < 2 {
		return nil
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 0 || n > 10 {
		return nil
	}
	return map[string]any{"risk_score": float64(n)}
}
