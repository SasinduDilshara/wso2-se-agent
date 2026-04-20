package phase

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/SasinduDilshara/wso2-se-agent/internal/state"
)

type Engine struct {
	registry *Registry
}

func NewEngine(registry *Registry) *Engine {
	return &Engine{registry: registry}
}

func (e *Engine) Run(ctx *PhaseContext, phases []Phase) error {
	total := len(phases)

	for i, p := range phases {
		phaseNum := i + 1
		ctx.Printer.PhaseBanner(phaseNum, total, p.Name(), string(p.Type()))

		// Check preconditions
		if err := p.Preconditions(ctx); err != nil {
			ctx.Printer.PhaseFailed(p.Name(), fmt.Sprintf("precondition failed: %v", err))
			return fmt.Errorf("phase %s precondition failed: %w", p.Name(), err)
		}

		// Execute
		start := time.Now()
		result, err := p.Execute(ctx)
		if err != nil {
			// Create a failed result if the phase didn't return one
			if result == nil {
				result = &state.PhaseResult{
					Phase:     p.Name(),
					Status:    state.StatusFailed,
					StartedAt: start,
					EndedAt:   time.Now(),
					Duration:  time.Since(start).String(),
					Error:     err.Error(),
				}
			}
			ctx.State.Phases[p.Name()] = result
			state.Save(ctx.Workspace, ctx.State)
			ctx.Printer.PhaseFailed(p.Name(), err.Error())
			return fmt.Errorf("phase %s failed: %w", p.Name(), err)
		}

		// Set timing if not already set
		if result.EndedAt.IsZero() {
			result.EndedAt = time.Now()
			result.Duration = time.Since(start).String()
		}

		// Validate expected artifacts
		for _, artifact := range p.ExpectedArtifacts() {
			artifactPath := filepath.Join(ctx.Workspace, artifact)
			if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
				result.Status = state.StatusFailed
				result.Error = fmt.Sprintf("expected artifact missing: %s", artifact)
				ctx.State.Phases[p.Name()] = result
				state.Save(ctx.Workspace, ctx.State)
				ctx.Printer.PhaseFailed(p.Name(), result.Error)
				return fmt.Errorf("phase %s: %s", p.Name(), result.Error)
			}
		}

		// Save result to state
		ctx.State.Phases[p.Name()] = result
		if err := state.Save(ctx.Workspace, ctx.State); err != nil {
			return fmt.Errorf("failed to save state: %w", err)
		}

		// Handle failure
		if result.Status == state.StatusFailed {
			ctx.Printer.PhaseFailed(p.Name(), result.Error)
			return fmt.Errorf("phase %s failed: %s", p.Name(), result.Error)
		}

		ctx.Printer.PhaseSuccess(p.Name(), phaseMessage(p.Name(), result))

		// Risk gate (special handling after risk-assessment)
		if p.Name() == "risk-assessment" {
			if score, ok := result.Metadata["risk_score"]; ok {
				scoreInt := int(score.(float64))
				ctx.State.RiskScore = &scoreInt
				state.Save(ctx.Workspace, ctx.State)

				if scoreInt > ctx.RiskThreshold {
					ctx.Printer.RiskGateBlocked(scoreInt, ctx.RiskThreshold)
					result.Status = state.StatusGated
					ctx.State.Phases[p.Name()] = result
					state.Save(ctx.Workspace, ctx.State)
					return fmt.Errorf("risk score %d exceeds threshold %d — pipeline halted for human review", scoreInt, ctx.RiskThreshold)
				}
				ctx.Printer.RiskGatePass(scoreInt, ctx.RiskThreshold)
			}
		}

		// Pause for review between AI phases (unless --yes)
		if p.Type() == PhaseTypeAI && !ctx.AutoApprove && i < total-1 {
			if !ctx.Printer.PauseForReview(p.Name()) {
				return fmt.Errorf("user aborted after phase %s. Resume with: --from %s", p.Name(), nextPhaseName(phases, i))
			}
		}
	}

	return nil
}

func phaseMessage(name string, result *state.PhaseResult) string {
	if result.CostUSD > 0 {
		return fmt.Sprintf("completed ($%.2f)", result.CostUSD)
	}
	return "completed"
}

func nextPhaseName(phases []Phase, currentIdx int) string {
	if currentIdx+1 < len(phases) {
		return phases[currentIdx+1].Name()
	}
	return "done"
}
