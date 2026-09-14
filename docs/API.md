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

### U+0000 and bytes that are not UTF-8

PostgreSQL stores the character U+0000 neither in a text column nor, written as
its JSON escape, in a `jsonb` value, and it refuses a byte sequence that is not
valid UTF-8 in any text, so a write carrying either would fail inside the
database. A JSON body cannot carry invalid UTF-8 — JSON decoding turns such a
byte into U+FFFD — but a form-encoded body, a query string and a header can. The
API refuses both in every text it stores, before anything is written, with the
`422` that field already answers an invalid value with:

| Where | Message |
|---|---|
| a menu text field on `POST` / `PUT /api/menus` | the field's own "metin olmalıdır" message — `Menü adı metin olmalıdır.`, `Menü açıklaması metin olmalıdır.`, `slogan alanı metin olmalıdır.`, … |
| a clearable menu field (`phone`, `address`, `instagram`, `wifi_ssid`, `wifi_password`, `logo_url`, …) | `<field> alanı metin veya boş olmalıdır.` |
| `name` / `slug` on `PUT /api/business` | `İşletme adı metin olmalıdır.` / `İşletme adresi metin olmalıdır.` |
| category or product `translations` (a name, description or ingredients text of a supported language) | `Çeviri alanı geçersiz.` |
| category `icon` / `image_url`, on create and update | `icon alanı metin veya boş olmalıdır.` / `image_url alanı metin veya boş olmalıdır.` |
| product `image_url`, on create and update | `Görsel adresi geçersiz.` |
| any field of a product badge, its `id` included | `Rozet listesi geçersiz.` |
| an option group name or an option name | `Seçenek listesi geçersiz.` |
| a link `label` or `url` | the link rules of section 8: `Link adı geçersiz karakter içeriyor.` / `Link adresi http:// veya https:// ile başlayan geçerli bir adres olmalıdır.` |

A link `id` is not refused: like any other id that is not a plain identifier, it
is replaced with a new UUID.

Sign-up refuses both as well — `business_name` with
`İşletme adı metin olmalıdır.` and `email` with
`Geçerli bir e-posta adresi giriniz.` An e-mail address is checked as it was
sent, before it is lowercased: lowercasing turns every byte that is not UTF-8
into U+FFFD, so `email=%FF%40example.test` in a form body would otherwise turn
into the valid address `U+FFFD@example.test`. A text that is only used to look
something up — whether it comes from a JSON body, a form body, a query string, a
path or a header — is answered the way that lookup answers any value that
matches nothing, since no stored text can hold either:

- an address on `login` is the usual `401` `E-posta veya şifre hatalı.` — also
  when an account's address holds U+FFFD where the request holds such a byte;
- an address on `forgot-password` — `email=%FF%40example.test` in a form body,
  say — gets the usual answer and no e-mail;
- a `search` on `GET /api/products` — `?search=%FF` — returns `[]`;
- a business or menu slug of the public menu is the usual `404`: `?menu=%FF` on
  a tenant host, and a business slug read from an `X-Forwarded-Host` header that
  holds U+0000.

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
`password` at least 8 characters and at most 72 bytes.

A longer password is `422`
`Şifre çok uzun. En fazla 72 bayt olabilir; Türkçe karakterler 2 bayt sayılır.`
The limit is bcrypt's, which hashes no more than 72 bytes, and it is counted in
bytes of UTF-8: a Turkish letter such as `ş` takes two.

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

A password longer than 72 bytes never matches — not even one whose first 72
bytes are the password, which bcrypt alone would accept — and gets the same
`401`.

### `GET /api/auth/me` 🔒

Response: `{ "user": {...}, "business": {...} }`

