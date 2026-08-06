# syntax=docker/dockerfile:1
# =============================================================================
# Chinwag backend image builder
#
# Single parameterized Dockerfile for every Go service in the backend workspace.
# Build context MUST be the `backend/` directory (contains go.work) so that the
# Go workspace and the `replace` directives for shared/auth resolve correctly.
#
#   docker build --build-arg SERVICE=gateway            -t chinwag/gateway:latest .
#   docker build --build-arg SERVICE=services/auth      -t chinwag/auth:latest .
#   docker build --build-arg SERVICE=services/room      -t chinwag/room:latest .
#   docker build --build-arg SERVICE=services/chat/command -t chinwag/chat-command:latest .
#   docker build --build-arg SERVICE=services/chat/query -t chinwag/chat-query:latest .
#
# (run from the backend/ directory, e.g. `docker build -f Dockerfile .`)
# =============================================================================

ARG GO_VERSION=1.26.5
ARG SERVICE=gateway

# ---------------------------------------------------------------------------
# Build stage
# ---------------------------------------------------------------------------
FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

# Copy go.mod/go.sum first so dependency resolution is cached across rebuilds.
COPY go.work go.work.sum ./
COPY gateway/go.mod gateway/go.sum ./gateway/
COPY shared/auth/go.mod shared/auth/go.sum ./shared/auth/
COPY services/auth/go.mod services/auth/go.sum ./services/auth/
COPY services/room/go.mod services/room/go.sum ./services/room/
COPY services/chat/command/go.mod services/chat/command/go.sum ./services/chat/command/
COPY services/chat/query/go.mod services/chat/query/go.sum ./services/chat/query/

RUN go mod download

# Copy the full workspace so the go.work and replace directives resolve.
COPY gateway/ ./gateway/
COPY shared/ ./shared/
COPY services/ ./services/

ARG SERVICE
RUN cd "${SERVICE}" \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app .

# ---------------------------------------------------------------------------
# Runtime stage
# ---------------------------------------------------------------------------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=build /out/app /app/app

# Non-root: services never write to the local filesystem (state lives in
# Postgres/Redis/NATS), so the unprivileged user is safe here.
USER 65532:65532

# Services bind to a port chosen at runtime via env (8000/8081/8082/8083/8084);
# 8080 is only a default hint for container metadata.
EXPOSE 8080

ENTRYPOINT ["/app/app"]
