# Multi-stage: сборка SPA (Vite) → раздача статики через Caddy (reverse-proxy /api → api:8080).
# Caddyfile монтируется в compose (deploy/Caddyfile). Story 1.7 walking skeleton.
FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM caddy:2-alpine
COPY --from=build /app/dist /srv
