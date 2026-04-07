#!/bin/bash
# Cleanup e2e test templates and projects
# Usage: ./tests/cleanup.sh [launcher_addr]
set -euo pipefail

LAUNCHER_ADDR="${1:-127.0.0.1:8090}"

# Install latest binaries
echo "Installing binaries..."
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
bash "./cmd/install.sh"

# Start launcher if not running
STARTED_LAUNCHER=false
if ! curl -s -o /dev/null "http://${LAUNCHER_ADDR}" 2>/dev/null; then
    echo "Starting launcher on ${LAUNCHER_ADDR}..."
    grroxy start --host "${LAUNCHER_ADDR}" &
    LAUNCHER_PID=$!
    STARTED_LAUNCHER=true
    sleep 3
fi

echo "Cleaning up e2e test data from ${LAUNCHER_ADDR}..."

# Delete test templates
for name in "e2e-add-header" "e2e-modify-path" "e2e-send-request" "e2e-create-label"; do
    RESP=$(curl -s "http://${LAUNCHER_ADDR}/api/collections/_templates/records?filter=(name='${name}')" 2>/dev/null || echo '{"items":[]}')
    ID=$(echo "$RESP" | jq -r '.items[0].id // empty')
    if [ -n "$ID" ]; then
        curl -s -X DELETE "http://${LAUNCHER_ADDR}/api/collections/_templates/records/${ID}" > /dev/null
        echo "  Deleted template: ${name}"
    fi
done

# Delete test projects (e2e-test-*)
PROJECTS=$(curl -s "http://${LAUNCHER_ADDR}/api/collections/_projects/records?perPage=100" 2>/dev/null || echo '{"items":[]}')
PROJECT_IDS=$(echo "$PROJECTS" | jq -r '.items[] | select(.name | startswith("e2e-test-")) | .id')

for ID in $PROJECT_IDS; do
    NAME=$(echo "$PROJECTS" | jq -r ".items[] | select(.id == \"${ID}\") | .name")
    curl -s -X POST "http://${LAUNCHER_ADDR}/api/project/delete" \
        -H "Content-Type: application/json" \
        -d "{\"id\": \"${ID}\"}" > /dev/null 2>&1 \
    || curl -s -X DELETE "http://${LAUNCHER_ADDR}/api/collections/_projects/records/${ID}" > /dev/null 2>&1
    echo "  Deleted project: ${NAME} (${ID})"
done

# Stop launcher if we started it
if [ "$STARTED_LAUNCHER" = true ]; then
    kill "$LAUNCHER_PID" 2>/dev/null || true
    wait "$LAUNCHER_PID" 2>/dev/null || true
    echo "Stopped launcher."
fi

echo "Done."