> **The browser client does not call this.** Calling it on every page load to
> answer "am I signed in?" would put an authenticated round trip in front of the
> landing page and of every customer menu — pages opened by strangers with no
> session. The panel remembers the account in `localStorage` (`krc_user`)
> instead, and is corrected by the first `SESSION_EXPIRED` it receives.
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
laptop. `new_password` must be at least 8 characters, at most 72 bytes — a
longer one is `422` with the message given under `register` — and different
from the current one. A `current_password` longer than 72 bytes never matches:
`401` `Mevcut şifreniz hatalı.`

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

`password` must be at least 8 characters and at most 72 bytes. A longer one is
`422` with the message given under `register`, and it is refused before the
token is used, so the same link still works for a password that fits.

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

`icon` and `image_url` are trimmed, and an empty or whitespace-only value is
stored as `null` — exactly what `PUT /api/categories/:id` stores — so a category
without an emoji or an image always reads back as `null`.

A create into a menu that is being deleted waits until the delete has finished,
and a menu deleted while the request runs — after the ownership check has seen
it — is gone by the time the category would be written. Either way the answer
is `404` `Menü bulunamadı.`, naming the record that vanished, never a `500`,
and no category is created.

### `PUT /api/categories/:id`

Partial update: `translations`, `icon`, `image_url`, `is_active`, and `menu_id`,
which moves the category to another menu of the same business. A target menu
deleted while the request runs is `404` `Menü bulunamadı.`, naming the record
that vanished, and the category stays where it was; a move into a menu that is
being deleted waits until the delete has finished and then gets that `404`.

### `DELETE /api/categories/:id`

The products of the category are deleted too (`ON DELETE CASCADE`).
Response `200`: `{ "success": true, "deleted_products": 8 }`

`deleted_products` is the number of products the category held once the delete
had locked them, which is every product the delete removes: a product create,
move or reorder into the category waits until the delete has finished — and
then gets `404` `Kategori bulunamadı.` — and a delete that starts while one of
those is writing waits for it.

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

`category_id` is required. A category of another business is `404`
`Kategori bulunamadı.`, exactly like one that does not exist. Response `201`.

A `price` or a `compare_price` the column could not hold is refused with `422`
before anything is written:

| Value | `price` | `compare_price` |
|---|---|---|
| below 0 | `Fiyat sıfırdan küçük olamaz.` | `Karşılaştırma fiyatı sıfırdan küçük olamaz.` |
| above `9999999999.99`, the largest value of their `NUMERIC(12,2)` columns | `Fiyat en fazla 9.999.999.999,99 olabilir.` | `Karşılaştırma fiyatı en fazla 9.999.999.999,99 olabilir.` |
| `NaN`, which a form or an XML body can carry | `Fiyat sıfır veya daha büyük bir sayı olmalıdır.` | `Karşılaştırma fiyatı geçersiz.` |

The limit applies to the value as sent, before it is rounded to two decimals:
`9999999999.99` is stored, `9999999999.991` is refused.

A create into a category that is being deleted — or whose menu is — waits until
the delete has finished. That category, like one deleted while the request runs
after the ownership check has seen it, is `404` `Kategori bulunamadı.`, never a
`500`, and no product is created.

`image_url` is trimmed and a blank value is stored as `null`, as on `PUT`.
Creating a product does not move the menu's price date — only a changed price
of an existing product does (see `PUT` below).

### `PUT /api/products/:id`

Partial update: `category_id` (move to another category), `translations`,
`price`, `compare_price`, `calories`, `image_url`, `allergens`, `badges`,
`options`, `is_active`, `is_featured`.

**Any price change moves the menu's price date** (`menus.price_updated_at`, the
date in the customer menu's "Fiyatlarımız … tarihinden itibaren geçerlidir"
line). A price change is a different `price`, a different `compare_price`, or a
different list of option surcharges; the menu is the one the product sits on
after the update, so a product moved into another menu with a new price dates
that menu and leaves the old one alone. Re-sending identical prices together
with other edits (the dashboard dialog sends every field on every save),
toggling `is_active` / `is_featured`, renaming an option, or moving a product
without a new price does not move the date. Surcharges compare numerically:
`10` and `10.0` are the same. The date is moved by a database trigger
(migration `010_price_change_date.sql`), not by the handler.

