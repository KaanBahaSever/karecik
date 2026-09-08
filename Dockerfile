# Karecik API — the Go service on its own.
#
# The frontend used to be built into this image so one container could serve
# both halves. It is not any more: the SPA is deployed to Cloudflare Pages, and
# this image carries only the API. That is why there is no Node stage here.
#
# What changed with it, and matters more than the missing stage:
#
#   - The two halves are on different ORIGINS now, so the session cookie has to
#     be sent and stored across them. That needs AllowCredentials on the server,
#     credentials:'include' on the client, and cookie settings that match where
#     the two are actually deployed — see COOKIE_* below and docs/DEPLOY.md.
#   - SERVE_STATIC is gone. Nothing in this image serves HTML.

# ----------------------------------------------------------------- builder
FROM golang:1.22-alpine AS builder
WORKDIR /app

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./

# CGO off gives a static binary that runs on a bare Alpine layer.
# -trimpath drops local paths from it; -s -w drop the debug tables.
# The SQL migrations are compiled in through //go:embed, so nothing else has to
# be copied for them.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/karecik ./cmd/api

# ----------------------------------------------------------------- runtime
FROM alpine:3.20
WORKDIR /app

# ca-certificates: outbound HTTPS needs a trust store.
# tzdata: the menu footer prints a local date, which needs a real zone.
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 karecik \
    && mkdir -p /data/uploads \
    && chown -R karecik:karecik /data

COPY --from=builder /out/karecik /app/karecik

USER karecik

# Defaults that are only right INSIDE a container. Every one is overridable from
# the platform dashboard.
#
#   HOST 0.0.0.0   the local default is 127.0.0.1, which keeps Windows Firewall
#                  quiet during development. In a container that would make the
#                  app unreachable from the platform's router and every health
#                  check would fail — the single most important line here.
#   COOKIE_SECURE  the session cookie crosses the public internet; without this
#                  it would travel in the clear and SameSite=None would be
#                  refused outright by the browser.
#   COOKIE_DOMAIN  left EMPTY on purpose. Set it to ".karecik.com" only when the
#                  API answers on that domain too (api.karecik.com); a Domain
#                  the response host does not belong to is rejected silently and
#                  every login appears to succeed while storing nothing.
#   UPLOAD_DIR     a mounted volume, NOT the image: the container filesystem is
#                  replaced on every deploy.
ENV HOST=0.0.0.0 \
    PORT=8080 \
    APP_ENV=production \
    UPLOAD_DIR=/data/uploads \
    COOKIE_SECURE=true \
    COOKIE_SAMESITE=Lax \
    COOKIE_DOMAIN=""

EXPOSE 8080

# The platform sends SIGTERM on redeploy and the app already drains on it, so
# the binary is PID 1 with no shell in between to swallow the signal.
ENTRYPOINT ["/app/karecik"]
