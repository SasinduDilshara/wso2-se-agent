#!/bin/bash
# Shared cleanup: stop WSO2 MI servers, free ports, delete extracted packs.
# Called as a pre-script before AI phases to guarantee a clean slate.
set -e

cd "$WSE_WORKSPACE"
echo "  Cleaning environment..."

# Graceful stop via --stop for any pack that has a PID file
for PACK_DIR in wso2mi-*/; do
  [ -d "$PACK_DIR" ] || continue
  if [ -f "$PACK_DIR/wso2carbon.pid" ]; then
    (cd "$PACK_DIR/bin" && sh micro-integrator.sh --stop 2>/dev/null) || true
  fi
done

# Wait for graceful shutdown (up to 30s)
for PID_FILE in wso2mi-*/wso2carbon.pid; do
  [ -f "$PID_FILE" ] || continue
  PID=$(cat "$PID_FILE" 2>/dev/null) || continue
  if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
    WAIT=0
    while kill -0 "$PID" 2>/dev/null && [ $WAIT -lt 30 ]; do
      sleep 2; WAIT=$((WAIT+2))
    done
    kill -9 "$PID" 2>/dev/null || true
  fi
done

# Kill any orphaned WSO2 carbon / MI processes (preferred per MI docs)
pkill -f "wso2carbon" 2>/dev/null || true
sleep 2
pkill -9 -f "wso2carbon" 2>/dev/null || true

# Also clean up any stray Bootstrap processes
for PID in $(ps aux | grep 'org.wso2.carbon.bootstrap.Bootstrap' | grep -v grep | awk '{print $2}'); do
  kill "$PID" 2>/dev/null || true
  sleep 3
  kill -9 "$PID" 2>/dev/null || true
done

# Account for port offset
OFFSET=${WSE_PORT_OFFSET:-0}
for BASE_PORT in 8290 8253 9164 9201; do
  PORT=$((BASE_PORT + OFFSET))
  PORT_PID=$(lsof -ti :$PORT 2>/dev/null) || true
  if [ -n "$PORT_PID" ]; then
    kill -9 "$PORT_PID" 2>/dev/null || true
    sleep 1
  fi
done

# Delete extracted pack directories (keep the .zip)
rm -rf wso2mi-*/ 2>/dev/null || true

echo "  Environment clean."
