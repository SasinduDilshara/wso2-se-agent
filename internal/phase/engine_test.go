package phase

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tharsanan1/wso2-se-agent/internal/config"
)

func TestShellQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "''"},
		{"simple", "simple"},
		{"/usr/local/bin", "/usr/local/bin"},
		{"https://github.com/org/repo/issues/1", "https://github.com/org/repo/issues/1"},
		{"/path with spaces/file.zip", "'/path with spaces/file.zip'"},
		{"has'apostrophe", `'has'\''apostrophe'`},
		{"star*glob", "'star*glob'"},
		{"dollar$var", "'dollar$var'"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := shellQuote(tc.in)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildResumeCommand(t *testing.T) {
	ctxWith := func(workspace string) *PhaseContext {
		return &PhaseContext{
			Workspace:   workspace,
			IssueURL:    "https://github.com/wso2/product-apim/issues/4856",
			IssueNumber: "4856",
			PackPath:    "/path with spaces/wso2am-4.7.0.zip",
			ProductConfig: &config.ProductConfig{
				Product: "apim",
				Version: "4.6.0",
			},
			GlobalConfig: &config.GlobalConfig{
				WorkspaceRoot: "/Users/u/wse-workspaces",
			},
		}
	}

	t.Run("default workspace is omitted", func(t *testing.T) {
		ctx := ctxWith(filepath.Join("/Users/u/wse-workspaces", "apim-issues-4856"))
		got := buildResumeCommand(ctx, "fix")
		want := "wso2-se-agent fix --product apim --version 4.6.0 --issue https://github.com/wso2/product-apim/issues/4856 --pack '/path with spaces/wso2am-4.7.0.zip' --from fix"
		if got != want {
			t.Errorf("got  %q\nwant %q", got, want)
		}
	})

	t.Run("explicit workspace is emitted and quoted when it has spaces", func(t *testing.T) {
		ctx := ctxWith("/my custom/workspace/apim-16437")
		got := buildResumeCommand(ctx, "fix")
		if !strings.Contains(got, "--workspace '/my custom/workspace/apim-16437'") {
			t.Errorf("expected quoted --workspace in resume command, got: %s", got)
		}
	})

	t.Run("no pack path omits the flag", func(t *testing.T) {
		ctx := ctxWith("")
		ctx.PackPath = ""
		got := buildResumeCommand(ctx, "verify")
		if strings.Contains(got, "--pack") {
			t.Errorf("should not emit --pack when PackPath is empty, got: %s", got)
		}
		if !strings.HasSuffix(got, "--from verify") {
			t.Errorf("fromPhase should terminate the command, got: %s", got)
		}
	})
}

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
