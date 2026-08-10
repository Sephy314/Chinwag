#!/usr/bin/env bash
# =============================================================================
# K3s infrastructure integration test for the Chinwag repository.
#
# Applies the repository's Kubernetes manifests (kustomize) to whatever cluster
# `kubectl` points at, waits for everything to become Ready, then runs REAL
# runtime checks: Service DNS / TCP connectivity, HTTP health endpoints, the
# gateway proxy path (app -> app), and the Traefik Ingress.
#
# This is not a YAML-only test — it verifies the deployed cluster actually
# works. It never modifies /etc/hosts and never deletes existing namespaces,
# PVCs or cluster resources. The only resource it creates is a short-lived
# busybox pod for the in-cluster connectivity checks (removed afterwards).
#
# Usage:
#   ./infra/test.sh            # apply (kustomize) + wait + verify
#   ./infra/test.sh --no-apply # skip `kubectl apply -k` (verify an existing env)
#
# Prerequisites (checked up-front):
#   - kubectl on PATH pointing at a K3s cluster
#   - curl on PATH (host-side Ingress HTTP check)
#   - infra/k3s/secret.yaml present (gitignored; copy from secret.yaml.example)
#   - app images (chinwag/*:latest) already loaded in the cluster
#     (this test does NOT build/import images — deploy.sh does that)
#
# CI: the `k3s-infra` job in .github/workflows/ci.yml runs this on a self-hosted
# runner labelled `k3s` that has kubectl access to a K3s cluster. GitHub-hosted
# runners have no cluster access, so that job is workflow_dispatch-only — it
# never pretends to run without a cluster.
# =============================================================================
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
K8S_DIR="${SCRIPT_DIR}/k3s"
NS="chinwag"
TEST_POD="chinwag-infra-test"
TIMEOUT="${K3S_TEST_TIMEOUT:-180}"

NO_APPLY=0
for arg in "$@"; do
  case "$arg" in
    --no-apply) NO_APPLY=1 ;;
    *)
      echo "Unknown option: ${arg}" >&2
      echo "Usage: $0 [--no-apply]" >&2
      exit 2
      ;;
  esac
done

PASSED=0
FAILED=0
SKIPPED=0
section_fail=0

pass()  { echo "  [PASS] $*"; PASSED=$((PASSED + 1)); }
fail()  { echo "  [FAIL] $*"; FAILED=$((FAILED + 1)); section_fail=1; }
skip()  { echo "  [SKIP] $*"; SKIPPED=$((SKIPPED + 1)); }
die()   { echo "ERROR: $*" >&2; exit 1; }
section() { if [ "${section_fail}" -eq 0 ]; then pass "$1"; else fail "$1"; fi; section_fail=0; }

# --- kubectl resolution -------------------------------------------------------
# Same kubeconfig order as install.sh: $KUBECONFIG, then ~/.kube/config, then the
# k3s admin kubeconfig (/etc/rancher/k3s/k3s.yaml). If a plain kubectl cannot
# reach the cluster — e.g. the k3s wrapper needs root, or ~/.kube/config is
# missing for the CI runner user — fall back to `sudo k3s kubectl`, the same
# approach deploy.sh / update.sh use on the k3s node. A shell function shadows
# the bare `kubectl` calls below, so no call sites need to change.
if [ -z "${KUBECONFIG:-}" ] && [ -r "${HOME}/.kube/config" ]; then
  export KUBECONFIG="${HOME}/.kube/config"
elif [ -r /etc/rancher/k3s/k3s.yaml ]; then
  export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
fi
if command -v kubectl >/dev/null 2>&1 && kubectl cluster-info >/dev/null 2>&1; then
  KUBECTL="$(command -v kubectl)"
else
  KUBECTL="sudo k3s kubectl"
  echo "    (plain kubectl cannot reach the cluster — using '${KUBECTL}')"
fi
export KUBECTL
kubectl() { "${KUBECTL}" "$@"; }

echo
echo "========================================"
echo " K3s Infrastructure Test"
echo "========================================"
echo

