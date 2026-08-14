#!/usr/bin/env bash
# =============================================================================
# Install the Chinwag Observability stack (Loki + Grafana Alloy + Prometheus +
# Alertmanager + Grafana + Chinwag Notifier) into the `monitoring` namespace.
#
# Usage:   ./install.sh          (as the k3s user)
#          sudo ./install.sh     (also works — root PATH often lacks helm)
#
# The script bootstraps its own tooling if needed:
#   - installs `helm` into /usr/local/bin (root/sudo) or ~/.local/bin (user)
#     when it is missing from PATH
#   - installs a standalone `kubectl` when the one on PATH (e.g. the k3s
#     wrapper) cannot reach the cluster as the current user
#
# The Chinwag Notifier (Alertmanager -> Discord) is also deployed here: its
# image is built + imported into k3s (requires docker + k3s on this host), and
# the Discord webhook URL is read from $DISCORD_WEBHOOK_URL (or a gitignored
# ./notifier.env) and stored in the chinwag-notifier-secrets Secret — it is
# NEVER committed to the repo or written to the logs.
#
# Order matters:
#   1. monitoring namespace
#   2. Loki (Alloy/Grafana depend on it)
#   3. Alloy (ships logs into Loki)
#   4. Prometheus (+ Alertmanager, rules -> notifier webhook)
#   5. Chinwag Notifier (receives Alertmanager webhooks, sends Discord embeds)
#   6. grafana-admin Secret (random password, never committed)
#   7. Grafana (provisions the Loki datasource)
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

# Versions pinned to the ones this stack was validated with (see README).
LOKI_CHART_VERSION="7.2.0"
ALLOY_CHART_VERSION="1.11.1"
GRAFANA_CHART_VERSION="10.5.15"
PROMETHEUS_CHART_VERSION="29.23.0"
# Optional override, e.g. KUBECTL_VERSION=v1.36.3 ./install.sh
KUBECTL_VERSION="${KUBECTL_VERSION:-}"

# --- Tool bootstrap: helm + kubectl ------------------------------------------
# Pick a writable bin dir: /usr/local/bin when we can write to it (root/sudo),
# otherwise the current user's ~/.local/bin.
BIN_DIR="${BIN_DIR:-}"
if [ -z "${BIN_DIR}" ]; then
  if [ -w /usr/local/bin ]; then
    BIN_DIR="/usr/local/bin"
  else
    BIN_DIR="${HOME}/.local/bin"
    mkdir -p "${BIN_DIR}"
  fi
fi
PATH="${BIN_DIR}:${PATH}"
export PATH

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)  echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) echo "ERROR: unsupported architecture: $(uname -m)" >&2; return 1 ;;
  esac
}

install_helm() {
  local ver arch
  arch="$(detect_arch)"
  ver="$(curl -s https://api.github.com/repos/helm/helm/releases/latest | grep '"tag_name"' | head -1 | cut -d'"' -f4)"
  [ -n "${ver}" ] || ver="v4.2.3"
  echo "==> helm not found on PATH — installing ${ver} (linux-${arch}) to ${BIN_DIR}/helm"
  curl -fsSL "https://get.helm.sh/helm-${ver}-linux-${arch}.tar.gz" -o /tmp/chinwag-helm.tgz
  tar -xzf /tmp/chinwag-helm.tgz -C /tmp "linux-${arch}/helm"
  install -m 0755 "/tmp/linux-${arch}/helm" "${BIN_DIR}/helm"
  rm -rf /tmp/chinwag-helm.tgz "/tmp/linux-${arch}"
}

install_kubectl() {
  local ver arch
  arch="$(detect_arch)"
  ver="${KUBECTL_VERSION:-$(curl -sL https://dl.k8s.io/release/stable.txt)}"
  [ -n "${ver}" ] || ver="v1.36.3"
  echo "==> installing standalone kubectl ${ver} (linux-${arch}) to ${BIN_DIR}/kubectl"
  curl -fsSL "https://dl.k8s.io/release/${ver}/bin/linux/${arch}/kubectl" -o "${BIN_DIR}/kubectl"
  chmod +x "${BIN_DIR}/kubectl"
}

# helm
if ! command -v helm >/dev/null 2>&1; then
  install_helm
fi
HELM="$(command -v helm)"

# Resolve a kubeconfig that BOTH kubectl and helm can use.
# (helm does not go through the k3s wrapper, so it needs an explicit
#  KUBECONFIG. Under `sudo`, HOME=/root, so also fall back to the k3s admin
#  kubeconfig at /etc/rancher/k3s/k3s.yaml, which root can read.)
if [ -n "${KUBECONFIG:-}" ] && [ -r "${KUBECONFIG}" ]; then
  : # user-provided KUBECONFIG is already set and readable
