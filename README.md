<div align="center">

<img src="frontend/public/logo.svg" alt="Karecik" width="88" height="88">

# Karecik

**Multi-tenant QR menu platform for cafés and restaurants.**

[![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Fiber](https://img.shields.io/badge/Fiber-v2-00ACD7?logo=go&logoColor=white)](https://gofiber.io)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=black)](https://react.dev)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-13%2B-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org)
[![License](https://img.shields.io/badge/License-GPLv3-blue)](LICENSE)

</div>

---

## Capabilities

- **Menu editor** — categories and products with drag-and-drop ordering, inline price editing and bulk percentage updates with rounding rules.
- **Multi-branch, multi-menu** — several branches per business, menus shared across branches or scoped to one, with per-branch price and availability overrides.
- **Multilingual** — six languages per menu, stored as JSONB translations and resolved server-side.
- **Product detail** — images, ingredients, allergen warnings, calorie counts and custom badges (free text, icon, colours).
- **Branding** — 6 themes, 8 typefaces, accent colour, solid or image background with an overlay, and a configurable splash screen with exit animations.
- **Live preview** — a true 390×844 mobile viewport in the dashboard, with on-demand splash replay.
- **QR codes** — PNG download, printing and address copying.
- **Customer menu** — served from `business.karecik.com/menu-slug` or `/m/:business/:menu`, with search, language switching, Wi-Fi credentials and automatic legal notices.

> The codebase is English; the shipped product is Turkish, with menus publishable in English, German, Russian, Arabic and French.

## Architecture

| Layer | Stack |
|---|---|
| API | Go 1.22 · Fiber v2 · pgx v5 · in-memory sessions, HttpOnly cookie |
| Database | PostgreSQL 13+ · embedded SQL migrations, applied on boot |
| Frontend | React 18 · Vite 5 · Tailwind 3 · @dnd-kit |
| Tenancy | Wildcard subdomain resolution with a path-based fallback |

```text
backend/    cmd/{api,resetpw} · internal/{config,database,models,repository,handlers,middleware,router,session,utils}
frontend/   src/{lib,themes,locales,components,pages}
docs/       SETUP · API · ARCHITECTURE · FRONTEND-CONTRACT
```

## Quick start

**Prerequisites** — Go 1.22+, Node.js 18+, PostgreSQL 13+.

```bash
# 1. Database
psql -U postgres -c "CREATE DATABASE karecik;"

# 2. Backend  →  http://localhost:8080
cd backend
cp .env.example .env          # Windows: Copy-Item .env.example .env
go mod tidy
go run ./cmd/api              # migrations run automatically

# 3. Frontend →  http://localhost:5173
cd frontend
npm install
npm run dev
```

On Windows, `start.bat` does all of the above and `stop.bat` shuts it down.

There are no fixtures and no demo login. The database starts empty, and an
account is created by signing up. A password can be set without e-mail:

```bash
cd backend
go run ./cmd/resetpw -email owner@example.com
```

Sessions live in the API process's memory, so a restart signs everyone out and
the API has to run as a single instance — see [DEPLOY](docs/DEPLOY.md).

**Environment** (`backend/.env`)

| Variable | Default | Purpose |
|---|---|---|
| `DATABASE_URL` | `postgres://postgres@localhost:5432/karecik?sslmode=disable` | Connection string |
| `COOKIE_DOMAIN` | *(empty)* | Leave empty — the session cookie is host-only. See [DEPLOY](docs/DEPLOY.md) |
| `COOKIE_SAMESITE` | `Lax` | `Lax`, `None` or `Strict`; `None` requires `COOKIE_SECURE` |
| `COOKIE_SECURE` | on in production | HTTPS-only session cookie |
| `PORT` | `8080` | API listen port |
| `APP_DOMAIN` | `karecik.com` | Production root domain for menu subdomains |
| `DEV_DOMAIN` | `localhost` | Local root domain for subdomain testing |
| `CORS_ORIGINS` | `http://localhost:5173` | Comma-separated allowed origins |
| `UPLOAD_DIR` | `./uploads` | Where uploaded images are written |
| `MAX_UPLOAD_BYTES` | `5242880` | Largest accepted upload |
| `SERVE_STATIC` | `false` | Serve `frontend/dist` from the API |
| `APP_ENV` | `development` | `development` or `production` |

Deeper reference: [SETUP](docs/SETUP.md) · [API](docs/API.md) · [ARCHITECTURE](docs/ARCHITECTURE.md) · [FRONTEND-CONTRACT](docs/FRONTEND-CONTRACT.md)

---

<div align="center">
<sub>Licensed under the <a href="LICENSE">GNU General Public License v3</a>.</sub>
</div>
