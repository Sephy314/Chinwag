# syntax=docker/dockerfile:1
# =============================================================================
# Chinwag frontend (Next.js) image
#
# Build context is the `frontend/` directory.
#
# NEXT_PUBLIC_WS_URL is inlined into the client bundle at BUILD time, so it must
# be passed as a build argument matching the externally reachable WebSocket
# origin (the k3s Ingress host), e.g.:
#
#   docker build \
#     --build-arg NEXT_PUBLIC_WS_URL=ws://chinwag.local \
#     -t chinwag/frontend:latest .
#
# Requires `output: "standalone"` in next.config.ts (already set).
# =============================================================================

# ---------------------------------------------------------------------------
# deps: install node_modules
# Uses npm + package-lock.json (the in-sync lockfile in this repo).
# ---------------------------------------------------------------------------
FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

# ---------------------------------------------------------------------------
# build: compile the Next.js standalone server
# ---------------------------------------------------------------------------
FROM node:22-alpine AS build
WORKDIR /app
ARG NEXT_PUBLIC_WS_URL=ws://localhost:8000
ENV NEXT_PUBLIC_WS_URL=${NEXT_PUBLIC_WS_URL}
ENV NEXT_TELEMETRY_DISABLED=1
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

# ---------------------------------------------------------------------------
# runtime: minimal standalone server image
# ---------------------------------------------------------------------------
FROM node:22-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static
COPY --from=build /app/public ./public

EXPOSE 3000
USER node
CMD ["node", "server.js"]
