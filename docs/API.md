# Karecik API Contract

Base URL (development): `http://localhost:8080`

Every response is JSON — except `204 No Content`, which `POST /api/public/events`
answers with and which has no body. Errors share one shape:

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
| an option group name or an option name, in any supported language | `Seçenek listesi geçersiz.` |
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
toggling `is_active` / `is_featured`, renaming an option (in any language), or
moving a product without a new price does not move the date. Surcharges compare numerically:
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

#### Product options

`options` is a list of groups, on `POST` and `PUT` alike:

```json
[ { "name": "Süt Tercihi",
    "translations": { "tr": { "name": "Süt Tercihi" }, "en": { "name": "Milk" } },
    "type": "single", "required": true,
    "items": [
      { "name": "Yulaf Sütü", "translations": { "en": { "name": "Oat milk" } }, "price": 25 },
      { "name": "Laktozsuz", "price": 10 } ] } ]
```

`name` is the group's or item's name in the default language of the menu the
product sits on, and it is required (1–60 characters). `translations` is
optional and holds the names in the menu's other languages, 1–60 characters
each. The rules, applied by `handlers.SanitizeOptions`:

- A translation in a language that is not one of `tr en de ru ar fr`, and a
  blank one, is dropped.
- A blank `name` is taken from the default-language translation when there is
  one; with neither, the group is `422` `Seçenek grubunun adı zorunludur.` and
  the item `422` `Seçenek adı zorunludur.`. When both are sent and differ,
  `name` wins.
- A translation over 60 characters is `422`, naming its language:
  `Seçenek grubunun adı (EN) en fazla 60 karakter olabilir.` /
  `Seçenek adı (EN) en fazla 60 karakter olabilir.`.
- `translations` is stored only when a language other than the default one is
  left, and then with a copy of `name` under the default language, so a later
  change of the menu's `default_language` keeps the old default-language name
  under its own code. An option with no other language is stored exactly as
  before translations existed — `{ "name", "price" }` — and every option stored
  that way stays valid.

The owner's endpoints return `translations` as stored. The customer menu and the
preview resolve every name into the requested language — its translation, else
the default language's, else `name` — and carry no `translations` key (see
section 7).

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

`multipart/form-data`, field name: `file`. Max **5 MB** (`MAX_UPLOAD_BYTES`).
Accepted: JPEG, PNG, WebP, GIF and **SVG** (`image/svg+xml`).

Response `201`: `{ "url": "/uploads/1724500000-a1b2c3.webp", "size": 84213 }` —
`size` is the number of bytes stored.

The part's `Content-Type` (or, when it is empty or generic such as
`application/octet-stream`, the file name's extension — `.jpg`, `.jpeg`, `.png`,
`.webp`, `.gif`, `.svg`) only says which check the file goes through. Nothing is
written to disk until the check passes, so a refused file is never served.

**Raster images** are verified by their first bytes (Go's
`http.DetectContentType`). A file whose bytes are not a JPEG, PNG, GIF or WebP
image — an HTML page renamed `logo.png`, an SVG sent as `image/png`, an empty
file — is `422` `Dosyanın içeriği geçerli bir JPG, PNG, WEBP veya GIF görseli
değil.` A real image sent under the wrong image type is stored under the
extension of what it really is (a JPEG sent as `image/png` becomes `.jpg`).

**SVG** is parsed as XML (`internal/svgsafe`) and refused with `422`
`SVG dosyası betik, dış bağlantı veya gömülü içerik barındıramaz. Lütfen
sadeleştirilmiş bir SVG yükleyin.` when:

- it is not well-formed XML with an `<svg>` root in the SVG namespace (or none
  — but not an explicit `xmlns=""`), or has anything but an `<?xml?>`
  declaration, comments and whitespace outside that root; a charset other than
  UTF-8 is refused too;
- it uses a namespace prefix that nothing in scope declares (`sodipodi:` with
  no `xmlns:sodipodi`, …) — no browser draws such a file. `xlink:` is the
  exception, see below;
- it has a `<!DOCTYPE>` / `<!ENTITY>` or any other declaration, or a processing
  instruction other than the XML declaration (`<?xml-stylesheet?>`);
- it contains `script`, `foreignObject`, `iframe`, `embed`, `object`, `handler`
  or `listener` in any namespace, or any element of the XHTML namespace;
