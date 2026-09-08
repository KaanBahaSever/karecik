# Karecik — Architecture

## Overview

Karecik is a multi-tenant QR menu platform. Every user account corresponds to
one business, and every business gets its own subdomain.

The diagram below is the **development** layout — two processes, Vite on :5173
proxying to Go on :8080. In production there is only one: the Go binary serves
the built React bundle from `STATIC_DIR` as well as `/api`, so the page and the
endpoint it calls always share an origin. See [Production topology](#production-topology)
below and [DEPLOY](DEPLOY.md).

```
                          ┌──────────────────────────────┐
 Business owner           │  React (Vite) — :5173        │
 karecik.com/panel  ─────▶│  · Landing (no animation)    │
                          │  · Dashboard                 │
 Customer                 │  · Customer menu             │
 kahve.karecik.com ──────▶│                              │
                          └──────────────┬───────────────┘
                                         │ /api  /uploads
                                         ▼
                          ┌──────────────────────────────┐
                          │  Go + Fiber — :8080          │
                          │  · Cookie session auth       │
                          │  · Subdomain resolution      │
                          │  · CRUD + bulk pricing       │
                          │  · Image uploads             │
                          └──────────────┬───────────────┘
                                         │ pgx/v5 pool
                                         ▼
                          ┌──────────────────────────────┐
                          │  PostgreSQL — :5432          │
                          │  users · businesses          │
                          │  categories · products       │
                          │  price_update_logs           │
                          └──────────────────────────────┘
```

> **Language convention:** the code (identifiers, comments, documentation) is
> English. The product copy — interface strings, legal notices, demo data — is
> Turkish, because Karecik serves Turkish businesses.

---

## Production topology

One container, one process, one origin per host.

```
{tenant}.karecik.com  ┐
karecik.com           ┴──▶  Go binary  ──┬──▶  /api/*        handlers
                                         ├──▶  /uploads/*    mounted volume
                                         └──▶  everything else, from STATIC_DIR
                                                (the built React bundle;
                                                 unknown paths -> index.html)
                              │
                              └──▶ PostgreSQL (managed)
```

What follows from it, and why several decisions elsewhere look the way they do:

- **No cross-origin requests exist.** The panel, every customer menu and the
  landing page's demo iframe are all served by the same process that answers
  their `/api` calls. That is why the CORS allow-list no longer includes
  `*.karecik.com`, and why the session cookie needs neither `SameSite=None` nor
  a `Domain` attribute.
- **The bundle is baked into the image**, so frontend and API can never be at
  different versions, and any `VITE_*` value is fixed at build time.
- **Sessions are in the process's memory**, so a deploy signs everyone out and
  the service must run as a single instance.
- **Uploads are the only mutable state on disk** and live on a volume, because
  the container filesystem is replaced on every deploy.

[DEPLOY](DEPLOY.md) has the variables and the traps.

---

## Folder layout

```
karecik/
├── docs/
│   ├── SETUP.md                Setup guide, from nothing installed to running
│   ├── API.md                  API contract (endpoints, bodies, error codes)
│   ├── ARCHITECTURE.md         This file
│   └── FRONTEND-CONTRACT.md    Signatures of the shared frontend modules
│
├── backend/
│   ├── cmd/api/main.go         Entry point: config → db → migrations → server
│   ├── internal/
│   │   ├── config/             .env parsing and defaults
│   │   ├── database/           pgxpool connection, migration runner, dev fixtures
│   │   ├── models/             Data structures plus the public menu DTOs
│   │   ├── repository/         SQL layer (handlers never write SQL)
│   │   ├── handlers/           HTTP endpoints (one Handler struct, split by file)
│   │   ├── middleware/         Session guard, subdomain resolution
│   │   ├── router/             Route registration, CORS, static files
│   │   ├── session/            In-memory session store (no DB, no Redis)
│   │   └── utils/              Session tokens, bcrypt, slug, currency, themes
│   ├── migrations/             001_init.sql, 002_brand_color.sql + embed.go
│   └── uploads/                Uploaded logos and product images
│
└── frontend/
    └── src/
        ├── lib/                api.js, auth.jsx, format.js, subdomain.js
        ├── themes/             themes.js (6 themes), fonts.js (8 typefaces)
        ├── locales/            languages, allergens, customer menu copy
        ├── components/
        │   ├── ui/             Modal, ConfirmModal, Loading, EmptyState,
        │   │                   Toast, ImageUploader
        │   ├── landing/        Header, PhoneFrame, SignUpModal
        │   ├── dashboard/      CategoryRow, ProductRow, modals, LivePreview
        │   └── menu/           MenuContent, SplashScreen, ProductDetailModal, MenuFooter
        └── pages/
            ├── Landing.jsx · Login.jsx · SignUp.jsx
            ├── dashboard/  DashboardLayout · MenuEditor · Design · Settings · QrCode
            └── menu/        CustomerMenu.jsx
```

---

## Multi-tenancy

**One user = one business.** `businesses.user_id` carries a `UNIQUE` constraint.

The tenant scope **always** comes from the session, never from the request:

```go
businessID := middleware.BusinessID(c)   // recorded when the session was issued
```

The client never sends a `business_id`. Every repository query includes
`WHERE ... AND business_id = $n`, so reaching another business' record ends in
`404` / `403`. This closes the most common SaaS vulnerability (IDOR).

---

## Subdomain routing

### Backend

`middleware.ExtractSubdomain(host, appDomain, devDomain)`:

| Host | Result |
|---|---|
| `kahve-duragi.karecik.com` | `kahve-duragi` |
| `kahve-duragi.localhost:5173` | `kahve-duragi` |
| `www.karecik.com` | `""` (reserved) |
| `karecik.com`, `localhost` | `""` |
| `127.0.0.1:8080` | `""` (IP address) |

`GET /api/public/menu` resolves the slug with this function. Path-based access
(`GET /api/public/menu/:slug`) always works and serves as the fallback in QR links.

### Frontend

`src/lib/subdomain.js` → `getSubdomain()` applies the same logic in the browser.
When a subdomain is present, `App.jsx` routes **every** path to the customer menu:

```jsx
const subdomain = getSubdomain()
if (subdomain) return <Routes><Route path="*" element={<CustomerMenu slug={subdomain} />} /></Routes>
```

### DNS in production

```
*.karecik.com   A   <server-ip>
karecik.com     A   <server-ip>
```

A wildcard SSL certificate is required (Let's Encrypt DNS-01 challenge).

---

## Data model

```
users ──1:1──▶ businesses ──1:N──▶ categories ──1:N──▶ products
                    │
                    └──1:N──▶ price_update_logs
```

### Multilingual content: JSONB

Category and product texts do not live in separate tables but in a
`translations` JSONB column:

```json
{ "tr": { "name": "Türk Kahvesi", "description": "...", "ingredients": "..." },
  "en": { "name": "Turkish Coffee", "description": "..." } }
```

Why: adding a language needs no schema change, everything is read in one row,
and there is no JOIN cost. On read, `Translations.Resolve(lang, fallback)` is
called: requested language → default language → the first non-empty entry.

`allergens` is a JSONB array as well: `["gluten", "sut"]`.

### Ordering

`categories.position` and `products.position` are integers. After a drag and
drop the client sends the **whole list** in order and the backend writes it in
one statement:

```sql
UPDATE categories c
SET position = data.ord - 1
FROM unnest($2::uuid[]) WITH ORDINALITY AS data(id, ord)
WHERE c.id = data.id AND c.business_id = $1
```

For products the same statement also updates `category_id`, which is why moving
a product to another category and reordering it share a single endpoint.

---

## Bulk price update

`POST /api/products/bulk-price`

1. The selected products are read (`ListPriceRows`).
2. For each price: `new = old × (1 + percentage/100)` → `RoundPrice(new, mode)` → `max(0, …)`.
3. With `apply: false` only a preview is returned; the database is untouched.
4. With `apply: true` the changed prices are written in a single transaction,
   `businesses.price_updated_at` is set to `now()` and a row is added to
   `price_update_logs`.

Rounding modes (`internal/utils/pricing.go`):

| Mode | 147.60 → |
|---|---|
| `none` | 147.60 |
| `integer` | 148.00 |
| `nearest_5` | 150.00 |
| `nearest_10` | 150.00 |
| `ends_50` | 147.50 |
| `ends_95` | 147.95 |
| `ends_99` | 147.99 |

`price_updated_at` feeds the
**"Fiyatlarımız 24.08.2026 tarihinden itibaren geçerlidir."** line in the
customer menu — the owner never has to type a date by hand.

---

## Themes and typefaces

Single source of truth: `backend/internal/utils/appearance.go`. The frontend
mirrors the same ids in `src/themes/themes.js` and `src/themes/fonts.js`, and
they can be verified through `GET /api/meta`.

When rendering the menu, the theme is turned into **CSS custom properties**:

```js
const style = themeVariables(business.theme, business.primary_color, fontStack(business.font_family))
// { '--menu-bg': ..., '--menu-text': ..., '--menu-primary': ..., ... }
```

Menu components use no Tailwind colour classes; they read `var(--menu-*)`. That
is what lets all six themes work with a single component tree, and what lets the
**live preview** show unsaved changes immediately.

---

## How the live preview works

The `LivePreview` component:

1. Fetches the menu through `GET /api/preview/menu` (inactive records included).
2. Merges the returned `menu.business` with the **still unsaved** draft
   `business` object from the dashboard.
3. Passes the result to `MenuContent`, the very component the customer menu uses.

The preview is therefore not a mock-up — it is the real thing.

---

## Authentication

- Passwords are hashed with `bcrypt` (default cost).
- `POST /api/auth/register|login` → a 256-bit random token from `crypto/rand`,
  returned as an `HttpOnly` cookie. The response body carries no credential.
- The server keeps only `sha256(token)`, as the key of a `session.Entry` in the
  API process's own memory. Nothing readable back into a working cookie is
  stored anywhere — not in the database, not on disk.
- On any `401` the client falls back to `/giris`. It has nothing to clear:
  the cookie is invisible to JavaScript, and the server clears it itself.

Three properties follow from where the sessions live, and they are the reason
the design is written down rather than assumed:

| | |
|---|---|
| Revocation is immediate | Every request resolves the session, so removing an entry ends it at once. That is what makes logout and "revoke my other sessions" real rather than decorative — a signed JWT could not be withdrawn before it expired. |
| A restart signs everyone out | The store is not persisted. Accepted deliberately in exchange for taking the database off the authentication path. |
| The API must run as ONE instance | A second replica has its own map and does not recognise the first one's sessions. Scaling out requires moving sessions to shared storage first. |

`session.Entry` carries the **business id** as well as the user id. Without it
the middleware would have to ask the database which business the user owns on
every authenticated request — reinstating exactly the round trip the in-memory
store exists to remove. It is safe to cache because a business id never changes
and a user owns exactly one.

A `sessions` table still exists in the schema from the earlier design. Nothing
reads or writes it; migration `008` explains why it was left rather than
dropped.
- The slug is generated at sign-up (`Kahve Durağı` → `kahve-duragi`); on a
  collision `-2`, `-3` … is appended. Reserved names (`www`, `api`, `panel`,
  `admin`, …) are never handed out.

---

## Migrations

The `backend/migrations/*.sql` files are embedded into the binary with
`//go:embed` and applied in order when the server starts. Applied versions are
tracked in `schema_migrations`, and each file runs in its own transaction.

To add a migration: create `003_xxx.sql` — nothing else is required.

---

## Why these technologies

| Decision | Rationale |
|---|---|
| **Fiber** (over Gin) | `c.Hostname()` makes subdomain resolution direct; the fasthttp base is lightweight for the many short requests QR traffic produces. |
| **pgx/v5** (over GORM) | Uses PostgreSQL features such as JSONB, `unnest … WITH ORDINALITY` and `NUMERIC` directly, with no ORM layer; the generated SQL holds no surprises. |
| **JSONB translations** | Adding a language requires no schema migration. |
| **@dnd-kit** | react-beautiful-dnd is unmaintained and awkward under React 18 StrictMode; dnd-kit is accessible (keyboard support) and actively maintained. |
| **No animation on the landing page** | A product requirement: a plain, calm, static first screen. That is why no animation library is part of the dependency tree at all. |
