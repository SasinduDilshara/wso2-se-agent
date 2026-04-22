# WSO2 SE Agent — Technical Walkthrough

Internal reference doc. Walk through this section by section with the team.

Assumes Go familiarity and general knowledge of Claude Code, git worktrees, and the WSO2 build layout. File references use `path:line` so we can jump to them together.

---

## 1. What it is, in one paragraph

A Go CLI that takes a GitHub issue URL and drives a 10-phase pipeline to produce a reviewed PR. Three phases are deterministic (setup: prereq checks, worktree creation, skill download); seven phases are AI-driven (reproduce → plan → risk-assessment → fix → verify → test-coverage → pr). The AI phases are implemented as invocations of the `claude` CLI in `--output-format stream-json` mode, with per-phase prompts, per-phase USD budget caps, and a hard risk gate between `risk-assessment` and `fix`.

State is persisted as JSON after every phase, so any run is resumable from any step. Each issue lives in its own workspace directory with its own git worktrees — runs don't step on each other.

---

## 2. Repo layout

```
cmd/wso2-se-agent/main.go      entry point; embeds products/
embed.go                        go:embed for products/
internal/
  cmd/                          cobra commands (root, run, config, setup-repos, status, clean)
  phase/
    engine.go                   sequential phase runner + gates
    phase.go                    Phase interface, PhaseContext, PhaseType
    registry.go                 default pipeline registration
    static/                     prereq, workspace, skills
    ai/                         runner.go + per-phase wrappers
  claude/                       invoker, stream-json parser, log writer
  config/                       global config, product config, repo registry
  state/                        WorkspaceState JSON persistence
  git/                          worktree/branch/remote ops
  issue/                        GitHub issue URL parsing
  script/                       pre/post hook shell execution
  ui/                           printer, colored output, prompts
products/                       embedded product configs (apim/latest, is/latest)
tests/                          integration tests with mock claude binary
```

Stats, for context: ~42 Go files, ~4,160 lines in `internal/`. Biggest package is `phase/` at ~1,464 lines. No external test framework; standard `testing` only.

---

## 3. The CLI commands

Six cobra commands. Walk through each.

### `run` — the main pipeline

```bash
wso2-se-agent run \
  --product apim --version latest \
  --issue https://github.com/wso2/product-apim/issues/4856 \
  --pack /path/to/wso2am-4.4.0.zip
```

Key flags:

| Flag | Effect |
|------|--------|
| `--phase <name>` | run a single phase only |
| `--from <phase> --to <phase>` | run a range (resumability) |
| `--yes` | skip the between-phase confirmation prompts (risk gate still applies) |
| `--risk-threshold N` | override the risk gate threshold (default 7) |
| `--max-budget-usd N` | override per-phase budget |
| `--dry-run` | print the phase plan and exit |
| `--verbose` | stream more output |

### `setup-repos`

One-time per product. Registers the local clones the agent will create worktrees from. Auto-detects forks via `gh`, offers to clone missing ones, writes `~/.wso2-se-agent/repos.yaml`.

### `status [workspace]`

Reads `.wse/state.json` from a workspace. Shows issue URL, product, risk score, PR URL (if set), and a table of phase results: name, status, cost USD, duration.

### `clean [workspace] [--all]`

Iterates `WorktreeEntry` records in the state file and removes each worktree cleanly (`git worktree remove`). `--all` nukes the workspace directory too.

### `config init` / `config show`

`init` writes `~/.wso2-se-agent/config.yaml` from interactive prompts (GitHub username, workspace root, risk threshold, Claude model). `show` prints the effective merged config (global + product).

### `version`

`git describe --tags` at build time.

---

## 4. The phase pipeline

Registered in `internal/phase/registry.go`. Default order:

| # | Phase | Type | Prompt to Claude (AI phases) | Produces |
|---|-------|------|------------------------------|----------|
| 1 | prereq | static | — | tool check results |
| 2 | workspace | static | — | `.wse/`, `.ai/logs/`, worktrees per repo |
| 3 | skills | static | — | `.claude/skills/`, generated `CLAUDE.md` |
| 4 | reproduce | AI | `/reproduce <issue-url>` | reproduction notes, logs |
| 5 | plan | AI | `/plan <issue-number>` | fix strategy |
| 6 | risk-assessment | AI | `/risk-assessment <issue-number>` | score 0–10 in `PhaseResult.Metadata["risk_score"]` |
| 7 | fix | AI | `/fix <issue-number>` | code changes in worktree(s) |
| 8 | verify | AI | `/verify-fix <issue-url>` | verification log |
| 9 | test-coverage | AI | `/create-tests <issue-number>` | new tests |
| 10 | pr | AI | `/submit-fix <issue-number>` | PR URL in `PhaseResult.Metadata["pr_url"]` |

