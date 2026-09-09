# Karecik API Contract

Base URL (development): `http://localhost:8080`

Every response is JSON. Errors share one shape:

```json
{ "error": "Human readable message", "code": "VALIDATION_ERROR" }
```

> The `error` text is written in **Turkish**, because it is displayed directly
> to the end user. The `code` is a stable machine-readable identifier.

Error codes: `VALIDATION_ERROR`, `UNAUTHORIZED`, `SESSION_EXPIRED`, `FORBIDDEN`,
`NOT_FOUND`, `CONFLICT`, `INTERNAL_ERROR`, `PAYLOAD_TOO_LARGE`, `RATE_LIMITED`,
`MAIL_NOT_CONFIGURED`, `RESET_TOKEN_INVALID`.

Protected endpoints authenticate with the `karecik_sid` cookie, which
`register` and `login` set. It is `HttpOnly`, so no script can read it and no
client code has to send it — the browser attaches it on its own. A cross-origin
caller must use `credentials: 'include'`, or the browser will neither store nor
send it.

The cookie is the SHA-256 key of a session held in the API process's memory, so
it stops working when the API restarts as well as when it expires.

### Two kinds of 401

They share a status and must not share a response, so they have different codes:

| Code | Meaning | What a client should do |
|---|---|---|
| `UNAUTHORIZED` | the **credentials in this request** were wrong — a mistyped password on `login`, or the wrong `current_password` on `change-password` | show the message on the form; the session is untouched |
| `SESSION_EXPIRED` | there is **no usable session** — it expired, was revoked by a password change, or the API restarted | discard any locally remembered account and send the user to the login page |

Keying on the status alone conflates them, and the failure is not subtle: a
user who mistypes their own password while changing it gets thrown out of the
panel instead of being told to try again.

---

## Shared types

### Translation map (`translations`)

Category and product names are multilingual. The shape is:

```json
{ "tr": { "name": "Türk Kahvesi", "description": "Geleneksel..." },
  "en": { "name": "Turkish Coffee", "description": "Traditional..." } }
```

When reading, a missing language falls back to the business'
`default_language`.

### Currency codes

| Code | Symbol | Position |
|---|---|---|
| `TRY` | ₺ | suffix (`100,00 ₺`) |
| `USD` | $ | prefix (`$100.00`) |
| `EUR` | € | prefix |
| `GBP` | £ | prefix |
| `AZN` | ₼ | suffix |
| `RUB` | ₽ | suffix |
| `SAR` | ﷼ | suffix |
| `AED` | د.إ | suffix |

Default: `TRY`.

### Allergens (`allergens`) — fixed code list

`gluten`, `sut`, `yumurta`, `findik`, `yer_fistigi`, `soya`, `balik`,
`kabuklu_deniz`, `susam`, `hardal`, `kereviz`, `sulfit`, `aci`, `vejetaryen`,
`vegan`, `alkol`, `kafein`

---

## 1. Health

### `GET /api/health`

```json
{ "status": "ok", "database": "up", "version": "1.0.0" }
```

When the server is configured to serve the frontend (`SERVE_STATIC`) and the
bundle is not where `STATIC_DIR` points, it answers **`503`** instead and names
the path it looked in:

```json
{ "status": "degraded", "database": "up",
  "error": "frontend bundle is missing: /app/frontend/dist/index.html",
  "version": "1.0.0" }
```

That case is the reason the check exists in this shape: a deploy once shipped
without the bundle and reported healthy the whole time, because this is an
`/api` route and knew nothing about static files.

A failed database ping is reported as `"database": "down"` inside a `200` — it
is deliberately not fatal, so a brief database blip does not restart the
container.

---

## 2. Authentication

### `POST /api/auth/register`

Request:
```json
{ "business_name": "Kahve Durağı", "email": "info@kahve.com", "password": "sifre1234" }
```

Rules: `business_name` 2–100 characters, `email` valid and unique,
`password` at least 8 characters.

The business record and its `slug` (subdomain) are generated automatically:
`Kahve Durağı` → `kahve-duragi`. On a collision `-2`, `-3` … is appended.

Response `201` — plus a `Set-Cookie: karecik_sid=...` header. The body
carries **no token**: there is nothing for the client to store, and therefore
nothing for a script on the page to steal.

```json
{
  "user": { "id": "uuid", "email": "info@kahve.com", "business_name": "Kahve Durağı" },
  "business": { "...Business object..." }
}
```

Error `409`: the email is already registered.

### `POST /api/auth/login`

