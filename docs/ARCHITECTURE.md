# Karecik — Mimari

## Genel Bakış

Karecik çok kiracılı (multi-tenant) bir QR menü platformudur. Her kullanıcı hesabı
bir işletmeye karşılık gelir; her işletmenin kendi subdomain'i vardır.

```
                          ┌──────────────────────────────┐
 İşletme sahibi           │  React (Vite) — :5173        │
 karecik.com/panel  ─────▶│  · Landing (animasyonsuz)    │
                          │  · Yönetim paneli            │
 Müşteri                  │  · Müşteri menüsü            │
 kahve.karecik.com ──────▶│                              │
                          └──────────────┬───────────────┘
                                         │ /api  /uploads
                                         ▼
                          ┌──────────────────────────────┐
                          │  Go + Fiber — :8080          │
                          │  · JWT kimlik doğrulama      │
                          │  · Subdomain çözümleme       │
                          │  · CRUD + toplu fiyat        │
                          │  · Görsel yükleme            │
                          └──────────────┬───────────────┘
                                         │ pgx/v5 havuzu
                                         ▼
                          ┌──────────────────────────────┐
                          │  PostgreSQL — :5432          │
                          │  users · businesses          │
                          │  categories · products       │
                          │  price_update_logs           │
                          └──────────────────────────────┘
```

---

## Klasör Yapısı

```
karecik/
├── docs/
│   ├── KURULUM.md              Sıfırdan kurulum rehberi
│   ├── API.md                  API sözleşmesi (uçlar, gövdeler, hata kodları)
│   ├── MIMARI.md               Bu dosya
│   └── FRONTEND-SOZLESME.md    Ortak frontend modüllerinin imzaları
│
├── backend/
│   ├── cmd/api/main.go         Giriş noktası: config → db → migration → sunucu
│   ├── internal/
│   │   ├── config/             .env okuma, varsayılanlar
│   │   ├── database/           pgxpool bağlantısı, migration koşucusu, demo veri
│   │   ├── models/             Veri yapıları + public menü DTO'ları
│   │   ├── repository/         SQL katmanı (handler'lar SQL yazmaz)
│   │   ├── handlers/           HTTP uçları (tek Handler struct'ı, dosyalara bölünmüş)
│   │   ├── middleware/         JWT koruması, subdomain çözümleme
│   │   ├── router/             Rota kaydı, CORS, statik dosyalar
│   │   └── utils/              JWT, bcrypt, slug, para birimi, yuvarlama, tema kataloğu
│   ├── migrations/             001_init.sql + embed.go (ikili dosyaya gömülür)
│   └── uploads/                Yüklenen logolar ve ürün görselleri
│
└── frontend/
    └── src/
        ├── lib/                api.js, auth.jsx, format.js, subdomain.js
        ├── themes/             themes.js (6 tema), fonts.js (8 yazı tipi)
        ├── locales/            diller, alerjenler, müşteri menüsü metinleri
        ├── components/
        │   ├── ui/             Modal, OnayModal, Yukleniyor, BosDurum,
        │   │                   Bildirim (toast), GorselYukleyici
        │   ├── landing/        Header, IPhoneCerceve, KayitModal
        │   ├── dashboard/      KategoriSatiri, UrunSatiri, modallar, CanliOnizleme
        │   └── menu/           MenuIcerik, KarsilamaEkrani, UrunDetayModal, MenuFooter
        └── pages/
            ├── Landing.jsx · Giris.jsx · Kayit.jsx
            ├── dashboard/  PanelDuzeni · MenuEditoru · Tasarim · Ayarlar · QrKod
            └── menu/        MusteriMenusu
```

---

## Çok Kiracılılık (Multi-tenancy)

**Bir kullanıcı = bir işletme.** `businesses.user_id` üzerinde `UNIQUE` kısıtı vardır.

Kapsam (tenant scope) **her zaman JWT'den** gelir:

