package ai

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/SasinduDilshara/wso2-se-agent/internal/claude"
	"github.com/SasinduDilshara/wso2-se-agent/internal/phase"
	"github.com/SasinduDilshara/wso2-se-agent/internal/script"
	"github.com/SasinduDilshara/wso2-se-agent/internal/state"
)

type AIPhaseRunner struct {
	invoker *claude.Invoker
}

func NewAIPhaseRunner() *AIPhaseRunner {
	return &AIPhaseRunner{
		invoker: claude.NewInvoker(),
	}
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

	// Run pre-script
	preScript := r.findScript(ctx, phaseName, "pre")
	if preScript != "" {
		ctx.Printer.Info(fmt.Sprintf("  Running pre-script: %s", filepath.Base(preScript)))
		if err := script.Run(preScript, r.scriptEnv(ctx), 5*time.Minute); err != nil {
			result.Status = state.StatusFailed
			result.Error = fmt.Sprintf("pre-script failed: %v", err)
			result.EndedAt = time.Now()
			return result, fmt.Errorf("%s", result.Error)
		}
	}

	// Invoke Claude
	logDir := filepath.Join(ctx.Workspace, ".ai", "logs")
	opts := claude.Options{
		Prompt:         prompt,
		WorkingDir:     ctx.Workspace,
		OutputFormat:   "stream-json",
		MaxBudgetUSD:   budget,
		Model:          ctx.GlobalConfig.ClaudeModel,
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

	// Run post-script
	postScript := r.findScript(ctx, phaseName, "post")
	if postScript != "" {
		ctx.Printer.Info(fmt.Sprintf("  Running post-script: %s", filepath.Base(postScript)))
		if err := script.Run(postScript, r.scriptEnv(ctx), 5*time.Minute); err != nil {
			result.Status = state.StatusFailed
			result.Error = fmt.Sprintf("post-script failed: %v", err)
			result.EndedAt = time.Now()
			return result, fmt.Errorf("%s", result.Error)
		}
	}

	result.Status = state.StatusSuccess
	result.EndedAt = time.Now()
	result.Duration = result.EndedAt.Sub(result.StartedAt).String()
	return result, nil
}

func (r *AIPhaseRunner) findScript(ctx *phase.PhaseContext, phaseName, timing string) string {
	scriptName := fmt.Sprintf("%s.%s.sh", phaseName, timing)
	path := filepath.Join(ctx.ProductConfig.SourceDir, "scripts", scriptName)
	if script.Exists(path) {
		return path
	}
	return ""
}

func (r *AIPhaseRunner) scriptEnv(ctx *phase.PhaseContext) script.EnvVars {
	return script.EnvVars{
		Workspace:   ctx.Workspace,
		IssueNumber: ctx.IssueNumber,
		IssueURL:    ctx.IssueURL,
		Product:     ctx.ProductConfig.Product,
		Version:     ctx.ProductConfig.Version,
		StateFile:   filepath.Join(ctx.Workspace, ".wse", "state.json"),
	}
}
