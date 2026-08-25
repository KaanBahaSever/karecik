# Frontend Sözleşmesi

Bu dosya, `frontend/src` altındaki **hazır omurga modüllerinin** dışa açtığı arayüzü
tanımlar. Yeni sayfa/bileşen yazarken bu imzalara birebir uy — hiçbirini değiştirme.

Tüm kod **JavaScript + JSX** (TypeScript yok), **React 18**, **react-router-dom v6**,
**Tailwind CSS v3**, ikonlar **lucide-react**, sürükle-bırak **@dnd-kit**.
Değişken/fonksiyon adları ve arayüz metinleri **Türkçe**.

---

## `src/lib/api.js`

```js
import api, { ApiError, getToken, setToken, clearToken } from '../lib/api'
```

| Çağrı | Döner |
|---|---|
| `api.meta()` | `{ currencies, themes, fonts, allergens, languages, rounding_modes }` |
| `api.health()` | `{ status, database, version }` |
| `api.register({ business_name, email, password })` | `{ token, user, business }` |
| `api.login({ email, password })` | `{ token, user, business }` |
| `api.me()` | `{ user, business }` |
| `api.getBusiness()` | Business |
| `api.updateBusiness(payload)` | Business |
| `api.listCategories()` | `Category[]` |
| `api.createCategory(payload)` | Category |
| `api.updateCategory(id, payload)` | Category |
| `api.deleteCategory(id)` | `{ success, deleted_products }` |
| `api.reorderCategories(ids)` | `Category[]` |
| `api.listProducts({ category_id, search })` | `Product[]` |
| `api.createProduct(payload)` | Product |
| `api.updateProduct(id, payload)` | Product |
| `api.updateProductPrice(id, price)` | Product |
| `api.deleteProduct(id)` | `{ success }` |
| `api.reorderProducts(categoryId, ids)` | `Product[]` |
| `api.bulkPrice({ percentage, rounding, category_ids, apply })` | `{ applied, affected, preview[], price_updated_at }` |
| `api.upload(file)` | `{ url, size }` |
| `api.previewMenu(lang)` | PublicMenu |
| `api.publicMenu(slug, lang)` | PublicMenu |

Hata durumunda `ApiError` fırlatır: `.message` (Türkçe), `.status`, `.code`.
Her çağrıyı `try/catch` ile sar ve `bildirim.hata(err.message)` ile göster.

### Veri şekilleri

```js
Business = { id, name, slug, logo_url, cover_url, currency, theme, font_family,
  primary_color, default_language, languages: ['tr'], splash_enabled, splash_duration,
  splash_bg_color, splash_text, show_vat_note, vat_note_text, show_price_date,
  price_updated_at, phone, address, instagram, wifi_password, is_active, menu_url }

Category = { id, translations: { tr: { name, description } }, icon, image_url,
  position, is_active, product_count }

Product = { id, category_id, translations: { tr: { name, description, ingredients } },
  price: 145.0, compare_price, image_url, allergens: ['sut'], is_active, is_featured, position }

PublicMenu = {
  business: { name, slug, logo_url, currency, currency_symbol, theme, font_family,
              primary_color, default_language, languages, splash_enabled, splash_duration,
              splash_bg_color, splash_text, show_vat_note, vat_note_text, show_price_date,
              price_updated_at, phone, address, instagram, wifi_password },
  categories: [{ id, name, description, icon, image_url, is_active,
                 products: [{ id, name, description, ingredients, price, compare_price,
                              image_url, allergens, is_featured, is_active }] }],
  footer: { price_note, vat_note, powered_by },
}
```

> Public menüde çeviriler **çözülmüştür**: `translations` yerine düz `name`/`description`.

---

## `src/lib/auth.jsx`

```js
import { useAuth } from '../lib/auth.jsx'

const { user, business, loading, isAuthenticated,
        login, register, logout, saveBusiness, refreshBusiness } = useAuth()
```

- `login(email, password)` → Promise
- `register(businessAdi, email, sifre)` → Promise
- `saveBusiness(payload)` → `api.updateBusiness` çağırır **ve context'i tazeler**.
  İşletme ayarı değiştiren her yer bunu kullanmalı (canlı önizleme buna bakar).

---

## `src/lib/format.js`

```js
import { fiyatBicimle, paraSimgesi, tarihBicimle, fiyatCozumle, fiyatGirdiye,
         PARA_BIRIMLERI, PARA_BIRIMI_LISTESI } from '../lib/format'
```

- `fiyatBicimle(145, 'TRY')` → `"145,00 ₺"`
- `fiyatCozumle('12,50')` → `12.5`
- `fiyatGirdiye(145)` → `"145,00"`
- `tarihBicimle(iso)` → `"24.08.2026"`

