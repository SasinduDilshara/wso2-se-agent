package ai

import (
	"testing"

	"github.com/Tharsanan1/wso2-se-agent/internal/config"
	"github.com/Tharsanan1/wso2-se-agent/internal/phase"
	"github.com/Tharsanan1/wso2-se-agent/internal/state"
)

// ctxFor builds a minimal PhaseContext for budget-resolution tests. Only the
// fields resolvePhaseBudget actually reads are populated.
func ctxFor(maxBudget, maxTotal float64, phaseLimits map[string]float64, priorPhases map[string]float64) *phase.PhaseContext {
	ws := &state.WorkspaceState{Phases: map[string]*state.PhaseResult{}}
	for name, cost := range priorPhases {
		ws.Phases[name] = &state.PhaseResult{Phase: name, CostUSD: cost}
	}
	pc := &config.ProductConfig{PhaseLimits: phaseLimits}
	return &phase.PhaseContext{
		ProductConfig:     pc,
		State:             ws,
		MaxBudgetUSD:      maxBudget,
		MaxTotalBudgetUSD: maxTotal,
	}
}

func TestResolvePhaseBudget_LayersAndTotalCap(t *testing.T) {
	cases := []struct {
		name             string
		maxBudget        float64
		maxTotal         float64
		phaseLimits      map[string]float64
		override         float64
		priorPhases      map[string]float64
		phase            string
		wantBudget       float64
		wantHalt         bool
		wantNarrowedFrom float64
	}{
		{
			name:       "explicit override wins over everything",
			maxBudget:  15, override: 3.0, phase: "fix",
			phaseLimits: map[string]float64{"fix": 20},
			wantBudget:  3.0,
		},
		{
			name:       "phase_limits beats global when no override",
			maxBudget:  15, phase: "plan",
			phaseLimits: map[string]float64{"plan": 5},
			wantBudget:  5,
		},
		{
			name:       "global used when no phase_limits entry",
			maxBudget:  15, phase: "verify",
			phaseLimits: map[string]float64{"plan": 5},
			wantBudget:  15,
		},
		{
			name:       "total cap leaves plenty of headroom — per-phase layers stand",
			maxBudget:  15, maxTotal: 100, phase: "fix",
			phaseLimits: map[string]float64{"fix": 10},
			priorPhases: map[string]float64{"reproduce": 4, "plan": 2},
			wantBudget:  10,
		},
		{
			name:             "total cap narrows the per-phase budget",
			maxBudget:        15, maxTotal: 20, phase: "fix",
			phaseLimits:      map[string]float64{"fix": 15},
			priorPhases:      map[string]float64{"reproduce": 4, "plan": 2}, // spent 6, remaining 14
			wantBudget:       14,
			wantNarrowedFrom: 15,
		},
		{
			name:      "total cap exactly matches spend — halts",
			maxBudget: 15, maxTotal: 10, phase: "fix",
			priorPhases: map[string]float64{"reproduce": 10},
			wantHalt:    true,
		},
		{
			name:      "total cap exceeded — halts, narrowing not applied",
			maxBudget: 15, maxTotal: 5, phase: "fix",
			priorPhases: map[string]float64{"reproduce": 4, "plan": 2},
			wantHalt:    true,
		},
		{
			name:       "total cap of 0 means disabled — no narrowing, no halt",
			maxBudget:  15, maxTotal: 0, phase: "fix",
			priorPhases: map[string]float64{"reproduce": 100},
			wantBudget:  15,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := ctxFor(tc.maxBudget, tc.maxTotal, tc.phaseLimits, tc.priorPhases)
			budget, halt, _, narrowedFrom := resolvePhaseBudget(ctx, tc.phase, tc.override)
			if halt != tc.wantHalt {
				t.Fatalf("halt: got %v, want %v", halt, tc.wantHalt)
			}
			if tc.wantHalt {
				return // budget/narrowedFrom are indeterminate when halt fires
			}
			if budget != tc.wantBudget {
				t.Errorf("budget: got %v, want %v", budget, tc.wantBudget)
			}
			if narrowedFrom != tc.wantNarrowedFrom {
				t.Errorf("narrowedFrom: got %v, want %v", narrowedFrom, tc.wantNarrowedFrom)
			}
		})
	}
}

func TestResolveModel(t *testing.T) {
	cases := []struct {
		name        string
		override    string
		phaseModels map[string]string
		globalModel string
		phaseName   string
		want        string
	}{
		{
			name:        "override beats everything",
			override:    "claude-haiku-4-5-20251001",
			phaseModels: map[string]string{"reproduce": "claude-opus-4-7"},
			globalModel: "claude-sonnet-4-6",
			phaseName:   "reproduce",
			want:        "claude-haiku-4-5-20251001",
		},
		{
			name:        "phase override beats global default",
			phaseModels: map[string]string{"fix": "claude-opus-4-7"},
			globalModel: "claude-sonnet-4-6",
			phaseName:   "fix",
			want:        "claude-opus-4-7",
		},
		{
			name:        "falls back to global when phase not in map",
			phaseModels: map[string]string{"reproduce": "claude-opus-4-7"},
			globalModel: "claude-sonnet-4-6",
			phaseName:   "fix",
			want:        "claude-sonnet-4-6",
		},
		{
			name:        "falls back to global when phase map nil",
			globalModel: "claude-sonnet-4-6",
			phaseName:   "fix",
			want:        "claude-sonnet-4-6",
		},
		{
			name:      "returns empty when nothing is set",
			phaseName: "fix",
			want:      "",
		},
		{
			name:        "empty phase entry falls through to global",
			phaseModels: map[string]string{"fix": ""},
			globalModel: "claude-sonnet-4-6",
			phaseName:   "fix",
			want:        "claude-sonnet-4-6",
		},
		{
			name:     "override wins even when global and phase are empty",
			override: "claude-opus-4-7",
			want:     "claude-opus-4-7",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &phase.PhaseContext{
				ModelOverride: tc.override,
				GlobalConfig: &config.GlobalConfig{
					ClaudeModel: tc.globalModel,
					PhaseModels: tc.phaseModels,
				},
			}
			got := resolveModel(ctx, tc.phaseName)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