Note: CLAUDE.md still lists 9 phases because `plan-and-fix` was recently split into `plan` + `fix` (commit 3c885e2). The registry is the source of truth.

---

## 5. The Phase interface

`internal/phase/phase.go`:

```go
type PhaseType string

const (
    PhaseStatic PhaseType = "static"
    PhaseAI     PhaseType = "ai"
)

type Phase interface {
    Name() string
    Type() PhaseType
    Preconditions(ctx *PhaseContext) error
    Execute(ctx *PhaseContext) (*state.PhaseResult, error)
    ExpectedArtifacts() []string
}
```

Five methods. That's it. Adding a phase means implementing the interface and appending to the registry slice.

`PhaseContext` is the immutable bundle threaded through the pipeline:
- workspace path, issue URL/number
- `GlobalConfig`, `ProductConfig`, `RepoRegistry`
- user flags: auto-approve, risk threshold, budget overrides
- shared `ui.Printer` so all phases produce consistent output

`PhaseResult` (in `internal/state/`):
- `Status`: success / failed / skipped
- `Duration`, `CostUSD`
- `Error`: surfaced error string, if any
- `Metadata map[string]any`: free-form; used for `risk_score`, `pr_url`, etc.

---

## 6. The engine

`internal/phase/engine.go`. The loop is deliberately boring:

1. Load `WorkspaceState` if it exists.
2. For each phase in the registered order:
   a. Check `Preconditions` — fail fast if unmet.
   b. Run `<phase>.pre.sh` if present in the product's scripts dir.
   c. Call `Execute`.
   d. Validate `ExpectedArtifacts` exist on disk.
   e. Run `<phase>.post.sh` if present.
   f. Append `PhaseResult` to `WorkspaceState`, write `.wse/state.json`.
   g. If this phase is `risk-assessment`, check the gate (section 8).
   h. If not `--yes`, prompt the user to continue to the next phase.

Pre/post scripts get these env vars:

```
WORKSPACE, ISSUE_NUMBER, ISSUE_URL, PRODUCT, VERSION, STATE_FILE
```

This is the hook surface for product-specific setup that doesn't belong in Go (starting a DB, custom cleanups, etc.).

---

## 7. Claude invocation

`internal/claude/invoker.go`. Every AI phase calls one shared runner (`internal/phase/ai/runner.go`) which:

1. Builds the prompt from the phase's template + issue context.
2. Spawns:
   ```
   claude -p "<prompt>" \
     --output-format stream-json \
     --max-budget <USD>
   ```
3. Reads stdout line by line. Each line is a JSON object; we parse into `map[string]any` — no generated types, because the schema evolves.
4. For each event, dispatch on the block type:
   - `text` → render + write to processed log
   - `thinking` → render dimmed + write to processed log
   - `tool_use` → render tool name + args + write to both logs
5. Track total cost from the stream's `usage` events.
6. Write **two log files** per phase:
   - `.ai/logs/issue-<N>-<phase>-<ts>.log` — processed, human-readable
   - `.ai/logs/issue-<N>-<phase>-<ts>.log-raw` — every JSON line, verbatim

The raw log is for debugging prompt/tool issues; the processed log is what you actually read.

Cost ends up in `PhaseResult.CostUSD`, which surfaces in `status` output.

---

## 8. Risk gating

`internal/phase/engine.go:106-121`. After `risk-assessment` runs, the engine:

```go
score := result.Metadata["risk_score"].(float64)
if score > ctx.RiskThreshold {
    // halt pipeline; user must re-run with --risk-threshold or fix manually
    return fmt.Errorf("risk score %.1f exceeds threshold %.1f", score, ctx.RiskThreshold)
}
```

Default threshold is 7. Configurable per-run (`--risk-threshold`) or globally (`config.yaml`). The gate is a hard halt in Go, not a prompt to the model — the LLM cannot argue its way past it.

The intent: block autonomous fixes to auth, crypto, core routing, schema migrations, anything that needs a human to look.

---

## 9. Workspace & worktree isolation

`internal/phase/static/workspace.go` + `internal/git/worktree.go`.

Each run creates:

```
<workspace-root>/<product>-<issue-number>/
├── .wse/state.json
├── .ai/logs/
├── .ai/screenshots/
├── .claude/skills/          (populated by skills phase)
├── CLAUDE.md                (generated by skills phase)
├── <repo-1>/                (git worktree)
├── <repo-2>/                (git worktree)
└── ...
```

