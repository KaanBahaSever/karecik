# Karecik — one container: the Go API and the React bundle it serves.
#
# The frontend briefly lived on Cloudflare Pages with the API alone in this
# image. That is reverted: the SPA is built here again and the Go binary serves
# it, so there is one origin, one deploy and one thing to keep alive.
#
# WHAT THE LAYOUT HAS TO SATISFY
#
# The server serves the bundle only when SERVE_STATIC is on, and it looks for
# index.html under STATIC_DIR. A RELATIVE STATIC_DIR resolves against the
# process's working directory, which is WORKDIR below. So with WORKDIR /app the
# bundle has to end up at:
#
#     /app/frontend/dist/index.html
#
# and then every plausible setting finds it — "frontend/dist", "./frontend/dist"
# and "/app/frontend/dist" all land on the same file. (The local default,
# "../frontend/dist", is the odd one out: it is right when you run the binary
# from backend/ during development and wrong in here, which is precisely the
# mismatch behind "sendfile: file frontend/dist/index.html not found".)
#
# Both stage outputs are checked before the image is finished. A container that
# starts and then cannot find its own HTML is a bad way to learn about a typo in
# a COPY path.

# --------------------------------------------------------- 1. frontend build
FROM node:20-alpine AS frontend
WORKDIR /src/frontend

# The manifest and the lockfile ALONE first. This layer is what npm ci depends
# on, so editing a component re-uses the cached install instead of downloading
# the dependency tree again. Copying the whole folder here would invalidate the
# install on every source edit — the single most expensive mistake in a
# frontend Dockerfile.
COPY frontend/package.json frontend/package-lock.json ./

# ci, not install: it installs exactly what the lockfile pins and fails instead
# of quietly resolving something newer, so the image matches what was tested.
# The lockfile is version 3 and carries every platform's optional binaries, so
# the musl builds of rollup and esbuild that Alpine needs are in there.
RUN npm ci

COPY frontend/ ./

# Vite substitutes these AT BUILD TIME — they are baked into the JavaScript, not
# read at runtime. Changing one means a rebuild, not a restart.
#
# READ THIS BEFORE ADDING AN ARG HERE. A Dockerfile build does not normally see
# the platform's service variables, but Railway matches a declared ARG by name
# and passes the matching variable in. So every ARG on this list is a switch the
# dashboard can throw, and a variable left over from an old deployment shape gets
# baked into the bundle with a green build and a green health check.
#
# VITE_API_URL is therefore NOT an ARG. It has exactly one correct value here —
# empty, meaning "call /api on whatever origin served this page" — and a stale
# dashboard entry pointing at a separate API host would break the panel (the
# session cookie is not sent cross-origin, so every request 401s) and the customer
# menus (the tenant is resolved from the request Host). Hard-coding it as a plain
# ENV means the dashboard cannot reach it at all.
ENV VITE_API_URL=""

# These two are genuinely per-deployment, so they stay overridable.
#   VITE_APP_DOMAIN     the root domain tenant menu addresses are built from.
#   VITE_DEMO_BUSINESS  the tenant shown in the phone frame on the landing page;
#                       empty renders a neutral placeholder instead, which is
#                       right until a real tenant with that slug exists.
# VITE_ROOT_DOMAIN is deliberately absent: it is a local-development knob, and
# baking a real domain into it would create a second, competing root domain for
# tenant resolution.
ARG VITE_APP_DOMAIN="karecik.com"
ARG VITE_DEMO_BUSINESS=""
ENV VITE_APP_DOMAIN=$VITE_APP_DOMAIN \
    VITE_DEMO_BUSINESS=$VITE_DEMO_BUSINESS

RUN npm run build

# Fail here rather than ship a container that boots and serves nothing. This is
# the exact failure this file was rewritten to fix, so it is worth one line to
# make it impossible to reach production again.
RUN test -f dist/index.html \
    || (echo "FATAL: the frontend build produced no dist/index.html" >&2; exit 1)

