package phase

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Tharsanan1/wso2-se-agent/internal/script"
	"github.com/Tharsanan1/wso2-se-agent/internal/state"
)

var riskVerdictRE = regexp.MustCompile(`(?m)^\*\*Verdict:\*\*\s*(GO|REVIEW REQUIRED|NO-GO)\b`)

// ErrRiskGateHalt signals that the pipeline stopped because the risk-assessment
// verdict was anything other than `GO`. It is NOT a failure — the engine has
// already surfaced the full blocked notice through the printer, written the
// gated phase result to state.json, and told the user how to resume. Callers
// wrapping engine errors should use `errors.Is` to detect this case and avoid
// re-rendering it as an `Error:` at the outer layer.
var ErrRiskGateHalt = errors.New("risk gate halted pipeline for human review")

// extractVerdictReason pulls a short human-readable paragraph from the
// risk-assessment artifact so the CLI can print WHY the gate blocked without
// forcing the user to open the file. It looks for the first non-empty,
// non-heading text block after the `**Verdict:** <verdict>` line and returns
// it joined with single spaces, truncated to `limit` characters. Returns "" if
// nothing suitable is found so callers can decide whether to print a hint.
func extractVerdictReason(body []byte, verdict string, limit int) string {
	lines := strings.Split(string(body), "\n")

	verdictIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "**Verdict:**") && strings.Contains(line, verdict) {
			verdictIdx = i
			break
		}
	}
	if verdictIdx < 0 {
		return ""
	}

	var buf []string
	for i := verdictIdx + 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		// Skip empties until we've found something; once we have collected
		// content, an empty line ends the paragraph.
		if line == "" {
			if len(buf) > 0 {
				break
			}
			continue
		}
		// Stop at the next heading unless we haven't collected anything —
		// then skip it and keep scanning so `## Recommendation` headers
		// above the real prose are not a dead end.
		if strings.HasPrefix(line, "#") {
			if len(buf) > 0 {
				break
			}
			continue
		}
		buf = append(buf, line)
	}

	reason := strings.TrimSpace(strings.Join(buf, " "))
	if limit > 0 && len(reason) > limit {
		// Truncate on a word boundary where possible.
		cutoff := limit
		if space := strings.LastIndex(reason[:limit], " "); space > limit*3/4 {
			cutoff = space
		}
		reason = strings.TrimSpace(reason[:cutoff]) + "…"
	}
	return reason
}

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
				reason := extractVerdictReason(body, verdict, 500)
				ctx.Printer.RiskGateBlocked(verdict, artifactPath, reason)
				result.Status = state.StatusGated
				ctx.State.Phases[p.Name()] = result
				state.Save(ctx.Workspace, ctx.State)
				// Wrap the sentinel so the outer layer can recognize this as a
				// designed halt (via errors.Is) and skip the `Error:` prefix.
				// The string form still carries the artifact path for users
				// who view it outside the CLI (e.g. grep logs later).
				return fmt.Errorf("%w: %s (see %s)", ErrRiskGateHalt, verdict, artifactPath)
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
