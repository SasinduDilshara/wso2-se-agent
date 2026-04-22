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
- Note: If architecture is arm64, Oracle XE Docker images require \`--platform linux/amd64\` flag and may be slow or unavailable. Consider code-level analysis for Oracle-specific issues instead of running a real Oracle instance.
EOF

echo "  Appended system info to CLAUDE.md"
