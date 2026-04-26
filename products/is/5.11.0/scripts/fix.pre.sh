#!/bin/bash
# Pre-script: clean environment before this phase
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
sh "$SCRIPT_DIR/clean-environment.sh"
