# syntax=docker/dockerfile:1
# =============================================================================
# Chinwag frontend (Next.js) image
#
# Build context is the `frontend/` directory.
#
# The WebSocket base is derived at RUNTIME from the browser origin
# (src/services/websocket-client.ts: http->ws, https->wss, same host), so no
# build-time WS argument is needed.
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
