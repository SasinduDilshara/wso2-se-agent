package ai

import "testing"

func TestExtractRiskScore(t *testing.T) {
	cases := []struct {
		name string
		text string
		// want is -1 when no score should be extracted (nil map).
		want float64
	}{
		{
			name: "simple bare score",
			text: "Score: 2/10",
			want: 2,
		},
		{
			name: "bold markdown score with label and recommendation",
			text: "**Score: 7/10 (Medium) — PROCEED**",
			want: 7,
		},
		{
			name: "Risk Score prefix",
			text: "Risk Score: 5/10",
			want: 5,
		},
		{
			name: "markdown heading",
			text: "## Risk Score: 2/10 (Low)",
			want: 2,
		},
		{
			name: "score inside a longer result text",
			text: "Risk assessment written to `.ai/risk-assessment-42.md`.\n\n**Score: 9/10 (High) — DO NOT PROCEED**\n\nSummary: ...",
			want: 9,
		},
		{
			name: "case-insensitive",
			text: "SCORE: 4/10",
			want: 4,
		},
		{
			name: "whitespace variations tolerated",
			text: "Score:  8 / 10",
			want: 8,
		},
		{
			name: "zero is valid",
			text: "Score: 0/10",
			want: 0,
		},
		{
			name: "ten is valid (upper bound)",
			text: "Score: 10/10",
			want: 10,
		},
		{
			name: "out of range → ignored",
			text: "Score: 99/10",
			want: -1,
		},
		{
			name: "no score present → nil",
			text: "I did the assessment but decided not to score it.",
			want: -1,
		},
		{
			name: "empty text → nil",
			text: "",
			want: -1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractRiskScore(tc.text)
			if tc.want < 0 {
				if got != nil {
					t.Errorf("expected nil (no extraction), got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected score %v, got nil", tc.want)
			}
			score, ok := got["risk_score"].(float64)
			if !ok {
				t.Fatalf("risk_score should be float64, got %T: %v", got["risk_score"], got["risk_score"])
			}
			if score != tc.want {
				t.Errorf("score: got %v, want %v", score, tc.want)
			}
		})
	}
}

func TestExtractPRURL(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string // "" means expect nil
	}{
		{
			name: "simple URL on a line",
			text: "PR opened: https://github.com/wso2/apim-apps/pull/1332",
			want: "https://github.com/wso2/apim-apps/pull/1332",
		},
		{
			name: "support-org URL",
			text: "**PR opened:** https://github.com/wso2-support/carbon-apimgt/pull/8141 (base `support-9.32.147.x-full`, head `Tharsanan1:fix/issue-16327`).",
			want: "https://github.com/wso2-support/carbon-apimgt/pull/8141",
		},
		{
			name: "takes the last URL when body also cites older PRs",
			text: "- Based on upstream https://github.com/wso2/carbon-apimgt/pull/13775\n- **PR opened:** https://github.com/wso2-support/carbon-apimgt/pull/8141",
			want: "https://github.com/wso2-support/carbon-apimgt/pull/8141",
		},
		{
			name: "no URL → nil",
			text: "Submitted the PR.",
			want: "",
		},
		{
			name: "empty text → nil",
			text: "",
			want: "",
		},
		{
			name: "non-PR github URL ignored",
			text: "See https://github.com/wso2/carbon-apimgt/issues/13775 for context.",
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractPRURL(tc.text)
			if tc.want == "" {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected pr_url %q, got nil", tc.want)
			}
			url, ok := got["pr_url"].(string)
			if !ok {
				t.Fatalf("pr_url should be string, got %T", got["pr_url"])
			}
			if url != tc.want {
				t.Errorf("pr_url: got %q, want %q", url, tc.want)
			}
		})
	}
}
