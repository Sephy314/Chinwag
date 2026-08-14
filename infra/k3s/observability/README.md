# Chinwag Observability (Loki + Grafana Alloy + Prometheus/Alertmanager + Grafana + Notifier)

Container log collection / search / **alerting** stack for the Chinwag k3s cluster.

```
Chinwag Pods
    │
    ├─ stdout/stderr ──► Grafana Alloy (DaemonSet) ──► Loki ──► Grafana
    │
    └─ /metrics (gateway) ──► Prometheus ──► Alertmanager ──► Notifier ──► Discord
```

- Log collector: **Grafana Alloy** (no Promtail)
- Metrics: **Prometheus** (cAdvisor + kube-state-metrics + gateway `/metrics`)
- Alerting: **Alertmanager** groups alerts and POSTs webhooks to the **Chinwag Notifier**
  (a stateless Go service) which renders Discord embeds and forwards them to a
  **Discord webhook**
- Loki / Alloy / Prometheus / Grafana are installed with **Helm** (in the `monitoring`
  namespace); the **Notifier** is a plain `kubectl apply` (Deployment + Service)
- The existing `infra/k3s/*.yaml` (kustomize / GitOps) structure is **left untouched**
- Built for a small **single-node k3s** setup. No HA / replication.

---

## Directory layout

```
infra/k3s/observability/
├── README.md                    # this document
├── loki-values.yaml             # Loki Helm values (monolithic / filesystem / 1 replica)
├── alloy-values.yaml            # Alloy Helm values (DaemonSet + River config)
├── prometheus-values.yaml       # Prometheus + Alertmanager Helm values + alert rules
├── grafana-values.yaml          # Grafana Helm values (Loki+Prometheus datasources, dashboards)
├── grafana-certificate.yaml     # cert-manager Certificate for the /grafana ingress (TLS)
├── set-grafana-password.sh      # set a fixed Grafana admin user/password (incl. custom users)
├── dashboards/                  # provisioned Grafana dashboards (chinwag-overview.json)
├── notifier.yaml                # Notifier Deployment + Service (monitoring namespace)
├── notifier/                    # Alertmanager -> Discord notification service (Go)
│   ├── main.go                  # HTTP server (POST /webhooks/alertmanager, GET /health)
│   ├── config.go                # env config (port, DISCORD_WEBHOOK_URL, timeout)
│   ├── handler.go               # webhook handler (parse -> validate -> deliver)
│   ├── discord.go               # Discord webhook client (stdlib http only)
│   ├── message.go               # payload -> Discord embed formatting
│   ├── payload.go               # Alertmanager webhook payload structs
│   ├── *_test.go                # unit + HTTP tests (httptest, no external network)
│   └── Dockerfile
└── install.sh                   # install script (namespace + secrets + notifier + Helm)
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

### Production (CD) deployment

The observability stack is **not** part of the GitOps `infra/k3s` kustomize
resources — it is installed via `install.sh` (Helm + a few `kubectl apply`
manifests). Deployment is **automatic**:

- **Dev** — `infra/k3s/deploy.sh` calls `./observability/install.sh` (skip with
  `--no-obs`).
- **Production (automatic)** — `.github/workflows/deploy-observability.yml`
  runs `observability/install.sh` on the self-hosted runner on the k3s node
  **after every successful `CD` run on `main`** (`workflow_run`), or on demand
  via `workflow_dispatch` (Actions → "Deploy Observability"). This means the
  notifier image, Alertmanager config, alert rules and the whole obs stack are
  re-deployed on every app deploy — the notifier is now under **automatic CD**.
  `infra/k3s/update.sh` (the CD fast path) also accepts `--obs` if you want to
  bundle the obs update into an app deploy, but it is **not** run by default.

All entry points are idempotent: the `grafana-admin` and
`chinwag-notifier-secrets` Secrets are only created once (existing values kept),
dashboards ConfigMap is re-applied, and the `grafana-tls` Certificate
auto-renews via cert-manager.

> **Safety**: the notifier's tests run in the CI `backend` job (via `make test`,
> which includes `infra/k3s/observability/notifier`). To guarantee a failed
> test can never deploy, require the `CI` workflow to pass on `main` (branch
> protection) — `cd.yml` already only deploys `main`, and the obs workflow runs
> after CD.

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

# 3) Prometheus + Alertmanager + alert rules (webhook -> notifier)
helm upgrade --install prometheus prometheus-community/prometheus -n monitoring \
  --version 29.23.0 -f prometheus-values.yaml --wait

# 4) Notifier (Alertmanager -> Discord) — build image, inject webhook secret, apply
DISCORD_WEBHOOK_URL='https://discord.com/api/webhooks/<id>/<token>' \
  docker build -t chinwag/notifier:latest -f notifier/Dockerfile notifier/
kubectl -n monitoring create secret generic chinwag-notifier-secrets \
  --from-literal=DISCORD_WEBHOOK_URL='<url>' \
  --dry-run=client -o yaml | kubectl -n monitoring apply -f -
kubectl apply -f notifier.yaml

# 5) Grafana admin Secret (random password, never committed to Git)
kubectl -n monitoring create secret generic grafana-admin \
  --from-literal=admin-user=admin \
  --from-literal=admin-password='<strong-password>'

# 6) Grafana — Loki datasource auto-provisioned
helm upgrade --install grafana grafana/grafana -n monitoring \
  --version 10.5.15 -f grafana-values.yaml --wait
```

