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

func (r *AIPhaseRunner) RunAIPhase(ctx *phase.PhaseContext, phaseName, prompt string, budgetOverride float64) (*state.PhaseResult, error) {
	result := &state.PhaseResult{
		Phase:     phaseName,
		StartedAt: time.Now(),
		Metadata:  make(map[string]any),
	}

	// Determine budget
	budget := budgetOverride
	if budget <= 0 {
		if limit, ok := ctx.ProductConfig.PhaseLimits[phaseName]; ok {
			budget = limit
		}
	}
	if budget <= 0 {
		budget = ctx.MaxBudgetUSD
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

	result.Status = state.StatusSuccess
	result.EndedAt = time.Now()
	result.Duration = result.EndedAt.Sub(result.StartedAt).String()
	return result, nil
}

