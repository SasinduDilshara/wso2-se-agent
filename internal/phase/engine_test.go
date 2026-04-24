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
			name: "first paragraph after verdict line",
			body: `# Risk Assessment

**Verdict:** REVIEW REQUIRED

The fix is a mechanical, surgical backport of an already-merged upstream PR.
Blast radius is low and the root cause is fully understood.

## Fields`,
			verdict: "REVIEW REQUIRED",
			limit:   500,
			want:    "The fix is a mechanical, surgical backport of an already-merged upstream PR. Blast radius is low and the root cause is fully understood.",
		},
		{
			name: "skips leading heading then grabs prose",
			body: `**Verdict:** NO-GO

## Why
This touches the auth layer; out of scope for autofix.`,
			verdict: "NO-GO",
			limit:   500,
			want:    "This touches the auth layer; out of scope for autofix.",
		},
		{
			name: "truncates on a word boundary with ellipsis",
			body: `**Verdict:** REVIEW REQUIRED

` + strings.Repeat("one two three four five ", 20),
			verdict: "REVIEW REQUIRED",
			limit:   60,
			want:    "one two three four five one two three four five one two…",
		},
		{
			name: "returns empty when verdict line not present",
			body: `Some random markdown with no verdict marker.

More text follows.`,
			verdict: "GO",
			limit:   500,
			want:    "",
		},
		{
			name: "returns empty when verdict mismatches",
			body: `**Verdict:** GO

Looks fine.`,
			verdict: "NO-GO",
			limit:   500,
			want:    "",
		},
		{
			name: "handles verdict line at end of file (nothing to extract)",
			body: `**Verdict:** REVIEW REQUIRED
`,
			verdict: "REVIEW REQUIRED",
			limit:   500,
			want:    "",
		},
		{
			name: "no limit when limit is zero",
			body: `**Verdict:** REVIEW REQUIRED

A very long reason that should not be truncated at all.`,
			verdict: "REVIEW REQUIRED",
			limit:   0,
			want:    "A very long reason that should not be truncated at all.",
		},
		{
			name: "joins multi-line paragraph with single spaces",
			body: `**Verdict:** REVIEW REQUIRED

Line one.
Line two.
Line three.

## Next section`,
			verdict: "REVIEW REQUIRED",
			limit:   500,
			want:    "Line one. Line two. Line three.",
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
