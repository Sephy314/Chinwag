# Chinwag on k3s

Kubernetes (k3s) deployment manifests. Deploys purely with
`backend/Dockerfile`, `frontend/Dockerfile`, and `infra/k3s/*.yaml` — no
application code changes required.

## Components

| Component | Kind | Port | Description |
|---|---|---|---|
| `postgres` | StatefulSet | 5432 | Creates the 4 schemas (`chinwag_auth`, `chinwag_room`, `chinwag_chat`, `chinwag_chat_projection`), `local-path` PVC |
| `redis` | Deployment | 6379 | AOF persistence, password from Secret |
| `nats` | StatefulSet | 4222 | Single-node JetStream, `local-path` PVC (`CHAT_EVENTS` stream is auto-created by chat-command) |
| `gateway` | Deployment (2) | 8000 | Echo reverse proxy |
| `auth` | Deployment | 8081 | Auth/JWKS/OAuth |
| `room` | Deployment | 8082 | Rooms/members/invites |
| `chat-command` | Deployment | 8083 | Message writes + WebSocket |
| `chat-query` | Deployment | 8084 | Message reads (projection) |
| `frontend` | Deployment (2) | 3000 | Next.js standalone server |
| `chinwag` Ingress | Traefik | 80→443 | `/auth /rooms /users /chat` → gateway, `/` → frontend (HTTPS) |

All resources are deployed in the `chinwag` namespace. TLS certificates are issued and
**auto-renewed** by **cert-manager** (installed in its own `cert-manager` namespace) — see §3.1.

## Prerequisites

- Local k3s cluster (`curl -sfL https://get.k3s.io | sh -`)
- Docker (for `docker build` / `docker save`)
- `kubectl` (installed with k3s)

## 0. One-click deploy (recommended)

Build → load into k3s → apply → wait for rollouts → health check in one step.

```bash
cd infra/k3s
./deploy.sh                  # Full deploy
./deploy.sh --no-build       # Skip image build/load (use already-loaded images)
./deploy.sh --apply          # Apply manifests only (skip verification)
```

Prerequisite: `secret.yaml` must exist on disk (`cp secret.yaml.example secret.yaml`, then fill in
values). On first run `deploy.sh` also installs cert-manager (needs network) and waits for the
TLS certificate to be ready — HTTPS is handled by cert-manager (see §3.1).

## 1. Building and loading images

k3s uses containerd, so images built with `docker build` must be imported into the k3s runtime.
If Docker and k3s are on the same host, the `--load` option handles it in one step.

```bash
# Build images + import into k3s
cd infra/k3s
./build-images.sh --load

# (Build images only)
./build-images.sh
# (Import into k3s only)
./load-images.sh
```

> The WebSocket URL is derived automatically from the origin the browser is connected to
> (`src/services/websocket-client.ts`, with the Ingress routing `/chat` to the gateway).
> No separate WS URL setting or build argument is required.

## 2. Applying

```bash
cd infra/k3s

# One-time: install cert-manager (v1.21.1) for TLS cert management
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.21.1/cert-manager.yaml

# Recommended: apply everything with kustomize (handles resource ordering)
kubectl apply -k .

# Or apply individual files in order
kubectl apply -f namespace.yaml
kubectl apply -f configmap.yaml -f secret.yaml
kubectl apply -f postgres.yaml -f redis.yaml -f nats.yaml
kubectl apply -f gateway.yaml -f auth.yaml -f room.yaml
kubectl apply -f chat-command.yaml -f chat-query.yaml -f frontend.yaml
kubectl apply -f ingress.yaml
kubectl apply -f clusterissuer.yaml -f certificate.yaml
```

## 3. Host setup

The service is served on the public domain **chinwag.duckdns.org** (A record → the node's public
IP; :80/:443 port-forwarded to the k3s node). No `/etc/hosts` entry is required.

- Web UI: https://chinwag.duckdns.org
- Gateway health: https://chinwag.duckdns.org/health (or internally `gateway:8000/health`)
- Swagger UI: `https://chinwag.duckdns.org/api/auth/docs`, `https://chinwag.duckdns.org/api/rooms/docs`,
  `https://chinwag.duckdns.org/api/chat/docs`

> The Ingress matches any host, so LAN access also works. If your router doesn't do hairpin NAT,
> map the domain to the node's LAN IP in `/etc/hosts` on LAN clients (the Let's Encrypt cert still
> validates, so no warning):
> `sudo sh -c 'echo "192.168.0.30 chinwag.duckdns.org" >> /etc/hosts'`

## 3.1 HTTPS (TLS)

TLS is managed by **cert-manager** in-cluster — it issues the `chinwag-tls` secret used by the
Traefik ingress and **auto-renews it** before expiry (no cron or manual step).

The public, browser-trusted cert is issued by **Let's Encrypt** via **DNS-01** (duckdns):

- `letsencrypt` ClusterIssuer (ACME) validates through a duckdns **TXT record** using the
  `cert-manager-webhook-duckdns` webhook (`duckdns-webhook.yaml`). No inbound :80/:443 or router
  hairpin NAT is needed.
