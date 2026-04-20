# CLAUDE.md

## Project overview

WSO2 SE Agent is a Go CLI that automates resolving GitHub issues on WSO2 products using AI agents. It orchestrates a 9-phase pipeline: prereq, workspace, skills, reproduce, risk-assessment, plan-and-fix, verify, test-coverage, pr. Static phases handle setup (git worktrees, tool checks, skills download); AI phases invoke Claude Code to do the actual work.

## Build and test

```bash
make build          # compile to bin/wso2-se-agent
make install        # install to $(go env GOPATH)/bin
make test           # go test ./internal/...
make lint           # golangci-lint run
make release        # cross-compile darwin-arm64, darwin-amd64, linux-amd64
```

Run all tests including integration:
```bash
go test ./internal/... ./tests/ -v
```

## Project structure

```
cmd/wso2-se-agent/main.go     # entry point, embeds product configs
embed.go                       # go:embed for products/ directory
internal/
  cmd/                         # CLI commands (cobra): root, run, config, setup-repos, status, clean
  phase/                       # phase orchestration engine
    engine.go                  # sequential phase runner
    phase.go                   # Phase interface
    registry.go                # default pipeline registration
    static/                    # static phases: prereq, workspace, skills
    ai/                        # AI phases: runner invokes claude CLI per phase
  claude/                      # Claude CLI invoker, stream-json parser, log writer
  config/                      # global config, product config, repo registry (YAML)
  state/                       # workspace state (state.json), phase results
  git/                         # git operations: worktree, branch, remote, fetch
  issue/                       # GitHub issue URL parser
  script/                      # pre/post phase shell script execution
  ui/                          # terminal output: printer, colored output, prompts
products/                      # embedded product configs (apim/latest/product-config.yaml)
tests/                         # integration tests with mock claude binary and fixtures
```

## Key architecture

- **Phase interface**: `Name()`, `Type()` (static/ai), `Preconditions()`, `Execute()`, `ExpectedArtifacts()`
- **PhaseContext**: carries all config, state, printer, and user input through the pipeline
- **State management**: `WorkspaceState` saved as JSON after each phase for resumability
- **Claude invocation**: spawns `claude -p <prompt> --output-format stream-json`, parses JSON lines for text, thinking, and tool_use blocks
- **Risk gating**: risk-assessment phase returns a score (0-10); pipeline blocks if score > threshold (default 7)
- **Git worktrees**: each issue gets isolated worktrees from upstream branches, named `fix/issue-<number>`

## Code conventions

- Error wrapping: `fmt.Errorf("context: %w", err)`
- Phase results: return `*state.PhaseResult` with status, duration, cost, error, metadata
- Configs: YAML files in `~/.wso2-se-agent/` with embedded defaults as fallback
- No external test frameworks; standard `testing` package only
- `map[string]any` for JSON parsing of Claude stream output (no generated types)

## Release process

Push to main, create a git tag (`vX.Y.Z`), push the tag. Users install via `go install ...@latest`. No CI/CD pipelines; the Go module proxy serves tagged versions.

```bash
git tag v0.X.Y && git push origin v0.X.Y
```

## Common workflows

**Adding a new phase**: implement the `Phase` interface in `internal/phase/static/` or `internal/phase/ai/`, register it in `internal/phase/registry.go`.

**Adding a new product**: create `products/<name>/<version>/product-config.yaml` following `docs/onboarding-a-new-product.md`.

**Debugging a run**: check `<workspace>/.wse/state.json` for phase results, `<workspace>/.ai/logs/` for processed and raw Claude logs.
