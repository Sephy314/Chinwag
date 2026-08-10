#!/usr/bin/env bash
# =============================================================================
# Set a fixed Grafana admin password on any cluster running the Chinwag
# observability stack (works on the dev cluster and the production server).
#
# Why both steps? The grafana-admin Secret is only read by Grafana at FIRST
# startup. To change an already-provisioned admin password you must reset it
# inside Grafana itself (grafana-cli), then update the Secret so a future Helm
# reinstall keeps the same value.
#
# Usage (run on the target host — production server or dev machine):
#   ./set-grafana-password.sh                     # prompts for user + password
#   GRAFANA_ADMIN_USER='admin' GRAFANA_PASSWORD='...' ./set-grafana-password.sh
#                                                 # or via env, not argv
#   sudo ./set-grafana-password.sh                # works too (kubeconfig fallback)
#
# The username and password are never taken from argv and never echoed, so they
# stay out of shell history / process list on the local side.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

NAMESPACE="${GRAFANA_NAMESPACE:-monitoring}"
DEPLOYMENT="${GRAFANA_DEPLOYMENT:-grafana}"
SECRET="${GRAFANA_SECRET:-grafana-admin}"
LOCAL_PASS_FILE=".grafana-admin.txt"

# --- kubectl --------------------------------------------------------------
# Same resolution order as install.sh: honour $KUBECONFIG, then ~/.kube/config,
# then the k3s admin kubeconfig (readable by root — makes `sudo` work).
if [ -z "${KUBECONFIG:-}" ] && [ -r "${HOME}/.kube/config" ]; then
  export KUBECONFIG="${HOME}/.kube/config"
fi
if ! command -v kubectl >/dev/null 2>&1; then
  if [ -x "${HOME}/.local/bin/kubectl" ]; then
    export PATH="${HOME}/.local/bin:${PATH}"
  else
    echo "ERROR: kubectl not found on PATH." >&2
    echo "Run ./install.sh first (it bootstraps kubectl), or put kubectl on PATH." >&2
    exit 1
  fi
fi
KUBECTL="$(command -v kubectl)"
echo "==> kubectl: ${KUBECTL} (KUBECONFIG=${KUBECONFIG:-<default>})"

# --- admin user + new password (env var or interactive input) --------------
if [ -n "${GRAFANA_ADMIN_USER:-}" ]; then
  ADMIN_USER="${GRAFANA_ADMIN_USER}"
else
  read -r -p "Grafana admin username [admin]: " ADMIN_USER
  ADMIN_USER="${ADMIN_USER:-admin}"
fi

if [ -n "${GRAFANA_PASSWORD:-}" ]; then
  NEW_PASS="${GRAFANA_PASSWORD}"
else
  read -r -s -p "New Grafana admin password (input hidden): " NEW_PASS
  echo
fi
if [ -z "${NEW_PASS}" ]; then
  echo "ERROR: empty password — aborting." >&2
  exit 1
fi

# --- reset inside Grafana ---------------------------------------------------
echo "==> Resetting admin password in Grafana (${NAMESPACE}/${DEPLOYMENT})"
"${KUBECTL}" -n "${NAMESPACE}" exec "deploy/${DEPLOYMENT}" -- \
  grafana-cli admin reset-admin-password "${NEW_PASS}"

# --- keep the Secret in sync (so reinstall keeps the same password) --------
echo "==> Updating ${SECRET} Secret (${NAMESPACE}) for future reinstalls"
"${KUBECTL}" -n "${NAMESPACE}" create secret generic "${SECRET}" \
  --from-literal=admin-user="${ADMIN_USER}" \
  --from-literal=admin-password="${NEW_PASS}" \
  --dry-run=client -o yaml | "${KUBECTL}" apply -f -

# Keep the gitignored lookup file in sync if one exists.
if [ -f "${LOCAL_PASS_FILE}" ]; then
  printf '%s:%s\n' "${ADMIN_USER}" "${NEW_PASS}" > "${LOCAL_PASS_FILE}"
  chmod 600 "${LOCAL_PASS_FILE}"
  echo "    updated ${LOCAL_PASS_FILE} (gitignored)"
fi

echo
echo "==> Done. Log in to Grafana with:"
echo "    user : ${ADMIN_USER}"
echo "    url  : http://localhost:3001   (after: kubectl -n ${NAMESPACE} port-forward svc/grafana 3001:80)"
echo
echo "    (Dev cluster only — this script only talks to the cluster your"
echo "     kubectl points at. For the production server, copy this file there"
echo "     and run it on the server itself.)"
