# Karecik — one image that serves both halves of the product.
#
# The Go binary can serve the built React bundle itself (SERVE_STATIC=true), so
# the whole thing runs as ONE container instead of a web service plus a static
# service. On Railway that is the difference between paying for two services and
# paying for one, and it also removes cross-origin traffic between them: the API
# and the menu answer on the same host, so no CORS entry is needed in production.
#
# Three stages, so the shipped image carries neither Node nor the Go toolchain.

# ---------------------------------------------------------------- frontend
FROM node:20-alpine AS frontend
WORKDIR /app/frontend

# The lockfile is copied on its own first so this layer is reused whenever the
# dependencies have not changed — which is nearly every deploy.
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

# ----------------------------------------------------------------- backend
FROM golang:1.22-alpine AS backend
WORKDIR /app/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./

# CGO off gives a static binary that runs on a bare Alpine layer.
# -trimpath drops local paths from the binary; -s -w drop the debug tables.
# The SQL migrations are compiled in through //go:embed, so nothing else has to
# be copied for them.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/karecik ./cmd/api

# ----------------------------------------------------------------- runtime
FROM alpine:3.20
WORKDIR /app

# ca-certificates: the seeded brand assets and any outbound HTTPS need a trust
# store. tzdata: the menu footer prints a local date, which needs a real zone.
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 karecik \
    && mkdir -p /data/uploads \
    && chown -R karecik:karecik /data

COPY --from=backend /out/karecik /app/karecik
COPY --from=frontend /app/frontend/dist /app/static

USER karecik

# Defaults that are only right INSIDE a container. Every one of them can still
# be overridden from the Railway dashboard.
#
#   HOST 0.0.0.0  the local default is 127.0.0.1, which keeps Windows Firewall
#                 quiet during development. In a container that would make the
#                 app unreachable from the platform's router and every health
#                 check would fail — this is the single most important line here.
#   UPLOAD_DIR    a mounted volume, NOT the image. See docs/DEPLOY.md: the
#                 container filesystem is replaced on every deploy, so uploaded
#                 logos and product photos must live outside it.
ENV HOST=0.0.0.0 \
    PORT=8080 \
    APP_ENV=production \
    SERVE_STATIC=true \
    STATIC_DIR=/app/static \
    UPLOAD_DIR=/data/uploads \
    SEED_DEMO=false

EXPOSE 8080

# The platform sends SIGTERM on redeploy and the app already listens for it and
# drains, so the binary is PID 1 with no shell in between to swallow the signal.
ENTRYPOINT ["/app/karecik"]