- any attribute's name starts with `on`;
- an `href`, `xlink:href` or `src` is anything but a same-document fragment
  (`#logo`) or a `data:image/(png|jpeg|gif|webp)` URI;
- an `animate`, `set`, `animateTransform` or `animateMotion` targets `href`,
  `xlink:href` or an `on*` attribute;
- any attribute value or stylesheet contains `javascript:`, `vbscript:` or
  `data:text` — entities are decoded, and whitespace and control characters
  inside the scheme are ignored, first;
- a stylesheet (`<style>` or a `style` attribute) uses `@import`,
  `expression(`, `behavior`, `-moz-binding` or a backslash escape, or any
  `url(...)` — in a stylesheet or a presentation attribute such as `fill` — is
  not a fragment or an image `data:` URI.

The rule that failed is written to the server log, not to the response. A clean
SVG is stored byte for byte, except that its `<svg>` root is given what a
browser needs to draw it as an image:

- a root with no namespace gets `xmlns="http://www.w3.org/2000/svg"`, and an
  `xlink:` prefix used without a declaration gets
  `xmlns:xlink="http://www.w3.org/1999/xlink"` — an HTML page supplies both
  implicitly, so SVG copied out of one often has neither, and without them the
  stored file draws nothing;
- a root that has a numeric (or `px`) `width` and `height` but no `viewBox`
  gets `viewBox="0 0 <width> <height>"`, so it scales inside an `<img>` instead
  of being cropped.

Files are written under `UPLOAD_DIR` and served statically at `GET /uploads/*`.
**Every** response for a stored file — `GET`, `HEAD`, a range (`206`) and a
revalidation (`304`) alike, however the address is spelled — carries
`X-Content-Type-Options: nosniff` and
`Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'; img-src data:; sandbox`,
so opening an uploaded file's URL directly as a page can never run script or
fetch anything; inside an `<img>` the policy changes nothing, and a raster
image opened directly still displays. The policy is not limited to `.svg`
addresses because an address can name the same file in more than one spelling
(percent-encoding, repeated slashes, letter case). An SVG is served as
`Content-Type: image/svg+xml`, decided by the file actually served.

Every accepted upload is recorded in the audit trail as `upload.create`
(section 11).

---

## 7. Public menu — no token required

### `GET /api/public/menu/:businessSlug[/:menuSlug]`

Query: `?lang=tr` (optional).

#### Language

The language of every text in the payload is **negotiated**, in this order:

1. `?lang=` when the menu's `languages` contains it (compared trimmed and
   lowercased) — the visitor's own choice in the language picker always wins;
2. otherwise the first language of the request's `Accept-Language` header, in
   quality order, that the menu's `languages` contains — regions and scripts are
   stripped (`de-DE` → `de`), `q=0` entries and the `*` wildcard are ignored, and
   a malformed entry drops only itself;
3. otherwise the menu's `default_language`.

A `?lang=` the menu does not offer (or an empty one) is treated as absent, so it
falls through to the header rather than pinning the visitor to the default.

The payload says which language it used in a top-level `"language"` field. A
directory payload (`menu_resolved: false`) carries it too, negotiated the same
way against every supported language (`tr`, `en`, `de`, `ru`, `ar`, `fr`) with
`tr` as the fallback, so the page can render its own strings in it.

