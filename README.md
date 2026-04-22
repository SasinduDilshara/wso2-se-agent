# wso2-se-agent

A CLI that developers can use to automate resolving GitHub issues on a product using AI agents.

## Prerequisites

- **Go 1.22+** — for installation via `go install`
- **git** — for worktree management and version control
- **[Claude Code](https://claude.ai/code)** (`claude` CLI) — AI agent that does the actual work
- **[GitHub CLI](https://cli.github.com/)** (`gh`) — for GitHub API access (fork detection, skills download, PR creation)
- **GitHub authentication** — `gh auth login` must be completed
- **Java + Maven** — required for building WSO2 products (`java`, `mvn`)
- **Node.js** — required for products with frontend components (`node`, `npm`)

## Install

```bash
go install github.com/Tharsanan1/wso2-se-agent/cmd/wso2-se-agent@latest
```

Make sure `$(go env GOPATH)/bin` is in your PATH.

## Uninstall

```bash
rm $(which wso2-se-agent)
rm -rf ~/.wso2-se-agent
```

## Quick Start

```bash
# 1. Register your repo clones for a product (one-time setup)
wso2-se-agent setup-repos --product apim --version latest

# 2. Run against an issue
wso2-se-agent run \
  --product apim --version latest \
  --issue https://github.com/wso2/product-apim/issues/4856 \
  --pack /path/to/wso2am-4.4.0.zip
```

GitHub username is auto-detected from `gh`. No `config init` needed — sensible defaults are used out of the box.

## Commands

| Command | Description |
|---------|-------------|
| `setup-repos` | Register local repo clones (auto-detects forks, offers to clone) |
| `run` | Run the issue-fixing pipeline |
| `status <workspace>` | Show phase results, costs, risk score |
| `clean <workspace>` | Remove worktrees and clean up |
| `config init` | (Optional) Create config with custom settings |
| `config show` | Show current configuration |

## Pipeline Phases

| # | Phase | Type | Description |
|---|-------|------|-------------|
| 1 | prereq | static | Check required tools and registered repos |
| 2 | workspace | static | Create git worktrees, copy product pack |
| 3 | skills | static | Download and install Claude skills, generate CLAUDE.md |
| 4 | reproduce | ai | Reproduce the bug |
| 5 | plan | ai | Plan the fix approach |
| 6 | risk-assessment | ai | Score fix risk (0-10), gate high-risk issues |
| 7 | fix | ai | Implement the fix |
| 8 | verify | ai | Verify the fix resolves the issue |
| 9 | test-coverage | ai | Write regression tests |
| 10 | pr | ai | Create pull request |

## Run Flags

```
--product        Product name (required)
--version        Product version (required)
--issue          GitHub issue URL (required)
--pack           Path to product pack zip (required)
--phase          Run a single phase only
--from / --to    Run a range of phases
--yes            Skip pause prompts (risk gate still applies)
--risk-threshold Override risk threshold (0-10, default 7)
--max-budget-usd Override per-phase budget in USD
--dry-run        Show pipeline plan without executing
--verbose        Verbose output
```

## Configuration

Configuration is optional. The CLI works with sensible defaults out of the box.

Set `WSE_CONFIG_DIR` to override the config directory location (useful for isolated/test setups). The default is `~/.wso2-se-agent/`.

To customize settings, run `wso2-se-agent config init` or edit files directly:

```
~/.wso2-se-agent/
├── config.yaml                          # global settings (optional)
├── repos.yaml                           # registered repo clones
└── products/
    └── apim/
        └── latest/
            ├── product-config.yaml      # product-specific config (editable)
            └── scripts/                 # pre/post phase scripts
                ├── clean-environment.sh
                ├── reproduce.pre.sh
                ├── plan-and-fix.pre.sh
                └── verify.pre.sh
```

## Testing

```bash
go test ./internal/... ./tests/ -v
```
