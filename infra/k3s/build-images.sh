#!/usr/bin/env bash
# =============================================================================
# Builds all Chinwag Docker images.
#
# Usage:
#   ./build-images.sh            # build only
#   ./build-images.sh --load     # build + import into local k3s (containerd)
#
# The WebSocket base is derived at runtime from the browser origin
# (src/services/websocket-client.ts), so no build-time WS arg is needed.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BACKEND_DIR="${ROOT_DIR}/backend"
FRONTEND_DIR="${ROOT_DIR}/frontend"

LOAD="${1:-}"

build_go() {
  local name="$1" service="$2"
  echo "==> Building chinwag/${name}:latest (SERVICE=${service})"
  docker build \
    --build-arg SERVICE="${service}" \
    -t "chinwag/${name}:latest" \
    -f "${BACKEND_DIR}/Dockerfile" \
    "${BACKEND_DIR}"
}

# --- Backend Go services ------------------------------------------------
build_go gateway gateway
build_go auth services/auth
build_go room services/room
build_go chat-command services/chat/command
build_go chat-query services/chat/query

# --- Frontend -----------------------------------------------------------
echo "==> Building chinwag/frontend:latest"
docker build \
  -t "chinwag/frontend:latest" \
  -f "${FRONTEND_DIR}/Dockerfile" \
  "${FRONTEND_DIR}"

# --- Optional: import into local k3s -------------------------------------
if [[ "${LOAD}" == "--load" ]]; then
  echo "==> Importing images into k3s (sudo k3s ctr images import)"
  for img in gateway auth room chat-command chat-query frontend; do
    docker save "chinwag/${img}:latest" | sudo k3s ctr images import -
  done
fi

echo "Done."