# --------------------------------------------------------------- 2. Go build
FROM golang:1.22-alpine AS builder
WORKDIR /src/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./

# CGO off gives a static binary that runs on a bare Alpine layer.
# -trimpath drops local paths from it; -s -w drop the debug tables.
# The SQL migrations are compiled in through //go:embed, so nothing else has to
# be copied for them.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/karecik ./cmd/api

# ----------------------------------------------------------------- 3. runtime
FROM alpine:3.20
WORKDIR /app

# ca-certificates: outbound HTTPS needs a trust store (the Postgres connection
# uses TLS). tzdata is NOT installed: nothing on the Go side loads a time zone —
# the menu's date is formatted in the browser, which uses the visitor's own.
RUN apk add --no-cache ca-certificates \
    && adduser -D -u 10001 karecik \
    && mkdir -p /data/uploads \
    && chown -R karecik:karecik /data

# Defaults that are only right INSIDE a container. Every one is overridable from
# the platform dashboard — and a dashboard value WINS over what is set here,
# which is worth remembering when one of these appears not to take effect.
#
#   HOST 0.0.0.0   the local default is 127.0.0.1, which keeps Windows Firewall
#                  quiet during development. In a container that would make the
#                  app unreachable from the platform's router and every health
#                  check would fail — the single most important line here.
#   SERVE_STATIC   on: this image contains the bundle, so the binary serves it.
#   STATIC_DIR     absolute, so it holds whatever the working directory is. The
#                  guard below asserts against this same variable rather than a
#                  copy of the path, so the two cannot drift apart.
#   COOKIE_SECURE  the session cookie crosses the public internet; without this
#                  it would travel in the clear.
#   COOKIE_SAMESITE Lax is right now that the page and the API share an origin.
#                  None is for genuinely cross-site setups and needs Secure.
#   COOKIE_DOMAIN  left EMPTY on purpose, which makes the cookie host-only. The
#                  panel is on karecik.com; setting ".karecik.com" would also
#                  send the session to every tenant's {slug}.karecik.com, which
#                  is a wider reach than the login needs.
#   UPLOAD_DIR     a mounted volume, NOT the image: the container filesystem is
#                  replaced on every deploy. NOTE: Railway mounts volumes as
#                  root, so a non-root image needs RAILWAY_RUN_UID=0 set on the
#                  service or the app cannot write here — see docs/DEPLOY.md.
ENV HOST=0.0.0.0 \
    PORT=8080 \
    APP_ENV=production \
    UPLOAD_DIR=/data/uploads \
    SERVE_STATIC=true \
    STATIC_DIR=/app/frontend/dist \
    COOKIE_SECURE=true \
    COOKIE_SAMESITE=Lax \
    COOKIE_DOMAIN=""

COPY --from=builder /out/karecik /app/karecik

# The bundle lands beside the binary, under WORKDIR, so that a relative
# STATIC_DIR resolves too. It stays ROOT-owned and world-readable on purpose:
# the app only ever reads it, and a process that cannot overwrite its own HTML
# is one less thing to worry about. Uploads are the only writable path, and they
# live on the volume at /data.
COPY --from=frontend /src/frontend/dist /app/frontend/dist

# Asserted against $STATIC_DIR, not against a repeated literal — this is the
# exact value the server will look in, so the check cannot pass while the app
# still fails. Neither Node nor the Go toolchain reaches this layer, only the
# static binary and the built assets.
RUN test -f "$STATIC_DIR/index.html" \
    || (echo "FATAL: $STATIC_DIR/index.html is missing — check the COPY paths and STATIC_DIR" >&2; exit 1)

USER karecik

EXPOSE 8080

# The platform sends SIGTERM on redeploy and the app already drains on it, so
# the binary is PID 1 with no shell in between to swallow the signal.
ENTRYPOINT ["/app/karecik"]