> Forgot the Grafana password after install?
> `kubectl -n monitoring get secret grafana-admin -o jsonpath='{.data.admin-password}' | base64 -d`

### Change the Grafana admin credentials

The `grafana-admin` Secret is only read at **first** startup, so editing the
Secret alone does not change an already-provisioned admin. Use the bundled
script — it:

1. Resets the built-in `admin` login via `grafana-cli` (takes effect
   immediately).
2. If a **custom username** is given, creates/updates that user via the Grafana
   HTTP Admin API and grants it the org **Admin** role (`grafana-cli` cannot
   create users — this was the "user not applied" pitfall).
3. Updates the Secret so a future reinstall keeps the same values.

```bash
./set-grafana-password.sh                          # prompts for user + password
GRAFANA_ADMIN_USER='admin' GRAFANA_PASSWORD='...' ./set-grafana-password.sh
                                                    # or via env vars
sudo ./set-grafana-password.sh                   # works too (kubeconfig fallback)
```

The credentials are never passed on argv or echoed. On the **production
server**, copy the script over and run it there (it only talks to whatever
cluster the local `kubectl` points at):

```bash
scp infra/k3s/observability/set-grafana-password.sh sephy314@server:/tmp/
ssh sephy314@server 'cd /tmp && ./set-grafana-password.sh'
```

---

## Alerting (Alertmanager → Notifier → Discord)

```
Prometheus ──(alert rules)──► Alertmanager ──(HTTP webhook)──► chinwag-notifier ──(Discord webhook)──► Discord
```

- **Prometheus** decides *when* something is wrong (alert rules in
  `prometheus-values.yaml` → `serverFiles.alerting_rules.yml`).
- **Alertmanager** manages the alert lifecycle and **groups** alerts
  (`group_by: [alertname, service, severity]`, `group_wait: 30s`,
  `group_interval: 5m`, `repeat_interval: 4h`) — this is the deduplication /
  spam control layer. No state is kept in the notifier.
- **Chinwag Notifier** (`notifier/`) is a stateless Go service that receives
  Alertmanager webhooks and forwards Discord embeds. It has **no database /
  Redis / NATS**.

### Notifier

- Go module at `infra/k3s/observability/notifier/` (only `joho/godotenv` for
  local `.env` loading). Deployed as a `Deployment` + `Service` in the
  `monitoring` namespace (`notifier.yaml`), image `chinwag/notifier:latest`.
- Local runs: copy `notifier/.env.example` to `notifier/.env` (gitignored) and
  fill in `DISCORD_WEBHOOK_URL` — godotenv loads it for `go run .`; in
  Kubernetes the Secret / Deployment env take precedence.
- Endpoints:
  - `POST /webhooks/alertmanager` — Alertmanager webhook receiver.
    `2xx` = accepted and delivered; `4xx` = malformed payload; `5xx` = internal
    error (Discord failure / not configured).
  - `GET /health` — liveness/readiness probe.
- One webhook notification becomes **one Discord message** with one embed per
  alert (capped at 10 embeds) — a batched Alertmanager notification does not
  spam Discord. `firing` embeds are red (🔴), `resolved` embeds are green (🟢)
  and include the duration.
- Unknown/extra labels never fail parsing (the payload struct only declares the
  fields the notifier uses), and missing labels/annotations fall back gracefully.

### Discord webhooks (secret) & category routing

Alerts are routed to **one Discord webhook per category**, so each channel can
be muted/silenced independently:

