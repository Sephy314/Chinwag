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

All resources are deployed in the `chinwag` namespace.

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
values). `deploy.sh` automatically runs `./tls.sh` before applying to create the HTTPS
`chinwag-tls` secret.

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

# Generate the HTTPS self-signed cert + chinwag-tls secret (once; idempotent)
./tls.sh

# Recommended: apply everything with kustomize (handles resource ordering)
kubectl apply -k .

# Or apply individual files in order
kubectl apply -f namespace.yaml
kubectl apply -f configmap.yaml -f secret.yaml
kubectl apply -f postgres.yaml -f redis.yaml -f nats.yaml
kubectl apply -f gateway.yaml -f auth.yaml -f room.yaml
kubectl apply -f chat-command.yaml -f chat-query.yaml -f frontend.yaml
kubectl apply -f ingress.yaml
```

## 3. Host setup

The Ingress host is `chinwag.local`. Register your k3s node IP in `/etc/hosts`.

```bash
# Replace with the IP of the node running k3s
sudo sh -c 'echo "<k3s-node-ip> chinwag.local" >> /etc/hosts'
```

- Web UI: https://chinwag.local
- Gateway health: https://chinwag.local/health (or internally `gateway:8000/health`)
- Swagger UI: `https://chinwag.local/auth/docs`, `https://chinwag.local/rooms/docs`, `https://chinwag.local/chat/docs`

> To use a different hostname, change the `host` in `ingress.yaml` and `FRONTEND_URL` in
> `configmap.yaml` together. WebSocket follows the browser origin, so no extra config is needed.

## 3.1 HTTPS (TLS)

`chinwag.local` is a local-only hostname, so Let's Encrypt is not an option — HTTPS is set up with
a **self-signed CA**.

- `./tls.sh` generates a CA + server certificate (`infra/k3s/.tls/`, gitignored) and registers it
  as the `chinwag-tls` TLS secret in the `chinwag` namespace. `deploy.sh` runs it automatically.
- `ingress.yaml` terminates TLS with `chinwag-tls` on the `websecure` entrypoint (:443).
- `ingress-redirect.yaml` + `middleware.yaml` redirect :80 requests to https.
- `FRONTEND_URL` is `https://chinwag.local` (used for OAuth redirects).

Renewing/regenerating the certificate:

```bash
cd infra/k3s
./tls.sh --force            # Regenerate CA + cert (rotates trust)
./tls.sh --ip 192.168.1.10  # Add a node IP to the SANs
```

To avoid browser warnings, register the local CA as a trusted root CA in your OS (as printed when
tls.sh completes):

```bash
sudo cp infra/k3s/.tls/ca.crt /usr/local/share/ca-certificates/chinwag-ca.crt
sudo update-ca-certificates
```

## 4. Verification

```bash
# Check pod status (all Running/Ready)
kubectl -n chinwag get pods

# Check logs
kubectl -n chinwag logs -l app=auth

# Check each service health via the gateway
curl -sk https://chinwag.local/health                      # gateway
curl -sk https://chinwag.local/auth/health                 # auth
curl -sk https://chinwag.local/rooms/health                # room
curl -sk https://chinwag.local/chat/health                 # chat-command / chat-query

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