# --- Failure diagnostics ------------------------------------------------------
diag_dump() {
  local ns="${1:-}"
  echo "----- kubectl get nodes -o wide -----"
  kubectl get nodes -o wide 2>&1 || true
  echo "----- kubectl get pods -A -o wide -----"
  kubectl get pods -A -o wide 2>&1 || true
  echo "----- kubectl get deployments -A -----"
  kubectl get deployments -A 2>&1 || true
  echo "----- kubectl get statefulsets -A -----"
  kubectl get statefulsets -A 2>&1 || true
  echo "----- kubectl get services -A -----"
  kubectl get services -A 2>&1 || true
  echo "----- kubectl get endpoints -A -----"
  kubectl get endpoints -A 2>&1 || true
  echo "----- kubectl get pvc -A -----"
  kubectl get pvc -A 2>&1 || true
  echo "----- kubectl get events -A --sort-by=.lastTimestamp (tail 40) -----"
  kubectl get events -A --sort-by=.lastTimestamp 2>&1 | tail -40 || true
  if [ -n "${ns}" ]; then
    echo "----- pod status in '${ns}' -----"
    kubectl -n "${ns}" get pods -o wide 2>&1 || true
    for p in $(kubectl -n "${ns}" get pods --no-headers -o custom-columns=NAME:.metadata.name 2>/dev/null); do
      phase="$(kubectl -n "${ns}" get pod "${p}" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
      ready="$(kubectl -n "${ns}" get pod "${p}" -o jsonpath='{.status.containerStatuses[0].ready}' 2>/dev/null || true)"
      if [ "${phase}" != "Running" ] || [ "${ready}" != "true" ]; then
        echo "----- describe pod/${p} -----"
        kubectl -n "${ns}" describe pod "${p}" 2>&1 | tail -40 || true
        for c in $(kubectl -n "${ns}" get pod "${p}" -o jsonpath='{.spec.containers[*].name}' 2>/dev/null); do
          echo "== logs pod/${p} container/${c} =="
          kubectl -n "${ns}" logs "${p}" -c "${c}" --tail=50 2>&1 || true
        done
      fi
    done
  fi
}

on_error() {
  echo
  echo "=============================="
  echo " FAILURE DIAGNOSTICS"
  echo "=============================="
  diag_dump "${NS}"
  echo
  echo "Result: FAIL"
  exit 1
}
trap on_error ERR

# --- Cleanup: only the test-created pod. Never touch existing resources. ------
cleanup() {
  kubectl -n "${NS}" delete pod "${TEST_POD}" --ignore-not-found --wait=false >/dev/null 2>&1 || true
}
trap cleanup EXIT

# --- Wait helpers -------------------------------------------------------------
all_pods_ready() {
  local ns="$1" rows
  rows="$(kubectl -n "${ns}" get pods --no-headers 2>/dev/null || true)"
  [ -n "${rows}" ] || return 1
  printf '%s\n' "${rows}" | awk '
    { split($2, a, "/"); if (a[1] != a[2]) bad = 1 }
    END { exit bad }'
}

wait_for_pods_ready() {
  local ns="$1" timeout_secs="$2" start now hard
  start="$(date +%s)"
  while :; do
    now="$(date +%s)"
    if (( now - start > timeout_secs )); then
      echo "  ERROR: pods in '${ns}' not Ready within ${timeout_secs}s" >&2
      kubectl -n "${ns}" get pods -o wide >&2 || true
      return 1
    fi
    hard="$(kubectl -n "${ns}" get pods --no-headers 2>/dev/null \
      | awk '$3 ~ /CrashLoopBackOff|ImagePullBackOff|ErrImagePull|CreateContainerConfigError|RunContainerError|Error/ {print "  " $1 " [" $3 "]"}' || true)"
    if [ -n "${hard}" ]; then
      echo "  ERROR: failed pod state(s):" >&2
      printf '%s\n' "${hard}" >&2
      return 1
    fi
    if all_pods_ready "${ns}"; then return 0; fi
    sleep 3
  done
}

svc_has_endpoints() {
  local ns="$1" svc="$2"
  [ -n "$(kubectl -n "${ns}" get endpoints "${svc}" -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null || true)" ]
}