Worktrees come from the clones registered in `repos.yaml`. Each worktree is on a branch named `fix/issue-<number>`, branched from the upstream base configured in `product-config.yaml`.

Tracked in state as `[]WorktreeEntry{Repo, LocalPath, Branch, BasePath}`. `clean` iterates this list and calls `git worktree remove` on each — no `rm -rf` guessing.

Two consequences worth calling out:
- Runs for different issues never interfere with each other.
- Runs for the same issue can be resumed in place — the worktree is already there.

---

## 10. Configuration layers

Three layers, merged in this order (later wins):

1. **Embedded defaults** — `products/<name>/<version>/product-config.yaml` baked into the binary via `//go:embed` (see `embed.go`, `cmd/wso2-se-agent/main.go`). This is what ships.
2. **User global** — `~/.wso2-se-agent/config.yaml` (from `config init`). GitHub username, workspace root, risk threshold, Claude model.
3. **User per-product override** — `~/.wso2-se-agent/products/<name>/<version>/product-config.yaml` if present, overrides embedded.

`WSE_CONFIG_DIR` env var overrides `~/.wso2-se-agent/`.

`product-config.yaml` controls:
- `repos:` list — which repos to worktree, which base branch to use
- `build:` patterns — how to build the product from source
- `runtime:` — start command, health check URL
- `skills_repo:` — where to pull Claude skills from
- `phase_budgets:` — per-phase USD caps (reproduce $10, plan $5, etc.)
- `skip_phases:` — phases to omit entirely
- `scripts_dir:` — where `<phase>.pre.sh` / `<phase>.post.sh` live

---

## 11. Extension points

**Add a phase**: implement `Phase` in `internal/phase/static/` or `internal/phase/ai/`, append to the slice in `internal/phase/registry.go`. If it's an AI phase, add a prompt template and wrap `ai.RunAIPhase`. Downstream phases should declare it in their `Preconditions`.

**Add a product**: create `products/<name>/<version>/product-config.yaml` following `docs/onboarding-a-new-product.md`. The `//go:embed` picks it up on the next build. Add pre/post scripts if the build or runtime needs them.

**Change the prompt for a phase**: prompts live with the AI phase wrapper. Change it in code, rebuild, done. No remote prompt registry; we want prompt changes to go through code review.

---

## 12. Build & release

Makefile:

```
make build     # bin/wso2-se-agent
make install   # $(go env GOPATH)/bin
make test      # go test ./internal/... ./tests/
make lint      # golangci-lint run
make release   # darwin-arm64, darwin-amd64, linux-amd64
```

Release process: push to main, `git tag vX.Y.Z`, `git push origin vX.Y.Z`. No CI; the Go module proxy serves tagged versions, users pick up via `go install .../cmd/wso2-se-agent@latest`.

Version string comes from `git describe --tags` at build time.

---

## 13. Debugging a run

Order of operations when something goes wrong:

1. `wso2-se-agent status <workspace>` — which phase failed, what was the error.
2. `.wse/state.json` — full `PhaseResult` including metadata.
3. `.ai/logs/issue-<N>-<phase>-<ts>.log` — what Claude was doing.
4. `.ai/logs/issue-<N>-<phase>-<ts>.log-raw` — if the processed log is missing something; full JSON stream.
5. Resume with `wso2-se-agent run --from <phase> ...` once the underlying issue is fixed.

---

## 14. Talking points for the meeting

Things worth pausing on when walking through this doc:

- **Why phases and not one big prompt?** Determinism, resumability, localized failures, budget control. Each phase is a commitable unit of progress.
- **Why stream-json?** We get live progress output, tool-use visibility, and cost tracking for free. The raw log is gold when a run goes sideways.
- **Why a hard risk gate?** The model can't be trusted to self-assess its way past dangerous changes. Fail closed.
- **Why worktrees instead of branches-in-place?** Multiple issues can run concurrently without clobbering each other; `clean` is trivial.
- **Why embed product configs?** Zero-config default path for users; the binary works out of the box.
- **What's not here yet?** Parallel phase execution (sequential today), diff-aware risk scoring (prompt-based today), shared cost/telemetry across runs, more products.

---

## Appendix — quick command reference

```bash
wso2-se-agent run --product <p> --version <v> --issue <url> --pack <zip>
wso2-se-agent run --from fix --to pr                  # resume
wso2-se-agent run --dry-run                           # plan only
wso2-se-agent setup-repos --product <p> --version <v>
wso2-se-agent status <workspace>
wso2-se-agent clean <workspace> [--all]
wso2-se-agent config init
wso2-se-agent config show
wso2-se-agent version
```
