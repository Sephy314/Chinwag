#!/usr/bin/env bash
# =============================================================================
# Self-signed TLS for the Chinwag ingress on k3s.
#
# Generates a local CA + server certificate for `chinwag.local` and stores it
# as the Kubernetes TLS secret `chinwag-tls` in the `chinwag` namespace. The
# Ingress (ingress.yaml) references this secret; without it Traefik cannot
# serve HTTPS on :443.
#
# `chinwag.local` is a local-only hostname, so Let's Encrypt is not an option —
# a self-signed CA is the practical choice. Install the CA cert (.tls/ca.crt)
# into your OS/browser trust store to silence certificate warnings.
#
# Usage:
#   ./tls.sh              # create cert + secret only if missing (idempotent)
#   ./tls.sh --force      # regenerate CA + cert + secret (rotates trust)
#   ./tls.sh --ip 192.168.1.10   # also add a node IP to the cert SANs
#
# All generated files live in infra/k3s/.tls/ (gitignored).
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

FORCE=0
EXTRA_IPS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --force) FORCE=1; shift ;;
    --ip) EXTRA_IPS+=("$2"); shift 2 ;;
    *)
      echo "Unknown option: $1" >&2
      echo "Usage: $0 [--force] [--ip <node-ip>]" >&2
      exit 1
      ;;
  esac
done

HOST="${CHINWAG_HOST:-chinwag.local}"
DIR=".tls"
CA_KEY="${DIR}/ca.key"
CA_CRT="${DIR}/ca.crt"
SRV_KEY="${DIR}/server.key"
SRV_CSR="${DIR}/server.csr"
SRV_CRT="${DIR}/server.crt"

# --- kubectl helper: prefer plain kubectl, fall back to sudo k3s kubectl ----
if command -v kubectl >/dev/null 2>&1 && kubectl cluster-info >/dev/null 2>&1; then
  KUBECTL="kubectl"
else
  KUBECTL="sudo k3s kubectl"
fi
echo "Using kubectl: ${KUBECTL}"

if ! command -v openssl >/dev/null 2>&1; then
  echo "ERROR: openssl is required." >&2
  exit 1
fi

# --- Certificate SANs --------------------------------------------------------
SAN="DNS:${HOST},DNS:localhost,IP:127.0.0.1"
if ((${#EXTRA_IPS[@]})); then
  for ip in "${EXTRA_IPS[@]}"; do SAN+=",IP:${ip}"; done
fi

mkdir -p "${DIR}"

# --- CA (only once, unless --force) ------------------------------------------
if [[ ! -f "${CA_CRT}" || "${FORCE}" -eq 1 ]]; then
  echo "==> Generating CA -> ${CA_CRT}"
  openssl req -x509 -newkey rsa:2048 -sha256 -days 3650 -nodes \
    -keyout "${CA_KEY}" -out "${CA_CRT}" \
    -subj "/CN=Chinwag Local CA" \
    -addext "basicConstraints=critical,CA:TRUE" \
    -addext "keyUsage=critical,keyCertSign,cRLSign"
else
  echo "==> Reusing existing CA (${CA_CRT}); use --force to regenerate"
fi

# --- Server key + CSR + signed cert -------------------------------------------
echo "==> Generating server key + CSR (${HOST})"
openssl req -newkey rsa:2048 -sha256 -nodes \
  -keyout "${SRV_KEY}" -out "${SRV_CSR}" \
  -subj "/CN=${HOST}"

cat > "${DIR}/san.cnf" <<EOF
[req]
distinguished_name = dn
[dn]
[ext]
subjectAltName = ${SAN}
basicConstraints = critical,CA:FALSE
keyUsage = critical,digitalSignature,keyEncipherment
extendedKeyUsage = serverAuth
EOF

echo "==> Signing server certificate"
openssl x509 -req -in "${SRV_CSR}" -CA "${CA_CRT}" -CAkey "${CA_KEY}" \
  -CAcreateserial -out "${SRV_CRT}" -days 365 -sha256 \
  -extfile "${DIR}/san.cnf" -extensions ext

echo ""
echo "==> Certificate SANs:"
openssl x509 -in "${SRV_CRT}" -noout -text | grep -A1 "Subject Alternative Name" || true

# --- Namespace + TLS secret ---------------------------------------------------
echo "==> Ensuring chinwag namespace exists"
${KUBECTL} create namespace chinwag --dry-run=client -o yaml | ${KUBECTL} apply -f - >/dev/null

if ${KUBECTL} -n chinwag get secret chinwag-tls >/dev/null 2>&1 && [[ "${FORCE}" -eq 0 ]]; then
  echo "==> Secret chinwag-tls already exists (use --force to replace)"
else
  echo "==> Creating/updating secret chinwag-tls"
  ${KUBECTL} -n chinwag create secret tls chinwag-tls \
    --cert="${SRV_CRT}" --key="${SRV_KEY}" \
    --dry-run=client -o yaml | ${KUBECTL} apply -f - >/dev/null
fi

echo ""
echo "Done. To avoid browser certificate warnings, trust the local CA:"
echo "  sudo cp ${PWD}/${DIR}/ca.crt /usr/local/share/ca-certificates/chinwag-ca.crt"
echo "  sudo update-ca-certificates"
echo ""
echo "Browse to: https://${HOST}"
