# Frontend Contract

This file documents the interface exposed by the **shared modules** under
`frontend/src`. Match these signatures exactly when writing new pages or
components — do not change them.

All code is **JavaScript + JSX** (no TypeScript), **React 18**,
**react-router-dom v6**, **Tailwind CSS v3**, icons from **lucide-react**,
drag and drop with **@dnd-kit**.

> **Language convention:** identifiers, comments and this documentation are
> English. Strings the user sees stay **Turkish** — the product serves Turkish
> businesses.

---

## `src/lib/api.js`

```js
import api, { ApiError, getToken, setToken, clearToken } from '../lib/api'
```

| Call | Returns |
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

On failure it throws `ApiError` with `.message` (Turkish, safe to show to the
user), `.status` and `.code`. Wrap every call in `try/catch` and surface the
message through `useToast().error(err.message)`.

### Data shapes

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

> In the public menu the translations are **already resolved**: plain
> `name` / `description` fields instead of the `translations` map.

---

## `src/lib/auth.jsx`

```js
import { useAuth } from '../lib/auth.jsx'

const { user, business, loading, isAuthenticated,
        login, register, logout, saveBusiness, refreshBusiness } = useAuth()
```

- `login(email, password)` → Promise
- `register(businessName, email, password)` → Promise
- `saveBusiness(payload)` calls `api.updateBusiness` **and refreshes the
  context**. Anything that changes business settings must use it, because the
  live preview reads from there.

---

## `src/lib/format.js`

```js
import { formatPrice, currencySymbol, formatDate, parsePrice, priceToInput,
         CURRENCIES, CURRENCY_LIST } from '../lib/format'
```

- `formatPrice(145, 'TRY')` → `"145,00 ₺"`
- `parsePrice('12,50')` → `12.5`
- `priceToInput(145)` → `"145,00"`
- `formatDate(iso)` → `"24.08.2026"`

## `src/lib/subdomain.js`

- `getSubdomain()` → `"kahve-duragi"` or `null`
- `menuUrl(slug)` → `http://kahve-duragi.localhost:5173`
- `menuPathUrl(slug)` → `http://localhost:5173/m/kahve-duragi`

## `src/themes/themes.js`

- `THEMES` → `[{ id, label, description, dark, colors{...}, style{...} }]`
- `findTheme(id)`, `DEFAULT_THEME`
- `themeVariables(themeOrId, accentColor, fontStack)` → the CSS custom
  properties to apply as the menu container's `style`
  (`--menu-bg`, `--menu-text`, `--menu-primary`, `--menu-surface`,
  `--menu-muted`, `--menu-border`, `--menu-radius`, `--menu-shadow`,
  `--menu-font`).

## `src/themes/fonts.js`

- `FONTS`, `findFont(id)`, `fontStack(id)`, `loadFont(id)`, `loadAllFonts()`

## `src/locales/index.js`

- `LANGUAGES`, `findLanguage(code)`, `languageShort(code)`, `isRtl(code)`
- `ALLERGENS` → `[{ code, emoji, tr, en }]`, `findAllergen(code)`, `allergenLabel(code, language)`
- `t(key, language)` → a customer menu interface string

> Never render flag emoji in the interface: Windows cannot draw them and prints
> the country code instead, which made English show up as "GB". Use
> `language.short` (TR / EN / DE) instead.

## `src/locales/landing.js`

- `LANDING_LANGUAGES`, `landingText(language)`, `readSavedLanguage()`, `saveLanguage(language)`

---

## Shared components (`src/components/ui/`)

```jsx
import Modal from '../ui/Modal.jsx'
<Modal open onClose={fn} title="" description="" width="max-w-lg" footer={<>...</>}>body</Modal>

import ConfirmModal from '../ui/ConfirmModal.jsx'
<ConfirmModal open onClose={fn} onConfirm={fn} title="" message="" confirmText="Sil" busy={false} />

import Loading from '../ui/Loading.jsx'
<Loading fullScreen text="..." />

import EmptyState from '../ui/EmptyState.jsx'
<EmptyState icon={LucideIcon} title="" description="" action={<button/>} />

import ImageUploader from '../ui/ImageUploader.jsx'
<ImageUploader value={url|null} onChange={(url)=>{}} label="" hint="" round={false} />

import { useToast } from '../ui/Toast.jsx'
const toast = useToast()   // toast.success(msg) / .error(msg) / .info(msg)
```

---

## Shared CSS classes (`src/index.css`)

`btn`, `btn-primary`, `btn-secondary`, `btn-ghost`, `btn-danger`, `btn-sm`,
`input`, `label`, `help-text`, `error-text`, `card`, `badge`,
`dragging`, `no-scrollbar`

Brand colours: `bg-brand-600`, `text-brand-600`, `border-brand-600` (shades 50–900).
Shadows: `shadow-card`, `shadow-panel`. Width: `max-w-content`.

---

## Routing (`src/App.jsx` — do not change)

| Path | Component |
|---|---|
| `/` | `pages/Landing.jsx` |
| `/giris` | `pages/Login.jsx` |
| `/kayit` | `pages/SignUp.jsx` |
| `/m/:slug` | `pages/menu/CustomerMenu.jsx` |
| `/demo` | `CustomerMenu` (`slug="demo-kafe" embedded`) |
| `/panel` | `pages/dashboard/DashboardLayout.jsx` (Outlet) |
| `/panel` (index) | `pages/dashboard/MenuEditor.jsx` |
| `/panel/tasarim` | `pages/dashboard/Design.jsx` |
| `/panel/ayarlar` | `pages/dashboard/Settings.jsx` |
| `/panel/qr` | `pages/dashboard/QrCode.jsx` |

> The URL paths stay Turkish on purpose — they are public, user-visible
> addresses that are already in use.

When the visitor arrives through a subdomain (`kahve-duragi.localhost`), every
path renders `CustomerMenu`.

---

## RULE: no animation on the landing page

Inside `src/pages/Landing.jsx` and `src/components/landing/*` the following are
**forbidden**:

- importing an animation library (framer-motion, gsap, aos, …)
- the `transition-*`, `animate-*`, `duration-*`, `@keyframes` and
  `hover:scale-*` classes

Colour-only `hover:bg-*` is allowed (it is instant, with no transition).
This rule applies **to the landing page only**; the dashboard and the customer
menu are free to animate.