## `src/lib/subdomain.js`

- `subdomainAl()` → `"kahve-duragi"` veya `null`
- `menuAdresi(slug)` → `http://kahve-duragi.localhost:5173`
- `menuYoluAdresi(slug)` → `http://localhost:5173/m/kahve-duragi`

## `src/themes/themes.js`

- `TEMALAR` → `[{ id, label, description, dark, renkler{...}, stil{...} }]`
- `temaBul(id)`, `VARSAYILAN_TEMA`
- `temaDegiskenleri(temaVeyaId, vurguRengi, fontStack)` → menü kapsayıcısına
  `style={...}` olarak verilecek CSS değişkenleri (`--menu-bg`, `--menu-text`,
  `--menu-primary`, `--menu-surface`, `--menu-muted`, `--menu-border`,
  `--menu-radius`, `--menu-shadow`, `--menu-font`).

## `src/themes/fonts.js`

- `FONTLAR`, `fontBul(id)`, `fontStack(id)`, `fontYukle(id)`, `tumFontlariYukle()`

## `src/locales/index.js`

- `DILLER`, `dilBul(code)`, `sagdanSolaMi(code)`
- `ALERJENLER` → `[{ code, emoji, tr, en }]`, `alerjenBul(code)`, `alerjenEtiketi(code, dil)`
- `metin(anahtar, dil)` → müşteri menüsü arayüz metni

---

## Ortak bileşenler (`src/components/ui/`)

```jsx
import Modal from '../ui/Modal.jsx'
<Modal acik kapat={fn} baslik="" aciklama="" genislik="max-w-lg" altBilgi={<>...</>}>gövde</Modal>

import OnayModal from '../ui/OnayModal.jsx'
<OnayModal acik kapat={fn} onayla={fn} baslik="" mesaj="" onayMetni="Sil" islemde={false} />

import Yukleniyor from '../ui/Yukleniyor.jsx'
<Yukleniyor tamEkran metin="..." />

import BosDurum from '../ui/BosDurum.jsx'
<BosDurum ikon={LucideIcon} baslik="" aciklama="" aksiyon={<button/>} />

import GorselYukleyici from '../ui/GorselYukleyici.jsx'
<GorselYukleyici deger={url|null} degisti={(url)=>{}} etiket="" ipucu="" yuvarlak={false} />

import { useBildirim } from '../ui/Bildirim.jsx'
const bildirim = useBildirim()   // bildirim.basari(msg) / .hata(msg) / .bilgi(msg)
```

---

## Ortak CSS sınıfları (`src/index.css`)

`btn`, `btn-birincil`, `btn-ikincil`, `btn-sessiz`, `btn-tehlike`, `btn-kucuk`,
`girdi`, `etiket`, `yardim`, `hata-metni`, `kart`, `rozet`,
`surukleniyor`, `kaydirma-gizli`

Marka renkleri: `bg-marka-600`, `text-marka-600`, `border-marka-600` (50–900 tonları).
Gölgeler: `shadow-kart`, `shadow-panel`. Genişlik: `max-w-icerik`.

---

## Yönlendirme (`src/App.jsx` — hazır, değiştirme)

| Yol | Bileşen |
|---|---|
| `/` | `pages/Landing.jsx` |
| `/giris` | `pages/Giris.jsx` |
| `/kayit` | `pages/Kayit.jsx` |
| `/m/:slug` | `pages/menu/MusteriMenusu.jsx` |
| `/demo` | `MusteriMenusu` (`slug="demo-kafe" gomulu`) |
| `/panel` | `pages/dashboard/PanelDuzeni.jsx` (Outlet) |
| `/panel` (index) | `pages/dashboard/MenuEditoru.jsx` |
| `/panel/tasarim` | `pages/dashboard/Tasarim.jsx` |
| `/panel/ayarlar` | `pages/dashboard/Ayarlar.jsx` |
| `/panel/qr` | `pages/dashboard/QrKod.jsx` |

Subdomain ile gelinirse (`kahve-duragi.localhost`) tüm yollar `MusteriMenusu`'na gider.

---

## KURAL: Landing page'de animasyon yok

`src/pages/Landing.jsx` ve `src/components/landing/*` içinde **kesinlikle**:
- animasyon kütüphanesi (framer-motion, gsap, aos...) **import edilmeyecek**
- `transition-*`, `animate-*`, `duration-*`, `@keyframes`, `hover:scale-*`
  sınıfları **kullanılmayacak**

Renk değişimi içeren `hover:bg-*` serbesttir (anlık, geçişsiz).
Bu kural **yalnızca landing** için geçerlidir; panel ve müşteri menüsü serbesttir.