Every answer of these endpoints — `200`, `304` and `404` alike — carries
`Vary: Accept-Language`, and because the `ETag` is computed from the body, a copy
cached for one language is never revalidated for another.

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
    "contact_in_footer": false,
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
    "price_date": "2026-08-24",
    "vat_note": "Fiyatlarımıza KDV dahildir.",
    "powered_by": "Karecik ile hazırlandı"
  },
  "language": "tr"
}
```

Important: on the public endpoints **translations are already resolved** — the
payload carries plain `name` / `description` fields instead of the
`translations` map. The same holds for the option groups and items of a
product: each carries its `name` in the requested language, falling back to the
menu's default language, and no `translations` key. Categories and products with `is_active = false` are
**omitted entirely**. Both lists are ordered by `position ASC`.

`footer.price_date` is the menu's `price_updated_at` as a bare `YYYY-MM-DD`
day, and `footer.price_note` the same day as a finished Turkish sentence in
`dd.MM.yyyy` format — both **on the Europe/Istanbul calendar**, never in the
server's own time zone, so a price changed at 01:30 Istanbul time names that day
and not the previous one. The customer menu builds its own, translated sentence
from `price_date`; `price_note` stays for older clients. Both are `""` when the
menu's `show_price_date` is `false`. The server binary embeds its own copy of the
zone database, so this does not depend on the host having one installed.

`footer.vat_note` is filled when the menu's `show_vat_note` is `true`: it is the
trimmed `vat_note_text`, or `Fiyatlarımıza KDV dahildir.` when that text is
blank — the sentence the dashboard's settings page shows as the placeholder of
an empty field. With `show_vat_note` off it is `""`.

`business.contact_display`, `business.contact_in_footer` and `business.links`
carry the menu's contact block settings — see section 8. When the block is drawn
**nowhere** — `contact_display` is `hidden` **and** `contact_in_footer` is
`false` — the payload itself leaves it out: `business.phone`,
`business.instagram`, `business.wifi_ssid` and `business.wifi_password` are
`null` and `business.links` is `[]`, whatever the menu stores. The owner still
reads the stored values through `GET /api/menus/:id`. `business.address` is not
part of the contact block and is sent in every mode, and every other combination
sends everything — it only changes where the block is drawn.

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
`404` for an unknown business or menu carries neither header (it does carry
`Vary: Accept-Language`).

### `GET /api/preview/menu` 🔒

Backs the **live preview** in the dashboard. It returns exactly the same body as
the public menu, with one difference: it uses the business from the token and
also includes inactive records, flagged with `"is_active": false` so the
dashboard can dim them.

The preview is built by the same code as the public menu, so a menu whose
contact block is drawn nowhere comes back redacted here too.

The preview reports its `"language"` too, but does **not** negotiate the owner's
`Accept-Language`: it is `?lang=` when the menu offers it, else the menu's
`default_language` — the preview follows the dashboard's language switch, not
the owner's browser.

---

## 8. Menus 🔒 — contact block and links

Every menu the dashboard API returns — `GET /api/menus`, `GET /api/menus/:id`,
and the menu in the response of `POST /api/menus` and `PUT /api/menus/:id` —
carries the three settings of the customer menu's contact block: the Wi-Fi,
Instagram and phone entries, followed by the owner's own links.

```json
{ "contact_display": "inline",
  "contact_in_footer": false,
  "links": [ { "id": "rezervasyon", "label": "Rezervasyon", "url": "https://rezervasyon.example/masa" } ] }
```

`links` is always an array — `[]` for a menu without links, never `null`. A new
menu starts with `"contact_display": "inline"`, `"contact_in_footer": false` and
`"links": []`.

### `contact_display`

Where the **home view** of the customer menu draws the block — and nothing else:

| Id | Label | Home view of the customer menu |
|---|---|---|
| `inline` | Yan yana | one row of chips — Wi-Fi, Instagram, Telefon, then each link; tapping a chip opens its details |
| `list` | Açık liste | the same entries as an always-open list |
| `hidden` | Ana sayfada gösterme | nothing |

`POST` and `PUT` accept these three ids, plus the **legacy** `footer`, which is
stored as the pair it always meant: `"contact_display": "hidden"` **and**
`"contact_in_footer": true` — also when the same body sends
`contact_in_footer: false`. Migration `012_menu_contact_in_footer.sql` converted
every stored `footer` row the same way. The database constraint still admits
`footer` for now — the previous release keeps serving, and writing it, while a
deploy starts this one — and a stored `footer` is read and answered as
`hidden` with `contact_in_footer: true`. A later migration narrows the
constraint once no release that writes it can still be running.
Anything else — another word, a different case, surrounding spaces, a number,
`null` — is `422` `Geçersiz iletişim görünümü.`

### `contact_in_footer`

`true` repeats the compact contact list in the footer of the product screens;
`false` — the default — leaves that footer to its standard content (the legal
notices, the "Yerli Üretim" badge, the price date and the credit). A boolean;
anything else is `422` `contact_in_footer alanı true/false olmalıdır.` (`null`
reads as `false`, like every other switch of a menu).

When `contact_display` is `hidden` and `contact_in_footer` is `false`, the block
is drawn nowhere and the public payload does not carry it at all (see
section 7).

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

`contact_display_modes` lists the ids `contact_display` stores, with their
Turkish labels, in this order (the legacy `footer` is accepted on write but not
listed — see section 8):

```json
{ "contact_display_modes": [
    { "id": "inline", "label": "Yan yana" },
    { "id": "list",   "label": "Açık liste" },
    { "id": "hidden", "label": "Ana sayfada gösterme" } ] }
