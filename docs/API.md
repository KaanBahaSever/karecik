# Karecik API Sözleşmesi

Base URL (geliştirme): `http://localhost:8080`

Tüm yanıtlar JSON. Hata yanıtları tek biçimdir:

```json
{ "error": "Insan tarafından okunabilir mesaj", "code": "VALIDATION_ERROR" }
```

Hata kodları: `VALIDATION_ERROR`, `UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`,
`CONFLICT`, `INTERNAL_ERROR`, `PAYLOAD_TOO_LARGE`.

Korumalı uçlar `Authorization: Bearer <token>` başlığı ister.

---

## Ortak Tipler

### Çeviri sözlüğü (`translations`)

Kategori ve ürün adları çok dillidir. Şema:

```json
{ "tr": { "name": "Türk Kahvesi", "description": "Geleneksel..." },
  "en": { "name": "Turkish Coffee", "description": "Traditional..." } }
```

Okuma sırasında istenen dil yoksa işletmenin `default_language` değerine düşülür.

### Para birimi kodları

| Kod | Simge | Konum |
|---|---|---|
| `TRY` | ₺ | son (`100,00 ₺`) |
| `USD` | $ | baş (`$100.00`) |
| `EUR` | € | baş |
| `GBP` | £ | baş |
| `AZN` | ₼ | son |
| `RUB` | ₽ | son |
| `SAR` | ﷼ | son |
| `AED` | د.إ | son |

Varsayılan: `TRY`.

### Alerjenler (`allergens`) — sabit kod listesi

`gluten`, `sut`, `yumurta`, `findik`, `yer_fistigi`, `soya`, `balik`, `kabuklu_deniz`,
`susam`, `hardal`, `kereviz`, `sulfit`, `aci`, `vejetaryen`, `vegan`, `alkol`, `kafein`

---

## 1. Sağlık

### `GET /api/health`

```json
{ "status": "ok", "database": "up", "version": "1.0.0" }
```

---

## 2. Kimlik Doğrulama

### `POST /api/auth/register`

İstek:
```json
{ "business_name": "Kahve Durağı", "email": "info@kahve.com", "password": "sifre1234" }
```

Kurallar: `business_name` 2–100 karakter, `email` geçerli ve benzersiz,
`password` en az 8 karakter.

Kayıt sırasında işletme kaydı ve `slug` (subdomain) otomatik üretilir:
`Kahve Durağı` → `kahve-duragi`. Çakışma olursa sonuna `-2`, `-3` eklenir.

Yanıt `201`:
```json
{
  "token": "eyJhbGciOi...",
  "user": { "id": "uuid", "email": "info@kahve.com", "business_name": "Kahve Durağı" },
  "business": { "...Business nesnesi..." }
}
```

Hata `409`: e-posta zaten kayıtlı.

### `POST /api/auth/login`

İstek: `{ "email": "...", "password": "..." }`
Yanıt `200`: register ile aynı gövde.
Hata `401`: `E-posta veya şifre hatalı.`

### `GET /api/auth/me` 🔒

Yanıt: `{ "user": {...}, "business": {...} }`

---

## 3. İşletme (Business) 🔒

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
  "primary_color": "#1a7f5a",
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

Kısmi güncelleme (gönderilen alanlar uygulanır). Güncellenebilir alanlar:
`name`, `slug`, `logo_url`, `cover_url`, `currency`, `theme`, `font_family`,
`primary_color`, `default_language`, `languages`, `splash_enabled`, `splash_duration`,
`splash_bg_color`, `splash_text`, `show_vat_note`, `vat_note_text`, `show_price_date`,
`phone`, `address`, `instagram`, `wifi_password`.

`slug` değişirse benzersizlik kontrol edilir → çakışırsa `409`.

Yanıt: güncel Business nesnesi.

---

## 4. Kategoriler 🔒

Business nesnesi token'dan çözülür; hiçbir uçta `business_id` gönderilmez.

