# syntax=docker/dockerfile:1

# ---------- build ----------
FROM golang:1.24-alpine AS builder

WORKDIR /src

# The module has no external dependencies today (stdlib only). Copying go.mod
# keeps the layer valid the moment the first dependency lands.
COPY go.mod ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w" \
    -o /out/gateway ./cmd/gateway

# ---------- runtime ----------
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S sigo \
    && adduser -S -G sigo -h /app sigo

WORKDIR /app

COPY --from=builder /out/gateway /app/gateway

# The gateway loads the OpenAPI contract at startup and exits if it is missing
# (OPENAPI_SPEC_PATH defaults to /app/openapi/sigo-dental.v1.yaml in the stack).
RUN mkdir -p /app/openapi && chown -R sigo:sigo /app

USER sigo

EXPOSE 8080 8443

ENTRYPOINT ["/app/gateway"]