```go
businessID := middleware.BusinessID(c)   // token'daki "bid" claim'i
```

İstemci hiçbir uçta `business_id` göndermez. Tüm repository sorguları
`WHERE ... AND business_id = $n` içerir; başka bir işletmenin kaydına erişim
`404`/`403` ile sonuçlanır. Bu, en yaygın SaaS güvenlik açığını (IDOR) kapatır.

---

## Subdomain Yönlendirme

### Backend

`middleware.ExtractSubdomain(host, appDomain, devDomain)`:

| Host | Sonuç |
|---|---|
| `kahve-duragi.karecik.com` | `kahve-duragi` |
| `kahve-duragi.localhost:5173` | `kahve-duragi` |
| `www.karecik.com` | `""` (ayrılmış) |
| `karecik.com`, `localhost` | `""` |
| `127.0.0.1:8080` | `""` (IP) |

`GET /api/public/menu` bu fonksiyonla slug'ı çözer. Yol tabanlı erişim
(`GET /api/public/menu/:slug`) her zaman çalışır ve QR bağlantılarında yedektir.

### Frontend

`src/lib/subdomain.js` → `subdomainAl()` aynı mantığı tarayıcıda uygular.
`App.jsx` subdomain varsa **tüm yolları** müşteri menüsüne yönlendirir:

```jsx
const altAlan = subdomainAl()
if (altAlan) return <Routes><Route path="*" element={<MusteriMenusu slug={altAlan} />} /></Routes>
```

### Üretimde DNS

```
*.karecik.com   A   <sunucu-ip>
karecik.com     A   <sunucu-ip>
```