| Category | Purpose | Example alerts |
|---|---|---|
| `incidents` | critical app/infra failures | High5xxRate, GatewayDown, PodFailed, CrashLoopBackOff, OOMKilled, … |
| `deployments` | rollout / replica issues | DeploymentReplicasMismatch |
| `traffic` | traffic & latency | TrafficSpike, HighLatency |
| `recoveries` | **resolved** alerts (any category) | — (status-based) |
| `warnings` | non-critical warnings | High4xxRate, PodRestarting, HighCPU, HighMemory, PVCLowSpace |

The category comes from the alert's `category` label (set on each rule in
`prometheus-values.yaml`); resolved alerts always go to `recoveries`. Alerts
with no category fall back to the **default** webhook.

The URLs are **never** committed. `install.sh` reads
`DISCORD_WEBHOOK_URL` / `DISCORD_WEBHOOK_URL_<CATEGORY>` from the gitignored
`infra/k3s/secret.yaml` (copied from `secret.yaml.example`), with explicit env /
`./notifier.env` as fallbacks, and stores them in the `chinwag-notifier-secrets`
Secret, which the Deployment injects via `envFrom`. The notifier never logs them.

```bash
# one time, on the deploy host — add to infra/k3s/secret.yaml (gitignored):
#   DISCORD_WEBHOOK_URL: "https://discord.com/api/webhooks/<id>/<token>"
#   DISCORD_WEBHOOK_URL_INCIDENTS: "https://discord.com/api/webhooks/<id>/<token>"
#   ... (optional per-category URLs)
./infra/k3s/observability/install.sh
```

If no webhook is configured, the notifier still starts and serves `/health`,
but alert webhooks return `500` and nothing is delivered (a warning is logged).

Notifier env vars (see `notifier/config.go`):

| Env | Default | Description |
|---|---|---|
| `NOTIFIER_PORT` | `9095` | HTTP listen port |
| `DISCORD_WEBHOOK_URL` | — | Default/fallback Discord webhook URL |
| `DISCORD_WEBHOOK_URL_INCIDENTS` | — | Incidents webhook |
| `DISCORD_WEBHOOK_URL_DEPLOYMENTS` | — | Deployments webhook |
| `DISCORD_WEBHOOK_URL_TRAFFIC` | — | Traffic webhook |
| `DISCORD_WEBHOOK_URL_RECOVERIES` | — | Recoveries webhook |
| `DISCORD_WEBHOOK_URL_WARNINGS` | — | Warnings webhook |
| `NOTIFIER_DISCORD_TIMEOUT` | `10s` | Max duration for a single Discord POST |

All webhook URLs are injected via the `chinwag-notifier-secrets` Secret.

### Alert rules

Rules are defined in `prometheus-values.yaml` under `serverFiles.alerting_rules.yml`
and are evaluated by Prometheus every 15s. Only metrics that actually exist in
this stack are used; a rule referencing a metric that never appears simply stays
silent (no fabricated metrics).

| Alert | Severity | Category | Signals |
|---|---|---|---|
| `ChinwagHigh5xxRate` | critical | incidents | >5% 5xx of gateway traffic per service (5m) |
| `ChinwagHigh4xxRate` | warning | warnings | >10% 4xx per service (10m) |
| `ChinwagTrafficSpike` | warning | traffic | current 5m traffic >3x the 1h average (10m) |
| `ChinwagHighLatency` | warning | traffic | p95 latency >1s (10m) |
| `ChinwagGatewayDown` | critical | incidents | `up{service="gateway"} == 0` (3m) |
| `ChinwagKubeStateMetricsDown` | critical | incidents | `up{service="kube-state-metrics"} == 0` (5m) |
| `ChinwagAlertmanagerDown` | critical | incidents | `up{service="prometheus-alertmanager"} == 0` (5m) |
| `ChinwagPodFailed` | critical | incidents | pod phase `Failed` (2m) |
| `ChinwagPodNotReady` | critical | incidents | pod not-ready (5m) |
| `ChinwagPodCrashLoopBackOff` | critical | incidents | container waiting `CrashLoopBackOff` (5m) |
| `ChinwagPodOOMKilled` | critical | incidents | container last terminated `OOMKilled` (2m) |
| `ChinwagPodRestarting` | warning | warnings | >3 restarts / 15m (10m) |
| `ChinwagDeploymentReplicasMismatch` | critical | deployments | desired ≠ available replicas (10m) |
| `ChinwagHighCPU` | warning | warnings | >90% of CPU request (10m) |
| `ChinwagHighMemory` | warning | warnings | >90% of memory request (10m) |
| `ChinwagPVCLowSpace` | warning | warnings | PVC <15% free (10m, via kubelet volume stats) |

