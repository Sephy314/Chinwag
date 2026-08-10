#!/usr/bin/env bash
# =============================================================================
# Install the Chinwag Observability stack (Loki + Grafana Alloy + Grafana)
# into the `monitoring` namespace using Helm.
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
# Order matters:
#   1. monitoring namespace
#   2. Loki (Alloy/Grafana depend on it)
#   3. Alloy (ships logs into Loki)
#   4. grafana-admin Secret (random password, never committed)
#   5. Grafana (provisions the Loki datasource)
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

# Versions pinned to the ones this stack was validated with (see README).
LOKI_CHART_VERSION="7.2.0"
ALLOY_CHART_VERSION="1.11.1"
GRAFANA_CHART_VERSION="10.5.15"
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

echo "==> Installing Loki (monolithic / filesystem / single replica)"
"${HELM}" upgrade --install loki grafana/loki \
  --namespace monitoring \
  --version "${LOKI_CHART_VERSION}" \
  --values loki-values.yaml \
  --wait

echo "==> Installing Grafana Alloy (DaemonSet log collector)"
"${HELM}" upgrade --install alloy grafana/alloy \
  --namespace monitoring \
  --version "${ALLOY_CHART_VERSION}" \
  --values alloy-values.yaml \
  --wait

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

echo "==> Installing Grafana (Loki datasource auto-provisioned)"
"${HELM}" upgrade --install grafana grafana/grafana \
  --namespace monitoring \
  --version "${GRAFANA_CHART_VERSION}" \
  --values grafana-values.yaml \
  --wait

echo
echo "==> Done. Verify with:"
echo "    ${HELM} list -n monitoring"
echo "    ${KUBECTL} get pods -n monitoring"
echo "    ${KUBECTL} get svc -n monitoring"
echo "    ${KUBECTL} get daemonset -n monitoring"
echo "    ${KUBECTL} -n monitoring get certificate grafana-tls"
echo
echo "    # Grafana UI (public, HTTPS): https://chinwag.duckdns.org/grafana"
echo "    # Grafana UI (port-forward, then open http://localhost:3001):"
echo "    # (3000 is the frontend's port, so Grafana uses local 3001)"
echo "    ${KUBECTL} -n monitoring port-forward svc/grafana 3001:80"