Request: `{ "email": "...", "password": "..." }`
Response `200`: the same body and the same `Set-Cookie` as register.
Error `401`: `E-posta veya şifre hatalı.`

### `GET /api/auth/me` 🔒

Response: `{ "user": {...}, "business": {...} }`

> **The browser client does not call this.** It used to run on every page load
> to answer "am I signed in?", which put an authenticated round trip in front
> of the landing page and of every customer menu — pages opened by strangers
> with no session. The panel now remembers the account in `localStorage`
> (`krc_user`) and is corrected by the first `SESSION_EXPIRED` it receives.
>
> The endpoint stays because it is the one honest "is this session live?"
> probe, and the backend test suite uses it as exactly that.

### `POST /api/auth/logout`

Not marked 🔒 on purpose: an expired or already-revoked cookie has to be able
to clear itself, and demanding a valid session to log out would strand exactly
the people who most need to.

Drops the session and clears the cookie. Always `200`, even when the cookie
matched nothing — a logout button that can fail is one people stop trusting.

### `POST /api/auth/change-password` 🔒

Request: `{ "current_password": "...", "new_password": "..." }`

The current password is required even though the caller is signed in: it is
what separates the account owner from someone who sat down at an unlocked
laptop. `new_password` must be at least 8 characters and different from the
current one.

Response `200`: `{ "success": true, "revoked_sessions": 2 }` — every OTHER
session of the user is destroyed, so a leaked password stops being useful. The
caller's own session survives, so the dashboard in front of them keeps working.

Error `401`: `Mevcut şifreniz hatalı.`

### `POST /api/auth/forgot-password`

Request: `{ "email": "..." }`

Response `200`: **the same body whatever happened.**

```json
{ "success": true, "message": "Bu adres kayıtlıysa, şifre sıfırlama bağlantısı gönderildi. ..." }
```

That is the design, not an oversight. Answering "no such account" would turn
this into a membership oracle: anyone could learn which addresses are
registered by asking. **Every** outcome below returns it — an unknown address, a
link already sent inside the cooldown, an account holding the maximum of 3 live
tokens, and the provider refusing the message. The mail is dispatched from a
goroutine so that a registered address is not measurably slower to answer than
an unregistered one; the identical bodies would be pointless if the response
time gave it away.

**Quota guards.** The provider's free tier allows 100 messages a day and 3,000
a month, so three limits sit in front of the send, each covering what the
others cannot:

| Guard | Scope | Rule |
|---|---|---|
| Route limiter | per IP | 2 requests per hour |
| `resetCooldown` | per account | no second e-mail within 30 minutes |
| `maxActiveResets` | per account | at most 3 unexpired links at once |

The per-IP limiter does nothing about requests arriving from many hosts, which
is exactly the shape of an attempt to drain the quota or bury one owner's
inbox — that is what the cooldown is for. A request inside the cooldown creates
**no token and sends no mail**, so the link already in the inbox stays valid for
its full hour.

Two failures *are* reported, because both are true regardless of which address
was submitted:

| Status | When |
|---|---|
| `503 MAIL_NOT_CONFIGURED` | `RESEND_API_KEY` / `MAIL_FROM` are unset on this deployment |
| `429 RATE_LIMITED` | more than 2 requests from one IP in an hour |

The `429` fires on the host before the address is looked at, so it cannot be
read as "this account exists".

### `POST /api/auth/reset-password`

Request: `{ "token": "...", "password": "..." }` — the token comes from the
e-mailed link (`/sifre-sifirla?token=...`). It is valid for **1 hour** and for
**one use**: the row is deleted in the same statement that reads it, so two
simultaneous clicks cannot both succeed.

Rate limited separately from `/forgot-password` — 10 requests per 15 minutes
per IP — and deliberately so. This endpoint sends no mail, and sharing the
stricter mail budget would lock someone out of finishing the reset they are in
the middle of after a couple of mistyped passwords.

Response `200`: `{ "success": true, "revoked_sessions": 3 }` — **every** session
of that user is destroyed, with no exception kept. This is the opposite of
`change-password`, which spares the caller's own session: someone resetting is
not signed in, and the usual reason to reset is that somebody else might be.

Error `410 RESET_TOKEN_INVALID`: expired, already used, or never existed. The
three are deliberately indistinguishable — telling them apart would confirm
that a token was real.

> `go run ./cmd/resetpw -email owner@example.com` remains the break-glass path
> for when mail is down or not configured. It cannot sign anyone out — restart
> the API for that. See [DEPLOY](DEPLOY.md).