```

---

## 10. Visitor analytics

### `POST /api/public/events`

Public, no session. The customer menu reports what a visitor looks at — usually
through `navigator.sendBeacon`, so the body is read as JSON **whatever the
`Content-Type`** (`text/plain` included). At most 2048 bytes (`413` above).

```json
{ "business_slug": "melly-coffee", "menu_slug": "ana-menu",
  "type": "product_view",
  "category_id": "uuid", "product_id": "uuid",
  "visitor_id": "k3J9x_2a", "language": "en" }
```

| Field | Rule |
|---|---|
| `business_slug`, `menu_slug` | required; must name a **published** menu of that business |
| `type` | `menu_view`, `category_view` or `product_view` |
| `category_id` | a uuid; required for `category_view`; must be a category of that menu |
| `product_id` | a uuid; required for `product_view`; must be a product in a category of that menu — and in `category_id`, when both are sent. Sent without `category_id`, the event is stored with the product's category |
| `visitor_id` | optional; 1–64 characters of `[A-Za-z0-9_-]` — a random id the page keeps in the browser |
| `language` | optional; one of the supported language codes |

Success is **`204 No Content`** with no body. A body that is not a JSON object
of strings is `400`; every other refusal — an unknown type, a missing or
malformed id, a menu that is not published, a category or product of another
menu or tenant, a bad `visitor_id` or `language` — is `422` and stores nothing.

`204` means **accepted**, not necessarily stored. A valid event is answered
`204` and **not** stored when:

- it is **not a customer's** — it comes from an address or range on the
  business's exclusion list, from one in the platform-wide
  `ANALYTICS_EXCLUDED_IPS`, or from a browser carrying the opt-out cookie
  `karecik_analytics_optout=1` (see "Excluded visits" below). This is decided
  before the two rules that follow, so such a view is never remembered as a
  repeat and never counts against the daily cap;
- it **repeats** an event of the same visitor (`visitor_id`, or address and
  user agent without one), type, menu, category and product sent less than
  **10 seconds** earlier — a double tap or a dialog opened twice is one look;
- its business has already stored **`ANALYTICS_DAILY_EVENT_CAP`** events
  (default `10000`; `0` = no cap) that Europe/Istanbul calendar day. The first
  such event of the day writes one line to the server log naming the business;
  the count survives a restart, because it starts from what the table holds.

Neither is signalled to the page, which has nothing to do differently.

Stored with each event: the visitor's address, **source port** and how the
address was established (`ip`, `port`, `ip_source` — the same resolver as the
request log line, see `CLIENT_PORT_HEADER` / `EDGE_SECRET`), the `User-Agent`
cut to 300 characters, the `language` and the time. Unique visitors are counted
by `visitor_id` when sent, otherwise by a hash of address and user agent.

Rate limited per client (`middleware.ClientIPKey`), with two budgets:

| Address | Budget | Why |
|---|---|---|
| proven (`EDGE_SECRET` matched) — the key is the visitor's own address | **120 a minute** | one person tapping as fast as they can read sends ~20; 120 is six of them behind one Wi-Fi or carrier NAT |
| anything else — the key may be a shared edge address | **600 a minute** | sized for a crowd behind one Cloudflare egress address |

Over it: `429` `RATE_LIMITED`. The budgets bound a rate; what bounds the
table is the repeat rule and the daily cap above.

**Retention.** These rows hold IP addresses and ports, which are personal data
under KVKK. Events older than `ANALYTICS_RETENTION_DAYS` (default `90`; `0`
keeps them forever; at most `3650` — a larger value is lowered to it) are
deleted at start-up and every 24 hours. Deleting a menu deletes its events.

### `GET /api/analytics/summary` 🔒

Query: `menu_id` (optional), `from`, `to` (optional, `YYYY-MM-DD`, inclusive,
**Europe/Istanbul** calendar days). With neither date the window is the last 30
days, today included; with only `from` it runs to today, with only `to` it is
the 30 days ending then. `from` after `to`, a window longer than 366 days, a
malformed date or a year outside 2000–9999 is `422`
(`Başlangıç tarihi YYYY-AA-GG biçiminde olmalıdır.` /
`Bitiş tarihi YYYY-AA-GG biçiminde olmalıdır.`); a malformed `menu_id` is
`400` and a menu of another business `404`.

```json
{ "from": "2026-08-27", "to": "2026-09-25",
  "total_visits": 412, "unique_visitors": 268,
  "menu_views": 412, "category_views": 903, "product_views": 1377,
  "daily": [ { "date": "2026-08-27", "visits": 12, "unique_visitors": 9,
               "category_views": 30, "product_views": 41 }, … ],
  "top_categories": [ { "id": "uuid", "name": "Kahveler", "views": 210 } ],
  "top_products": [ { "id": "uuid", "name": "Latte", "category_name": "Kahveler", "views": 96 } ] }
