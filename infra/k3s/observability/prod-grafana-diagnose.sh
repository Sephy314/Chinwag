#!/usr/bin/env bash
# Run ON the production k3s node (from infra/k3s/observability).
# Diagnoses why the Grafana Deployment isn't Available after obs deploy.
set -u
export PATH="$HOME/.local/bin:$PATH"
K="kubectl"

echo "===== 1) grafana deployment ====="
${K} -n monitoring get deploy grafana -o wide 2>&1

echo "===== 2) grafana pods (incl. terminating) ====="
${K} -n monitoring get pods -o wide 2>&1 | grep -i grafana

echo "===== 3) grafana PVC ====="
${K} -n monitoring get pvc grafana -o wide 2>&1

echo "===== 4) pod conditions / status / describe / logs ====="
for p in $(${K} -n monitoring get pods --no-headers -o custom-columns=NAME:.metadata.name 2>/dev/null | grep -i grafana); do
  echo "--- pod ${p} ---"
  ${K} -n monitoring get pod "${p}" -o jsonpath='Ready={.status.conditions[?(@.type=="Ready")].status} phase={.status.phase}{"\n"}' 2>&1
  ${K} -n monitoring get pod "${p}" -o jsonpath='{range .status.containerStatuses[*]}{.name}: ready={.ready} restarts={.restartCount} state={.state}{"\n"}{end}' 2>&1
  echo "-- describe (tail) --"
  ${K} -n monitoring describe pod "${p}" 2>&1 | tail -25
  echo "-- logs (tail) --"
  ${K} -n monitoring logs "${p}" --tail=30 2>&1
done

echo "===== 5) events (grafana-related) ====="
${K} -n monitoring get events --sort-by=.lastTimestamp 2>&1 | grep -i grafana | tail -20
