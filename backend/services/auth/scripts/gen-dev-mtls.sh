#!/usr/bin/env bash
# Generates a local dev CA + server/client certificates for the internal mTLS
# listener (backend/services/auth, INTERNAL_TLS_CERT/KEY/CA).
#
# The client cert (client.crt/client.key) is shared with other services
# (room / chat-command) that write audit events via POST /internal/audit.
#
# Usage: from backend/services/auth:  scripts/gen-dev-mtls.sh
set -euo pipefail
cd "$(dirname "$0")/.."

mkdir -p .mtls
cd .mtls

DAYS=825 # ~ Chrome's max cert lifetime

echo "==> generating CA"
openssl genrsa -out ca.key 2048 2>/dev/null
openssl req -x509 -new -nodes -key ca.key -sha256 -days 3650 \
  -subj "/CN=chinwag-dev-ca" -out ca.crt

echo "==> generating internal server cert (localhost / 127.0.0.1)"
openssl genrsa -out server.key 2048 2>/dev/null
openssl req -new -key server.key -subj "/CN=auth-internal" -out server.csr
cat > server.ext <<'EOF'
subjectAltName=DNS:localhost,IP:127.0.0.1
extendedKeyUsage=serverAuth
EOF
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
  -days ${DAYS} -sha256 -extfile server.ext -out server.crt

echo "==> generating client cert for other services (room/chat)"
openssl genrsa -out client.key 2048 2>/dev/null
openssl req -new -key client.key -subj "/CN=chinwag-service" -out client.csr
cat > client.ext <<'EOF'
extendedKeyUsage=clientAuth
EOF
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
  -days ${DAYS} -sha256 -extfile client.ext -out client.crt

rm -f server.csr server.ext client.csr client.ext ca.srl

echo "==> done. Files written to .mtls/"
echo "   server.crt / server.key  -> auth internal listener"
echo "   client.crt / client.key  -> other services (INTERNAL_CLIENT_CERT/KEY)"
echo "   ca.crt                   -> shared trust anchor"