App-level HTTP metrics come from the **gateway's `/metrics`** endpoint
(`backend/gateway/middleware/metrics.go`) — the gateway is the single edge for
all HTTP traffic, so `http_requests_total{service,method,code}` and
`http_request_duration_seconds` give per-service error/traffic/latency signals
(the `service` label maps gateway route prefixes → auth/room/chat/admin). The
gateway Service carries `prometheus.io/scrape` annotations
(`infra/k3s/gateway.yaml`) and is scraped by the chart's built-in
`kubernetes-service-endpoints` job over the cluster network — `/metrics` is
**not** exposed on the public ingress.

> **Dependencies**: Postgres / Redis / NATS have **no exporters** in this stack,
> so no dependency-health rules are defined (per the "don't fabricate metrics"
> rule). A backend outage still surfaces through gateway 5xx alerts (the gateway
> returns 503 when a backend is unreachable). To monitor dependencies directly,
> add `postgres-exporter` / `redis-exporter` / a NATS exporter and extend the
> rules.

### Severity → Discord grouping

Alertmanager groups by `alertname + service + severity`, so all `critical`
instances of one alert are grouped into a single webhook → single Discord
message. Repeat notifications for a still-firing alert come every 4h.

### Tests

`notifier/` has table-driven tests covering: valid firing/resolved payloads,
invalid JSON, trailing data, empty `alerts`, missing webhook URL, Discord API
errors, Discord timeouts, sparse alerts (missing labels/annotations), unknown
fields, and embed formatting (title/color/fields/duration, embed cap). Discord
calls use an in-process `httptest` mock — **no external network**. The tests are
picked up automatically by the backend `make test` / `make vet` (the notifier is
in the `SERVICES` list), so the CI `backend` job covers them.

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

Grafana is served by the k3s Traefik ingress at **https://chinwag.duckdns.org/grafana**
(HTTPS, cert-manager `grafana-tls` — see `grafana-certificate.yaml`; the path is
**not** stripped, Grafana serves itself from the `/grafana` subpath via
`server.serve_from_sub_path`). A `port-forward` also works for local access:

```bash
# Local access (alternative to the public URL)
# (local 3000 is the frontend's port — next dev / k8s frontend:3000 — so Grafana uses 3001)
kubectl -n monitoring port-forward svc/grafana 3001:80
# then open http://localhost:3001 in a browser

# Login (admin / password from the Secret above)
# Connections → Data sources → Loki → Save & test  → "Success"
```

> The public URL answers only if the DNS for `chinwag.duckdns.org` points at the
> cluster serving this ingress (this dev cluster). For local dev, add a hosts
> entry mapping `chinwag.duckdns.org` → the node IP (e.g. `127.0.0.1`) if the
> public A record points at another host.

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

## Metrics (Prometheus) & dashboards

Prometheus (`prometheus-community/prometheus`) collects metrics from the
**kubelet cAdvisor** (per-container CPU/RAM) and **kube-state-metrics**
(pod/deployment counts). App-level request counts and error rates are derived
from the **Loki** logs in Grafana (the Go services are not instrumented with
`/metrics`).

- Datasources in Grafana: **Loki** (default) + **Prometheus**
  (`http://prometheus-server.monitoring.svc.cluster.local:80`).
- Provisioned dashboard: **Chinwag Overview** (folder `chinwag`), loaded from
  `dashboards/chinwag-overview.json` via the `grafana-dashboards` ConfigMap
  (`dashboardsConfigMaps` + `dashboardProviders` in grafana-values.yaml).
- Panels: CPU usage per service, memory usage per service, pod count per
  service, request count per service (Loki), error rate per service (Loki),
  recent error logs (Loki).
- On **WSL2**, `prometheus-node-exporter` is disabled (host `/` mount is not
  shared/slave); container metrics come from cAdvisor, so this is fine.

> `install.sh` creates the `grafana-dashboards` ConfigMap before installing
> Grafana, so dashboards provision automatically. Re-running it after changing
> `dashboards/*.json` updates the dashboard.

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
- **Minimize external exposure**: Loki/Alloy are ClusterIP-only (no ingress).
  Grafana is exposed at `/grafana` on the existing HTTPS ingress (TLS via
  `grafana-tls`, login required, anonymous auth off) — it is never bound to a
  NodePort/LoadBalancer directly.
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
