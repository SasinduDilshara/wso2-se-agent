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

if [ -n "${WSE_PORT_OFFSET:-}" ]; then
  OFFSET="$WSE_PORT_OFFSET"
  cat >> "$CLAUDE_MD" <<EOF

## Port Offset

A port offset of **$OFFSET** has been allocated for this workspace to avoid
conflicts with other running WSO2 servers on this machine.

- Pass \`-DportOffset=$OFFSET\` when starting the server (or set \`Ports.Offset\`
  in \`repository/conf/carbon.xml\`).
- All default ports are shifted by $OFFSET:
  9443 → $((9443 + OFFSET)), 9000 → $((9000 + OFFSET)), 9001 → $((9001 + OFFSET)).
- Use the shifted ports for curl, health checks, API calls, and any docs
  or test code you generate. Do NOT use the default ports.
EOF
fi

echo "  Appended system info to CLAUDE.md"
