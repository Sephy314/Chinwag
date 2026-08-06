#!/usr/bin/env bash
# =============================================================================
# Imports locally-built Chinwag images into the k3s containerd runtime.
# Run after ./build-images.sh when k3s runs on the same host as Docker.
# =============================================================================
set -euo pipefail

IMAGES=(gateway auth room chat-command chat-query frontend)

for img in "${IMAGES[@]}"; do
  echo "==> Importing chinwag/${img}:latest"
  docker save "chinwag/${img}:latest" | sudo k3s ctr images import -
done

echo "Done. Images are now available to the k3s cluster."
echo "Verify with: sudo k3s ctr images ls | grep chinwag"
