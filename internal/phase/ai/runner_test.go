package ai

import (
	"testing"

	"github.com/Tharsanan1/wso2-se-agent/internal/config"
	"github.com/Tharsanan1/wso2-se-agent/internal/phase"
)

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
