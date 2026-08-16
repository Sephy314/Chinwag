<p align="right"><b>🌐 Language:</b> <a href="README.md">English</a> · <a href="README.ko.md">한국어</a></p>

<div align="center">

# 💬 Chinwag

**A real-time streaming chat service · Go microservice architecture**

🔗 **Live service: [https://chinwag.duckdns.org](https://chinwag.duckdns.org)**

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Echo](https://img.shields.io/badge/Echo-v5-00ADD8?logo=go&logoColor=white)](https://echo.labstack.com/)
[![NATS](https://img.shields.io/badge/NATS-JetStream-27AAE1?logo=natsdotio&logoColor=white)](https://nats.io/)
[![Redis](https://img.shields.io/badge/Redis-7-DC382D?logo=redis&logoColor=white)](https://redis.io/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791?logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![k3s](https://img.shields.io/badge/k3s-deployed-326CE5?logo=kubernetes&logoColor=white)](https://k3s.io/)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

</div>

---

## 📋 Table of Contents

- Overview
- MSA Architecture
- Security: DPoP (RFC 9449)
- CQRS (Command / Query Separation)
- Reliability & Incident Response
- Tech Stack
- API Docs
- Deployment (k3s)
- Observability (Loki + Prometheus + Grafana)
- Directory Structure
- License

---

## 🚀 Overview

A real-time chat service that applies DPoP (RFC 9449) authentication, CQRS, and microservice principles in production.

**Core design principles**

| | |
|---|---|
| 🔐 **DPoP (RFC 9449)** | Sender-constrained tokens with nonce/jti replay protection, JWKS-based verification |
| 🧩 **CQRS** | Command/Query service separation, outbox pattern + NATS JetStream for eventual consistency |
| 🏗️ **MSA** | Gateway-based service decomposition over shared infrastructure (Postgres/Redis/NATS) |
| 🛡️ **Incident response** | Token-reuse detection & lineage revocation, outbox retry/backoff, single-use WS tickets, self-healing schedulers |

---

## 🗺️ MSA Architecture

```
Client (browser)
    │  HTTP / WebSocket
    ▼
API Gateway (Echo reverse proxy)                            :8000
    │
    ├── /auth/*                 ──▶ Auth Service             :8081
    ├── /rooms/*, /users/*      ──▶ Room Service             :8082
    ├── /chat (POST/PUT/DELETE) ──▶ Chat Command             :8083
    ├── /chat/rooms/:id/ws (GET)──▶ Chat Command (WebSocket) :8083
    └── /chat (GET)             ──▶ Chat Query               :8084
```

| Service | Path | Role |
|:---|:---|:---|
| 🌐 **API Gateway** | `backend/gateway` | Path/method-based routing, CORS, reverse proxy |
| 🔐 **Auth Service** | `backend/services/auth` | Users, JWT/DPoP, JWKS, OAuth, refresh-token rotation |
| 🏠 **Room Service** | `backend/services/room` | Room/member/invite-link management, pop scheduler |
| ✍️ **Chat Command** | `backend/services/chat/command` | Message writes + WebSocket hub |
| 📖 **Chat Query** | `backend/services/chat/query` | Message reads (CQRS projection) |
| 🔗 **Shared Auth** | `backend/shared/auth` | Shared DPoP validator, JWT/JWKS middleware |

### Shared Infrastructure

- 🐘 **PostgreSQL** — per-service schemas (`chinwag_auth`, `chinwag_room`, `chinwag_chat`, `chinwag_chat_projection`)
- 🟥 **Redis** — refresh tokens, DPoP nonce/jti, WS tickets, caching (atomic ops via Lua scripts)
- 📡 **NATS JetStream** — event stream (`CHAT_EVENTS`), outbox publishing & CQRS consumption

### Inter-service Communication

- **Synchronous**: gateway → REST reverse proxy
- **Asynchronous**: NATS JetStream events (`chat.room.>` / `chat.system`)

---

## 🔐 Security: DPoP Authentication (RFC 9449)

Every service uses the shared validator in `backend/shared/auth/dpop`. Key prefixes are shared so the nonce space stays consistent across services.

### Proof verification (sender-constrained)

- Clients attach an **ES256-signed DPoP proof** to every request via the `DPoP` header (`typ=dpop+jwt`, P-256)
- Authentication succeeds only if the proof key's **`jkt` (RFC 7638 thumbprint)** matches the access token's **`cnf.jkt`** — a stolen token cannot be replayed from another device
- `htm`/`htu` binding: `htu` is reconstructed from the gateway-set `X-Forwarded-*` headers (`RequestHTU`)

### Nonce & replay defense (atomic Redis Lua)

- **Single-use nonce** (`dpop:nonce:*`) — a fresh nonce is issued via the `DPoP-Nonce` response header; reuse triggers a `use_dpop_nonce` challenge
- **jti replay prevention** (`dpop:jti:*`) — a proof's `jti` is allowed only once within its TTL (default 2 min); reuse returns `invalid_dpop_proof`
- The client retries at most once on a nonce challenge

### JWK & key rotation

- Public keys are published at `/.well-known/jwks.json`
- A **midnight-based automatic key-rotation scheduler** swaps signing keys

---

## 🧩 CQRS (Command / Query Separation)

### Flow

```
[WRITE PATH]                                       [READ PATH]
Chat Command ──single tx──▶ DB(chat) + outbox_events
      │
      │ outbox publisher (100ms poll · batch 50)
      │ exponential backoff retry on failure (max 30s)
      ▼
NATS JetStream (CHAT_EVENTS) ──▶ consumer (chat-projection) ──▶ projection table
      │                                                            │
      └─▶ WebSocket hub (1:N broadcast)                            ▼
                                                       Redis cache + cursor pagination
```

### Command (writes)

- Message persistence and outbox event insertion (`message_created`/`updated`/`deleted`) happen in **a single DB transaction**, so a committed event is always present in the publish queue
- The outbox publisher polls pending events and publishes to NATS, then `MarkPublished`

### Query (reads)

- The NATS consumer (`chat-projection`, AckExplicit) builds the projection table
- Reads are optimized with Redis caching (`chat:*`) and cursor-based pagination

### WebSocket

- To avoid leaking the access token in the URL, a **single-use WS ticket** (TTL 30s) is issued via DPoP auth (`/ws-ticket`) and consumed atomically at upgrade
- Heartbeats (ping/pong) and exponential-backoff reconnection

---

## 🛡️ Reliability & Incident Response

### 1. Token theft / reuse response (Auth)

- Refresh tokens are **single-use** — rotated on every refresh
- Atomic consumption via **Redis Lua** + `lineage ID` tracking (`rt:lineage:*`)
- On **reuse detection, the entire lineage is revoked** (`RevokeLineage`) — a stolen token is neutralized immediately

### 2. Request replay prevention (DPoP)

- Both single-use nonces and jti duplicate checks run as **atomic Redis Lua operations** — replayed requests are rejected at the source

### 3. No access token in URLs

- WebSocket upgrades use only a **30s-TTL, single-use ticket**; re-issuance is required once it is consumed or expired

### 4. Event loss prevention (Outbox + JetStream)

- **Transactional outbox**: DB commit and event recording are atomic — events cannot be lost before publishing
- On publish failure: **exponential backoff retries** (max 30s) with retry count persisted in the DB
- JetStream **FileStorage** persistence survives service restarts (at-least-once)

### 5. Self-healing schedulers

- **JWK key rotation** (auth, midnight) — periodic signing-key swap
- **pop scheduler** (room, 1-min interval) — flips a room to read-only when `pop_at` is reached

### 6. Deployment stability (k3s)

- `update.sh`: rebuild images → import into k3s → apply → **rollout restart** → digest verification
- Per-service health checks + rollout waits for zero-downtime deploys

---

## 🛠️ Tech Stack

**Backend (core)**

- Go 1.26+ / Echo v5
- golang-jwt · lestrrat-go/jwx (JWK/DPoP)
- gorilla/websocket
- NATS JetStream
- go-redis (Lua scripts)
- PostgreSQL (sqlx)
- Swagger (per-service `/docs`)

**Frontend (minimal)**

- Next.js 16 · React 19 · TypeScript
- WebSocket client

---

## �️ Admin Panel

The platform ships with a **role-gated admin panel** for platform operators. Access is enforced **on the backend** (`RequireRole(ADMIN)`), never by the UI alone.

- **UI**: `/admin` in the frontend (linked from the sidebar for `ADMIN`-role users)
- **Roles**: `USER` → `MANAGER` → `ADMIN` (set via the admin Users page)

| Area | Endpoints (via gateway) | Service |
|:---|:---|:---|
| Users | `/auth/admin/users[/:id][/role][/restore][/sessions]` | auth |
| Sessions | `/auth/admin/sessions[/:id]` | auth |
| Audit log | `/auth/admin/audit` | auth |
| Stats | `/auth/admin/stats/*`, `/admin/stats/rooms`, `/chat/admin/stats/messages` | auth / room / chat-query |
| Rooms | `/admin/rooms[/:id][/members]`, `/admin/users/:userId/rooms` | room |
| Messages | `/chat/admin/messages[/:messageId]` | chat-query / chat-command |

Notes:

- All admin routes require a valid **DPoP** access token whose `role` claim is `ADMIN`.
- Every admin mutation records an event in the **audit log** (`admin_audit_log`) via the auth service's internal mTLS endpoint (`POST /internal/audit`, `INTERNAL_PORT=8085`). Audit reporting is best-effort — a failure is logged but does not fail the operation.
- Admin room/message operations bypass the normal user guards (e.g. popped-room read-only, message authorship) so operators can moderate the platform.
- Message deletion is **CQRS-propagated**: `chat-command` soft-deletes and emits a `message_deleted` event through the outbox/NATS so the `chat-query` projection stays consistent.

---

## �📚 API Docs

Each service exposes a **Swagger UI** at `/docs`, reachable through the gateway's `/auth`, `/rooms`, and `/chat` paths.

---

## ☸️ Deployment (k3s)

Live service: **https://chinwag.duckdns.org**

A pure-manifest Kubernetes deployment for **k3s** (no Helm required). All deployment artifacts live in [`infra/k3s`](infra/k3s).

| Artifact | Purpose |
|:---|:---|
| [`backend/Dockerfile`](backend/Dockerfile) | Parameterized multi-stage builder for every Go service (`--build-arg SERVICE=...`) |
| [`infra/k3s/*.yaml`](infra/k3s) | Namespace, ConfigMap, Secret, Postgres/Redis/NATS, all service Deployments + Services, Traefik Ingress |
| [`infra/k3s/build-images.sh`](infra/k3s/build-images.sh) | Builds images and optionally imports them into local k3s |
| [`infra/k3s/deploy.sh`](infra/k3s/deploy.sh) | One-click deploy (build → import → apply → rollout wait → health verify) |
| [`infra/k3s/update.sh`](infra/k3s/update.sh) | Redeploy (rebuild → import → apply → rollout restart → digest verify) |

Key deployment facts:

- A single **PostgreSQL** StatefulSet serves all four service schemas (`chinwag_auth`, `chinwag_room`, `chinwag_chat`, `chinwag_chat_projection`)
- **Redis** (AOF) and **NATS JetStream** are deployed alongside; the `CHAT_EVENTS` stream is created automatically by the chat-command service
- A single **Traefik Ingress** routes `/auth`, `/rooms`, `/users`, `/chat` to the gateway (including WebSocket upgrade) and `/` to the frontend
- **HTTPS**: Let's Encrypt certificates are issued via the duckdns **DNS-01 webhook** (for environments where HTTP-01 is not possible)
- Secrets (DB/Redis passwords, optional Google OAuth) live in `infra/k3s/secret.yaml`; rotate them before any non-local deployment

---

## � Observability (Loki + Prometheus + Grafana)

A container log collection / search / **alerting** stack for the k3s cluster. All artifacts live in [`infra/k3s/observability`](infra/k3s/observability); full docs are in its own [README](infra/k3s/observability/README.md).

```
Chinwag Pods
    │
    ├─ stdout/stderr ──► Grafana Alloy (DaemonSet) ──► Loki ──► Grafana
    │
    └─ /metrics (gateway) ──► Prometheus ──► Alertmanager ──► Notifier ──► Discord
```

| Component | Role |
|:---|:---|
| **Grafana Alloy** | Log collector (DaemonSet), ships stdout/stderr → Loki |
| **Loki** | Log storage & query (monolithic, filesystem) |
| **Prometheus + Alertmanager** | Metrics (cAdvisor + kube-state-metrics + gateway `/metrics`), alert rules, grouping / dedup |
| **Grafana** | Dashboards (provisioned `chinwag-overview`), Loki + Prometheus datasources |
| **Chinwag Notifier** | Stateless Go service: Alertmanager webhook → **Discord** embeds |

- **Metrics**: app-level HTTP metrics come from the gateway's `/metrics` (per-service `http_requests_total`, `http_request_duration_seconds`); the gateway Service carries `prometheus.io/scrape` annotations and `/metrics` is cluster-network only (never exposed on the public ingress).
- **Alerting → Discord**: Prometheus rules (`prometheus-values.yaml`) → Alertmanager (groups by alert/service/severity, repeats every 4h) → Notifier → **per-category Discord webhooks** (`incidents`, `deployments`, `traffic`, `recoveries`, `warnings`) so each channel can be muted independently. Webhook URLs live only in the gitignored `infra/k3s/secret.yaml` and are never committed.
- **Alert rules**: ~18 rules covering HTTP 5xx/4xx, latency, traffic spikes, gateway / dependency down, pod failures / restarts / OOM, deployment replica mismatch, CPU / memory (memory = >150% of request **and** ≥128Mi working set), PVC low space — each scoped to metrics that actually exist in the stack.
- **Deployment**: installed by `infra/k3s/observability/install.sh` (Helm for Loki / Alloy / Prometheus / Grafana, plain `kubectl apply` for the Notifier) into the `monitoring` namespace. `infra/k3s/deploy.sh` calls it (skip with `--no-obs`); CD (`cd.yml`) runs `update.sh --obs` after every push to `main`; the manual-only `.github/workflows/deploy-observability.yml` re-runs it on demand.
- **Grafana access**: served at `/grafana` behind the ingress (cert-manager TLS). The admin password is random and stored in the `grafana-admin` Secret (`kubectl -n monitoring get secret grafana-admin -o jsonpath='{.data.admin-password}' | base64 -d`); change it with `set-grafana-password.sh`.

---

## �📁 Directory Structure

```
backend/
├── gateway/                 # API gateway (reverse proxy)
├── go.work                  # Go workspace
├── shared/auth/             # Shared auth: DPoP validator, JWT/JWKS middleware
└── services/
    ├── auth/                # User auth (JWT/DPoP/JWKS/OAuth/refresh rotation)
    ├── room/                # Rooms/members/invites, pop scheduler
    └── chat/
        ├── command/         # Message writes + WebSocket hub (CQRS command)
        └── query/           # Message reads + projection (CQRS query)

frontend/                    # Next.js web client (minimal)
infra/k3s/                   # k3s deployment manifests + scripts
└── observability/           # obs stack: Loki/Alloy/Prometheus/Grafana + notifier
```

---

## 📄 License

[Apache License 2.0](LICENSE)
