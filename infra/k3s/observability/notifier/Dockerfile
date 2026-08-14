# syntax=docker/dockerfile:1
# =============================================================================
# Chinwag Notifier — Alertmanager -> Discord webhook notification service.
#
# Self-contained Go binary (stdlib only, no external Go modules), built into a
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

# The notifier has no external Go modules, so there is no separate go.sum
# dependency-download step — just copy the module and build.
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
