#!/bin/bash
# Post-skills script: append system context to CLAUDE.md
set -e

CLAUDE_MD="$WSE_WORKSPACE/CLAUDE.md"

if [ ! -f "$CLAUDE_MD" ]; then
  exit 0
fi

ARCH=$(uname -m)
OS=$(uname -s)
DOCKER_ARCH=$(docker info --format '{{.Architecture}}' 2>/dev/null || echo "not available")

cat >> "$CLAUDE_MD" <<EOF

## System Info (auto-detected)
- Architecture: $ARCH
- OS: $OS
- Docker platform: $DOCKER_ARCH
EOF

if [ -n "${WSE_PORT_OFFSET:-}" ] && [ "${WSE_PORT_OFFSET}" -gt 0 ]; then
  OFFSET="$WSE_PORT_OFFSET"
  cat >> "$CLAUDE_MD" <<EOF

## Port Offset

A port offset of **$OFFSET** has been allocated for this workspace to avoid
conflicts with other running WSO2 servers on this machine.

- Pass \`-DportOffset=$OFFSET\` when starting the server (or set \`Ports.Offset\`
  in \`repository/conf/carbon.xml\`).
- All default ports are shifted by $OFFSET:
  9443 → $((9443 + OFFSET)), 9763 → $((9763 + OFFSET)),
  8280 → $((8280 + OFFSET)), 8243 → $((8243 + OFFSET)),
  5672 → $((5672 + OFFSET)).
- Use the shifted ports for curl, health checks, API calls, and any docs
  or test code you generate. Do NOT use the default ports.
EOF
fi

cat >> "$CLAUDE_MD" <<EOF

## Working on additional WSO2 repos

This workspace is for APIM **$WSE_VERSION** (support). If your fix needs a
WSO2 repo that isn't already a worktree in this workspace, follow these rules
— do NOT default to \`wso2/*\` or \`master\`:

1. **Org**: always \`wso2-support/<repo>\`.
2. **Find the target version in \`carbon-apimgt-support/pom.xml\`.** Look in its
   \`<properties>\` block. Skip \`product-apim-support/pom.xml\` — it's a thin
   aggregator and has no version properties. Property names are NOT
   mechanically derivable from the repo name; search by best-guess string
   and skim the results. Real examples (versions shown are from APIM 4.6.0
   and will differ for other APIM versions — only the shape matters):
   - \`carbon-kernel\` → \`<carbon.kernel.version>4.9.33</...>\`
   - \`carbon-identity-framework\` → \`<carbon.identity.version>5.25.736</...>\` (note the unexpected name)
   - \`identity-inbound-auth-oauth\` → \`<carbon.identity-inbound-auth-oauth.version>6.13.41</...>\`
   - \`carbon-commons\` → \`<carbon.commons.version>4.9.18</...>\`
   - \`carbon-mediation\` → \`<carbon.mediation.version>4.7.263</...>\`
   - \`carbon-deployment\` → \`<carbon.deployment.version>4.11.24</...>\`
   - \`carbon-multitenancy\` → \`<carbon.multitenancy.version>4.9.42</...>\`
   - \`carbon-governance\` → \`<carbon.governance.version>4.8.37</...>\`
   - \`carbon-registry\` → \`<carbon.registry.version>4.8.50</...>\`
   - If a value uses \`\${something}\`, resolve the reference to its literal
     version before step 3.
3. **Build the branch name**: take the first three dot-segments of the
   version (strip trailing \`-SNAPSHOT\`, \`-wso2vNN\`, build qualifiers, etc.).
   Branch is \`support-<X>.<Y>.<Z>.x-full\` on \`wso2-support/<repo>\`.
   - \`4.9.33\` → \`support-4.9.33.x-full\`
   - \`9.32.147.71-SNAPSHOT\` → \`support-9.32.147.x-full\`
4. **Before opening a PR** against \`wso2-support/<repo>\`, verify a fork
   exists under your GitHub user. If not, stop and report — do not push.
EOF

echo "  Appended system info and support-branches guidance to CLAUDE.md"
