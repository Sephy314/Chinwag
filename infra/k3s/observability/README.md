# Chinwag Observability (Loki + Grafana Alloy + Grafana)

Container log collection / search stack for the Chinwag k3s cluster.

```
Chinwag Pods
    ↓
Grafana Alloy (DaemonSet, collects container stdout/stderr via the Kubernetes API)
    ↓
Loki (monolithic / filesystem storage, namespace: monitoring)
    ↓
Grafana (Loki datasource auto-provisioned, namespace: monitoring)
```

- Log collector: **Grafana Alloy** (no Promtail)
- Loki / Alloy / Grafana are all installed with **Helm** (in the `monitoring` namespace)
- The existing `infra/k3s/*.yaml` (kustomize / GitOps) structure is **left untouched**
- Built for a small **single-node k3s** setup. No HA / replication.

---

## Directory layout

```
infra/k3s/observability/
├── README.md            # this document
├── loki-values.yaml     # Loki Helm values (monolithic / filesystem / 1 replica)
├── alloy-values.yaml    # Alloy Helm values (DaemonSet + River config)
├── grafana-values.yaml  # Grafana Helm values (Loki datasource provisioning)
└── install.sh           # Helm install script (creates namespace + secret)
```

## Prerequisites

- `kubectl` and `helm` available — **`install.sh` bootstraps them automatically**
  if they are missing: it installs `helm` (and a standalone `kubectl` if the one
  on PATH can't reach the cluster) into `/usr/local/bin` (root/sudo) or
  `~/.local/bin` (normal user).
- A working kubeconfig for the cluster.
  - If k3s runs as root and the node user cannot read the kubeconfig (permission
    denied), run this once:

  ```bash
  sudo mkdir -p ~/.kube \
    && sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config \
    && sudo chown -R $(whoami):$(whoami) ~/.kube \
    && sudo chmod 600 ~/.kube/config
  ```

  > On k3s nodes, `/usr/local/bin/kubectl` is often a symlink to the `k3s` wrapper
  > that reads `/etc/rancher/k3s/k3s.yaml`. Use a standalone `kubectl`/`helm` on
  > `PATH` (e.g. `~/.local/bin`) or set `KUBECONFIG` so they pick up `~/.kube/config`.

- k3s default storage class `local-path` (already used by postgres/nats/redis)

## Installation

```bash
cd infra/k3s/observability
./install.sh
```

What `install.sh` does (equivalent to doing it manually):

```bash
kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -

helm repo add grafana https://grafana.github.io/helm-charts
helm repo update grafana

# 1) Loki — monolithic, filesystem storage, single replica
helm upgrade --install loki grafana/loki -n monitoring \
  --version 7.2.0 -f loki-values.yaml --wait

# 2) Alloy — DaemonSet log collector
helm upgrade --install alloy grafana/alloy -n monitoring \
  --version 1.11.1 -f alloy-values.yaml --wait

# 3) Grafana admin Secret (random password, never committed to Git)
kubectl -n monitoring create secret generic grafana-admin \
  --from-literal=admin-user=admin \
  --from-literal=admin-password='<strong-password>'

# 4) Grafana — Loki datasource auto-provisioned
helm upgrade --install grafana grafana/grafana -n monitoring \
  --version 10.5.15 -f grafana-values.yaml --wait
```

> Forgot the Grafana password after install?
> `kubectl -n monitoring get secret grafana-admin -o jsonpath='{.data.admin-password}' | base64 -d`

### Change the Grafana admin password

The `grafana-admin` Secret is only read at **first** startup, so editing the
Secret alone does not change an already-provisioned admin password. Use the
bundled script — it resets the password inside Grafana (`grafana-cli`) **and**
updates the Secret so a future reinstall keeps the same value:

```bash
./set-grafana-password.sh                          # prompts for user + password
GRAFANA_ADMIN_USER='admin' GRAFANA_PASSWORD='...' ./set-grafana-password.sh
                                                    # or via env vars
sudo ./set-grafana-password.sh                   # works too (kubeconfig fallback)
```

The password is never passed on argv or echoed. On the **production server**,
copy the script over and run it there (it only talks to whatever cluster the
local `kubectl` points at):

```bash
scp infra/k3s/observability/set-grafana-password.sh sephy314@server:/tmp/
ssh sephy314@server 'cd /tmp && ./set-grafana-password.sh'
```

---

## Component details

### Loki (`loki-values.yaml`)

| Item | Value |
|---|---|
| Helm chart | `grafana/loki` 7.2.0 (Loki 3.6.11) |
| deploymentMode | `SingleBinary` (the Loki Helm chart's name for **monolithic single-instance** mode) |
| replicas | 1 (`singleBinary.replicas: 1`) |
| replication_factor | 1 |
| Schema | **TSDB** (`schema: v13`, `store: tsdb`, `object_store: filesystem`) |
| Storage | **filesystem** (`/var/loki/chunks`) |
| Persistent volume | `local-path` PVC 10Gi → logs survive pod restarts |
| HTTP API port | **3100** (`loki.server.http_listen_port`) |
| Auth | `auth_enabled: false` (cluster-internal only) |
| Service | `loki` (ClusterIP) → `http://loki.monitoring.svc.cluster.local:3100` |

Being a monolithic single instance, it does not require object storage the way
`SimpleScalable`/`Distributed` do. Filesystem is enough; see
"[Switch to object storage](#switch-to-object-storage-optional)" for moving to S3 later.

> Note: when `deploymentMode: SingleBinary`, the chart requires the
> SimpleScalable/Distributed targets (`write`/`read`/`backend`/...) to have
> `replicas: 0` (already set in `loki-values.yaml`) or the chart validation fails.

### Grafana Alloy (`alloy-values.yaml`)

- **DaemonSet** (`controller.type: daemonset`) — 1 pod on the single k3s node
- `loki.source.kubernetes` collects each container's stdout/stderr via the
  **Kubernetes API** → no host filesystem log files are created (no privileged/hostPath)
- Collects all namespaces (including `chinwag`)
- Labels added:
  - `namespace` ← Pod namespace
  - `pod` ← Pod name
  - `container` ← container name
  - `node` ← node name
  - `app` / `service` ← the Pod's `app` label (`__meta_kubernetes_pod_label_app`)
    - Chinwag services: `gateway`, `auth`, `room`, `chat-command`, `chat-query`, `frontend`
- Alloy's own log level: `logging { level = "warn" }` in the River config (not verbose)
- Loki endpoint: `http://loki.monitoring.svc.cluster.local:3100/loki/api/v1/push` (Service DNS, no auth)
- Anonymous usage reporting (`enableReporting`) is off

> The River config lives in `alloy.configMap.content` inside `alloy-values.yaml`.
> The chart's configReloader applies changes automatically.

### Grafana (`grafana-values.yaml`)

- Helm chart `grafana/grafana` 10.5.15 (Grafana 12.3.1)
- **Loki datasource auto-provisioned** (`datasources.datasources.yaml`)
  - url: `http://loki.monitoring.svc.cluster.local:3100`
  - `isDefault: true` → Loki is the default datasource for Explore
- Admin credentials: `admin.existingSecret: grafana-admin` (never stored in Git)
- Service: ClusterIP (not exposed externally); access via `port-forward`
- Anonymous access disabled, default org role: Viewer

---

## Verification

### 1. Helm release status

```bash
helm list -n monitoring
# NAME     NAMESPACE   REVISION  UPDATED      STATUS   CHART              APP VERSION
# alloy    monitoring  1         ...          deployed alloy-1.11.1      v1.18.1
# grafana  monitoring  1         ...          deployed grafana-10.5.15   12.3.1
# loki     monitoring  1         ...          deployed loki-7.2.0        3.6.11
```

### 2–4. Pod status

```bash
kubectl get pods -n monitoring
# NAME                     READY  STATUS    RESTARTS  AGE
# alloy-XXXXX-XXXXX        2/2    Running   0         2m   (DaemonSet + config reloader)
# grafana-XXXXX-XXXXX      1/1    Running   0         2m
# loki-0                   1/1    Running   0         3m   (StatefulSet, single replica)
```

### 5. Loki Service exists

```bash
kubectl get svc -n monitoring
# NAME            TYPE        CLUSTER-IP   PORT(S)          AGE
# alloy           ClusterIP   10.43.x.x   12345/TCP        2m
# grafana         ClusterIP   10.43.x.x   80/TCP           2m
# loki            ClusterIP   10.43.x.x   3100/TCP,9095/TCP 3m

# Call the Loki API from inside the cluster (the gateway pod has wget)
kubectl -n chinwag exec deploy/gateway -- wget -qO- \
  'http://loki.monitoring.svc.cluster.local:3100/loki/api/v1/labels'
```

### 6. Grafana → Loki datasource connection

```bash
# Port-forward, then open http://localhost:3001 in a browser
# (local 3000 is the frontend's port — next dev / k8s frontend:3000 — so Grafana uses 3001)
kubectl -n monitoring port-forward svc/grafana 3001:80

# Login (admin / password from the Secret above)
# Connections → Data sources → Loki → Save & test  → "Success"
```

Check the datasource over the Grafana API:

```bash
ADMIN_PASS=$(kubectl -n monitoring get secret grafana-admin -o jsonpath='{.data.admin-password}' | base64 -d)
curl -s -u "admin:${ADMIN_PASS}" \
  'http://localhost:3001/api/datasources' | head
# confirm an entry with type: loki, url: http://loki.monitoring.svc.cluster.local:3100
```

### 7. Alloy → Loki log delivery

```bash
# Alloy pod logs (only check errors/warnings)
kubectl -n monitoring logs ds/alloy -c alloy --tail=20

# Inspect the targets Alloy is tailing (Alloy UI)
kubectl -n monitoring port-forward svc/alloy 12345:12345
# browser http://localhost:12345 → Components → loki.source.kubernetes.pod_logs

# Confirm Loki actually received chinwag logs (Loki API range query)
#  ⚠️ Log queries ({...}) must use the range endpoint (/query_range), not instant /query.
kubectl -n chinwag exec deploy/gateway -- wget -qO- \
  'http://loki.monitoring.svc.cluster.local:3100/loki/api/v1/query_range?query=%7Bnamespace%3D%22chinwag%22%7D&limit=5'
# if the result array contains log lines (JSON), it works — includes namespace/pod/container/app/service labels
```

### 8. Final check (kubectl logs ↔ Grafana Explore show the same logs)

```bash
kubectl get pods -n chinwag
kubectl logs -n chinwag <pod>          # e.g. kubectl logs -n chinwag deploy/auth
```

And in Grafana Explore the same logs are visible with:

```
{namespace="chinwag"}
```

---

## LogQL examples (validated against the real labels / log format)

Chinwag services log with Go `log/slog`'s JSON handler. A real line looks like:

```json
{"time":"2026-08-09T10:00:00.000000000Z","level":"INFO","msg":"request","request_id":"...","client_ip":"...","method":"GET","path":"/health","status":200,"latency":"1.2ms"}
```

> ⚠️ `slog` writes `level` in **UPPERCASE**: `DEBUG` / `INFO` / `WARN` / `ERROR`
> (match `"ERROR"`, not lowercase `error`).

Available labels: `namespace`, `pod`, `container`, `node`, `app`, `service`, `job`, `instance`
(plus `detected_level` — lowercase, auto-detected by Loki 3.6 — and `service_name`).

| # | Purpose | LogQL |
|---|---|---|
| 1 | All Chinwag logs | `{namespace="chinwag"}` |
| 2 | A specific service | `{namespace="chinwag", service="auth"}` (or `app="auth"`) |
| 3 | ERROR logs (string filter) | `{namespace="chinwag"} |= "ERROR"` |
| 4 | A specific pod | `{namespace="chinwag", pod=~"auth-.*"}` |
| 5 | A specific time range | set the range with the Explore time picker (e.g. Last 15 minutes). For range-vector aggregations: `rate({namespace="chinwag"}[5m])` |
| 6 | JSON level filter | `{namespace="chinwag"} | json | level="ERROR"` |
| 6b | Level, case-insensitive | `{namespace="chinwag"} | json | level=~"(?i)error"` |
| 7 | Error count per service (last 5m) | `sum by (service) (count_over_time({namespace="chinwag"} | json | level="ERROR" [5m]))` |
| 8 | Error rate, last 5m | `sum by (service) (rate({namespace="chinwag"} | json | level="ERROR" [5m])) / sum by (service) (rate({namespace="chinwag"} [5m]))` |

More useful queries:

```logql
# Request logs for one service only
{namespace="chinwag", service="gateway"} | json | msg="request"

# 5xx status requests
{namespace="chinwag"} | json | status >= 500

# Filter by Loki's auto-detected level (lowercase label)
{namespace="chinwag", detected_level="error"}

# Distribution of ERROR logs per pod
sum by (pod) (count_over_time({namespace="chinwag"} | json | level="ERROR" [1h]))
```

> To inspect how labels were actually attached, use the Log browser in Grafana
> Explore or the API:
> `kubectl -n chinwag exec deploy/gateway -- wget -qO- 'http://loki.monitoring.svc.cluster.local:3100/loki/api/v1/label/service/values'`

---

## Security

- **Grafana admin password** lives only in the `monitoring/grafana-admin` Secret.
  It is never committed to Git (`install.sh` generates a random one; not in
  `secret.yaml` or the values files).
- **Avoid logging sensitive data**:
  - Make sure API tokens, JWTs, refresh tokens, DPoP proofs, etc. are masked or
    removed in application logging. Everything in Loki is readable by Grafana
    (admin). (No Chinwag code was changed here — the logs are already structured JSON.)
- **Minimize external exposure**: Loki/Alloy are ClusterIP-only, and Grafana is
  ClusterIP + `port-forward` by default. Nothing is exposed directly to the internet.
- Even though Loki has no auth, it is only reachable from inside the cluster network.

---

## Switch to object storage (optional)

This is a single-node filesystem install (`local-path` PVC). To move to
S3-compatible storage (MinIO, SeaweedFS, R2, ...) change only the `loki.storage`
block in `loki-values.yaml` (the values are intentionally separated for this):

```yaml
loki:
  storage:
    type: s3
    s3:
      endpoint: http://minio.monitoring.svc.cluster.local:9000
      region: us-east-1
      secret_access_key: <secret>
      access_key_id: <access>
      s3forcepathstyle: true
      insecure: true
    bucketNames:
      chunks: loki-chunks
      ruler: loki-ruler
  schemaConfig:
    configs:
      - from: "2024-04-01"
        store: tsdb
        object_store: s3
        schema: v13
        index:
          prefix: index_
          period: 24h
```

> Don't put secrets in the values file — inject them via `global.extraEnvFrom` +
> env references (`$(VAR)`) in the Loki config, or `--set-file`.

---

## Troubleshooting

```bash
# Loki won't start
kubectl -n monitoring logs statefulset/loki --tail=50
kubectl -n monitoring describe pod -l app.kubernetes.io/name=loki

# Alloy can't pick up targets (RBAC / API access)
kubectl -n monitoring logs ds/alloy -c alloy --tail=50
kubectl -n monitoring get clusterrolebinding -l app.kubernetes.io/name=alloy

# Grafana pod can't find the admin Secret
kubectl -n monitoring describe pod -l app.kubernetes.io/name=grafana

# No logs reaching Loki — check the Alloy → Loki endpoint
kubectl -n monitoring exec deploy/loki -- sh -c 'wget -qO- http://localhost:3100/loki/api/v1/labels' \
  || kubectl -n chinwag exec deploy/gateway -- wget -qO- \
     'http://loki.monitoring.svc.cluster.local:3100/loki/api/v1/labels'

# Redeploy the whole stack
helm upgrade --install loki grafana/loki -n monitoring --version 7.2.0 -f loki-values.yaml
helm upgrade --install alloy grafana/alloy -n monitoring --version 1.11.1 -f alloy-values.yaml
helm upgrade --install grafana grafana/grafana -n monitoring --version 10.5.15 -f grafana-values.yaml
```

## Teardown

```bash
helm uninstall grafana alloy loki -n monitoring
kubectl delete namespace monitoring
# Note: helm uninstall deletes Loki's PVC (singleBinary.persistence.whenDeleted: Delete).
# To keep the logs, back up the PVC first or set whenDeleted to Retain.
```

## Validated environment / versions (2026-08-09)

| Item | Version |
|---|---|
| k3s / Kubernetes | k3s server (containerd) |
| helm | v4.2.3 |
| Loki chart / app | 7.2.0 / 3.6.11 |
| Alloy chart / app | 1.11.1 / v1.18.1 |
| Grafana chart / app | 10.5.15 / 12.3.1 |
| Storage class | `local-path` (k3s default) |
