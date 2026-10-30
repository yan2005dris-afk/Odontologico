# syntax=docker/dockerfile:1

# ---------- dependencies ----------
FROM node:22-alpine AS deps

WORKDIR /app

# No lockfile is committed, so npm resolves the semver ranges in package.json.
# Once a lockfile lands, switch this to `npm ci` for reproducible builds.
COPY package.json ./
RUN npm install --no-audit --no-fund

# ---------- build ----------
FROM node:22-alpine AS builder

WORKDIR /app

ENV NEXT_TELEMETRY_DISABLED=1

COPY --from=deps /app/node_modules ./node_modules
COPY . .

# NEXT_PUBLIC_* values are inlined into the client bundle at build time, so they
# arrive as build args (wired from the same variables in deploy/compose.yaml)
# instead of runtime environment variables.
ARG NEXT_PUBLIC_API_GATEWAY_URL=https://localhost:8443
ARG NEXT_PUBLIC_IDLE_TIMEOUT_MS=900000
ENV NEXT_PUBLIC_API_GATEWAY_URL=${NEXT_PUBLIC_API_GATEWAY_URL}
ENV NEXT_PUBLIC_IDLE_TIMEOUT_MS=${NEXT_PUBLIC_IDLE_TIMEOUT_MS}

RUN npm run build

# ---------- runtime ----------
FROM node:22-alpine AS runner

WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

RUN addgroup -S sigo \
    && adduser -S -G sigo -h /app sigo

# `output: "standalone"` in next.config.ts emits a self-contained server bundle.
COPY --from=builder --chown=sigo:sigo /app/.next/standalone ./
COPY --from=builder --chown=sigo:sigo /app/.next/static ./.next/static

USER sigo

EXPOSE 3000

CMD ["node", "server.js"]
