package ai

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/Tharsanan1/wso2-se-agent/internal/claude"
	"github.com/Tharsanan1/wso2-se-agent/internal/phase"
	"github.com/Tharsanan1/wso2-se-agent/internal/state"
)

type AIPhaseRunner struct {
	invoker *claude.Invoker
}

func NewAIPhaseRunner() *AIPhaseRunner {
	return &AIPhaseRunner{
		invoker: claude.NewInvoker(),
	}
}

// MetadataExtractor pulls structured fields out of Claude's final result text
// and returns them as a map to merge into PhaseResult.Metadata. Phases that
// don't need extraction pass nil. See risk_assessment.go and pr.go for the
// extractors that feed the risk gate and the PR URL surfacing.
type MetadataExtractor func(resultText string) map[string]any

// resolvePhaseBudget figures out the per-phase USD cap to pass to Claude,
// honoring (in priority order):
//
//   1. budgetOverride — an explicit value from the phase's Execute method
//      (non-zero means "this phase hard-codes its cap — use exactly this").
//   2. productCfg.PhaseLimits[phaseName] — the per-phase entry in the product
//      config.
//   3. ctx.MaxBudgetUSD — the run-wide --max-budget-usd flag or the global
//      max_budget_usd default.
//
// Then, if ctx.MaxTotalBudgetUSD > 0, the function subtracts what every prior
// phase in state.json has already spent (including prior attempts of this
// phase and spend from earlier --from resumes). The returned values are:
//
//   - budget: the cap to pass to Claude. Zero only when halt is true.
//   - halt:   true when the cumulative cap has already been reached; the
//     caller must refuse to invoke Claude and record the phase as failed.
//   - haltSpent: the cumulative cost observed at halt time (for the error
//     message).
//   - narrowedFrom: when > 0, the budget Claude would have received without
//     the cumulative cap; the caller can print a notice. Zero when the
//     cumulative cap did not change the effective budget.
func resolvePhaseBudget(ctx *phase.PhaseContext, phaseName string, budgetOverride float64) (budget float64, halt bool, haltSpent float64, narrowedFrom float64) {
	budget = budgetOverride
	if budget <= 0 {
		if limit, ok := ctx.ProductConfig.PhaseLimits[phaseName]; ok {
			budget = limit
		}
	}
	if budget <= 0 {
		budget = ctx.MaxBudgetUSD
	}

	if ctx.MaxTotalBudgetUSD > 0 {
		spent := 0.0
		for _, r := range ctx.State.Phases {
			spent += r.CostUSD
		}
		remaining := ctx.MaxTotalBudgetUSD - spent
		if remaining <= 0 {
			return 0, true, spent, 0
		}
		if remaining < budget {
			return remaining, false, 0, budget
		}
	}
	return budget, false, 0, 0
}

// resolveModel picks the Claude model for this invocation. Precedence:
//   1. --model flag on the run command (ctx.ModelOverride)
//   2. phase-specific entry in global phase_models map
//   3. global claude_model
//   4. empty string (lets `claude` pick its own default)
func resolveModel(ctx *phase.PhaseContext, phaseName string) string {
	if ctx.ModelOverride != "" {
		return ctx.ModelOverride
	}
	if m, ok := ctx.GlobalConfig.PhaseModels[phaseName]; ok && m != "" {
		return m
	}
	return ctx.GlobalConfig.ClaudeModel
}

func (r *AIPhaseRunner) RunAIPhase(ctx *phase.PhaseContext, phaseName, prompt string, budgetOverride float64, extract MetadataExtractor) (*state.PhaseResult, error) {
	result := &state.PhaseResult{
		Phase:     phaseName,
		StartedAt: time.Now(),
		Metadata:  make(map[string]any),
	}

	budget, halt, haltSpent, narrowedFrom := resolvePhaseBudget(ctx, phaseName, budgetOverride)
	if halt {
		result.Status = state.StatusFailed
		result.Error = fmt.Sprintf(
			"total budget exhausted: $%.2f already spent vs $%.2f cap. "+
				"Raise --max-total-budget-usd or omit it to continue.",
			haltSpent, ctx.MaxTotalBudgetUSD)
		result.EndedAt = time.Now()
		return result, fmt.Errorf("%s", result.Error)
	}
	if narrowedFrom > 0 {
		ctx.Printer.Info(fmt.Sprintf(
			"  Total-budget cap active: $%.2f of $%.2f remaining — narrowing %s phase cap from $%.2f to $%.2f",
			budget, ctx.MaxTotalBudgetUSD, phaseName, narrowedFrom, budget))
	}

	// Invoke Claude
	logDir := filepath.Join(ctx.Workspace, ".ai", "logs")
	opts := claude.Options{
		Prompt:         prompt,
		WorkingDir:     ctx.Workspace,
		OutputFormat:   "stream-json",
		MaxBudgetUSD:   budget,
		Model:          resolveModel(ctx, phaseName),
		SkipPerms:      true,
		Verbose:        true,
		IncludePartial: true,
	}

	invokeResult, err := r.invoker.Invoke(opts, logDir, ctx.IssueNumber, phaseName, ctx.RunTimestamp)
	if err != nil {
		result.Status = state.StatusFailed
		result.Error = fmt.Sprintf("claude invocation failed: %v", err)
		result.EndedAt = time.Now()
		return result, fmt.Errorf("%s", result.Error)
	}

	result.CostUSD = invokeResult.TotalCostUSD

	if invokeResult.ExitCode != 0 {
		result.Status = state.StatusFailed
		result.Error = fmt.Sprintf("claude exited with code %d", invokeResult.ExitCode)
		result.EndedAt = time.Now()
		return result, fmt.Errorf("%s (check logs at %s)", result.Error, logDir)
	}

	// Pull structured fields out of Claude's final text (if the phase cares).
	// Examples: risk-assessment extracts the 0-10 score so the engine's gate
	// can actually fire; pr extracts the opened PR URL so `status` can show it.
	if extract != nil && invokeResult.ResultText != "" {
		for k, v := range extract(invokeResult.ResultText) {
			result.Metadata[k] = v
		}
	}

	result.Status = state.StatusSuccess
	result.EndedAt = time.Now()
	result.Duration = result.EndedAt.Sub(result.StartedAt).String()
	return result, nil
}

