# Onboarding a New Product

This guide walks you through adding a new product to the `wso2-se-agent` CLI so you can use it to automate resolving GitHub issues for that product.

## What You Need

1. A **product config file** that tells the CLI about your product's repos, skills, and phase budgets
2. A **skills repo** on GitHub containing Claude Code skills and a `CLAUDE.md` for your product
3. Optionally, **pre/post scripts** for phases that need environment cleanup (e.g., stopping servers)

## Step 1: Create the Product Config Directory

Create a folder under your local config directory:

```bash
mkdir -p ~/.wso2-se-agent/products/<product>/<version>/
```

For example, for WSO2 Identity Server 7.0.0:

```bash
mkdir -p ~/.wso2-se-agent/products/identity-server/7.0.0/
```

## Step 2: Write the Product Config

Create `~/.wso2-se-agent/products/<product>/<version>/product-config.yaml`:

```yaml
product: identity-server
version: 7.0.0

# Repos that the CLI needs to set up as git worktrees for each issue.
# name:     logical name used throughout the CLI
# upstream: the GitHub org/repo (used for fork detection in setup-repos)
# branch:   the branch to base worktrees on (fetched from upstream)
repos:
  - name: carbon-identity-framework
    upstream: wso2/carbon-identity-framework
    branch: master
  - name: product-is
    upstream: wso2/product-is
    branch: master

# Ports your product uses. The CLI scans these to find a free port offset
# so multiple workspaces can run in parallel without port conflicts.
runtime:
  default_ports: [9443, 9763, 8280, 8243]

# Max budget (USD) per AI phase. When Claude hits this limit, the phase stops.
# Set higher for complex phases (plan-and-fix) and lower for simple ones (pr).
phase_limits:
  reproduce: 10.0
  risk-assessment: 2.0
  plan-and-fix: 15.0
  verify: 10.0
  test-coverage: 10.0
  pr: 5.0

# Phases to skip. For example, skip test-coverage if your product
# doesn't have a test framework the AI can work with yet.
skip_phases: []

# Skills repo: a GitHub repo containing Claude Code skills for your product.
# The CLI downloads this repo and copies skills into each workspace.
skills_repo: "YourOrg/your-skills-repo"
skills_branch: "main"

# Path within the skills repo to your product-specific skills.
# Must contain a skills/ directory with SKILL.md files, and a CLAUDE.md.
skills_ref: "identity-server-specific/v1"

# Path within the skills repo to generic skills shared across products.
# These are copied first, then product-specific skills override them.
generic_skills_ref: "skills"
```

### Field Reference

| Field | Required | Description |
|-------|----------|-------------|
| `product` | Yes | Product identifier, matches the `--product` flag |
| `version` | Yes | Version identifier, matches the `--version` flag |
| `repos` | Yes | List of repos to set up as worktrees |
| `repos[].name` | Yes | Logical name for the repo |
| `repos[].upstream` | Yes | GitHub `org/repo` for fork detection |
| `repos[].branch` | Yes | Branch to base worktrees on |
| `runtime.default_ports` | Yes | Ports to check for free port offset allocation |
| `phase_limits` | Yes | USD budget cap per AI phase |
| `skip_phases` | No | List of phase names to skip |
| `skills_repo` | Yes | GitHub `org/repo` containing your skills |
| `skills_branch` | Yes | Branch/tag of the skills repo to use |
| `skills_ref` | Yes | Path within skills repo to product-specific skills |
| `generic_skills_ref` | No | Path within skills repo to generic skills (default: `"skills"`) |

## Step 3: Set Up the Skills Repo

The skills repo is where Claude gets its instructions. It needs two things for your product:

### CLAUDE.md

A product context file at `<skills_ref>/CLAUDE.md` that tells Claude how to work with your product — how to build it, start it, patch it, interact with it. This is the most important file. Look at `api-manager-specific/v4/CLAUDE.md` in the existing skills repo for an example.

### Skills (SKILL.md files)

Skill files at `<skills_ref>/skills/<skill-name>/SKILL.md`. Each skill defines what Claude does for a specific phase:

```
your-skills-repo/
├── skills/                              # generic skills (shared across products)
│   └── risk-assessment/
│       └── SKILL.md
└── identity-server-specific/
    └── v1/
        ├── CLAUDE.md                    # product context
        └── skills/
            ├── reproduce/SKILL.md       # overrides generic reproduce if exists
            ├── plan-fix/SKILL.md
            ├── verify-fix/SKILL.md
            ├── create-tests/SKILL.md
            └── submit-fix/SKILL.md
```

The CLI installs skills in this order:
1. Generic skills from `<generic_skills_ref>/` — copied first
2. Product-specific skills from `<skills_ref>/skills/` — copied on top, overriding generic ones with the same name

## Step 4: Add Pre/Post Scripts (Optional)

If your product needs environment cleanup before certain phases (e.g., stopping running servers, freeing ports), create scripts at:

```bash
mkdir -p ~/.wso2-se-agent/products/<product>/<version>/scripts/
```

The naming convention is `<phase-name>.pre.sh` or `<phase-name>.post.sh`:

```
scripts/
├── reproduce.pre.sh       # runs before the reproduce phase
├── plan-and-fix.pre.sh    # runs before plan-and-fix
├── verify.pre.sh          # runs before verify
└── verify.post.sh         # runs after verify (if needed)
```

Scripts receive these environment variables:

| Variable | Description |
|----------|-------------|
| `WSE_WORKSPACE` | Absolute path to the workspace directory |
| `WSE_ISSUE_NUMBER` | Issue number (e.g., `4856`) |
| `WSE_ISSUE_URL` | Full GitHub issue URL |
| `WSE_PRODUCT` | Product name |
| `WSE_VERSION` | Product version |
| `WSE_PORT_OFFSET` | Allocated port offset |
| `WSE_STATE_FILE` | Path to `.wse/state.json` |

A script returning exit code 0 means continue. Non-zero fails the phase.

If a phase has no script file, the CLI simply skips it — no script file means no script runs.

## Step 5: Register Repos and Run

```bash
# Register your repo clones
wso2-se-agent setup-repos --product identity-server --version 7.0.0

# Run against an issue
wso2-se-agent run \
  --product identity-server --version 7.0.0 \
  --issue https://github.com/wso2/product-is/issues/1234 \
  --pack /path/to/wso2is-7.0.0.zip
```

## Example: Minimal Onboarding

If you just want to get started quickly with a single repo and no custom scripts:

```yaml
# ~/.wso2-se-agent/products/my-product/latest/product-config.yaml
product: my-product
version: latest

repos:
  - name: my-repo
    upstream: myorg/my-repo
    branch: main

runtime:
  default_ports: [9443]

phase_limits:
  reproduce: 10.0
  risk-assessment: 2.0
  plan-and-fix: 15.0
  verify: 10.0
  test-coverage: 10.0
  pr: 5.0

skip_phases: ["test-coverage"]

skills_repo: "myorg/my-skills"
skills_branch: "main"
skills_ref: "my-product-specific/v1"
generic_skills_ref: "skills"
```

Then create the skills repo with at minimum a `CLAUDE.md` and a `reproduce/SKILL.md`, and you're ready to go.
