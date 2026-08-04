<div align="center">

# 💬 Chinwag

**A real-time streaming chat service built with a small microservice architecture.**

Backend in **Go (Echo)** · Frontend in **Next.js (React 19)**

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Echo](https://img.shields.io/badge/Echo-v5-00ADD8?logo=go&logoColor=white)](https://echo.labstack.com/)
[![Next.js](https://img.shields.io/badge/Next.js-16-000000?logo=next.js&logoColor=white)](https://nextjs.org/)
[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=white)](https://react.dev/)
[![NATS](https://img.shields.io/badge/NATS-JetStream-27AAE1?logo=natsdotio&logoColor=white)](https://nats.io/)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

</div>

---

## 📋 Table of Contents

- [Features](#-features)
- [System Layout](#-system-layout)
- [Backend Architecture](#-backend-architecture)
- [Tech Stack](#-tech-stack)
- [API Docs](#-api-docs)
- [Directory Structure](#-directory-structure)
- [License](#-license)

---

## ✨ Features

| | |
|---|---|
| ⚡ **Real-time chat** | WebSocket-based 1:N streaming of chat messages (create/update/delete, system messages) |
| 🔐 **DPoP (RFC 9449) authentication** | Sender-constrained bearer tokens with nonce and replay protection, JWKS-based token verification |
| 🧩 **CQRS** | Separate Command and Query services with eventual consistency via NATS JetStream + the outbox pattern |
| 🏠 **Room management** | Room creation, member management, invite links |
| 💥 **"Pop" rooms** | At a configured time (`pop_at`) a room "pops" and becomes read-only |
| 👤 **User authentication** | Signup/login/logout, refresh-token rotation with lineage tracking, optional Google OAuth |
| 🔑 **JWK key rotation** | Automatic signing-key rotation scheduler |
| 🌐 **API gateway** | Path/method based reverse proxy |

---

## 🗺️ System Layout

```
Frontend (Next.js)
        │  HTTP / WebSocket
        ▼
API Gateway (Echo reverse proxy)
        │
        ├── /auth/*   ──▶ Auth Service
        ├── /rooms/*  ──▶ Room Service
        ├── /users/*  ──▶ Room Service
        └── /chat/*   ──▶ Chat Command Service   [POST/PUT/DELETE + WS]
                    └──▶ Chat Query  Service   [GET]
```

<div align="center">

| Component | Path | Role |
|:---|:---|:---|
| 🌐 **API Gateway** | `backend/gateway` | Auth, CORS, routing reverse proxy |
| 🔐 **Auth Service** | `backend/services/auth` | Users, JWT/DPoP, JWKS, OAuth, refresh tokens |
| 🏠 **Room Service** | `backend/services/room` | Room/member/invite-link management |
| ✍️ **Chat Command** | `backend/services/chat/command` | Message writes + WebSocket hub |
| 📖 **Chat Query** | `backend/services/chat/query` | Message reads (CQRS projection) |
| 🔗 **Shared Auth** | `backend/shared/auth` | Shared DPoP validator, JWT/JWKS middleware |
| 💻 **Frontend** | `frontend` | Next.js web client |

</div>

### 🧱 Shared Infrastructure

- 🐘 **PostgreSQL** — per-service schemas (projections/messages/rooms, etc.)
- 🟥 **Redis** — refresh tokens, DPoP nonce/jti, WS tickets, caching, distributed locks
- 📡 **NATS JetStream** — event stream (`CHAT_EVENTS`), outbox publishing and CQRS consumption

---

## ⚙️ Backend Architecture

### 🔐 DPoP Authentication (RFC 9449)

The shared validator in `backend/shared/auth/dpop` is used by every service. Key prefixes are shared so the nonce space stays consistent across services.

- Clients attach an ES256-signed DPoP proof to every request via the `DPoP` header
- Authentication succeeds only if the proof's `jkt` (RFC 7638 thumbprint) matches the `cnf.jkt` claim of the access token
- Redis Lua scripts atomically enforce nonce reuse protection (`dpop:nonce:*`) and jti replay protection (`dpop:jti:*`)
- On a `use_dpop_nonce` challenge, the client reads the `DPoP-Nonce` response header and retries at most once
- `htu` is reconstructed from the gateway-set `X-Forwarded-*` headers (`RequestHTU`)

### 👤 Authentication (Auth Service)

- JWT generation/verification, JWK issuance (`/.well-known/jwks.json`), and a key-rotation scheduler that runs at midnight
- Refresh-token rotation: a Redis Lua script consumes tokens and tracks a lineage ID to detect replay and theft — upon a detected reuse, the entire lineage is revoked
- Optional Google OAuth login

### 🧩 Chat CQRS

- **Command** — On write, the transaction persists the message to the DB and inserts an outbox event (`message_created`/`updated`/`deleted`). The outbox publisher publishes to NATS (`chat.room.*`), and the WebSocket hub broadcasts in real time
- **Query** — The NATS consumer (`chat-projection`) builds the projection table and reads with Redis caching (`chat:*`), using cursor-based pagination
- **WebSocket** — To avoid leaking the access token in the URL, a **single-use WS ticket** is issued with DPoP auth (`/ws-ticket`) and consumed atomically at the upgrade. Heartbeats (ping/pong) and exponential reconnection are supported

### 🏠 Room

- Room/member management, token-based invite links, and a `pop_at` scheduler that turns a room read-only

---

## 🛠️ Tech Stack

<table>
<tr>
<td valign="top" width="50%">

**Backend**
- Go 1.26+
- Echo v5
- golang-jwt
- lestrrat-go/jwx
- gorilla/websocket
- NATS JetStream
- go-redis
- godotenv
- Swagger (per-service `/docs`)

</td>
<td valign="top" width="50%">

**Frontend**
- Next.js 16
- React 19
- TypeScript
- Tailwind CSS v4
- TanStack Query
- react-hook-form
- zod
- shadcn/ui-style components
- Vitest

</td>
</tr>
</table>

---

## 📚 API Docs

Each service exposes a **Swagger UI** at `/docs`, reachable through the gateway's `/auth`, `/rooms`, and `/chat` paths.

---

## 📁 Directory Structure

```
backend/
├── gateway/                 # API gateway (reverse proxy)
├── go.work                  # Go workspace
├── shared/auth/             # Shared auth: DPoP, JWT/JWKS middleware
└── services/
    ├── auth/                # User authentication service
    ├── room/                # Room/member/invite service
    └── chat/
        ├── command/         # Message writes + WS hub (CQRS command)
        └── query/           # Message reads + projections (CQRS query)

frontend/
├── src/
│   ├── app/                 # Next.js App Router pages
│   ├── components/          # Shared UI components
│   ├── features/            # Domain features (auth/room/chat)
│   ├── lib/                 # API client, DPoP helpers
│   ├── services/            # WebSocket client
│   └── types/               # Shared types
└── package.json
```

---

## 📄 License

[Apache License 2.0](LICENSE)
