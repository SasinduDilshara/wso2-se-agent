#!/bin/bash
# Post-workspace script: download additional context files to workspace
set -e

echo "  Downloading additional workspace files..."

# Download a sample API definition for testing
curl -sL "https://petstore3.swagger.io/api/v3/openapi.json" -o "$WSE_WORKSPACE/sample-openapi.json" 2>/dev/null || true

if [ -f "$WSE_WORKSPACE/sample-openapi.json" ]; then
  echo "  Downloaded sample-openapi.json"
else
  echo "  Warning: failed to download sample file (skipped)"
fi
