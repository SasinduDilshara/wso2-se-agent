package phase

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/Tharsanan1/wso2-se-agent/internal/script"
	"github.com/Tharsanan1/wso2-se-agent/internal/state"
)

var riskVerdictRE = regexp.MustCompile(`(?m)^\*\*Verdict:\*\*\s*(GO|REVIEW REQUIRED|NO-GO)\b`)

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

		// Run pre-script
		if preScript := findScript(ctx, p.Name(), "pre"); preScript != "" {
			ctx.Printer.Info(fmt.Sprintf("  Running pre-script: %s", filepath.Base(preScript)))
			if err := script.Run(preScript, scriptEnv(ctx), 5*time.Minute); err != nil {
				ctx.Printer.PhaseFailed(p.Name(), fmt.Sprintf("pre-script failed: %v", err))
				return fmt.Errorf("phase %s pre-script failed: %w", p.Name(), err)
			}
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

		// Run post-script
		if postScript := findScript(ctx, p.Name(), "post"); postScript != "" {
			ctx.Printer.Info(fmt.Sprintf("  Running post-script: %s", filepath.Base(postScript)))
			if err := script.Run(postScript, scriptEnv(ctx), 5*time.Minute); err != nil {
				ctx.Printer.PhaseFailed(p.Name(), fmt.Sprintf("post-script failed: %v", err))
				return fmt.Errorf("phase %s post-script failed: %w", p.Name(), err)
			}
		}

		ctx.Printer.PhaseSuccess(p.Name(), phaseMessage(p.Name(), result))

		// Risk gate (special handling after risk-assessment)
		if p.Name() == "risk-assessment" {
			artifactPath := filepath.Join(ctx.Workspace, ".ai", fmt.Sprintf("risk-assessment-%s.md", ctx.IssueNumber))
			body, readErr := os.ReadFile(artifactPath)
			if readErr != nil {
				result.Status = state.StatusFailed
				result.Error = fmt.Sprintf("risk-assessment artifact missing: %s", artifactPath)
				ctx.State.Phases[p.Name()] = result
				state.Save(ctx.Workspace, ctx.State)
				ctx.Printer.PhaseFailed(p.Name(), result.Error)
				return fmt.Errorf("%s", result.Error)
			}
			match := riskVerdictRE.FindSubmatch(body)
			if match == nil {
				result.Status = state.StatusFailed
				result.Error = fmt.Sprintf("risk-assessment artifact missing verdict line: %s", artifactPath)
				ctx.State.Phases[p.Name()] = result
				state.Save(ctx.Workspace, ctx.State)
				ctx.Printer.PhaseFailed(p.Name(), result.Error)
				return fmt.Errorf("%s", result.Error)
			}
			verdict := string(match[1])
			ctx.State.RiskVerdict = verdict
			if err := state.Save(ctx.Workspace, ctx.State); err != nil {
				return fmt.Errorf("failed to save state: %w", err)
			}

			if verdict != "GO" {
				ctx.Printer.RiskGateBlocked(verdict, artifactPath)
				result.Status = state.StatusGated
				ctx.State.Phases[p.Name()] = result
				state.Save(ctx.Workspace, ctx.State)
				return fmt.Errorf("risk-assessment verdict %s — pipeline halted for human review. Review %s and resume with --from fix", verdict, artifactPath)
			}
			ctx.Printer.RiskGatePass(verdict)
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

func findScript(ctx *PhaseContext, phaseName, timing string) string {
	scriptName := fmt.Sprintf("%s.%s.sh", phaseName, timing)
	path := filepath.Join(ctx.ProductConfig.SourceDir, "scripts", scriptName)
	if script.Exists(path) {
		return path
	}
	return ""
}

func scriptEnv(ctx *PhaseContext) script.EnvVars {
	return script.EnvVars{
		Workspace:   ctx.Workspace,
		IssueNumber: ctx.IssueNumber,
		IssueURL:    ctx.IssueURL,
		Product:     ctx.ProductConfig.Product,
		Version:     ctx.ProductConfig.Version,
		PortOffset:  portOffsetFromState(ctx.State),
		StateFile:   filepath.Join(ctx.Workspace, ".wse", "state.json"),
	}
}

// portOffsetFromState reads the port offset allocated by the workspace phase
// out of state, formatted as a string for the hook env. Handles both int (set
// in-process) and float64 (round-tripped through JSON).
func portOffsetFromState(s *state.WorkspaceState) string {
	if s == nil {
		return ""
	}
	ws, ok := s.Phases["workspace"]
	if !ok || ws == nil || ws.Metadata == nil {
		return ""
	}
	switch v := ws.Metadata["port_offset"].(type) {
	case int:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%d", int(v))
	default:
		return ""
	}
}
