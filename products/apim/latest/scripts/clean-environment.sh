#!/bin/bash
# Shared cleanup: stop WSO2 servers, free ports, delete extracted packs.
# Called as a pre-script before AI phases to guarantee a clean slate.
#
# Process-killing policy (from product-docs/starting-the-server.md):
#   Always use `pkill -f "wso2carbon"` — never kill by specific PID.
#   Multiple MI/APIM processes (wrappers, children) may be running, and
#   PID-based kills often miss children and leave ports occupied.
set -e

cd "$WSE_WORKSPACE"
echo "  Cleaning environment..."

# Graceful stop via --stop for any pack that has a PID file
for PACK_DIR in wso2am-*/; do
  [ -d "$PACK_DIR" ] || continue
  if [ -f "$PACK_DIR/wso2carbon.pid" ]; then
    (cd "$PACK_DIR/bin" && sh api-manager.sh --stop 2>/dev/null) || true
  fi
done

# Give graceful shutdown a brief window to complete
sleep 5

# Force-kill any surviving carbon processes by pattern (NOT by PID)
pkill -f "wso2carbon" 2>/dev/null || true
sleep 2
pkill -9 -f "wso2carbon" 2>/dev/null || true
sleep 2

# MANDATORY: verify no carbon processes remain before we touch the pack dir
REMAINING=$(ps aux | grep '[w]so2carbon' | grep -v grep || true)
if [ -n "$REMAINING" ]; then
  echo "ERROR: WSO2 Carbon processes still running after pkill -9:"
  echo "$REMAINING"
  exit 1
fi

# Account for port offset.
# NOTE on WSE_PORT_OFFSET semantics: the BASE_PORT values below are APIM's
# documented defaults (no offset applied). WSE_PORT_OFFSET is the raw offset
# value set in deployment.toml's [server] section — e.g., WSE_PORT_OFFSET=1
# → 9444/9764/8281/8244/5673.
OFFSET=${WSE_PORT_OFFSET:-0}
for BASE_PORT in 9443 9763 8280 8243 5672; do
  PORT=$((BASE_PORT + OFFSET))
  PORT_PID=$(lsof -ti :$PORT 2>/dev/null) || true
  if [ -n "$PORT_PID" ]; then
    kill -9 "$PORT_PID" 2>/dev/null || true
    sleep 1
  fi
done

# Delete extracted pack directories (keep the .zip) — always re-extract fresh
rm -rf wso2am-*/ 2>/dev/null || true

echo "  Environment clean."