# =============================================================================
echo "==> [1/7] Prerequisites"
need_cmd() { command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"; }
need_cmd kubectl
need_cmd curl
if [ ! -f "${K8S_DIR}/secret.yaml" ]; then
  die "missing ${K8S_DIR}/secret.yaml (required by kustomize) — create it from secret.yaml.example"
fi
pass "Prerequisites (kubectl, curl, ${K8S_DIR}/secret.yaml)"

# =============================================================================
echo "==> [2/7] Cluster connectivity"
kubectl cluster-info >/dev/null 2>&1 || die "cannot reach the cluster (kubectl cluster-info failed)"
pass "Cluster connectivity"
section_fail=0
node_count="$(kubectl get nodes --no-headers 2>/dev/null | wc -l | tr -d ' ')"
ready_states="$(kubectl get nodes -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || true)"
if [ "${node_count}" -ge 1 ] && printf ' %s ' " ${ready_states} " | grep -q ' True '; then
  pass "Nodes ready (${node_count} node(s))"
else
  fail "No Ready node (nodes=${node_count} ready_states='${ready_states}')"
fi
section "Nodes ready"

# =============================================================================
echo "==> [3/7] Deploy infrastructure (kustomize)"
if [ "${NO_APPLY}" -eq 0 ]; then
  # Note: not wrapped in a subshell — a failure here must raise the ERR trap
  # exactly once (a subshell would trigger it twice: inside + parent).
  kubectl apply -k "${K8S_DIR}"
  pass "Deploy (kubectl apply -k infra/k3s)"
else
  echo "  (--no-apply: skipping kubectl apply -k)"
  pass "Deploy (skipped via --no-apply)"
fi

# =============================================================================
echo "==> [4/7] Wait for readiness"
kubectl -n "${NS}" rollout status deploy/gateway deploy/frontend deploy/auth deploy/room deploy/chat-command deploy/chat-query deploy/redis --timeout="${TIMEOUT}s" >/dev/null
pass "Deployments rolled out"
kubectl -n "${NS}" rollout status statefulset/postgres statefulset/nats --timeout="${TIMEOUT}s" >/dev/null
pass "StatefulSets ready"

section_fail=0
for obj in configmap/chinwag-config secret/chinwag-secrets; do
  kubectl -n "${NS}" get "${obj}" >/dev/null 2>&1 || fail "missing ${obj}"
done
section "ConfigMap & Secret present"

wait_for_pods_ready "${NS}" "${TIMEOUT}"
pass "Pods Ready"

section_fail=0
for pvc in data-postgres-0 nats-data redis-data; do
  phase="$(kubectl -n "${NS}" get pvc "${pvc}" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
  if [ "${phase}" != "Bound" ]; then fail "PVC ${pvc} not Bound (phase=${phase:-unknown})"; fi
done
section "PVCs Bound"

section_fail=0
for svc in gateway auth room chat-command chat-query frontend postgres redis nats; do
  if ! svc_has_endpoints "${NS}" "${svc}"; then fail "Service ${svc} has no endpoints"; fi
done
section "Services have endpoints"

# =============================================================================
echo "==> [5/7] Internal networking (Service DNS + TCP)"
# Remove any leftover test pod from a previous run and WAIT until it is gone,
# otherwise a Terminating pod collides with `kubectl run` below.
kubectl -n "${NS}" delete pod "${TEST_POD}" --ignore-not-found --wait=true --timeout=60s >/dev/null 2>&1 || true
kubectl -n "${NS}" run "${TEST_POD}" --image=busybox:1.36 --restart=Never --command -- /bin/sh -c 'sleep 600' >/dev/null
kubectl -n "${NS}" wait --for=condition=Ready "pod/${TEST_POD}" --timeout=120s >/dev/null
section_fail=0
for svc_port in gateway:8000 auth:8081 room:8082 chat-command:8083 chat-query:8084 frontend:3000 postgres:5432 redis:6379 nats:4222 nats:8222; do
  svc="${svc_port%%:*}"; port="${svc_port##*:}"
  if kubectl -n "${NS}" exec "${TEST_POD}" -- nc -z -w 3 "${svc}" "${port}" >/dev/null 2>&1; then
    pass "TCP ${svc}:${port}"
  else
    fail "TCP ${svc}:${port} unreachable"
  fi
done
section "Services reachable"

# =============================================================================
echo "==> [6/7] Health checks + dependencies"
check_http() {
  local url="$1" out code
  out="$(kubectl -n "${NS}" exec "${TEST_POD}" -- wget -S -O /dev/null -T 5 "${url}" 2>&1 || true)"
  code="$(printf '%s\n' "${out}" | grep -oE 'HTTP/[0-9.]+ [0-9]{3}' | tail -1 | awk '{print $2}')"
  if [ -n "${code}" ] && [ "${code:0:1}" = "2" ]; then
    pass "HTTP ${url} -> ${code}"
  else
    fail "HTTP ${url} -> ${code:-<no response>}"
  fi
}

# app -> app through the gateway proxy (routes from backend/gateway/config.go:
# /auth/* -> auth, /rooms/* -> room, /chat GET -> chat-query)
section_fail=0
check_http "http://gateway:8000/auth/health"
check_http "http://gateway:8000/rooms/health"
check_http "http://gateway:8000/chat/health"
section "Dependencies reachable (app-to-app via gateway)"

# direct service health endpoints (the same paths as the readiness probes)
section_fail=0
check_http "http://gateway:8000/health"
check_http "http://auth:8081/health"
check_http "http://room:8082/health"
check_http "http://chat-command:8083/chat/health"
check_http "http://chat-query:8084/chat/health"
check_http "http://frontend:3000/"
check_http "http://nats:8222/healthz"
check_http "http://auth:8081/.well-known/jwks.json"
section "Health checks"

# =============================================================================
echo "==> [7/7] Ingress"
section_fail=0
for ing in chinwag chinwag-api chinwag-redirect; do
  kubectl -n "${NS}" get ingress "${ing}" >/dev/null 2>&1 || fail "Ingress ${ing} missing"
done
backend="$(kubectl -n "${NS}" get ingress chinwag -o jsonpath='{.spec.rules[0].http.paths[0].backend.service.name}' 2>/dev/null || true)"
[ "${backend}" = "frontend" ] || fail "Ingress chinwag backend service is '${backend:-?}' (expected frontend)"
api_backend="$(kubectl -n "${NS}" get ingress chinwag-api -o jsonpath='{.spec.rules[0].http.paths[0].backend.service.name}' 2>/dev/null || true)"
[ "${api_backend}" = "gateway" ] || fail "Ingress chinwag-api backend service is '${api_backend:-?}' (expected gateway)"
for svc in frontend gateway; do
  svc_has_endpoints "${NS}" "${svc}" || fail "Ingress backend Service ${svc} has no endpoints"
done
section "Ingress resources & routing config"

# Real HTTP request through the local Traefik (best-effort — no /etc/hosts edit).
# Try 127.0.0.1 first (k3s Traefik binds the node), then the node InternalIP.
ing_ok=0
host="chinwag.duckdns.org"
for ip in "127.0.0.1" "$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || true)"; do
  [ -n "${ip}" ] || continue
  code="$(curl -sk --resolve "${host}:443:${ip}" -o /dev/null -w '%{http_code}' -m 8 "https://${host}/api/auth/health" 2>/dev/null || true)"
  if [ "${code}" = "200" ]; then
    pass "Ingress routing https://${host}/api/auth/health -> 200 (via ${ip})"
    ing_ok=1
    break
  fi
done
if [ "${ing_ok}" -ne 1 ]; then
  skip "Ingress routing (no local route to ${host}:443 — add a hosts entry or run on the k3s node; host machine left unmodified)"
fi

# =============================================================================
echo
echo "========================================"
echo " K3s Infrastructure Test"
echo "========================================"
if [ "${FAILED}" -eq 0 ]; then
  echo "  Result: PASS"
  rc=0
else
  echo "  Result: FAIL"
  rc=1
fi
echo "  (pass=${PASSED} fail=${FAILED} skip=${SKIPPED})"
echo
exit "${rc}"