### `GET /api/categories`

`position ASC` sıralı dizi. Her kategori `product_count` içerir.

```json
[ { "id": "uuid", "translations": {"tr":{"name":"Sıcak İçecekler","description":""}},
    "image_url": null, "icon": "coffee", "position": 0, "is_active": true,
    "product_count": 8, "created_at": "...", "updated_at": "..." } ]
```

### `POST /api/categories`

```json
{ "translations": { "tr": { "name": "Tatlılar", "description": "" } },
  "icon": "cake", "image_url": null, "is_active": true }
```

`position` otomatik olarak son sıra + 1 atanır. Yanıt `201`: kategori nesnesi.
Varsayılan dilde `name` boşsa `422`.

### `PUT /api/categories/:id`

Kısmi güncelleme: `translations`, `icon`, `image_url`, `is_active`.

### `DELETE /api/categories/:id`

Kategoriye bağlı ürünler de silinir (`ON DELETE CASCADE`). Yanıt `200`:
`{ "success": true, "deleted_products": 8 }`

### `PUT /api/categories/reorder`

Sürükle-bırak sonrası çağrılır.

```json
{ "ids": ["uuid-3", "uuid-1", "uuid-2"] }
```

Dizideki sıra `position` olarak yazılır (0,1,2...). Tek transaction.
Yanıt: güncel kategori dizisi.

---

## 5. Ürünler 🔒

### `GET /api/products`

Query: `?category_id=uuid` (opsiyonel), `?search=metin` (opsiyonel).
Sıralama: `category position ASC, product position ASC`.

```json
[ { "id": "uuid", "category_id": "uuid",
    "translations": { "tr": { "name": "Latte", "description": "...", "ingredients": "Espresso, süt" } },
    "price": 145.00, "compare_price": null, "image_url": "/uploads/x.jpg",
    "allergens": ["sut", "kafein"], "is_active": true, "is_featured": false,
    "position": 0, "created_at": "...", "updated_at": "..." } ]
```

> `price` JSON'da **number** olarak döner (string değil), 2 ondalık hassasiyet.

### `POST /api/products`

```json
{ "category_id": "uuid",
  "translations": { "tr": { "name": "Latte", "description": "", "ingredients": "" } },
  "price": 145, "compare_price": null, "image_url": null,
  "allergens": ["sut"], "is_active": true, "is_featured": false }
```

`category_id` zorunlu ve aynı işletmeye ait olmalı (değilse `403`).
`price >= 0` zorunlu. Yanıt `201`.

### `PUT /api/products/:id`

Kısmi güncelleme: `category_id` (başka kategoriye taşıma), `translations`, `price`,
`compare_price`, `image_url`, `allergens`, `is_active`, `is_featured`.

### `PATCH /api/products/:id/price`

Hızlı fiyat değişimi (menü editöründeki satır içi düzenleme).

İstek: `{ "price": 155.50 }` → Yanıt: güncel ürün.

### `DELETE /api/products/:id`

Yanıt: `{ "success": true }`

### `PUT /api/products/reorder`

```json
{ "category_id": "uuid", "ids": ["uuid-2", "uuid-1"] }
```

Ürünler `category_id` içindeki sıraya göre yeniden konumlanır. Sürükleyerek **başka
kategoriye taşıma** da bu uçla yapılır: ürünün `category_id`'si listede verilen
kategoriye güncellenir. Yanıt: o kategorinin güncel ürün dizisi.

### `POST /api/products/bulk-price` — Toplu Fiyat Güncelleme

```json
{ "percentage": 10,
  "rounding": "nearest_5",
  "category_ids": ["uuid-1"],
  "apply": true }
```

| Alan | Açıklama |
|---|---|
| `percentage` | −90 … +1000 arası. `10` → %10 zam, `-15` → %15 indirim |
| `rounding` | `none`, `integer` (tam sayı), `nearest_5` (5'in katı), `nearest_10`, `ends_99` (…,99), `ends_95`, `ends_50` (0,50'nin katı) |
| `category_ids` | Boş/eksikse **tüm** ürünler |
| `apply` | `false` → sadece önizleme, veritabanı değişmez |

