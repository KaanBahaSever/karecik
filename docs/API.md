# Karecik API Contract

Base URL (development): `http://localhost:8080`

Every response is JSON. Errors share one shape:

```json
{ "error": "Human readable message", "code": "VALIDATION_ERROR" }
```

> The `error` text is written in **Turkish**, because it is displayed directly
> to the end user. The `code` is a stable machine-readable identifier.

Error codes: `VALIDATION_ERROR`, `UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`,
`CONFLICT`, `INTERNAL_ERROR`, `PAYLOAD_TOO_LARGE`.

Protected endpoints require an `Authorization: Bearer <token>` header.

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

Response `201`:
```json
{
  "token": "eyJhbGciOi...",
  "user": { "id": "uuid", "email": "info@kahve.com", "business_name": "Kahve Durağı" },
  "business": { "...Business object..." }
}
```

Error `409`: the email is already registered.

### `POST /api/auth/login`

Request: `{ "email": "...", "password": "..." }`
Response `200`: the same body as register.
Error `401`: `E-posta veya şifre hatalı.`

### `GET /api/auth/me` 🔒

Response: `{ "user": {...}, "business": {...} }`

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