```

- `total_visits` = `menu_views` = the number of `menu_view` events.
- `unique_visitors` counts distinct visitors over **all** events of the window.
- `daily` has one entry per day of the window, empty days included (zeros).
- `top_categories` / `top_products`: at most 10 each, most viewed first, among
  the records that still exist; names are in the default language of the menu
  the record is on. A deleted record's views still count in the totals.

### `GET /api/analytics/events` 🔒

Query (all optional): `menu_id`, `type`, `from`, `to` (as above, either may be
left out; the same 2000–9999 year range), `ip` (a prefix of hex digits, `.`
and `:` — `85.105.` matches a whole block), `limit` (default 50, at most 200 —
a larger value is lowered), `offset` (default 0). Anything malformed is `422`;
another business's `menu_id` is `404`.

```json
{ "items": [
    { "id": "uuid", "created_at": "2026-09-25T14:03:11.52+03:00",
      "type": "product_view",
      "menu_id": "uuid", "menu_name": "Ana Menü",
      "category_id": "uuid", "category_name": "Kahveler",
      "product_id": "uuid", "product_name": "Latte",
      "ip": "85.105.12.34", "port": 51234, "ip_source": "cloudflare",
      "visitor_id": "k3J9x_2a", "language": "en",
      "user_agent": "Mozilla/5.0 (iPhone; …)" } ],
  "total": 1377, "limit": 50, "offset": 0 }
```

Newest first. `ip` and `port` are `null` when they could not be established
(`port` is known only for requests proven to come through Cloudflare — see
`EDGE_SECRET`). `category_name` / `product_name` are `null` when that record has
been deleted since; the ids stay.

Both dashboard reads answer `Cache-Control: no-store` and only ever read the
session's own business.

### Excluded visits

The owner's own visits — and the platform operator's — are never stored. Three
mechanisms, all enforced by `POST /api/public/events` itself (a `204` that
stores nothing, before the repeat rule and the daily cap):

| Mechanism | Who manages it | Matches |
|---|---|---|
| the business's **exclusion list** | the owner, below | a visit to **this business's** menus from an address inside one of its entries |
| **`ANALYTICS_EXCLUDED_IPS`** | the platform operator (environment) | a visit to **any** business's menu from an address inside one of its entries. Never exposed by any endpoint |
| the **opt-out cookie** `karecik_analytics_optout=1` | the owner's browser, below | every visit from that browser, to any menu. The customer page does not even send them |

The address matched is the one the visit log shows (`ip` of
`GET /api/analytics/events`, from the same resolver). An address that could not
be established never matches.

#### `GET /api/analytics/excluded-ips` 🔒

```json
{ "items": [
    { "id": "uuid", "cidr": "198.18.139.87/32", "display": "198.18.139.87",
      "label": "Kasa", "created_at": "2026-09-26T10:12:03.4+03:00",
      "created_by_email": "owner@example.com" },
    { "id": "uuid", "cidr": "203.0.113.0/24", "display": "203.0.113.0/24",
      "label": "", "created_at": "…", "created_by_email": "owner@example.com" } ],
  "current_ip": "198.18.139.87", "current_ip_source": "cloudflare",
  "current_ip_excluded": true, "optout": false, "max": 50 }