`price` and `compare_price` are refused as on `POST` when they are above
`9999999999.99`. A negative or `NaN` `price` is `422`
`Fiyat sıfır veya daha büyük bir sayı olmalıdır.`, a negative or `NaN`
`compare_price` is `422` `Karşılaştırma fiyatı geçersiz.`, and
`"compare_price": null` clears it.

A `category_id` of another business, or one that does not exist, is `404`
`Kategori bulunamadı.`. So is one whose category is deleted while the request
runs — after the ownership check, before the row is written — and the product
is left exactly as it was: a move into a category, or into a menu, that is being
deleted waits until the delete has finished and then gets that `404`.

### `PATCH /api/products/:id/price`

Quick price change (the inline editing in the menu editor).

Request: `{ "price": 155.50 }` → Response: the updated product.

A different price moves the menu's `price_updated_at` exactly like `PUT` does;
sending the price the product already has leaves it alone.

`price` is refused as on `POST`: below 0, above `9999999999.99` or `NaN` is
`422` with the messages given there.

A product that does not exist is `404` `Ürün bulunamadı.`; a foreign key failure
reaching this write is answered the same way.

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

A `category_id` of another business, or one that does not exist, is `404`
`Kategori bulunamadı.`. A target category deleted while the request runs gets
the same `404`, and no product is moved; a reorder into a category, or into a
menu, that is being deleted waits until the delete has finished.

Every listed product is locked before the first position is written, and the
positions are written in the same transaction, so two reorders of one category
that run at the same time never leave duplicate positions: the one that
finishes last decides the order of the products it lists.

### `POST /api/products/bulk-price` — bulk price update

```json
{ "menu_id": "uuid",
  "percentage": 10,
  "rounding": "nearest_5",
  "category_ids": ["uuid-1"],
  "apply": true }
```

| Field | Meaning |
|---|---|
| `menu_id` | **Required** (`422` when missing). The one menu whose prices change; a menu of another business is `403` |
| `percentage` | −90 … +1000. `10` → +10%, `-15` → −15%. Anything else, `NaN` included, is `422` `Yüzde değeri -90 ile 1000 arasında olmalıdır.` |
| `rounding` | `none`, `integer`, `nearest_5`, `nearest_10`, `ends_99`, `ends_95`, `ends_50` |
| `category_ids` | Empty or missing → **every** product of the menu |
| `apply` | `false` → preview only, nothing is written |

Order of operations: `new = old * (1 + percentage/100)` → rounding → `max(0, result)`.

Response:
```json
{ "applied": true, "affected": 42,
  "preview": [ { "id": "uuid", "name": "Latte", "old_price": 145.00, "new_price": 160.00 } ],
  "price_updated_at": "2026-08-24T12:30:00Z" }
```

With `apply: false` nothing is locked or written; the preview is computed from
the prices as they are read. With `apply: true` no earlier read is reused: the
update locks the products, reads their prices under that lock and computes the
new prices from exactly those, all in one transaction, so a price edited while
the update waited for its locks is the price the percentage is applied to, and a
product moved to another category of the same menu meanwhile is still raised.
The answer then describes what that transaction wrote: `old_price` is the price
it read, `new_price` the price the product has now, and `affected` the number of
products it gave a new price. A preview lists the same rows and counts as
`affected` the products that would get a new price. Both list the products in
menu editor order: category position, then product position.

When a new price would be larger than `9999999999.99` — the largest value
`products.price` holds — the request is `422`
`Bu değişiklik bazı fiyatları izin verilen en yüksek değerin (9.999.999.999,99) üzerine çıkarıyor.`
for a preview and an apply alike, and nothing is written: no price, no price
date and no `price_update_logs` row.

