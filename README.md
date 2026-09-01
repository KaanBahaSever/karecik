# Karecik

**A QR menu service for your business.** A multi-tenant QR menu platform for
cafés and restaurants: a business signs up, builds its menu with drag and drop,
and customers open the menu at `businessname.karecik.com`.

```
Go + Fiber   ·   PostgreSQL   ·   React + Vite + Tailwind   ·   @dnd-kit
```

> **Language note:** the codebase — identifiers, comments and documentation — is
> English. The product itself ships in **Turkish**, because Karecik serves
> Turkish businesses: the interface copy, the legal notices ("Fiyatlarımıza KDV
> dahildir."), the default currency (₺) and the demo menu are all Turkish by
> design. Customer menus can additionally be published in English, German,
> Russian, Arabic and French.

---

## Quick start

### The easy way (Windows)

Once the tooling is installed, **double-click** the file in the project folder:

| File | What it does |
|---|---|
| **`start.bat`** | Checks Go / Node.js / PostgreSQL, installs missing packages, starts both servers in their own windows and opens the browser once they are ready |
| **`stop.bat`** | Stops the servers (only ports 8080 and 5173 — other applications are untouched) |

On the first run `start.bat` creates `.env` if it is missing and opens it in
Notepad; you only need to fill in your PostgreSQL password on the
`DATABASE_URL` line.

### Running it by hand

> If Go, Node.js or PostgreSQL are not installed yet, follow
> **[docs/SETUP.md](docs/SETUP.md)** from the top — it lists the downloads, the
> installation steps and the connection settings one by one.

```powershell
# 1. Create the database (once)
psql -U postgres -c "CREATE DATABASE karecik;"

# 2. Backend (terminal 1)
cd backend
Copy-Item .env.example .env      # set your password in DATABASE_URL
go mod tidy
go run ./cmd/api                 # http://localhost:8080

# 3. Frontend (terminal 2)
cd frontend
npm install
npm run dev                      # http://localhost:5173
```

The tables are created automatically when the server starts; there is no
separate migration command to run.

**Demo account:** `demo@karecik.com` / `demo1234` — sample café menu:
[localhost:5173/m/demo-kafe](http://localhost:5173/m/demo-kafe)

---

## Features

### Landing page
- A calm, clean and **completely animation-free** first screen (a deliberate
  design decision — no animation library is part of the dependency tree).
- On the right, a **real** sample café menu running inside an `<iframe>` in an
  iPhone frame.
- Sign-up dialog: business name · email · password.
- Language picker (TR / EN / DE) for the landing copy.

### Dashboard
- Create, edit and delete categories and products.
- **Drag and drop** ordering for both categories and products (@dnd-kit, with
  keyboard support).
- Inline **quick price editing**.
- Product details: image, description, ingredients, **allergen warnings**,
  featured flag.
- **Per-language product entry** — Turkish, English, German, Russian, Arabic, French.
- **Currency** selection: ₺ by default, plus $ € £ ₼ ₽ ﷼ د.إ.
- **Bulk price update**: a percentage increase or discount in one click, with
  price rounding (integer, multiples of 5 or 10, .50 · .95 · .99).
- **Design**: 6 themes, 8 typefaces, a custom accent colour.
- **Live preview**: every change appears instantly on a device mock-up.
- **QR code**: PNG download, printing, address copying.

### Customer menu
- `businessname.karecik.com` — wildcard subdomain, the menu is read live from
  PostgreSQL.
- A short splash screen that the business **can switch on and customise**
  (logo, background colour, duration, text).
- Logo in the top left, category cards underneath.
- Product detail sheet, search, language switching.
- **Automatic legal notices**:
  "Fiyatlarımız 24.08.2026 tarihinden itibaren geçerlidir." ·
  "Fiyatlarımıza KDV dahildir."
  (The date refreshes itself after every bulk price update.)

---

## Documentation

| File | Contents |
|---|---|
| [docs/SETUP.md](docs/SETUP.md) | Installing Go, Node.js and PostgreSQL; `.env` and the connection string; testing subdomains locally |
| [docs/API.md](docs/API.md) | Every API endpoint, request/response body and error code |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Folder layout, multi-tenancy, subdomain resolution, data model, technology decisions |
| [docs/FRONTEND-CONTRACT.md](docs/FRONTEND-CONTRACT.md) | Signatures of the shared frontend modules |

---

## Project layout

```
backend/     Go + Fiber API      → cmd/api, internal/{config,database,models,
                                     repository,handlers,middleware,router,utils}
frontend/    React + Vite        → src/{lib,themes,locales,components,pages}
docs/        Documentation
```

---

## Deploying

```powershell
cd frontend && npm run build          # frontend/dist
cd ../backend && go build -o karecik.exe ./cmd/api
```

With `SERVE_STATIC=true` in `.env` the backend also serves the built frontend,
so a single executable is enough. You need a `*.karecik.com` wildcard A record
in DNS and a wildcard SSL certificate. Details: [docs/SETUP.md](docs/SETUP.md) §10.
