#!/usr/bin/env bash
# =============================================================================
# Deploy Chinwag to a local single-node k3s cluster.
#
# Usage:
#   ./deploy.sh             # build images, load into k3s, apply, wait, verify
#   ./deploy.sh --no-build  # skip image build/load (use already-loaded images)
#   ./deploy.sh --apply     # only apply manifests (no rollout wait / verify)
#
# Requirements:
#   - k3s running on this host (systemd service "k3s")
#   - docker (for building images)
#   - infra/k3s/secret.yaml present on disk (gitignored; it is created from
#     secret.yaml.example:  cp secret.yaml.example secret.yaml)
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

NO_BUILD=0
ONLY_APPLY=0
for arg in "$@"; do
  case "${arg}" in
    --no-build) NO_BUILD=1 ;;
    --apply)    ONLY_APPLY=1 ;;
    *)
      echo "Unknown option: ${arg}" >&2
      echo "Usage: $0 [--no-build] [--apply]" >&2
      exit 1
      ;;
  esac
done

# --- kubectl helper: prefer plain kubectl, fall back to sudo k3s kubectl ----
if command -v kubectl >/dev/null 2>&1 && kubectl cluster-info >/dev/null 2>&1; then
  KUBECTL="kubectl"
else
  KUBECTL="sudo k3s kubectl"
fi
echo "Using kubectl: ${KUBECTL}"

# --- Preflight -------------------------------------------------------------
echo "==> Preflight checks"
if ! systemctl is-active --quiet k3s 2>/dev/null && ! sudo -n systemctl is-active --quiet k3s 2>/dev/null; then
  echo "ERROR: k3s service is not active." >&2
  echo "       Start it with: sudo systemctl start k3s" >&2
  exit 1
fi
if [ ! -f secret.yaml ]; then
  echo "ERROR: infra/k3s/secret.yaml not found." >&2
  echo "       Copy the template and fill in real values:" >&2
  echo "         cp secret.yaml.example secret.yaml" >&2
  exit 1
fi
if ! command -v docker >/dev/null 2>&1; then
  echo "ERROR: docker is not available." >&2
  exit 1
fi
echo "    all checks passed"

# --- TLS secret ------------------------------------------------------------
# Self-signed cert for chinwag.local (created once; idempotent).
# Run `./tls.sh --force` to rotate, or `./tls.sh --ip <node-ip>` to add an IP SAN.
echo "==> Ensuring TLS secret (chinwag-tls)"
./tls.sh

# --- Build + load images ----------------------------------------------------
if [ "${NO_BUILD}" -eq 0 ]; then
  ./build-images.sh --load
else
  ./load-images.sh
fi

# --- Apply manifests --------------------------------------------------------
echo "==> Applying manifests (kustomize)"
${KUBECTL} apply -k .

if [ "${ONLY_APPLY}" -eq 1 ]; then
  echo "Done (apply only). Check status with: ${KUBECTL} -n chinwag get pods"
  exit 0
fi

# --- Wait for rollouts --------------------------------------------------------
echo "==> Waiting for deployments to roll out"
for deploy in gateway auth room chat-command chat-query frontend redis; do
  echo "    -> ${deploy}"
  ${KUBECTL} -n chinwag rollout status "deploy/${deploy}" --timeout=120s >/dev/null ||
    { echo "ERROR: ${deploy} failed to roll out" >&2; exit 1; }
done
echo "    -> postgres"
${KUBECTL} -n chinwag rollout status statefulset/postgres --timeout=120s >/dev/null ||
  { echo "ERROR: postgres failed to roll out" >&2; exit 1; }
echo "    -> nats"
${KUBECTL} -n chinwag rollout status statefulset/nats --timeout=120s >/dev/null ||
  { echo "ERROR: nats failed to roll out" >&2; exit 1; }

# --- Verify -------------------------------------------------------------------
echo "==> Pods"
${KUBECTL} -n chinwag get pods

echo "==> Health endpoints (via ingress https://chinwag.local)"
if grep -q "chinwag.local" /etc/hosts; then
  sleep 5
  for path in / /auth/health /rooms/health /chat/health; do
    # -k: self-signed cert; -L: follow the http->https redirect if any
    code="$(curl -skL -m 5 -o /dev/null -w '%{http_code}' "https://chinwag.local${path}" || true)"
    echo "  GET ${path} -> ${code}"
  done
else
  echo "  WARN: chinwag.local not in /etc/hosts — add:"
  echo "        127.0.0.1 chinwag.local"
fi

echo "Deployment complete."