The price date lives on the menu, in `menus.price_updated_at`, and feeds the
"Fiyatlarımız … tarihinden itibaren geçerlidir" line in the customer menu. With
`apply: true` it moves to now **only when at least one price actually
changed** — an apply that leaves every price where it was (`affected: 0`) keeps
the old date, and a preview never touches it. `price_updated_at` is present
when `apply: true` and carries the menu's value after the update, moved or not.

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
    "phone": "...", "address": "...", "instagram": "...", "wifi_password": "...",
    "contact_display": "inline",
    "links": [ { "id": "rezervasyon", "label": "Rezervasyon", "url": "https://rezervasyon.example/masa" } ]
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

`footer.price_note` is generated on the backend from the menu's
`price_updated_at` in `dd.MM.yyyy` format, **on the Europe/Istanbul calendar** —
never in the server's own time zone, so a price changed at 01:30 Istanbul time
names that day and not the previous one. The server binary embeds its own copy
of the zone database, so this does not depend on the host having one installed.

`footer.vat_note` is filled when the menu's `show_vat_note` is `true`: it is the
trimmed `vat_note_text`, or `Fiyatlarımıza KDV dahildir.` when that text is
blank — the sentence the dashboard's settings page shows as the placeholder of
an empty field. With `show_vat_note` off it is `""`.

`business.contact_display` and `business.links` carry the menu's contact block
settings — see section 8. When `contact_display` is `hidden`, the payload
itself leaves the block out: `business.phone`, `business.instagram`,
`business.wifi_ssid` and `business.wifi_password` are `null` and
`business.links` is `[]`, whatever the menu stores. The owner still reads the
stored values through `GET /api/menus/:id`. `business.address` is not part of
the contact block and is sent in every mode, and the other three modes send
everything — they only change where the block is drawn.

`business.links` is not a copy of the stored list. Every stored entry is checked
against the link rules of section 8 again, and an entry that breaks one is left
out, so a row written past the API never reaches a customer. At most 8 entries
are sent: the first ones that pass.
Each entry sent carries its `label` and `url` cleaned the way a save cleans
them, and an `id` that is unique within the payload: the first entry with a
given non-blank id keeps it, and an entry whose id is blank or repeats an
earlier one gets `link-<index>`, its position in `business.links` — or, when
that id is already taken, the next free `link-<n>`. The same row always yields
the same payload. The owner's own endpoints (`GET /api/menus`,
`GET /api/menus/:id`) return the stored entries as they are, so a bad one stays
visible to the owner, who can fix it.

A stored `links` value never fails a read, on any endpoint. A value that cannot
be read as a JSON array at all — one that is not an array, or one nested more
than 10000 levels deep, deeper than Go's JSON decoder reads — reads as `[]`, and
the server writes a log line each time it reads one. An element that is not an
object, or whose `label` or `url` is not a string, is skipped. A missing or
non-string `id` reads as `""`.

A successful (`200`) answer from either public endpoint carries
`Cache-Control: no-cache` and an `ETag`. The browser may keep its copy but has
to ask before reusing it: an unchanged menu comes back as `304 Not Modified`
with no body, and a changed one as a full `200` — so a category or a price saved
in the dashboard shows up on the very next load of the customer menu, where a
`max-age` would let a browser show a menu that is minutes old without asking. A
`404` for an unknown business or menu carries neither header.

### `GET /api/preview/menu` 🔒

Backs the **live preview** in the dashboard. It returns exactly the same body as
the public menu, with one difference: it uses the business from the token and
also includes inactive records, flagged with `"is_active": false` so the
dashboard can dim them.

The preview is built by the same code as the public menu, so a menu in `hidden`
contact mode comes back redacted here too.

---

## 8. Menus 🔒 — contact block and links

