#!/usr/bin/env bash
# =============================================================================
# K3s infrastructure integration test for CI — runs infra/test.sh against an
# EPHEMERAL k3d cluster on a GitHub-hosted runner (ubuntu-latest).
#
# GitHub-hosted runners have no permanent cluster, and the self-hosted runner
# points at the production node (running the test there would apply the PR's
# manifests to production). So this script creates a throwaway k3d cluster,
# installs cert-manager (the kustomize manifests require its CRDs), builds the
# Chinwag images, imports them, and runs the real test against it. The dev and
# production clusters are never touched.
#
# What it does:
#   1. installs k3d + kubectl if missing
#   2. builds all 6 Chinwag images IN PARALLEL (background jobs — single runner,
#      no extra Actions minutes; GHA layer cache speeds up repeat runs)
#   3. while the builds run, creates a throwaway k3d cluster (Traefik ingress +
#      local-path storage) and installs cert-manager (needed by
#      clusterissuer/certificate/duckdns CRs)
#   4. ensures infra/k3s/secret.yaml exists (uses the local one, else a dummy)
#   5. imports all images into k3d in ONE call (single tools-node spin-up)
#   6. pre-applies kustomize and waits for the internal certs (chinwag-ca,
#      postgres-tls) so the StatefulSets don't hit a missing-secret race
#   7. runs infra/test.sh (which re-applies idempotently and verifies runtime)
#   8. tears the cluster down
#
# Usage:
#   ./infra/ci-test.sh
#   K3S_TEST_TIMEOUT=300 ./infra/ci-test.sh   # longer readiness wait
#
# CI: invoked by the `k3s-infra` job in .github/workflows/ci.yml on every push
# and PR. Requires Docker (available on GitHub-hosted runners).
# =============================================================================
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
K8S_DIR="${SCRIPT_DIR}/k3s"

CLUSTER="chinwag-ci"
CERT_MANAGER_VERSION="v1.21.1"
IMAGES=(gateway auth room chat-command chat-query frontend)
SERVICE_ARGS=(gateway:gateway auth:services/auth room:services/room chat-command:services/chat/command chat-query:services/chat/query)

# GitHub Actions exposes these when setup-buildx-action creates a container
# driver; add GHA layer caching only then (plain `docker build` otherwise).
CACHE_ARGS=()
if [ -n "${ACTIONS_CACHE_URL:-}" ]; then
  CACHE_ARGS=(--cache-from=type=gha --cache-to=type=gha,mode=max)
fi

# --- Helpers -------------------------------------------------------------
say()  { echo "==> $*"; }
die()  { echo "ERROR: $*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"; }

docker_build() {
  local tag="$1" dockerfile="$2" context="$3"; shift 3
  if command -v docker buildx >/dev/null 2>&1 && docker buildx ls >/dev/null 2>&1; then
    docker buildx build "${CACHE_ARGS[@]}" -t "${tag}" -f "${dockerfile}" --load "$@" "${context}"
  else
    docker build -t "${tag}" -f "${dockerfile}" "$@" "${context}"
  fi
}

cleanup() {
  # Kill any still-running background builds, then tear down the cluster.
  for pid in "${BUILD_PIDS[@]:-}"; do kill "${pid}" >/dev/null 2>&1 || true; done
  say "Tearing down k3d cluster '${CLUSTER}'"
  k3d cluster delete "${CLUSTER}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# --- Parallel build helpers -------------------------------------------------
# Build all images concurrently (single runner — no extra Actions minutes):
# each `docker build` is CPU/network-bound, so wall-clock drops from the sum of
# 6 sequential builds to roughly the slowest one. GHA layer cache (buildx
# type=gha, see CACHE_ARGS) makes repeat runs even faster.
BUILD_PIDS=()
build_in_bg() {
  local tag="$1" dockerfile="$2" context="$3"; shift 3
  say "Building ${tag} (background)"
  docker_build "${tag}" "${dockerfile}" "${context}" "$@" &
  BUILD_PIDS+=("$!")
}
wait_for_builds() {
  local p rc=0
  for p in "${BUILD_PIDS[@]}"; do
    wait "${p}" || rc=1
  done
  BUILD_PIDS=()
  [ "${rc}" -eq 0 ] || die "one or more parallel image builds failed"
}

export PATH="${HOME}/.local/bin:${PATH}"

# --- 1. Tooling ------------------------------------------------------------
# Install to ~/.local/bin (already on PATH above) — no sudo needed, so it works
# both on GitHub-hosted runners and on dev boxes without NOPASSWD sudo.
if ! command -v k3d >/dev/null 2>&1; then
  say "Installing k3d to ${HOME}/.local/bin"
  mkdir -p "${HOME}/.local/bin"
  case "$(uname -m)" in
    x86_64|amd64) K3D_ARCH="amd64" ;;
    aarch64|arm64) K3D_ARCH="arm64" ;;
    *) die "unsupported architecture: $(uname -m)" ;;
  esac
  K3D_VER="$(curl -fsSL https://api.github.com/repos/k3d-io/k3d/releases/latest \
    | grep '"tag_name"' | head -1 | cut -d'"' -f4)"
  [ -n "${K3D_VER}" ] || K3D_VER="v5.8.4"
  curl -fsSL "https://github.com/k3d-io/k3d/releases/download/${K3D_VER}/k3d-linux-${K3D_ARCH}" \
    -o "${HOME}/.local/bin/k3d"
  chmod +x "${HOME}/.local/bin/k3d"
