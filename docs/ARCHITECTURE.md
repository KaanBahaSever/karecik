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
  their `/api` calls. That is why the CORS allow-list does not include
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
one transaction: a `SELECT ... ORDER BY id FOR NO KEY UPDATE` locks the listed
rows of the business, and then this statement writes them:

```sql
UPDATE categories c
SET position = data.ord - 1
FROM unnest($2::uuid[]) WITH ORDINALITY AS data(id, ord)
WHERE c.id = data.id AND c.business_id = $1
```

For products the lock is `FOR UPDATE` and the statement also updates
`category_id`, which is why moving a product to another category and reordering
it share a single endpoint; that transaction first takes the advisory locks of
the target category and its menu. Both lock every listed row, in ascending id
order, before they write the first one — see [Lock order](#lock-order).

---

## Bulk price update

`POST /api/products/bulk-price`

1. For each price: `new = old × (1 + percentage/100)` → `RoundPrice(new, mode)`
   → `max(0, …)` (`PlanPriceChanges`).
2. With `apply: false` the selected products of one menu (`menu_id`) are read
   without locking them (`ListPriceRows`) and only a preview is returned; the
   database is untouched.
3. With `apply: true` nothing read for a preview is reused. `ApplyPrices` runs
   one transaction: the preview's `SELECT` names the selected products, a
   `SELECT ... WHERE id = ANY(...) ORDER BY id FOR UPDATE` locks exactly those
   rows, the preview's `SELECT` reads them again once every one of them is
   locked, the new prices are computed in Go from exactly those rows, and one
   `UPDATE` writes the rows whose price changes. The response's `preview` and
   `affected` describe what that transaction read and wrote, so a price edited
   while the apply waited for its locks is the price the percentage is applied
   to, and a product moved to another category of the same menu meanwhile is
   raised like every other.
4. A preview and an apply alike refuse the whole update with a `422`, and write
   nothing, when a new price would be larger than `9999999999.99` — the largest
   value `products.price`, a `NUMERIC(12,2)`, holds (`utils.MaxPrice`).
5. After an apply the menu's `menus.price_updated_at` is read back into the
   response, and a row is added to `price_update_logs`. The handler does not
   write the date itself — see below.

The preview and the apply both list the products in menu editor order —
category position, then product position — and count as `affected` only the
products whose price changes.

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

### The price date

`menus.price_updated_at` feeds the
**"Fiyatlarımız 24.08.2026 tarihinden itibaren geçerlidir."** line in the
customer menu — the owner never has to type a date by hand.

No handler writes it. The `products_touch_menu_price_date` trigger (migration
`010_price_change_date.sql`) moves it to `now()` after every `UPDATE` of a
product that is a **price change** — one where at least one of these differs
(`IS DISTINCT FROM`) between the old and the new row:

- `price`
- `compare_price`
- the ordered list of option surcharges,
  `jsonb_path_query_array(options, '$[*].items[*].price')`

The menu it moves is the one the product's **new** category belongs to, so a
product moved into another menu with a new price dates that menu. Creating or
deleting a product, reordering, moving a product without a new price, toggling
`is_active` / `is_featured`, and editing translations, allergens, badges, the
image, calories or an option's name are not price changes. Surcharges compare
as jsonb, which compares numbers numerically: `10` and `10.0` are the same.

The product dialog (`PUT /api/products/:id`, which sends every field on every
save), the inline quick edit (`PATCH /api/products/:id/price`) and the bulk
update therefore all follow one rule, and a bulk apply that changes no price
leaves the date alone.

Two details keep the trigger cheap and its lock order predictable:

- `now()` is constant inside a transaction, and the function only writes a menu
  whose date is not already `now()`. An N-row bulk `UPDATE` writes the menu row
  once, not N times.
- `ApplyPrices` writes with one `UPDATE`, not a loop of single-row updates,
  and runs it after its locking `SELECT` has locked every product. Row-level AFTER
  trigger events fire at the end of the statement, so the bulk update holds all
  of its product row locks before the trigger locks the menu row — the order a
  single-product edit uses. A loop inside one transaction would lock the menu
  after its first product and could deadlock against a concurrent inline edit
  of a later one.

### Lock order

A price edit locks its product row and then, through the trigger, the menu
row. A product created in or moved into a category also takes a KEY SHARE lock
on that category — the foreign key check on `products.category_id` — and a
category created in or moved into a menu takes one on that menu. Two rules keep
the writers from waiting on each other in a circle.

**Row locks in one direction.** Every writer that locks several rows takes them
in the same direction: product rows first, in ascending id order, then category
rows, in ascending id order, and the menu row last.

**Advisory locks before row locks.** A write into a parent reaches it only
through its own foreign key check — after it has locked the row it moves, when
it moves one — so row order alone cannot keep it apart from a delete of that
parent. Deletes, and every write that puts a record into a menu or a category,
therefore also take a transaction-level advisory lock on the parent —
`pg_advisory_xact_lock(namespace, hashtext(id::text))`, or
`pg_advisory_xact_lock_shared` with the same keys, with one fixed namespace for
menus and one for categories (`repository/locks.go`) — before any row lock of
their transaction. A delete takes the lock of what it deletes exclusively; a
create, a move or a reorder takes the lock of the parent it writes into shared —
for a category, its menu's lock first and then its own. A write into a parent
that is being deleted therefore waits for the delete while it holds no row
lock, and then finds its target gone (404); a delete waits for a write that is
already writing into its parent before the delete has locked anything. Shared
locks do not conflict, so creates, moves and reorders into the same parent
still run side by side. And since every transaction takes its advisory locks
before its first row lock, nothing that holds a row lock ever waits for one, so
no cycle can pass through them.

A lock is only taken on a menu or a category the business owns — the repository
checks that before it takes the lock, and checks the parent again once the lock
is held — so a request of another tenant never waits on one. Two ids whose
`hashtext` values collide share a key, which can add a wait but never remove
one. Price edits without a move, bulk price updates, category reorders,
product deletes and menu creates and saves take no advisory lock: none of them
puts a record into a menu or a category.

| Writer | Advisory locks, taken first | Row locks, in this order |
|---|---|---|
| `PATCH /api/products/:id/price`, and `PUT /api/products/:id` without a `category_id` | — | the product row, then — on a price change — the menu row |
| `PUT /api/products/:id` with `category_id` C (`UpdateProduct`) — a move, or a save that names the product's own category | C's menu, then C, both shared | the product row, then — when C is a new category — KEY SHARE on C, then — on a price change — the menu row |
| `POST /api/products` into category C (`CreateProduct`) | C's menu, then C, both shared | KEY SHARE on C |
| `DELETE /api/products/:id` | — | the product row |
| `PUT /api/products/reorder` into category C (`ReorderProducts`) | C's menu, then C, both shared | the listed product rows in ascending id order, then — for a product that changes category — KEY SHARE on C |
| `POST /api/products/bulk-price` with `apply: true` (`ApplyPrices`) | — | the product rows in ascending id order, then — when a price changes — the menu row |
| `PUT /api/categories/:id` without a `menu_id` | — | the category row |
| `PUT /api/categories/:id` with `menu_id` M (`UpdateCategory`) — a move, or a save that names the category's own menu | M, shared | the category row, then — when M is a new menu — KEY SHARE on M |
| `POST /api/categories` into menu M (`CreateCategory`) | M, shared | KEY SHARE on M |
| `PUT /api/categories/reorder` (`ReorderCategories`) | — | the listed category rows in ascending id order, `FOR NO KEY UPDATE` |
| `POST /api/menus` | — | KEY SHARE on the business row |
| `PUT /api/menus/:id` | — | the menu row |
| `DELETE /api/categories/:id` (`DeleteCategory`) | the category, exclusive | its product rows in ascending id order, then the category row |
| `DELETE /api/menus/:id` (`DeleteMenu`) | the menu, exclusive | its product rows in ascending id order, then its category rows in ascending id order, then the menu row |

`ReorderProducts` and `ReorderCategories` lock the listed rows with a
`SELECT ... WHERE id = ANY(...) ORDER BY id FOR UPDATE` (`FOR NO KEY UPDATE`
for the categories), and `ApplyPrices` locks the products it has just read with
the same kind of statement; each then writes with a separate `UPDATE` in the
same transaction. Every row is therefore locked before the first one is
written, and the `UPDATE`, which takes its snapshot after the last lock, sees
the latest version of every row it names and writes all of them. None of these
locking statements joins another table. Under READ COMMITTED a row that changed
while a statement waited for it is checked again against the statement's
conditions, with the rows of every joined table as the statement first read
them, so a join to `categories` would drop a product moved to another category
of the same menu while the statement waited — and a bulk apply would skip it
without an error. Locking and writing in one statement — a locking CTE and an
`UPDATE ... FROM` a join against it — is not used either: it works from the
snapshot the statement started with, and can leave a row that another
transaction wrote in the meantime unwritten, without an error.

`DeleteCategory` and `DeleteMenu` run their locking `SELECT`s and then the
`DELETE` inside one transaction. `DeleteMenu`'s product step does reach the
products through a join to their categories. The only product whose category
can change while that step waits is one that leaves the menu, which is right
to drop out, because every write into a category of the menu waits for the
delete at the menu's advisory lock.

Each locking statement carries the same `business_id` predicate as the write it
prepares, so a request naming another tenant's rows never locks — or waits on —
theirs. `backend/tests/lock_tenant_scope_test.go` holds a row of tenant A and
fails when a request of tenant B waits on it — for the deletes, both reorders
and the bulk price update — and it fails when a write of tenant B into B's own
records waits while A's delete holds the advisory lock of A's record.
`backend/tests/repository/tenant_scope_db_test.go` calls the writers
directly with the records of another business while their advisory locks are
held, and requires each one to refuse at once without writing.

Each of these interleavings is replayed by a regression test that stages it
with locks of the test's own and asserts that PostgreSQL counts no deadlock,
that no retry is logged, that no request answers 5xx and that the rows agree
with the answers:

- **Menu delete and price edit** (`backend/tests/product_lock_order_test.go`).
  A delete that locked the menu row first and reached the products only through
  the cascade would take the two locks the other way round from a price edit in
  the same menu, and the two would deadlock. Hence products before the menu
  row. The same file checks the multi-row product writers for their ascending
  order.
- **Menu delete and a product moved into that menu**
  (`backend/tests/menu_delete_race_test.go`). The move holds its product and the
  KEY SHARE lock on the target category and, with a new price, needs the menu
  row, while the delete's cascade needs that category `FOR UPDATE`. Hence
  categories before the menu row, and the menu's advisory lock, at which the
  move waits for the delete before it locks anything.
- **Menu delete and category reorder**
  (`backend/tests/category_lock_order_test.go`). A reorder that locked
  categories in any order but the delete's id order could hold one category
  while waiting for another the delete holds. Hence the reorder locks in id
  order too. The same file checks both category writers for their ascending
  order.
- **Writes into a parent that is being deleted**
  (`backend/tests/lock_cycles_test.go`, `backend/tests/create_lock_test.go`).
  Row order alone cannot prevent these: a category moved into, or created in, a
  menu being deleted, followed by a price edit of one of its products; a product
  moved into, or created in, a category of a menu being deleted — between the
  delete's product and category lock steps — followed by a price edit of that
  product or a bulk price update of the menu; and a product moved into, or
  created in, a category being deleted, followed by a reorder or a bulk price
  update that locks it together with a product the delete holds. The advisory
  locks make the write wait for the delete before it holds any row, and
  `deleted_products` counts every product a category delete removes.
  `lock_cycles_test.go` also stages a move whose target category moves to
  another menu between the move's first read of it and its lock, which
  `writeIntoCategory` answers by reading the menu again and starting over.
- **Lost writes** (`backend/tests/bulk_price_race_test.go`). A bulk apply while
  other writers touch the same products, a bulk apply while an inline price
  edit commits, a bulk apply while a product moves to another category of the
  same menu, two product reorders of one category and two category reorders.
  Hence the separate locking and writing statements, locking statements without
  a join, and prices computed from the rows the apply has locked.
- **Shared locks** (`backend/tests/advisory_lock_sharing_test.go`). Writes into
  the same menu or category run side by side, a write into a category holds its
  menu's lock while it waits for the category's, and a category reorder does
  not wait for the KEY SHARE lock a product write holds on its category.

A write into a menu or a category that a concurrent delete removes after the
handler's ownership check does not become a 500 either. A create, a move or a
reorder that waited for the delete at its advisory lock checks its parent again
once it holds the lock, finds it gone and writes nothing
(`repository.ErrParentNotFound`, staged by
`backend/tests/repository/parent_gone_db_test.go`). A write racing a delete
that took no advisory lock — a row removed past the API — fails its foreign key
check instead: SQLSTATE `23503` once the delete commits
(`repository.IsForeignKeyViolation`). The handlers of `CreateProduct`,
`UpdateProduct`, `PatchProductPrice`, `ReorderProducts`, `CreateCategory` and
`UpdateCategory` answer both with a 404 that names the record that vanished:
`Kategori bulunamadı.` for a product written into a category,
`Ürün bulunamadı.` for a price edit, which names no category, and
`Menü bulunamadı.` for a category created in or moved to a menu.
`backend/tests/foreign_key_race_test.go` stages
the product and category moves, the product reorder and both creates against
an uncommitted delete of the test's own.

**The safety net.** Lock order and the advisory locks remove the cycles
described above. For a cycle nobody has spotted yet, the handlers of
`CreateProduct`, `UpdateProduct`, `PatchProductPrice`, `DeleteProduct`,
`ReorderProducts`, `BulkPrice` (the apply step), `CreateCategory`,
`UpdateCategory`, `DeleteCategory`, `ReorderCategories` and `DeleteMenu` run
their write through `repository.RetryOnConflict`: a write that fails with
SQLSTATE `40P01` (deadlock detected) or `40001` (serialization failure) runs
again — at most three runs in all, after a jittered 25–100 ms pause. Every
wrapped call is one statement or its own transaction, so a retry never runs
inside a transaction PostgreSQL has already aborted. Besides such a cycle, the
net covers the one case the advisory locks leave open on purpose: the last of
the three runs of `writeIntoCategory` writes under the locks it holds even when
its category has moved to another menu once more, so a delete of that menu can
meet it without having waited.

A retry that succeeds looks like any other success to the client, so every
retry writes one line to the log, naming the handler, the SQLSTATE of the run
it follows and the run it starts:

```text
[karecik] retrying DeleteMenu after 40P01 (attempt 2/3)
```

Such a line is a conflict the lock order above did not prevent, and worth a
look.

The loop also stops early once its context is cancelled — but for a request
that context is Fiber's `c.Context()`, the fasthttp `RequestCtx`, and fasthttp
(v1.51.0) closes its `Done` channel only when the server shuts down, never when
the client disconnects. A request whose client has gone away still runs its
retries to success or to the last run.

The footer formats the date on the Europe/Istanbul calendar (`utils.Istanbul`),
never in the server's own time zone. The binary embeds the zone database
(`time/tzdata`), so that does not depend on the container image.

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

The schema also holds a `sessions` table, which nothing reads or writes;
migration `008` explains why it is kept rather than dropped.
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
