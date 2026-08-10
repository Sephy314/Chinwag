#!/usr/bin/env bash
# =============================================================================
# Chinwag refresh: rebuild the latest images, load them into k3s, apply the
# current manifests and force every Deployment onto the new images.
#
# This is the "update the running infra to the latest code" fast path. It
# differs from deploy.sh (one-shot install: cert-manager bootstrap, TLS wait,
# health sweep) in that it ALWAYS rebuilds + re-imports and then force-restarts
# the Deployments. imagePullPolicy=IfNotPresent would otherwise keep the
# previously cached image for the unchanged :latest tag, which is the classic
# "stale image" pitfall here.
#
# Run it ON the k3s node (from infra/k3s), with docker available. The repo
# checkout on the node must be on the branch you want to deploy (e.g. after a
# merge: git fetch && git checkout <branch> && git pull).
#
# Usage:
#   ./update.sh                 # build (cached) + load + apply + restart + verify
#   ./update.sh --no-cache      # docker build --no-cache (slower, no stale layers)
#   ./update.sh --frontend      # refresh only the frontend
#   ./update.sh --backends      # refresh only the Go backend services
#   ./update.sh --apply-only    # skip build/load; just apply + restart + verify
#                               #   (use when images are already imported)
#   ./update.sh --no-obs        # skip the observability stack (Loki/Alloy/Grafana)
#
# The observability stack (infra/k3s/observability) is installed/updated too
# (Loki + Grafana Alloy + Prometheus + Grafana via Helm + cert-manager
# grafana-tls), unless --no-obs is given. This is what runs on the CD
# self-hosted runner (see .github/workflows/cd.yml), so a push to main also
# updates the monitoring stack on the production node.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

ROOT_DIR="$(cd ../.. && pwd)"
BACKEND_DIR="${ROOT_DIR}/backend"
FRONTEND_DIR="${ROOT_DIR}/frontend"

NO_CACHE=0
FRONTEND=0
BACKENDS=0
APPLY_ONLY=0
SKIP_OBS=0
for arg in "$@"; do
  case "${arg}" in
    --no-cache)   NO_CACHE=1 ;;
    --frontend)   FRONTEND=1 ;;
    --backends)   BACKENDS=1 ;;
    --apply-only) APPLY_ONLY=1 ;;
    --no-obs)     SKIP_OBS=1 ;;
    *)
      echo "Unknown option: ${arg}" >&2
      echo "Usage: $0 [--no-cache] [--frontend|--backends] [--apply-only] [--no-obs]" >&2
      exit 1
      ;;
  esac
done

# Default: refresh everything.
if [[ "${FRONTEND}" -eq 0 && "${BACKENDS}" -eq 0 ]]; then
  FRONTEND=1
  BACKENDS=1
fi

# --- kubectl helper: prefer plain kubectl, fall back to sudo k3s kubectl ----
if command -v kubectl >/dev/null 2>&1 && kubectl cluster-info >/dev/null 2>&1; then
  KUBECTL="kubectl"
else
  KUBECTL="sudo k3s kubectl"
fi
echo "Using kubectl: ${KUBECTL}"

# --- Build -------------------------------------------------------------------
CACHE_FLAG=()
[[ "${NO_CACHE}" -eq 1 ]] && CACHE_FLAG=(--no-cache)

build_go() {
  local name="$1" service="$2"
  echo "==> Building chinwag/${name}:latest (SERVICE=${service})"
  docker build "${CACHE_FLAG[@]}" \
    --build-arg SERVICE="${service}" \
    -t "chinwag/${name}:latest" \
    -f "${BACKEND_DIR}/Dockerfile" \
    "${BACKEND_DIR}"
}

load_image() {
  local name="$1"
  echo "==> Importing chinwag/${name}:latest into k3s (containerd)"
  docker save "chinwag/${name}:latest" | sudo k3s ctr images import -
}

IMAGES=()
if [[ "${FRONTEND}" -eq 1 ]]; then
  IMAGES+=(frontend)
  if [[ "${APPLY_ONLY}" -eq 0 ]]; then
    echo "==> Building chinwag/frontend:latest"
    docker build "${CACHE_FLAG[@]}" \
      -t "chinwag/frontend:latest" \
      -f "${FRONTEND_DIR}/Dockerfile" \
      "${FRONTEND_DIR}"
    load_image frontend
  fi
fi