elif [ -r "${HOME}/.kube/config" ]; then
  export KUBECONFIG="${HOME}/.kube/config"
elif [ -r /etc/rancher/k3s/k3s.yaml ]; then
  export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
fi

# kubectl: make a working client available. Both the standalone client and the
# k3s wrapper honour $KUBECONFIG; fall back to the standalone binary only if the
# resolved client still cannot reach the cluster.
if ! command -v kubectl >/dev/null 2>&1; then
  install_kubectl
elif ! kubectl cluster-info >/dev/null 2>&1; then
  install_kubectl
fi
KUBECTL="$(command -v kubectl)"

echo "==> Using helm   : ${HELM}"
echo "==> Using kubectl: ${KUBECTL} (KUBECONFIG=${KUBECONFIG:-<default>})"
echo

echo "==> Creating monitoring namespace"
"${KUBECTL}" create namespace monitoring --dry-run=client -o yaml | "${KUBECTL}" apply -f -

echo "==> Adding Grafana Helm repo"
"${HELM}" repo add grafana https://grafana.github.io/helm-charts >/dev/null 2>&1 || true
"${HELM}" repo update grafana >/dev/null

echo "==> Adding prometheus-community Helm repo"
"${HELM}" repo add prometheus-community https://prometheus-community.github.io/helm-charts >/dev/null 2>&1 || true
"${HELM}" repo update prometheus-community >/dev/null

echo "==> Installing Loki (monolithic / filesystem / single replica)"
"${HELM}" upgrade --install loki grafana/loki \
  --namespace monitoring \
  --version "${LOKI_CHART_VERSION}" \
  --values loki-values.yaml \
  --wait --timeout 10m

echo "==> Installing Grafana Alloy (DaemonSet log collector)"
"${HELM}" upgrade --install alloy grafana/alloy \
  --namespace monitoring \
  --version "${ALLOY_CHART_VERSION}" \
  --values alloy-values.yaml \
  --wait --timeout 10m

echo "==> Installing Prometheus (metrics: CPU/RAM via cAdvisor, pod counts via kube-state-metrics)"
"${HELM}" upgrade --install prometheus prometheus-community/prometheus \
  --namespace monitoring \
  --version "${PROMETHEUS_CHART_VERSION}" \
  --values prometheus-values.yaml \
  --wait --timeout 10m

# --- Chinwag Notifier (Alertmanager -> Discord) ------------------------------
# Stateless Go service that receives Alertmanager webhooks (the Alertmanager
# receiver is configured in prometheus-values.yaml) and forwards Discord embeds
# to a Discord webhook. The image is built + imported into k3s here. The Discord
# webhook URL is read from $DISCORD_WEBHOOK_URL (or a gitignored ./notifier.env)
# and stored in the chinwag-notifier-secrets Secret — never committed, never
# echoed to the logs.
echo "==> Deploying Chinwag Notifier (Alertmanager -> Discord)"

if [ -f notifier.env ]; then
  set -a; . ./notifier.env; set +a
fi

if [ -n "${DISCORD_WEBHOOK_URL:-}" ]; then
  "${KUBECTL}" -n monitoring create secret generic chinwag-notifier-secrets \
    --from-literal=DISCORD_WEBHOOK_URL="${DISCORD_WEBHOOK_URL}" \
    --dry-run=client -o yaml | "${KUBECTL}" -n monitoring apply -f -
  echo "    chinwag-notifier-secrets Secret updated (DISCORD_WEBHOOK_URL)"
else
  echo "    WARNING: DISCORD_WEBHOOK_URL not set (env or notifier.env) — notifier will start but alerts will not be delivered"
fi

if command -v docker >/dev/null 2>&1; then
  echo "==> Building chinwag/notifier:latest"
  docker build -t chinwag/notifier:latest -f notifier/Dockerfile notifier/
  if sudo -n k3s ctr version >/dev/null 2>&1; then
    echo "==> Importing chinwag/notifier:latest into k3s (containerd)"
    docker save chinwag/notifier:latest | sudo k3s ctr images import -
  elif sudo -n ctr version >/dev/null 2>&1; then
    echo "==> Importing chinwag/notifier:latest into k3s (containerd)"
    docker save chinwag/notifier:latest | sudo ctr -n k8s.io images import -
  else
    echo "    WARNING: no k3s/ctr available to import the image — assuming chinwag/notifier:latest is already loaded"
  fi
