package phase

import (
	"strings"
	"testing"
)

func TestExtractVerdictReason(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		verdict string
		limit   int
		want    string
	}{
		{
			name: "reason on the same line as the bold marker",
			body: `**Verdict:** REVIEW REQUIRED
**Inputs:** ia-16437.md y, plan-16437.md y

## Recommendation

**REVIEW REQUIRED.** The fix is a mechanical, surgical backport. No NO-GO forcing rule fires.
`,
			verdict: "REVIEW REQUIRED",
			limit:   500,
			want:    "The fix is a mechanical, surgical backport. No NO-GO forcing rule fires.",
		},
		{
			name: "NO-GO marker with its own reason",
			body: `**Verdict:** NO-GO

## Recommendation

**NO-GO.** Touches the auth layer; out of scope for autofix.
`,
			verdict: "NO-GO",
			limit:   500,
			want:    "Touches the auth layer; out of scope for autofix.",
		},
		{
			name:    "reason ends at EOF (no trailing newline)",
			body:    `**REVIEW REQUIRED.** Prose without a trailing newline`,
			verdict: "REVIEW REQUIRED",
			limit:   500,
			want:    "Prose without a trailing newline",
		},
		{
			name:    "truncates on a word boundary with ellipsis",
			body:    `**REVIEW REQUIRED.** ` + strings.Repeat("one two three four five ", 20),
			verdict: "REVIEW REQUIRED",
			limit:   60,
			want:    "one two three four five one two three four five one two…",
		},
		{
			name:    "returns empty when marker is missing",
			body:    "## Risk Assessment\n\nSome prose without the bold marker.\n",
			verdict: "REVIEW REQUIRED",
			limit:   500,
			want:    "",
		},
		{
			name: "returns empty when marker is the wrong verdict",
			body: `**GO.** Safe.
`,
			verdict: "REVIEW REQUIRED",
			limit:   500,
			want:    "",
		},
		{
			name: "limit of zero means no truncation",
			body: `**REVIEW REQUIRED.** A very long reason that should not be truncated at all.
`,
			verdict: "REVIEW REQUIRED",
			limit:   0,
			want:    "A very long reason that should not be truncated at all.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractVerdictReason([]byte(tc.body), tc.verdict, tc.limit)
			if got != tc.want {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
}
