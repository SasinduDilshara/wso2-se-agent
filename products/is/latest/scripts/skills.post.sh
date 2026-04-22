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

echo "  Appended system info to CLAUDE.md"