---

## 3. Business 🔒

### `GET /api/business`

```json
{
  "id": "uuid",
  "name": "Kahve Durağı",
  "slug": "kahve-duragi",
  "logo_url": "/uploads/abc.png",
  "cover_url": null,
  "currency": "TRY",
  "theme": "modern-light",
  "font_family": "inter",
  "primary_color": "#1d4ed8",
  "default_language": "tr",
  "languages": ["tr", "en"],
  "splash_enabled": true,
  "splash_duration": 1200,
  "splash_bg_color": "#0f172a",
  "splash_text": "Hoş geldiniz",
  "show_vat_note": true,
  "vat_note_text": "Fiyatlarımıza KDV dahildir.",
  "show_price_date": true,
  "price_updated_at": "2026-08-24T10:00:00Z",
  "phone": "+90 555 000 00 00",
  "address": "Kadıköy / İstanbul",
  "instagram": "kahveduragi",
  "wifi_password": "kahve2026",
  "is_active": true,
  "menu_url": "http://kahve-duragi.localhost:5173",
  "created_at": "...", "updated_at": "..."
}
```

### `PUT /api/business`

Partial update (only the fields present in the body are applied). Updatable
fields: `name`, `slug`, `logo_url`, `cover_url`, `currency`, `theme`,
`font_family`, `primary_color`, `default_language`, `languages`,
`splash_enabled`, `splash_duration`, `splash_bg_color`, `splash_text`,
`show_vat_note`, `vat_note_text`, `show_price_date`, `phone`, `address`,
`instagram`, `wifi_password`.

A changed `slug` is checked for uniqueness → `409` on a collision.

Response: the updated Business object.

---

## 4. Categories 🔒

The business is resolved from the token; no endpoint accepts a `business_id`.

### `GET /api/categories`

An array ordered by `position ASC`. Each category carries `product_count`.

```json
[ { "id": "uuid", "translations": {"tr":{"name":"Sıcak İçecekler","description":""}},
    "image_url": null, "icon": "☕", "position": 0, "is_active": true,
    "product_count": 8, "created_at": "...", "updated_at": "..." } ]
```

### `POST /api/categories`

```json
{ "translations": { "tr": { "name": "Tatlılar", "description": "" } },
  "icon": "🍰", "image_url": null, "is_active": true }
```

`position` is assigned automatically as the current maximum + 1.
Response `201`: the category object.
An empty `name` in the default language yields `422`.

### `PUT /api/categories/:id`

Partial update: `translations`, `icon`, `image_url`, `is_active`.

### `DELETE /api/categories/:id`

The products of the category are deleted too (`ON DELETE CASCADE`).
Response `200`: `{ "success": true, "deleted_products": 8 }`

### `PUT /api/categories/reorder`

Called after a drag and drop.

```json
{ "ids": ["uuid-3", "uuid-1", "uuid-2"] }
```

The array order is written into `position` (0, 1, 2 …) in a single transaction.
Response: the updated category array.

---

## 5. Products 🔒

### `GET /api/products`

Query: `?category_id=uuid` (optional), `?search=text` (optional).
Ordering: `category position ASC, product position ASC`.

```json
[ { "id": "uuid", "category_id": "uuid",
    "translations": { "tr": { "name": "Latte", "description": "...", "ingredients": "Espresso, süt" } },
    "price": 145.00, "compare_price": null, "image_url": "/uploads/x.jpg",
    "allergens": ["sut", "kafein"], "is_active": true, "is_featured": false,
    "position": 0, "created_at": "...", "updated_at": "..." } ]
```

> `price` is a JSON **number** (not a string) with 2 decimal precision.

### `POST /api/products`

```json
{ "category_id": "uuid",
  "translations": { "tr": { "name": "Latte", "description": "", "ingredients": "" } },
  "price": 145, "compare_price": null, "image_url": null,
  "allergens": ["sut"], "is_active": true, "is_featured": false }
```

`category_id` is required and must belong to the same business (otherwise `403`).
`price >= 0` is required. Response `201`.

### `PUT /api/products/:id`

Partial update: `category_id` (move to another category), `translations`,
`price`, `compare_price`, `image_url`, `allergens`, `is_active`, `is_featured`.

### `PATCH /api/products/:id/price`

Quick price change (the inline editing in the menu editor).

Request: `{ "price": 155.50 }` → Response: the updated product.

### `DELETE /api/products/:id`

Response: `{ "success": true }`

### `PUT /api/products/reorder`

