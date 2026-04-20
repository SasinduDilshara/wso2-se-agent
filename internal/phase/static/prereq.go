package static

import (
	"fmt"
	"os/exec"

	"github.com/SasinduDilshara/wso2-se-agent/internal/phase"
	"github.com/SasinduDilshara/wso2-se-agent/internal/state"
)

type PrereqPhase struct{}

func NewPrereqPhase() *PrereqPhase { return &PrereqPhase{} }

func (p *PrereqPhase) Name() string        { return "prereq" }
func (p *PrereqPhase) Type() phase.PhaseType { return phase.PhaseTypeStatic }

func (p *PrereqPhase) Preconditions(ctx *phase.PhaseContext) error {
	return nil
}

func (p *PrereqPhase) Execute(ctx *phase.PhaseContext) (*state.PhaseResult, error) {
	result := &state.PhaseResult{
		Phase:    "prereq",
		Metadata: make(map[string]any),
	}

	required := []string{"git", "claude", "gh"}
	var missing []string

	for _, bin := range required {
		if _, err := exec.LookPath(bin); err != nil {
			missing = append(missing, bin)
		}
	}

	// Check optional tools based on product
	optional := map[string]bool{
		"java":    false,
		"mvn":     false,
		"node":    false,
		"python3": false,
	}
	for bin := range optional {
		if _, err := exec.LookPath(bin); err == nil {
			optional[bin] = true
		}
	}

	// Check repos are registered
	for _, repo := range ctx.ProductConfig.Repos {
		if _, ok := ctx.RepoRegistry.Repos[repo.Name]; !ok {
			missing = append(missing, fmt.Sprintf("repo:%s (run: wso2-se-agent setup-repos --product %s --version %s)",
				repo.Name, ctx.ProductConfig.Product, ctx.ProductConfig.Version))
		}
	}

	// Print results
	headers := []string{"Check", "Status"}
	var rows [][]string
	for _, bin := range required {
		status := "OK"
		if _, err := exec.LookPath(bin); err != nil {
			status = "MISSING"
		}
		rows = append(rows, []string{bin, status})
	}
	for _, repo := range ctx.ProductConfig.Repos {
		status := "OK"
		if _, ok := ctx.RepoRegistry.Repos[repo.Name]; !ok {
			status = "NOT REGISTERED"
		}
		rows = append(rows, []string{"repo:" + repo.Name, status})
	}
	ctx.Printer.Table(headers, rows)

	if len(missing) > 0 {
		result.Status = state.StatusFailed
		result.Error = fmt.Sprintf("missing: %v", missing)
		return result, fmt.Errorf("pre-flight checks failed: %v", missing)
	}

	result.Status = state.StatusSuccess
	return result, nil
}

func (p *PrereqPhase) ExpectedArtifacts() []string {
	return []string{}
}