```

- `items` — the session's business only, oldest first. `cidr` is always the
  CIDR text; `display` is the bare address for a single host (`/32`, `/128`),
  the CIDR text otherwise. `created_by_email` is `null` once that user is gone.
- `current_ip` — the address **this request** was resolved to (what the panel
  offers to add), `null` when unknown; `current_ip_source` says how it was
  established (`cloudflare`, `cloudflare-unverified`, `edge`, `peer`,
  `unknown` …).
- `current_ip_excluded` — whether **this business's** list covers
  `current_ip`. The platform list is not consulted, so the answer says nothing
  about it.
- `optout` — whether this request carries the opt-out cookie.
- `max` — the most entries a business may have (`50`).

`Cache-Control: no-store`.

#### `GET /api/analytics/excluded-ips/match-count?cidr=` 🔒

How many of this business's **stored** events fall inside the address or range —
what adding it with `delete_history: true` would delete.

```json
{ "cidr": "198.18.139.0/24", "count": 42 }
```

`cidr` comes back normalised (see below). The input obeys exactly the rules of
an add: missing, malformed or too broad is `422`.

#### `POST /api/analytics/excluded-ips` 🔒

```json
{ "cidr": "198.18.139.87", "label": "Kasa", "delete_history": true }
```

| Field | Rule |
|---|---|
| `cidr` | required; an IPv4 or IPv6 address, or a CIDR range. Normalised: host bits masked (`198.18.139.87/24` → `198.18.139.0/24`), IPv4-mapped IPv6 unmapped (`::ffff:1.2.3.4` → `1.2.3.4/32`), IPv6 lowercased and compressed; a single address is stored as `/32` or `/128`. No zone (`%eth0`), no port. **At most `/16` wide for IPv4 and `/48` for IPv6** — a broader range would exclude a whole provider's customers |
| `label` | optional; trimmed, at most 60 characters, no control characters |
| `delete_history` | **required** boolean: `true` also deletes this business's stored events from inside the range; `false` keeps them |

**`201`**

```json
{ "item": { "id": "uuid", "cidr": "198.18.139.87/32", "display": "198.18.139.87",
            "label": "Kasa", "created_at": "…", "created_by_email": "owner@example.com" },
  "deleted_events": 42 }
```

The entry, the history deletion and the audit row are **one transaction**, and
the events endpoint sees the change on the very next event (its cached copy of
the list is dropped on commit). Only this business's events are deleted; a
stored `ip` that is not an address (`unknown`, `-`, empty) never matches and
never fails the request.

| Status | When |
|---|---|
| `400` | the body is not JSON of the right types |
| `409` `Bu IP zaten listede.` | the range is already listed, or lies inside an entry that is (`198.18.139.87` under `198.18.139.0/24`). A broader range over narrower entries is accepted |
| `422` | `cidr` missing, malformed or too broad; `label` too long or with control characters; `delete_history` missing; the list already has 50 entries |

#### `DELETE /api/analytics/excluded-ips/:id` 🔒

`204`. A malformed id is `400`; an id the business does not have — another
tenant's included — is `404` `Bu IP listede bulunamadı.` Removing an entry does
not bring back visits that were not stored.

#### `POST /api/analytics/optout` · `DELETE /api/analytics/optout` 🔒

`204`, with the opt-out cookie set (`POST`) or expired (`DELETE`):

```
Set-Cookie: karecik_analytics_optout=1; max-age=31536000; domain=karecik.com; path=/; secure; SameSite=Lax
```

- `Domain` is `APP_DOMAIN` in production, so every tenant's
  `{slug}.karecik.com` and the path-form menu on the apex receive it; in
  development the cookie is host-only.
- `Secure` exactly when the session cookie is (`COOKIE_SECURE`).
- **Not** `HttpOnly`: the customer page reads it and sends nothing. It holds no
  secret — a `1` that grants nothing.
- The `DELETE` writes the same cookie, empty and already expired, with the same
  `Domain`, `Path`, `SameSite` and `Secure`.

Nothing is written to the database or the audit trail: the mark belongs to a
browser, not to the business.

---

## 11. Audit trail 🔒

### `GET /api/audit-logs`

Query (all optional): `entity_type`, `action`, `limit` (default 50, at most
200), `offset`. An unknown `entity_type` or `action` is `422`
`Geçersiz kayıt türü.` / `Geçersiz işlem türü.`

```json
{ "items": [
    { "id": "uuid", "created_at": "2026-09-25T14:03:11.52+03:00",
      "user_id": "uuid", "user_email": "owner@example.com",
      "action": "menu.update", "entity_type": "menu",
      "entity_id": "uuid", "entity_label": "Ana Menü",
      "changes": {
        "phone": { "old": "+90 555 000 00 00", "new": "+90 555 111 11 11" },
        "wifi_password": { "old": "••••", "new": "••••" },
        "logo_url": { "old": null, "new": "/uploads/1790000000-ab12cd34.svg" } },
      "ip": "85.105.12.34", "port": 51234, "ip_source": "cloudflare" } ],
  "total": 214, "limit": 50, "offset": 0 }