fi
if ! command -v kubectl >/dev/null 2>&1; then
  say "Installing kubectl to ${HOME}/.local/bin"
  mkdir -p "${HOME}/.local/bin"
  ver="$(curl -fsSL https://dl.k8s.io/release/stable.txt)"
  curl -fsSL -o /tmp/chinwag-kubectl "https://dl.k8s.io/release/${ver}/bin/linux/amd64/kubectl"
  install -m 0755 /tmp/chinwag-kubectl "${HOME}/.local/bin/kubectl"
fi
need k3d
need kubectl
need docker

# --- 2. Build images (PARALLEL) -----------------------------------------------
# Kick off all 6 image builds in the background, then do the cluster + cert
# setup while they run, then import once. This overlaps the ~30-60s of builds
# with cluster creation instead of serializing them.
for entry in "${SERVICE_ARGS[@]}"; do
  name="${entry%%:*}"; svc="${entry##*:}"
  build_in_bg "chinwag/${name}:latest" "${ROOT_DIR}/backend/Dockerfile" "${ROOT_DIR}/backend" \
    --build-arg SERVICE="${svc}"
done
build_in_bg "chinwag/frontend:latest" "${ROOT_DIR}/frontend/Dockerfile" "${ROOT_DIR}/frontend"

# --- 3. Ephemeral cluster ---------------------------------------------------
# (Runs while the image builds are still going in the background.)
say "Creating k3d cluster '${CLUSTER}'"
k3d cluster create "${CLUSTER}" --servers 1 --agents 0 --wait
KUBECONFIG_FILE="$(mktemp)"
k3d kubeconfig write "${CLUSTER}" -o "${KUBECONFIG_FILE}"
export KUBECONFIG="${KUBECONFIG_FILE}"
kubectl cluster-info >/dev/null 2>&1 || die "cannot reach the ephemeral cluster"

# --- 4. cert-manager ----------------------------------------------------------
# (Also runs while the image builds are still going in the background.)
say "Installing cert-manager ${CERT_MANAGER_VERSION}"
kubectl apply -f \
  "https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml"
kubectl -n cert-manager rollout status deploy/cert-manager deploy/cert-manager-webhook deploy/cert-manager-cainjector --timeout=180s

# --- 5. secret.yaml (gitignored) ----------------------------------------------
# Prefer the existing local secret (a dev checkout already has real values that
# work on any cluster since postgres/redis are recreated inside k3d). In CI the
# checkout has no secret.yaml, so generate a self-consistent dummy one.
if [ ! -f "${K8S_DIR}/secret.yaml" ]; then
  say "Generating test secret.yaml (dummy values — not committed)"
  PW="chinwag-ci-pw"
  cat > "${K8S_DIR}/secret.yaml" <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: chinwag-secrets
  namespace: chinwag
type: Opaque
stringData:
  POSTGRES_PASSWORD: "${PW}"
  REDIS_PASSWORD: "${PW}"
  AUTH_DB_URL: "postgres://chinwag:${PW}@postgres:5432/chinwag_auth?sslmode=disable"
  ROOM_DB_URL: "postgres://chinwag:${PW}@postgres:5432/chinwag_room?sslmode=disable"
  CHAT_DB_URL: "postgres://chinwag:${PW}@postgres:5432/chinwag_chat?sslmode=disable"
  CHAT_QUERY_DB_URL: "postgres://chinwag:${PW}@postgres:5432/chinwag_chat_projection?sslmode=disable"
  GOOGLE_CLIENT_ID: ""
  GOOGLE_CLIENT_SECRET: ""
  GOOGLE_REDIRECT_URL: ""
EOF
fi

# Wait for the parallel builds to finish, then import everything in ONE call
# (k3d only spins up its tools node once, instead of once per image).
wait_for_builds
say "Importing all images into k3d (single call)"
k3d image import -c "${CLUSTER}" "${IMAGES[@]/#/chinwag/:latest}"

# --- 6. Pre-apply + wait for internal certs ------------------------------------
# Postgres mounts the `postgres-tls` secret (issued by cert-manager from the
# self-signed `chinwag-ca`). Applying the manifests and waiting for these certs
# BEFORE test.sh prevents a CreateContainerConfigError race (pod scheduled
# before the secret exists). test.sh re-applies idempotently afterwards.
say "Pre-applying manifests and waiting for internal certificates"
kubectl apply -k "${K8S_DIR}"
kubectl -n chinwag wait --for=condition=Ready certificate/chinwag-ca --timeout=120s
kubectl -n chinwag wait --for=condition=Ready certificate/postgres-tls --timeout=120s

# --- 7. Run the real test --------------------------------------------------------
say "Running infra/test.sh against the ephemeral cluster"
chmod +x "${SCRIPT_DIR}/test.sh"
"${SCRIPT_DIR}/test.sh"

say "Done — CI infra test passed."
