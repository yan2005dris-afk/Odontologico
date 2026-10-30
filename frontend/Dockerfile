# syntax=docker/dockerfile:1

# ---------- dependencies ----------
FROM node:22-alpine AS deps

WORKDIR /app

# The portal is installed with pnpm and a frozen lockfile (wi-14 S1): the
# image is reproducible and `pnpm install` cannot silently resolve a newer
# version than the one the tests ran against.
ENV PNPM_HOME=/pnpm
ENV PATH=$PNPM_HOME:$PATH
RUN corepack enable && corepack prepare pnpm@12.16.1 --activate

# `pnpm-workspace.yaml` carries the `allowBuilds` list that pnpm >= 12 needs to
# run the postinstall of esbuild and sharp; without it the install aborts with
# ERR_PNPM_IGNORED_BUILDS.
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

# ---------- build ----------
FROM node:22-alpine AS builder

WORKDIR /app

ENV NEXT_TELEMETRY_DISABLED=1
ENV PNPM_HOME=/pnpm
ENV PATH=$PNPM_HOME:$PATH
RUN corepack enable && corepack prepare pnpm@12.16.1 --activate

COPY --from=deps /app/node_modules ./node_modules
COPY . .

# NEXT_PUBLIC_* values are inlined into the client bundle at build time, so they
# arrive as build args (wired from the same variables in deploy/compose.yaml)
# instead of runtime environment variables.
ARG NEXT_PUBLIC_API_GATEWAY_URL=https://localhost:8443
ARG NEXT_PUBLIC_IDLE_TIMEOUT_MS=900000
# Renders the development role picker on the login screen (wi-18 S3). It ships
# the seeded credentials in the browser bundle, so the default is OFF and only
# the development stack turns it on.
ARG NEXT_PUBLIC_DEV_ROLE_LOGIN=false
ENV NEXT_PUBLIC_API_GATEWAY_URL=${NEXT_PUBLIC_API_GATEWAY_URL}
ENV NEXT_PUBLIC_IDLE_TIMEOUT_MS=${NEXT_PUBLIC_IDLE_TIMEOUT_MS}
ENV NEXT_PUBLIC_DEV_ROLE_LOGIN=${NEXT_PUBLIC_DEV_ROLE_LOGIN}

RUN pnpm build

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