else
  echo "    WARNING: docker not found — skipping image build (assuming chinwag/notifier:latest is already loaded)"
fi

echo "==> Applying Chinwag Notifier manifests (Deployment + Service)"
"${KUBECTL}" apply -f notifier.yaml
"${KUBECTL}" -n monitoring rollout status deploy/chinwag-notifier --timeout=120s >/dev/null ||
  { echo "ERROR: chinwag-notifier failed to roll out" >&2; exit 1; }

echo "==> Creating Grafana admin Secret (random password, not committed to Git)"
if ! "${KUBECTL}" -n monitoring get secret grafana-admin >/dev/null 2>&1; then
  ADMIN_PASS="$(openssl rand -base64 24 | tr -d '=+/' | head -c 24)"
  "${KUBECTL}" -n monitoring create secret generic grafana-admin \
    --from-literal=admin-user=admin \
    --from-literal=admin-password="${ADMIN_PASS}"
  echo "    admin user : admin"
  echo "    admin pass : ${ADMIN_PASS}   (keep safe — shown once)"
  # Optional: write it to a gitignored file so you can look it up later.
  # printf 'admin:%s\n' "${ADMIN_PASS}" > .grafana-admin.txt && chmod 600 .grafana-admin.txt
else
  echo "    grafana-admin Secret already exists — keeping existing password."
fi

echo "==> Issuing Grafana ingress TLS certificate (grafana-tls / letsencrypt)"
"${KUBECTL}" apply -f grafana-certificate.yaml

echo "==> Creating grafana-dashboards ConfigMap (provisioned dashboards)"
# NOTE: must specify -n monitoring on BOTH sides. Without it, kubectl uses the
# current kubeconfig context's default namespace (e.g. `default` on the CD
# runner), so the ConfigMap lands in the wrong namespace and the Grafana pod
# fails to mount it -> "configmap \"grafana-dashboards\" not found",
# ContainerCreating, Deployment not Available.
"${KUBECTL}" -n monitoring create configmap grafana-dashboards \
  --from-file=dashboards/ \
  --dry-run=client -o yaml | "${KUBECTL}" -n monitoring apply -f -

echo "==> Installing Grafana (Loki + Prometheus datasources, provisioned dashboards)"
# Before upgrading, delete the Grafana Deployment if it exists. Why: when the
# strategy is switched from RollingUpdate (chart default) to Recreate, Helm's
# server-side apply keeps the previously-owned `strategy.rollingUpdate` field on
# the live Deployment, which the API server rejects with:
#   spec.strategy.rollingUpdate: Forbidden: may not be specified when strategy
#   'type' is 'Recreate'
# Setting `rollingUpdate: null` in grafana-values.yaml alone does NOT clear it
# (Helm 4 drops the null before SSA). Deleting the Deployment and letting Helm
# recreate it fresh (with the PVC intact — Grafana data persists) resolves it.
# On a first install the Deployment does not exist yet, so this is a no-op.
"${KUBECTL}" -n monitoring delete deployment grafana --ignore-not-found --wait=true --timeout=120s >/dev/null 2>&1 || true
"${HELM}" upgrade --install grafana grafana/grafana \
  --namespace monitoring \
  --version "${GRAFANA_CHART_VERSION}" \
  --values grafana-values.yaml \
  --wait --timeout 10m

echo
echo "==> Done. Verify with:"
echo "    ${HELM} list -n monitoring"
echo "    ${KUBECTL} get pods -n monitoring"
echo "    ${KUBECTL} get svc -n monitoring"
echo "    ${KUBECTL} get daemonset -n monitoring"
echo "    ${KUBECTL} -n monitoring get certificate grafana-tls"
echo "    ${KUBECTL} -n monitoring get configmap grafana-dashboards"
echo "    ${KUBECTL} -n monitoring get deploy/chinwag-notifier"
echo
echo "    # Alerting: set DISCORD_WEBHOOK_URL (or notifier.env), then"
echo "    #   curl -s localhost:9095/health   (port-forward the notifier to test)"
echo
echo "    # Grafana UI (public, HTTPS): https://chinwag.duckdns.org/grafana"
echo "    # Dashboard: Dashboards -> chinwag -> Chinwag Overview"
echo "    # Grafana UI (port-forward, then open http://localhost:3001):"
echo "    # (3000 is the frontend's port, so Grafana uses local 3001)"
echo "    ${KUBECTL} -n monitoring port-forward svc/grafana 3001:80"
