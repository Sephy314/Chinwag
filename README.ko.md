<p align="right"><b>🌐 언어:</b> <a href="README.md">English</a> · <a href="README.ko.md">한국어</a></p>

<div align="center">

# 💬 Chinwag

**실시간 스트리밍 채팅 서비스 · Go 마이크로서비스 아키텍처**

🔗 **라이브 서비스: [https://chinwag.duckdns.org](https://chinwag.duckdns.org)**

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Echo](https://img.shields.io/badge/Echo-v5-00ADD8?logo=go&logoColor=white)](https://echo.labstack.com/)
[![NATS](https://img.shields.io/badge/NATS-JetStream-27AAE1?logo=natsdotio&logoColor=white)](https://nats.io/)
[![Redis](https://img.shields.io/badge/Redis-7-DC382D?logo=redis&logoColor=white)](https://redis.io/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791?logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![k3s](https://img.shields.io/badge/k3s-deployed-326CE5?logo=kubernetes&logoColor=white)](https://k3s.io/)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

</div>

---

## 📋 목차

- 개요
- MSA 아키텍처
- 보안: DPoP (RFC 9449)
- CQRS (명령/조회 분리)
- 신뢰성 & 장애 대응
- 기술 스택
- API 문서
- 배포 (k3s)
- 디렉토리 구조
- 라이선스

---

## 🚀 개요

DPoP(RFC 9449) 기반 인증, CQRS, MSA 원칙을 실제로 적용한 실시간 채팅 서비스.

**핵심 설계 원칙**

| | |
|---|---|
| 🔐 **DPoP (RFC 9449)** | sender-constrained 토큰 + nonce/jti 재생 방지, JWKS 검증 |
| 🧩 **CQRS** | 명령/조회 서비스 분리, outbox 패턴 + NATS JetStream 기반 eventual consistency |
| 🏗️ **MSA** | gateway 기반 서비스 분리, 공유 인프라(Postgres/Redis/NATS) |
| 🛡️ **장애 대응** | 토큰 재사용 감지·lineage 폐기, outbox 재시도/백오프, 단회용 WS 티켓, 자동 복구 스케줄러 |

---

## 🗺️ MSA 아키텍처

```
클라이언트 (브라우저)
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

| 서비스 | 경로 | 역할 |
|:---|:---|:---|
| 🌐 **API Gateway** | `backend/gateway` | 경로·메서드 기반 라우팅, CORS, 역방향 프록시 |
| 🔐 **Auth Service** | `backend/services/auth` | 사용자, JWT/DPoP, JWKS, OAuth, refresh token 회전 |
| 🏠 **Room Service** | `backend/services/room` | 방/멤버/초대 링크 관리, pop 스케줄러 |
| ✍️ **Chat Command** | `backend/services/chat/command` | 메시지 쓰기 + WebSocket 허브 |
| 📖 **Chat Query** | `backend/services/chat/query` | 메시지 읽기 (CQRS projection) |
| 🔗 **Shared Auth** | `backend/shared/auth` | 공용 DPoP 검증기, JWT/JWKS 미들웨어 |

### 공유 인프라

- 🐘 **PostgreSQL** — 서비스별 스키마 분리 (`chinwag_auth`, `chinwag_room`, `chinwag_chat`, `chinwag_chat_projection`)
- 🟥 **Redis** — refresh token, DPoP nonce/jti, WS ticket, 캐시 (원자 연산은 Lua 스크립트)
- 📡 **NATS JetStream** — 이벤트 스트림(`CHAT_EVENTS`), outbox 발행 & CQRS 소비

### 서비스 간 통신

- **동기**: gateway → REST 역방향 프록시
- **비동기**: NATS JetStream 이벤트 (`chat.room.>` / `chat.system`)

---

## 🔐 보안: DPoP 인증 (RFC 9449)

모든 서비스는 `backend/shared/auth/dpop`의 공용 검증기를 사용하며, nonce 공간이 서비스 간에 일관되도록 key prefix를 공유한다.

### Proof 검증 (sender-constrained)

- 클라이언트는 모든 요청에 **ES256 서명된 DPoP proof**를 `DPoP` 헤더로 첨부 (`typ=dpop+jwt`, P-256)
- proof에 내장된 공개키의 **`jkt`(RFC 7638 thumbprint)**가 access token의 **`cnf.jkt`**와 일치해야만 인증 성공 → 토큰을 탈취해도 다른 기기에서 재사용 불가
- `htm`/`htu` 바인딩: `htu`는 gateway가 설정한 `X-Forwarded-*` 헤더로 재구성(`RequestHTU`)

### Nonce & replay 방어 (Redis Lua 원자 연산)

- **nonce 단회용** (`dpop:nonce:*`) — 응답 `DPoP-Nonce` 헤더로 새 nonce 발급, 재사용 시 `use_dpop_nonce` 챌린지
- **jti 재생 방지** (`dpop:jti:*`) — proof의 `jti`를 TTL(기본 2분) 동안 1회만 허용, 재사용 시 `invalid_dpop_proof`
- 클라이언트는 nonce 챌린지에 최대 1회 재시도

### JWK & 키 로테이션

- `/.well-known/jwks.json`으로 공개키 발급
- **자정 기준 자동 키 로테이션 스케줄러**로 서명키 교체

---

## 🧩 CQRS (명령/조회 분리)

### 흐름

```
[쓰기 경로]                                       [읽기 경로]
Chat Command ──1개 트랜잭션──▶ DB(chat) + outbox_events
      │
      │ outbox publisher (100ms 폴링 · batch 50)
      │ 발행 실패 시 지수 백오프 재시도 (최대 30s)
      ▼
NATS JetStream (CHAT_EVENTS) ──▶ consumer (chat-projection) ──▶ projection 테이블
      │                                                            │
      └─▶ WebSocket hub (1:N 브로드캐스트)                         ▼
                                                       Redis 캐시 + cursor 페이지네이션
```

### Command (쓰기)

- 메시지 저장과 outbox 이벤트 삽입(`message_created`/`updated`/`deleted`)을 **하나의 DB 트랜잭션**으로 처리 → 커밋된 이벤트는 반드시 발행 대기열에 존재
- outbox publisher가 pending 이벤트를 폴링해 NATS로 발행 후 `MarkPublished`

### Query (읽기)

- NATS consumer(`chat-projection`, AckExplicit)가 projection 테이블 구축
- Redis 캐시(`chat:*`) + cursor 기반 페이지네이션으로 읽기 최적화

### WebSocket

- access token이 URL에 노출되지 않도록 **단회용 WS ticket**(TTL 30s)을 DPoP 인증으로 발급(`/ws-ticket`), 업그레이드 시 Lua로 원자적 소비
- heartbeat(ping/pong) + 지수 백오프 재연결 지원

---

## 🛡️ 신뢰성 & 장애 대응

### 1. 토큰 탈취 / 재사용 대응 (Auth)

- refresh token은 **1회용** — 재발급마다 회전(rotation)
- Redis Lua로 **원자적 소비** + `lineage ID` 추적(`rt:lineage:*`)
- **재사용 감지 시 해당 lineage 전체 폐기**(`RevokeLineage`) → 탈취 토큰 즉시 무력화

### 2. 요청 재생 방지 (DPoP)

- nonce 단회용 + jti 중복 검사 모두 **Redis Lua 원자 연산**으로 실행 → 재생 요청 원천 차단

### 3. 액세스 토큰 URL 노출 방지

- WS 업그레이드에 30초 TTL의 **단회용 티켓**만 사용, 소진/만료 시 재발급 필요

### 4. 이벤트 유실 방지 (Outbox + JetStream)

- **transactional outbox**: DB 커밋과 이벤트 기록이 원자적 → 발행 전 유실 불가
- 발행 실패 시 **지수 백오프 재시도**(최대 30s) + retry count DB 영속화
- JetStream **FileStorage** 기반 지속 저장 → 서비스 재시작에도 이벤트 보존 (at-least-once)

### 5. 자동 복구 스케줄러

- **JWK 키 로테이션** (auth, 자정) — 서명키 주기 교체
- **pop 스케줄러** (room, 1분 주기) — `pop_at` 도달 시 방을 read-only로 전환

### 6. 배포 안정성 (k3s)

- `update.sh`: 이미지 rebuild → k3s import → apply → **rollout restart** → digest 검증
- 서비스별 health check + rollout 대기로 무중단 배포

---

## 🛠️ 기술 스택

**Backend (핵심)**

- Go 1.26+ / Echo v5
- golang-jwt · lestrrat-go/jwx (JWK/DPoP)
- gorilla/websocket
- NATS JetStream
- go-redis (Lua 스크립트)
- PostgreSQL (sqlx)
- Swagger (서비스별 `/docs`)

**Frontend (최소)**

- Next.js 16 · React 19 · TypeScript
- WebSocket 클라이언트

---

## 📚 API 문서

각 서비스는 **Swagger UI**를 `/docs`로 제공하며, gateway의 `/auth`, `/rooms`, `/chat` 경로를 통해 접근할 수 있다.

---

## ☸️ 배포 (k3s)

라이브 서비스: **https://chinwag.duckdns.org**

k3s 매니페스트 기반 배포(Helm 불필요). 모든 배포 산출물은 [`infra/k3s`](infra/k3s)에 있다.

| 산출물 | 역할 |
|:---|:---|
| [`backend/Dockerfile`](backend/Dockerfile) | Go 서비스 공용 멀티스테이지 빌더 (`--build-arg SERVICE=...`) |
| [`infra/k3s/*.yaml`](infra/k3s) | 네임스페이스·ConfigMap·Secret·Postgres/Redis/NATS·서비스·Traefik Ingress |
| [`infra/k3s/build-images.sh`](infra/k3s/build-images.sh) | 이미지 빌드 + k3s 로컬 import |
| [`infra/k3s/deploy.sh`](infra/k3s/deploy.sh) | 원클릭 배포 (빌드→import→apply→rollout 대기→health 검증) |
| [`infra/k3s/update.sh`](infra/k3s/update.sh) | 재배포 (rebuild→import→apply→rollout restart→digest 검증) |

배포 구성 요점:

- **PostgreSQL** StatefulSet 1대로 4개 스키마를 모두 서빙 (`chinwag_auth`, `chinwag_room`, `chinwag_chat`, `chinwag_chat_projection`)
- **Redis**(AOF)와 **NATS JetStream** 병행 배포, `CHAT_EVENTS` 스트림은 chat-command가 자동 생성
- 단일 **Traefik Ingress**: `/auth` `/rooms` `/users` `/chat` → gateway(WebSocket 포함), `/` → frontend
- **HTTPS**: Let's Encrypt 인증서를 duckdns **DNS-01 webhook**으로 발급 (HTTP-01 불가 환경 대응)
- 시크릿(DB/Redis 비밀번호, 선택적 Google OAuth)은 `infra/k3s/secret.yaml`에서 관리, 공개 배포 전 반드시 교체

---

## 📁 디렉토리 구조

```
backend/
├── gateway/                 # API gateway (역방향 프록시)
├── go.work                  # Go workspace
├── shared/auth/             # 공용 인증: DPoP 검증기, JWT/JWKS 미들웨어
└── services/
    ├── auth/                # 사용자 인증 (JWT/DPoP/JWKS/OAuth/refresh 회전)
    ├── room/                # 방/멤버/초대, pop 스케줄러
    └── chat/
        ├── command/         # 메시지 쓰기 + WebSocket 허브 (CQRS command)
        └── query/           # 메시지 읽기 + projection (CQRS query)

frontend/                    # Next.js 웹 클라이언트 (최소 구성)
infra/k3s/                   # k3s 배포 매니페스트 + 스크립트
```

---

## 📄 라이선스

[Apache License 2.0](LICENSE)
