package ai

import (
	"fmt"
	"regexp"

	"github.com/Tharsanan1/wso2-se-agent/internal/phase"
	"github.com/Tharsanan1/wso2-se-agent/internal/state"
)

type PRPhase struct {
	runner *AIPhaseRunner
}

func NewPRPhase() *PRPhase {
	return &PRPhase{runner: NewAIPhaseRunner()}
}

func (p *PRPhase) Name() string        { return "pr" }
func (p *PRPhase) Type() phase.PhaseType { return phase.PhaseTypeAI }

func (p *PRPhase) Preconditions(ctx *phase.PhaseContext) error {
	if !ctx.State.PhaseSucceeded("verify") {
		return fmt.Errorf("verify phase must succeed first. Run: --phase verify")
	}
	return nil
}

func (p *PRPhase) Execute(ctx *phase.PhaseContext) (*state.PhaseResult, error) {
	// Pass the full URL — same reason as PlanPhase: skills that hit `gh issue view`
	// (e.g. to read the issue title for the PR description) need the org/repo
	// from the URL, not just the number.
	prompt := fmt.Sprintf("/submit-fix %s", ctx.IssueURL)
	result, err := p.runner.RunAIPhase(ctx, "pr", prompt, 0, extractPRURL)
	// Surface the PR URL on WorkspaceState.PRURL too so `status` can show it
	// after the run. The engine doesn't do this automatically for pr (unlike
	// risk_score); we do it here so the data lands in exactly one place.
	if err == nil && result != nil && result.Metadata != nil {
		if url, ok := result.Metadata["pr_url"].(string); ok && url != "" {
			ctx.State.PRURL = url
		}
	}
	return result, err
}

func (p *PRPhase) ExpectedArtifacts() []string {
	return []string{}
}

// prURLRE matches a GitHub PR URL. Anchors on the /pull/<digits> suffix so it
// won't pick up references to older PRs the agent may have cited while
// writing the PR body (those typically appear earlier in the text as
// `#NNNN` or as plain upstream-PR URLs). The last match wins, which in
// practice is the "**PR opened:**" line at the end of Claude's result.
var prURLRE = regexp.MustCompile(`https://github\.com/[\w.-]+/[\w.-]+/pull/\d+`)

// extractPRURL finds the opened PR's URL in Claude's final text. Returns
// nil (no metadata) if no URL is present — the caller's State.PRURL stays
// empty and `status` prints nothing, which is the desired "unknown" signal.
func extractPRURL(resultText string) map[string]any {
	all := prURLRE.FindAllString(resultText, -1)
	if len(all) == 0 {
		return nil
	}
	return map[string]any{"pr_url": all[len(all)-1]}
}