Wildcard SSL sertifikası gerekir (Let's Encrypt DNS-01 challenge).

---

## Veri Modeli

```
users ──1:1──▶ businesses ──1:N──▶ categories ──1:N──▶ products
                    │
                    └──1:N──▶ price_update_logs
```

### Çok dillilik: JSONB

Kategori ve ürün metinleri ayrı tablolarda değil, `translations` JSONB sütununda tutulur:

```json
{ "tr": { "name": "Türk Kahvesi", "description": "...", "ingredients": "..." },
  "en": { "name": "Turkish Coffee", "description": "..." } }
```

Neden: yeni dil eklemek şema değişikliği gerektirmez, tek satırda okunur,
JOIN maliyeti yoktur. Okurken `Translations.Resolve(lang, fallback)` çağrılır:
istenen dil → varsayılan dil → dolu olan ilk dil.

`allergens` da JSONB dizisidir: `["gluten", "sut"]`.

### Sıralama

`categories.position` ve `products.position` tam sayıdır. Sürükle-bırak sonrası
istemci **tüm listeyi** sırasıyla gönderir; backend tek sorguda yazar:

```sql
UPDATE categories c
SET position = data.ord - 1
FROM unnest($2::uuid[]) WITH ORDINALITY AS data(id, ord)
WHERE c.id = data.id AND c.business_id = $1
```

Ürünlerde aynı sorgu `category_id`'yi de günceller — böylece bir ürünü başka
kategoriye taşımak ile sıralamak aynı uçtan yapılır.

---

## Toplu Fiyat Güncelleme

`POST /api/products/bulk-price`

1. Seçili ürünler okunur (`ListPriceRows`).
2. Her fiyat için: `yeni = eski × (1 + yüzde/100)` → `RoundPrice(yeni, mod)` → `max(0, …)`.
3. `apply: false` ise sadece önizleme döner, veritabanı değişmez.
4. `apply: true` ise değişen fiyatlar tek transaction'da yazılır,
   `businesses.price_updated_at = now()` yapılır ve `price_update_logs`'a kayıt düşülür.

Yuvarlama modları (`internal/utils/pricing.go`):

| Mod | 147,60 → |
|---|---|
| `none` | 147,60 |
| `integer` | 148,00 |
| `nearest_5` | 150,00 |
| `nearest_10` | 150,00 |
| `ends_50` | 147,50 |
| `ends_95` | 147,95 |
| `ends_99` | 147,99 |

`price_updated_at`, müşteri menüsündeki
**"Fiyatlarımız 24.08.2026 tarihinden itibaren geçerlidir."** satırını besler —
işletme sahibinin elle tarih girmesi gerekmez.

---

## Tema ve Yazı Tipi

Tek kaynak: `backend/internal/utils/appearance.go`. Frontend aynı kimlikleri
`src/themes/themes.js` ve `src/themes/fonts.js` içinde taşır ve `GET /api/meta`
ile doğrulanabilir.

Menü render'ında tema **CSS değişkenlerine** çevrilir:

```js
const stil = temaDegiskenleri(business.theme, business.primary_color, fontStack(business.font_family))
// { '--menu-bg': ..., '--menu-text': ..., '--menu-primary': ..., ... }
```

Menü bileşenleri Tailwind renk sınıfı kullanmaz, `var(--menu-*)` okur.
Bu sayede 6 tema tek bir bileşen ağacıyla çalışır ve **canlı önizleme**
kaydedilmemiş değişiklikleri anında gösterebilir.

---

## Canlı Önizleme Nasıl Çalışır

`CanliOnizleme` bileşeni:

1. `GET /api/preview/menu` ile menüyü çeker (pasif kayıtlar da dahil).
2. Dönen `menu.business` alanını, panelde **henüz kaydedilmemiş** taslak
   `business` nesnesiyle birleştirir.
3. Sonucu müşteri menüsünün gerçek bileşeni olan `MenuIcerik`'e verir.

Yani önizleme bir taklit değil, müşterinin göreceği bileşenin ta kendisidir.

---

## Kimlik Doğrulama

- Şifreler `bcrypt` (varsayılan cost) ile hashlenir.
- `POST /api/auth/register|login` → HS256 imzalı JWT, 30 gün geçerli.
- Token `localStorage`'da (`karecik_token`) tutulur, `Authorization: Bearer` ile gönderilir.
- `401` dönen her yanıtta istemci token'ı temizler ve `/giris` sayfasına düşer.
- Kayıtta slug otomatik üretilir (`Kahve Durağı` → `kahve-duragi`), çakışırsa `-2`, `-3` eklenir.
  Ayrılmış adlar (`www`, `api`, `panel`, `admin`, …) verilmez.

---

## Migration'lar

`backend/migrations/*.sql` dosyaları `//go:embed` ile ikili dosyaya gömülür ve
sunucu açılışında sırayla uygulanır. Uygulananlar `schema_migrations` tablosunda
tutulur, her dosya kendi transaction'ında çalışır.

Yeni migration eklemek: `002_xxx.sql` dosyası oluştur — başka hiçbir şey gerekmez.

---

## Neden Bu Teknolojiler

| Karar | Gerekçe |
|---|---|
| **Fiber** (Gin yerine) | `c.Hostname()` ile subdomain çözümü doğrudan; fasthttp tabanlı, QR trafiği gibi çok sayıda kısa istek için hafif. |
| **pgx/v5** (GORM yerine) | JSONB, `unnest … WITH ORDINALITY`, `NUMERIC` gibi PostgreSQL özelliklerini ORM katmanı olmadan doğrudan kullanır; üretilen SQL sürpriz yapmaz. |
| **JSONB çeviri** | Yeni dil eklemek şema göçü gerektirmez. |
| **@dnd-kit** | react-beautiful-dnd artık bakımsız ve React 18 StrictMode ile sorunlu; dnd-kit erişilebilir (klavye desteği) ve bakımlı. |
| **Landing'de animasyon yok** | Ürün gereksinimi: sade, temiz, duruk karşılama ekranı. Bu yüzden hiçbir animasyon kütüphanesi bağımlılığa eklenmedi. |