```json
{ "category_id": "uuid", "ids": ["uuid-2", "uuid-1"] }
```

Products are repositioned according to the order inside `category_id`. **Moving
a product to another category** goes through the same endpoint: the product's
`category_id` is set to the one given in the request. Response: the updated
product array of that category.

### `POST /api/products/bulk-price` — bulk price update

```json
{ "percentage": 10,
  "rounding": "nearest_5",
  "category_ids": ["uuid-1"],
  "apply": true }
```

| Field | Meaning |
|---|---|
| `percentage` | −90 … +1000. `10` → +10%, `-15` → −15% |
| `rounding` | `none`, `integer`, `nearest_5`, `nearest_10`, `ends_99`, `ends_95`, `ends_50` |
| `category_ids` | Empty or missing → **every** product |
| `apply` | `false` → preview only, nothing is written |

Order of operations: `new = old * (1 + percentage/100)` → rounding → `max(0, result)`.

Response:
```json
{ "applied": true, "affected": 42,
  "preview": [ { "id": "uuid", "name": "Latte", "old_price": 145.00, "new_price": 160.00 } ],
  "price_updated_at": "2026-08-24T12:30:00Z" }
```

When `apply: true`, `businesses.price_updated_at` is set to now, which feeds the
"Fiyatlarımız … tarihinden itibaren geçerlidir" line in the customer menu.

---

## 6. File upload 🔒

### `POST /api/uploads`

`multipart/form-data`, field name: `file`.
Accepted types: `image/jpeg`, `image/png`, `image/webp`, `image/gif`. Max **5 MB**.

Response `201`: `{ "url": "/uploads/1724500000-a1b2c3.webp", "size": 84213 }`

Files are written under `UPLOAD_DIR` and served statically at `GET /uploads/*`.

---

## 7. Public menu — no token required

### `GET /api/public/menu/:slug`

Query: `?lang=tr` (optional, defaults to the business' `default_language`).

### `GET /api/public/menu` (via subdomain)

The subdomain is resolved from the `Host` header:
`kahve-duragi.karecik.com` → slug `kahve-duragi`. A missing or `www` subdomain
yields `404`.

Both endpoints return the same body:

```json
{
  "business": {
    "name": "Kahve Durağı", "slug": "kahve-duragi", "logo_url": "/uploads/logo.png",
    "currency": "TRY", "currency_symbol": "₺", "theme": "modern-light",
    "font_family": "inter", "primary_color": "#1d4ed8",
    "default_language": "tr", "languages": ["tr", "en"],
    "splash_enabled": true, "splash_duration": 1200,
    "splash_bg_color": "#0f172a", "splash_text": "Hoş geldiniz",
    "show_vat_note": true, "vat_note_text": "Fiyatlarımıza KDV dahildir.",
    "show_price_date": true, "price_updated_at": "2026-08-24T10:00:00Z",
    "phone": "...", "address": "...", "instagram": "...", "wifi_password": "..."
  },
  "categories": [
    { "id": "uuid", "name": "Sıcak İçecekler", "description": "", "icon": "☕",
      "image_url": null,
      "products": [
        { "id": "uuid", "name": "Latte", "description": "...", "ingredients": "...",
          "price": 145.00, "compare_price": null, "image_url": "/uploads/x.jpg",
          "allergens": ["sut"], "is_featured": false } ] } ],
  "footer": {
    "price_note": "Fiyatlarımız 24.08.2026 tarihinden itibaren geçerlidir.",
    "vat_note": "Fiyatlarımıza KDV dahildir.",
    "powered_by": "Karecik ile hazırlandı"
  }
}
```

Important: on the public endpoints **translations are already resolved** — the
payload carries plain `name` / `description` fields instead of the
`translations` map. Categories and products with `is_active = false` are
**omitted entirely**. Both lists are ordered by `position ASC`.

`footer.price_note` is generated on the backend from `price_updated_at` in
`dd.MM.yyyy` format.

### `GET /api/preview/menu` 🔒

Backs the **live preview** in the dashboard. It returns exactly the same body as
the public menu, with one difference: it uses the business from the token and
also includes inactive records, flagged with `"is_active": false` so the
dashboard can dim them.

---

## Status code summary

| Code | Meaning |
|---|---|
| 200 | Success |
| 201 | Created |
| 400 | Malformed request body |
| 401 | Missing or invalid token |
| 403 | Access to another business' record |
| 404 | Record or business not found |
| 409 | Email or slug collision |
| 413 | File too large |
| 422 | Validation error (required field, invalid price, …) |
| 500 | Server error |