Every menu the dashboard API returns — `GET /api/menus`, `GET /api/menus/:id`,
and the menu in the response of `POST /api/menus` and `PUT /api/menus/:id` —
carries the two settings of the customer menu's contact block: the Wi-Fi,
Instagram and phone entries, followed by the owner's own links.

```json
{ "contact_display": "inline",
  "links": [ { "id": "rezervasyon", "label": "Rezervasyon", "url": "https://rezervasyon.example/masa" } ] }
```

`links` is always an array — `[]` for a menu without links, never `null`. A new
menu starts with `"contact_display": "inline"` and `"links": []`, and every menu
that already existed received the same two values when migration
`011_menu_contact_links.sql` added the columns.

### `contact_display`

| Id | Label | Home view of the customer menu | Footer of the product screens |
|---|---|---|---|
| `inline` | Yan yana | one row of chips — Wi-Fi, Instagram, Telefon, then each link; tapping a chip opens its details | the compact list |
| `list` | Açık liste | the same entries as an always-open list | the compact list |
| `footer` | Sadece alt bilgi | nothing | the compact list |
| `hidden` | Hiç gösterme | nothing | nothing |

`POST` and `PUT` accept exactly these four ids. Anything else — another word, a
different case, surrounding spaces, a number, `null` — is `422`
`Geçersiz iletişim görünümü.` In `hidden` mode the public payload does not carry
the block at all (see section 7).

### `links`

`POST` and `PUT` accept an array of `{ "id", "label", "url" }` objects in the
order the owner wants them shown. `"links": null` stores `[]`, exactly like
`"links": []`; only a `PUT` that does not name `links` at all leaves them as
they are.

The list is validated as a whole: a single problem refuses the request with
`422`, nothing is stored, and no entry is ever dropped or shortened on the
owner's behalf.

A `links` value that is not an array of link objects at all — a string, a
number, a single object, an entry that is not an object, an `id`, `label` or
`url` that is not a string — is `422` `Link listesi geçersiz.`, before any rule
below is looked at. Inside an entry, `null` for `id`, `label` or `url` counts as
a missing member, and a missing member is read as the empty string: a missing
`label` is `Link adı zorunludur.`, a missing `url` (with a valid label) is the
invalid-address message, and a missing `id` is replaced with a new UUID. A
`null` entry counts as an entry with all three members missing, so it is
`Link adı zorunludur.`.

Otherwise the count is checked first, then each entry in order — its label
before its address — and the first rule that fails gives the message:

| Rule | Message |
|---|---|
| more than 8 entries | `En fazla 8 link ekleyebilirsiniz.` |
| the trimmed `label` contains a C0 or C1 control character (U+0000–U+001F, U+007F–U+009F) | `Link adı geçersiz karakter içeriyor.` |
| nothing visible is left of the trimmed `label`: nothing remains once the invisible format characters are removed and the rest is trimmed again | `Link adı zorunludur.` |
| the trimmed `label` is longer than 40 characters (counted in runes, so `ş` is one) | `Link adı en fazla 40 karakter olabilir.` |
| the trimmed `url` is longer than 500 bytes | `Link adresi en fazla 500 karakter olabilir.` |
| the trimmed `url` is not a valid address (below) | `Link adresi http:// veya https:// ile başlayan geçerli bir adres olmalıdır.` |

**Trim.** `label` and `url` are trimmed of Unicode White_Space exactly as Go's
`strings.TrimSpace` does: tab, newline, vertical tab, form feed, carriage return
and space, plus U+0085, U+00A0, U+1680, U+2000–U+200A, U+2028, U+2029, U+202F,
U+205F and U+3000. U+FEFF and U+180E are not White_Space and are not trimmed.
Because the trim comes first, a label that is only newlines is "required", while
a label with a newline inside it, or one made of BEL, is "invalid characters".