```

Newest first, the session's business only. One row per successful write:

| `action` | `entity_type` | `entity_id` / `entity_label` | `changes` |
|---|---|---|---|
| `product.create` | product | the product / its name | every field the product carries, `old: null` |
| `product.update` | product | the product / its name | only the fields that changed; translations as `translations.<lang>.<field>` |
| `product.price` | product | the product / its name | `price` (the inline quick edit) |
| `product.delete` | product | the product / its name | a snapshot of the product, `new: null` |
| `product.bulk_price` | product | `null` / the menu's name | `percentage`, `rounding`, `affected`, `category_ids` (when narrowed) and `prices`: `old` and `new` lists of `{id, name, price}` for every product whose price moved |
| `product.reorder` | product | the target category / its name | `order`: the product names in their new order |
| `category.create` / `category.update` / `category.delete` | category | the category / its name | as for products; a delete adds `deleted_products` |
| `category.reorder` | category | `null` / `""` | `order`: the category names in their new order |
| `menu.create` | menu | the menu / its name | the settings the request named |
| `menu.update` | menu | the menu / its name | **every** setting that changed — logo, cover, phone, address, Instagram, Wi-Fi, links, `contact_display`, `contact_in_footer`, theme, font, colours, languages, splash, header, VAT / price-date / Yerli Üretim switches, slug, publication, … |
| `menu.delete` | menu | the menu / its name | `name`, `slug` |
| `business.update` | business | the business / its name | `name`, `slug` |
| `account.password_change` | account | the user / the e-mail | `method`: `dashboard` or `reset_link` |
| `upload.create` | upload | the stored file name / the uploaded file's own name | `url`, `size`, `content_type` |
| `analytics.exclude_ip.add` | analytics_exclusion | the entry / its `display` | `cidr` (the CIDR text), `label` (when not empty), `deleted_events` (0 without `delete_history`) — all `old: null` |
| `analytics.exclude_ip.remove` | analytics_exclusion | the entry / its `display` | `cidr`, `label` (when not empty) — `new: null` |

A save that changes nothing, a re-sent identical price and a bulk update that
moves no price record nothing; a refused request records nothing. A Wi-Fi
password is always stored as `"••••"` (`null` when there is none), and an
account password is never stored in any form — not the old one, not the new
one, not a hash.

`user_email` and `entity_label` are snapshots taken when the row was written.
`ip`, `port` and `ip_source` come from the same resolver as the request log
line. Every row except `upload.create` is written **inside the transaction of
the write it describes**, so a write and its row commit or roll back together
and a write retried after a lock conflict is recorded once. The `old` side of
an update, and a delete's snapshot, are read in that same transaction under
the write's own row lock, so a change another request committed a moment
earlier is never reported as this one's, and a change that puts back what
another request had just changed is still recorded.

---

## Status code summary

| Code | Meaning |
|---|---|
| 200 | Success |
| 201 | Created |
| 204 | Event accepted (`POST /api/public/events`) — no body; an excluded visit, a repeat or an event past the daily cap is accepted without being stored. Also a removed exclusion and the opt-out endpoints |
| 400 | Malformed request body |
| 401 | Missing or invalid token |
| 403 | A menu of another business named as the target of a write — the `menu_id` of a category create or move, or of a bulk price update |
| 404 | Record or business not found — including a category of another business named by a product write, and a menu or category deleted while a write into it was running |
| 409 | Email or slug collision; an analytics exclusion that is already listed |
| 413 | File too large, or an event body over 2048 bytes |
| 422 | Validation error (required field, invalid price, an unsafe SVG, a raster file whose bytes are not an image, …) |
| 429 | Rate limited (`RATE_LIMITED`) — password reset, events |
| 500 | Server error |
