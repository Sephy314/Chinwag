# syntax=docker/dockerfile:1
# =============================================================================
# Chinwag Notifier — Alertmanager -> Discord webhook notification service.
#
# Single Go module (only joho/godotenv for local .env loading), built into a
# minimal Alpine runtime image, mirroring the backend/Dockerfile style.
#
#   docker build -t chinwag/notifier:latest -f infra/k3s/observability/notifier/Dockerfile .
#   (run from the notifier directory:  cd infra/k3s/observability/notifier && docker build -t chinwag/notifier:latest .)
# =============================================================================

ARG GO_VERSION=1.26.5

# ---------------------------------------------------------------------------
# Build stage
# ---------------------------------------------------------------------------
FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

# Copy module files first so dependency resolution is cached across rebuilds.
# (.env / .env.example are excluded via .dockerignore.)
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/notifier .

# ---------------------------------------------------------------------------
# Runtime stage
# ---------------------------------------------------------------------------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=build /out/notifier /app/notifier

# Non-root: the notifier is stateless and never writes to the filesystem.
USER 65532:65532

# Default HTTP port (overridable via NOTIFIER_PORT at runtime).
EXPOSE 9095

ENTRYPOINT ["/app/notifier"]