**Invisible format characters:** U+00AD, U+061C, U+180E, U+200B–U+200F,
U+202A–U+202E, U+2060–U+2064, U+2066–U+206F and U+FEFF. The label check removes
them, trims what is left once more, and refuses the label when nothing remains:
a label of an invisible character, a space and another invisible character is
`Link adı zorunludur.`, and so is a label of U+200E or U+200F alone. The label
that is stored is still the one trimmed once, so invisible characters around or
inside a visible word are kept.

**A valid address** meets every one of these:

- it contains no whitespace (the White_Space set above), no C0 or C1 control
  character, no invisible format character, and none of
  `<` `>` `"` `` ` `` `{` `}` `|` `\` `^` `[` `]`, anywhere. The square brackets
  are refused outright because Go's `net/url` removes them from a host name, so
  `https://[ornek.com]` would otherwise pass as `ornek.com` although no browser
  opens it;
- it starts with `http://` or `https://`, in any letter case;
- Go's `net/url` parses it, and it carries no userinfo — no `user@` before the
  host. `net/url` refuses, among others, a malformed percent escape such as
  `%zz` in the path or the fragment, and a percent escape of an ASCII character
  in the host;
- its host name, without the port, is 1–253 characters (counted in runes, like
  the label) of dot-separated labels:
  at least two labels, each 1–63 characters of letters (any Unicode letter),
  ASCII digits or `-`, not starting or ending with `-`, and the last label holds
  at least one letter. `localhost`, `192.168.1.1`, `[::1]`, `example.com.` and
  the `https` host that `https://https//ornek.com` parses into are all refused;
- a `:` after the host name is followed by a port of 1–5 digits whose value is
  1–65535; `https://example.com:` and `https://example.com:0` are refused.

An accepted list is stored with `label` and `url` trimmed and only the scheme of
`url` lowercased (`HTTPS://Example.COM/Masa` is stored as
`https://Example.COM/Masa`). An `id` that matches `^[A-Za-z0-9_-]{1,64}$` is kept
as sent unless an earlier entry of the same list already has it; any other —
missing, `null`, empty, longer than 64 characters, with other characters, or a
repeat of an earlier entry's id — is replaced with a new UUID. The ids of a
stored list are therefore unique, and sending the list back exactly as it was
read keeps every one of them.

### `position`

`POST` and `PUT` accept a `position` from `0` to `2147483647`, the largest value
of the `INTEGER` column. A negative one is `422`
`Sıra değeri sıfır veya daha büyük olmalıdır.` and a larger one `422`
`Sıra değeri çok büyük.`. A menu created without a position comes after the
last one; when that one is at `2147483647` already, the new menu gets the same
position and is listed after it.

---

## 9. Meta

### `GET /api/meta`

Public. The fixed catalogues the dashboard builds its pickers from:
`currencies`, `themes`, `fonts`, `allergens`, `badge_icons`,
`splash_entrances`, `splash_exit_animations`, `splash_easings`,
`splash_display_modes`, `slide_fade_modes`, `header_display_modes`,
`contact_display_modes`, `languages` and `rounding_modes`.

`contact_display_modes` lists the ids `contact_display` accepts, with their
Turkish labels, in this order:

```json
{ "contact_display_modes": [
    { "id": "inline", "label": "Yan yana" },
    { "id": "list",   "label": "Açık liste" },
    { "id": "footer", "label": "Sadece alt bilgi" },
    { "id": "hidden", "label": "Hiç gösterme" } ] }
```

---

## Status code summary

| Code | Meaning |
|---|---|
| 200 | Success |
| 201 | Created |
| 400 | Malformed request body |
| 401 | Missing or invalid token |
| 403 | A menu of another business named as the target of a write — the `menu_id` of a category create or move, or of a bulk price update |
| 404 | Record or business not found — including a category of another business named by a product write, and a menu or category deleted while a write into it was running |
| 409 | Email or slug collision |
| 413 | File too large |
| 422 | Validation error (required field, invalid price, …) |
| 500 | Server error |