if [[ "${BACKENDS}" -eq 1 ]]; then
  IMAGES+=(gateway auth room chat-command chat-query)
  if [[ "${APPLY_ONLY}" -eq 0 ]]; then
    build_go gateway gateway
    build_go auth services/auth
    build_go room services/room
    build_go chat-command services/chat/command
    build_go chat-query services/chat/query
    for img in gateway auth room chat-command chat-query; do
      load_image "${img}"
    done
  fi
fi

# --- Apply latest manifests ---------------------------------------------------
echo "==> Applying manifests (kustomize)"
${KUBECTL} apply -k .

# --- Force Deployments onto the new images ------------------------------------
# rollout restart stamps the pod template (restartedAt) so a new ReplicaSet is
# created and the pods re-resolve the :latest tag from containerd (now pointing
# at the freshly imported image).
DEPLOYS=()
[[ "${FRONTEND}" -eq 1 ]] && DEPLOYS+=(frontend)
[[ "${BACKENDS}" -eq 1 ]] && DEPLOYS+=(gateway auth room chat-command chat-query)

echo "==> Restarting deployments to pick up new images: ${DEPLOYS[*]}"
for d in "${DEPLOYS[@]}"; do
  ${KUBECTL} -n chinwag rollout restart "deploy/${d}"
done

echo "==> Waiting for rollouts"
for d in "${DEPLOYS[@]}"; do
  echo "    -> ${d}"
  ${KUBECTL} -n chinwag rollout status "deploy/${d}" --timeout=180s >/dev/null ||
    { echo "ERROR: ${d} failed to roll out" >&2; exit 1; }
done

# --- Observability stack (Loki + Grafana Alloy + Prometheus + Grafana) ---------
# Deployed from infra/k3s/observability via Helm (see observability/install.sh),
# which also creates the monitoring namespace, the grafana-admin Secret and the
# grafana-dashboards ConfigMap, and issues the grafana-tls certificate (needs
# cert-manager + letsencrypt ClusterIssuer + duckdns webhook, all applied by the
# kustomize step above). Idempotent — safe to run on every deploy.
if [[ "${SKIP_OBS}" -eq 0 ]]; then
  echo "==> Installing/updating observability stack (Loki + Alloy + Prometheus + Grafana)"
  ./observability/install.sh

  echo "==> Waiting for Grafana TLS certificate (grafana-tls) to be Ready"
  ${KUBECTL} -n monitoring wait --for=condition=Ready certificate/grafana-tls --timeout=300s >/dev/null ||
    { echo "ERROR: grafana-tls not ready — see 'kubectl -n monitoring describe certificate/grafana-tls'" >&2; exit 1; }
else
  echo "==> Skipping observability stack (--no-obs)"
fi

# --- Verify -------------------------------------------------------------------
echo "==> Pods"
${KUBECTL} -n chinwag get pods

if [[ "${APPLY_ONLY}" -eq 0 ]]; then
  echo "==> Verifying running image digests match the imported ones"
  for img in "${IMAGES[@]}"; do
    # k3s ctr (not plain ctr) so we read the k3s containerd, matching the
    # import step. REF may be bare or docker.io/-prefixed, so grep for the tag.
    imported="$(sudo k3s ctr -n k8s.io images ls 2>/dev/null | grep -E "chinwag/${img}:latest" | grep -oE 'sha256:[a-f0-9]{64}' | head -1)"
    running="$(${KUBECTL} -n chinwag get pod -l "app=${img}" -o jsonpath='{.items[0].status.containerStatuses[0].imageID}' 2>/dev/null | sed 's|containerd://||')"
    if [[ -n "${imported}" && "${running}" == "${imported}" ]]; then
      echo "  ✔ ${img}: ${running}"
    else
      echo "  ! ${img}: imported=${imported:-?} running=${running:-?}"
    fi
  done
fi

# Frontend bundle sanity check: the /api prefix should be baked in.
if [[ "${FRONTEND}" -eq 1 ]]; then
  echo "==> Frontend bundle sanity check (expect /api/chat/rooms WS path)"
  ${KUBECTL} -n chinwag exec "deploy/frontend" -- sh -c \
    "grep -rl 'api/chat/rooms' .next 2>/dev/null | head -1 || echo 'NO_MATCH (check branch/build)'" \
    || true
fi

echo "==> Health endpoints (via ingress https://chinwag.duckdns.org)"
sleep 3
for path in / /api/auth/health /api/rooms/health /api/chat/health /grafana/api/health; do
  code="$(curl -skL -m 5 -o /dev/null -w '%{http_code}' "https://chinwag.duckdns.org${path}" || true)"
  echo "  GET ${path} -> ${code}"
done

echo "Update complete."
