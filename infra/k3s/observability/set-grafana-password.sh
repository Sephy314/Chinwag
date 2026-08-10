#!/usr/bin/env bash
# =============================================================================
# Set a fixed Grafana admin password on any cluster running the Chinwag
# observability stack (works on the dev cluster and the production server).
#
# Why the extra steps? The grafana-admin Secret is only read by Grafana at
# FIRST startup, so changing an already-provisioned admin needs to happen inside
# Grafana itself. `grafana-cli admin reset-admin-password` only resets the built-in
# login "admin" (it takes no username) — for a custom login the script creates /
# updates that user via the Grafana HTTP Admin API and grants org Admin. The
# Secret is updated afterwards so a future Helm reinstall keeps the same values.
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

# --- reset built-in admin login via grafana-cli -----------------------------
# grafana-cli only resets the built-in login "admin" (no username argument), so
# this guarantees admin access with a known password for the API calls below.
# It writes Grafana's DB directly and takes effect immediately.
echo "==> Resetting built-in 'admin' password in Grafana (${NAMESPACE}/${DEPLOYMENT})"
"${KUBECTL}" -n "${NAMESPACE}" exec "deploy/${DEPLOYMENT}" -- \
  grafana-cli admin reset-admin-password "${NEW_PASS}"

# --- ensure the target user exists (for a custom login) ----------------------
# grafana-cli cannot create/rename a user, so a custom login is provisioned via
# the Grafana HTTP Admin API (curl is available in the Grafana image). Validated
# against Grafana 12.3:
#   POST  /api/admin/users                  create (error if already exists)
#   GET   /api/users/search?query=<login>   resolve the user id
#   PUT   /api/admin/users/{id}/password    idempotent password set
#   PATCH /api/orgs/1/users/{id}            grant org Admin role (idempotent)
if [ "${ADMIN_USER}" != "admin" ]; then
  echo "==> Ensuring user '${ADMIN_USER}' exists as an org admin"
  API() {
    "${KUBECTL}" -n "${NAMESPACE}" exec "deploy/${DEPLOYMENT}" -- \
      curl -s -u "admin:${NEW_PASS}" "$@"
  }

  # Create the user; ignore the error when the login already exists.
  API -X POST -H 'Content-Type: application/json' \
    "http://localhost:3000/api/admin/users" \
    -d "{\"login\":\"${ADMIN_USER}\",\"name\":\"${ADMIN_USER}\",\"password\":\"${NEW_PASS}\",\"orgId\":1}" \
    >/dev/null || true

  USER_ID="$(API "http://localhost:3000/api/users/search?perpage=100&query=${ADMIN_USER}" \
    | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2 || true)"
  if [ -z "${USER_ID}" ]; then
    echo "ERROR: could not resolve a user id for login '${ADMIN_USER}'" >&2
    exit 1
  fi

  # Set the password (covers the 'already exists' case).
  API -X PUT -H 'Content-Type: application/json' \
    "http://localhost:3000/api/admin/users/${USER_ID}/password" \
    -d "{\"password\":\"${NEW_PASS}\"}" >/dev/null || true

  # Grant the org Admin role (idempotent).
  API -X PATCH -H 'Content-Type: application/json' \
    "http://localhost:3000/api/orgs/1/users/${USER_ID}" \
    -d '{"role":"Admin"}' >/dev/null || true

  echo "    user '${ADMIN_USER}' (id=${USER_ID}) is now an org admin"
  echo "    note: the built-in 'admin' login still exists with the same password."
  echo "          To disable it for security, run:"
  echo "          kubectl -n ${NAMESPACE} exec deploy/${DEPLOYMENT} -- \\"
  echo "            curl -s -u '${ADMIN_USER}:${NEW_PASS}' -X POST http://localhost:3000/api/admin/users/1/disable"
fi

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