Hesap sırası: `yeni = eski * (1 + yüzde/100)` → sonra yuvarlama → `max(0, sonuç)`.

Yanıt:
```json
{ "applied": true, "affected": 42,
  "preview": [ { "id": "uuid", "name": "Latte", "old_price": 145.00, "new_price": 160.00 } ],
  "price_updated_at": "2026-08-24T12:30:00Z" }
```

`apply: true` ise `businesses.price_updated_at` = şimdi olarak güncellenir
(müşteri menüsündeki "Fiyatlarımız … tarihinden itibaren geçerlidir" satırını besler).

---

## 6. Dosya Yükleme 🔒

### `POST /api/uploads`

`multipart/form-data`, alan adı: `file`.
İzinli tipler: `image/jpeg`, `image/png`, `image/webp`, `image/gif`. Maksimum **5 MB**.

Yanıt `201`: `{ "url": "/uploads/1724500000-a1b2c3.webp", "size": 84213 }`

Dosyalar `UPLOAD_DIR` altına kaydedilir, `GET /uploads/*` ile statik sunulur.

---

## 7. Genel (Public) Menü — token istemez

### `GET /api/public/menu/:slug`

Query: `?lang=tr` (opsiyonel, varsayılan işletmenin `default_language` değeri).

### `GET /api/public/menu` (subdomain ile)

`Host` başlığından subdomain çözülür: `kahve-duragi.karecik.com` → slug `kahve-duragi`.
Subdomain yoksa/`www` ise `404`.

Her iki uç da aynı gövdeyi döner:

```json
{
  "business": {
    "name": "Kahve Durağı", "slug": "kahve-duragi", "logo_url": "/uploads/logo.png",
    "currency": "TRY", "currency_symbol": "₺", "theme": "modern-light",
    "font_family": "inter", "primary_color": "#1a7f5a",
    "default_language": "tr", "languages": ["tr", "en"],
    "splash_enabled": true, "splash_duration": 1200,
    "splash_bg_color": "#0f172a", "splash_text": "Hoş geldiniz",
    "show_vat_note": true, "vat_note_text": "Fiyatlarımıza KDV dahildir.",
    "show_price_date": true, "price_updated_at": "2026-08-24T10:00:00Z",
    "phone": "...", "address": "...", "instagram": "...", "wifi_password": "..."
  },
  "categories": [
    { "id": "uuid", "name": "Sıcak İçecekler", "description": "", "icon": "coffee",
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

Önemli: public uçta **çeviriler çözülmüştür** — `translations` sözlüğü yerine düz
`name`/`description` alanları döner. `is_active = false` olan kategori/ürünler
**hiç görünmez**. Kategoriler ve ürünler `position ASC` sıralıdır.

`footer.price_note` backend'de `price_updated_at` alanından `dd.MM.yyyy` biçiminde üretilir.

### `GET /api/preview/menu` 🔒

Yönetim panelindeki **Canlı Önizleme** için. Public menü ile birebir aynı gövdeyi
döner, farkı: token'daki işletmeyi kullanır ve `is_active = false` kayıtları da
`"is_active": false` işaretiyle döndürür (panelde soluk gösterilir).

---

## Durum Kodları Özeti

| Kod | Anlam |
|---|---|
| 200 | Başarılı |
| 201 | Oluşturuldu |
| 400 | Bozuk istek gövdesi |
| 401 | Token yok/geçersiz |
| 403 | Başka işletmenin kaydına erişim |
| 404 | Kayıt/işletme yok |
| 409 | E-posta veya slug çakışması |
| 413 | Dosya çok büyük |
| 422 | Doğrulama hatası (zorunlu alan, geçersiz fiyat vb.) |
| 500 | Sunucu hatası |