- `Certificate/chinwag-tls` (secret `chinwag-tls`) covers `chinwag.duckdns.org` and is auto-renewed.
- `chinwag-selfsigned` ClusterIssuer mints a self-signed CA (`chinwag-ca`) used only for the
  **internal** `postgres-tls` cert — postgres is only reachable inside the cluster, so it doesn't
  need a public cert.
- `ingress.yaml` terminates TLS with `chinwag-tls` on the `websecure` entrypoint (:443).
- `ingress-redirect.yaml` + `middleware.yaml` redirect :80 requests to https.
- `FRONTEND_URL` is `https://chinwag.duckdns.org` (used for OAuth redirects); `GOOGLE_REDIRECT_URL`
  must use the same host.

One-time setup (duckdns API token, in the `cert-manager` namespace):

```bash
kubectl -n cert-manager create secret generic duckdns-credentials \
  --from-literal=token='<your-duckdns-api-token>'
```

`deploy.sh` installs cert-manager on first run, applies the duckdns webhook, and waits for the
certificate to become Ready.

Check status:

```bash
kubectl -n chinwag get certificate
kubectl -n chinwag describe certificate/chinwag-tls
kubectl -n chinwag get clusterissuer letsencrypt
```

Troubleshooting: if the certificate never becomes Ready, inspect the ACME order/challenge and the
webhook:

```bash
kubectl -n chinwag get order,challenge
kubectl -n chinwag describe certificate/chinwag-tls
kubectl -n cert-manager logs deploy/cert-manager-webhook-duckdns --tail=50
kubectl -n cert-manager get apiservice v1alpha1.acme.duckdns.org
```

> **Known issue on this WSL host**: the DNS-01 challenge can hang with
> `SOA record ... SERVFAIL` because CoreDNS forwards external queries to the WSL
> DNS proxy (`/etc/resolv.conf` → `10.255.255.254`), which fails SOA lookups.
> Fix (applied to the live cluster on 2026-08-09): point cert-manager's DNS-01
> lookups at a public recursive resolver by adding these args to the
> `cert-manager` controller Deployment:
>
> ```
> --dns01-recursive-nameservers=8.8.8.8:53
> --dns01-recursive-nameservers-only
> ```
>
> ⚠️ `deploy.sh` re-applies the static cert-manager manifest, which would drop
> these args — re-apply them after any cert-manager reinstall. (Alternative,
> more global fix: set CoreDNS `forward .` to a public resolver instead of
> `/etc/resolv.conf`.)

> The old self-signed CA flow remains available in `tls.sh` as a legacy fallback for a local-only
> deployment.

## 4. Verification

```bash
# Check pod status (all Running/Ready)
kubectl -n chinwag get pods

# Check logs
kubectl -n chinwag logs -l app=auth

# Check each service health via the gateway
curl -sk https://chinwag.duckdns.org/health                # gateway
curl -sk https://chinwag.duckdns.org/api/auth/health           # auth
curl -sk https://chinwag.duckdns.org/api/rooms/health          # room
curl -sk https://chinwag.duckdns.org/api/chat/health           # chat-command / chat-query

# Verify internal DNS/SVC (from inside a pod)
kubectl -n chinwag exec deploy/gateway -- wget -qO- http://auth:8081/health

# Check events
kubectl -n chinwag get events --sort-by=.lastTimestamp
```

End-to-end verification: sign up / sign in → create a room → send a message (received in real time
over WebSocket).

## 5. Changing environment variables / secrets

| File | Contents |
|---|---|
| `configmap.yaml` | Non-secret shared config (ports, service URLs, `FRONTEND_URL`, `NATS_URL`, etc.) |
| `secret.yaml` | DB passwords, Redis password, Google OAuth (includes local-dev defaults) |

After replacing a Secret, redeploy:

```bash
kubectl -n chinwag create secret generic chinwag-secrets \
  --from-literal=POSTGRES_PASSWORD='<new>' \
  --from-literal=REDIS_PASSWORD='<new>' \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl -n chinwag rollout restart deploy auth room chat-command chat-query redis
```

> ⚠️ To enable Google OAuth, set `GOOGLE_CLIENT_ID/SECRET/REDIRECT_URL` in `secret.yaml` and make
> sure `GOOGLE_REDIRECT_URL` matches `FRONTEND_URL`.

## 6. Deletion

```bash
kubectl delete -k infra/k3s
```

PVC (`local-path`) data survives deletion. For a full teardown:

```bash
kubectl -n chinwag delete pvc --all
```

## 7. Changing image tags

To change the `image: chinwag/<service>:latest` tag to a version of your choice, update both the
`image` field in each deployment file and the tag in `build-images.sh`. If you use a private
registry, change `imagePullPolicy: IfNotPresent` to `Always` and use a tag that includes the
registry address.

## 8. Scaling notes

- `gateway` and `frontend` are stateless, so scaling replicas up is safe.
- `auth`, `room`, `chat-command`, and `chat-query` run **DB migrations at startup**, so keep
  replicas at 1 (to avoid migration races). `chat-command` uses a fixed per-pod JetStream durable
  consumer (`chat-ws-<pod>`), so it resumes without loss even across pod restarts.
