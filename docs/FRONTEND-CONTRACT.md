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
import api, { ApiError } from '../lib/api'

// No token helpers: the session is an HttpOnly cookie the browser attaches by
// itself. Every request goes out with credentials:'include' so it travels
// cross-origin too. There is nothing to read, store or clear.
```

| Call | Returns |
|---|---|
| `api.meta()` | `{ currencies, themes, fonts, allergens, languages, rounding_modes, contact_display_modes }` |
| `api.health()` | `{ status, database, version }` |
| `api.register({ business_name, email, password })` | `{ user, business }` + session cookie |
| `api.login({ email, password })` | `{ user, business }` + session cookie |
| `api.me()` | `{ user, business }` |
| `api.logout()` | `{ success }`, cookie cleared |
| `api.changePassword(current, next)` | `{ success, revoked_sessions }` |
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
| `api.getMenu(id)` | Menu (one menu of the caller's business) |
| `api.previewMenu(lang)` | PublicMenu |
| `api.publicMenu(slug, lang)` | PublicMenu |

On failure it throws `ApiError` with `.message` (Turkish, safe to show to the
user), `.status` and `.code`. Wrap every call in `try/catch` and surface the
message through `useToast().error(err.message)`.

The menu editor (`pages/dashboard/MenuEditor.jsx`) reads two of those errors as
a sign that its own lists are out of date. When one of its writes — a product
saved, created, moved, deleted or reordered, a price or visibility change, or a
category created, edited, deleted or reordered — answers 404
`"Kategori bulunamadı."` or `"Menü bulunamadı."`, the category or menu was
deleted in another tab. After the error toast the editor reloads its categories
and products, without showing that same message again if the reload fails with
it, and refreshes the live preview, so the stale category disappears; a delete
confirmation for a category that is already gone closes. `ProductModal` and
`CategoryModal` hand a failed save to their `onSaveFailed(error)` prop for this.

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
              price_updated_at, phone, address, instagram, wifi_ssid, wifi_password,
              contact_display, links: [{ id, label, url }] },
  categories: [{ id, name, description, icon, image_url, is_active,
                 products: [{ id, name, description, ingredients, price, compare_price,
                              image_url, allergens, is_featured, is_active }] }],
  footer: { price_note, vat_note, powered_by },
}
```

> **Contact details.** A dashboard menu (`/api/menus`) and the public `business`
> both carry `contact_display` — `'inline'`, `'list'`, `'footer'` or `'hidden'` —
> and `links`, always an array of `{ id, label, url }` in the owner's order. A
> save has to pass the link rules (see `src/lib/contact.js` below, and section 8
> of `docs/API.md`). The public `business.links` carries only the stored entries
> that pass them, at most 8, each with an id unique within the payload; the
> owner's own endpoints return the stored entries as they are, so a stored link
> may break the rules there. When `contact_display` is `'hidden'` the public
> payload arrives with `phone`, `instagram`, `wifi_ssid` and `wifi_password` set
> to `null` and `links: []`. Whenever `show_vat_note` is on, `footer.vat_note` is
> the trimmed `vat_note_text`, or `"Fiyatlarımıza KDV dahildir."` when that text
> is blank.
>
> Every contact field may be empty, and an empty one is simply not drawn — no
> chip, row, icon or gap. `src/lib/contact.js` decides what is usable and
> `components/menu/ContactInfo.jsx` lays it out.
>
> In the public menu the translations are **already resolved**: plain
> `name` / `description` fields instead of the `translations` map.
>
> **A category's icon and image are optional.** Either may be `null`, empty or
> unusable — a legacy icon code such as `"coffee"`, a whitespace-only string, an
> image URL that no longer loads. The customer menu draws the first one that
> works, in this order: **image → emoji → nothing**. A category with neither is
> a name-only card: there is no placeholder glyph, because an owner who chose
> no picture asked for none. `src/lib/category.js` decides
> what counts as usable (`categoryImageUrl`, `categoryEmoji`), and
> `normalizeCategories` hands the menu one shape it can render without guards.
>
> `product_count` is still returned by `/api/categories` (the dashboard's bulk
> price dialog shows it), but the customer menu no longer shows a product count
> on its category cards.

---

## `src/lib/auth.jsx`

```js
import { useAuth } from '../lib/auth.jsx'

const { user, business, isAuthenticated,
        login, register, logout, saveBusiness, refreshBusiness } = useAuth()
```

- `login(email, password)` → Promise
- `register(businessName, email, password)` → Promise
- `saveBusiness(payload)` calls `api.updateBusiness` **and refreshes the
  context**. Anything that changes business settings must use it, because the
  live preview reads from there.

### There is no `loading`

The account is read synchronously from `localStorage` (`krc_user`) during the
first render, so there is no window in which the answer is unknown and nothing
to put a spinner over. `loading` was removed rather than left permanently
false — a flag that is always one value is a trap for the next reader.

### It is optimistic, not verified

`isAuthenticated` means "this browser remembers signing in", not "the server
agrees". Nothing here is an access check: the session is an HttpOnly cookie
only the server can read, and it decides every request on its own.

A forged `krc_user` therefore buys somebody the panel **shell** in their own
browser and nothing inside it — every request 401s with `SESSION_EXPIRED`, the
interceptor in `lib/api.js` clears the key, and they land on `/giris`.

**Public pages must never trigger an auth call.** The landing page, `/demo` and
every customer menu route make zero requests to `/api/auth/*`; a QR menu opened
by a stranger makes exactly one call, for the menu. If you add a hook that
verifies the session, put it inside the panel, not above the router.

---

## `src/lib/menuContext.jsx`

```js
import { useActiveMenu } from '../lib/menuContext.jsx'

const { menus, loading, error, activeMenu, activeMenuId, hasMenus, setActiveMenuId,
        reload, createMenu, deleteMenu, saveActiveMenu, refreshMenu,
        setMenuCategoryCount } = useActiveMenu()
```

- `saveActiveMenu(payload)` persists the active menu and swaps the server's
  response into `menus`, keeping the `category_count` already in state.
- `refreshMenu(id)` refetches one menu (`api.getMenu`) and merges it into
  `menus` the same way. It never touches `loading` — MenuEditor waits for that
  flag, so toggling it would reload the whole editor — and a failure is only a
  `console.warn`. MenuEditor calls it after every successful price change
  (inline price edit, product save, bulk apply), so the price date on the
  settings page is current without a page reload.
- `setMenuCategoryCount(menuId, count)` sets one menu's `category_count` in
  `menus`. Only the list endpoint computes that count, so without it the "Menü
  Değiştir" dialog would show a stale count until a reload. It makes no request
  and never touches `loading`, and a count that is already right leaves `menus`
  unchanged. MenuEditor calls it when a menu's categories have loaded, after a
  category is created and after one is deleted.
- The settings page resets its form whenever the active menu's values change,
  leaving out `price_updated_at`, `updated_at` and `category_count`: a price
  change refetched through `refreshMenu` and a count set through
  `setMenuCategoryCount` never wipe an unsaved draft there.

---

## `src/lib/format.js`

```js
import { formatPrice, currencySymbol, formatDate, parsePrice, priceToInput,
         CURRENCIES, CURRENCY_LIST } from '../lib/format'
```

- `formatPrice(145, 'TRY')` → `"145,00 ₺"`
- `parsePrice('12,50')` → `12.5`
- `priceToInput(145)` → `"145,00"`
- `formatDate(iso)` → `"24.08.2026"`: the calendar day in **Europe/Istanbul**,
  whatever time zone the browser is in (the local day if `Intl` cannot do it)

## `src/lib/category.js`

```js
import { normalizeCategories, categoryEmoji, categoryImageUrl } from '../lib/category'
```

- `categoryEmoji(icon)` → the trimmed icon when it is a glyph (non-empty, no
  ASCII letters or digits, at most 32 UTF-16 code units), otherwise `null`:
  `"coffee"`, `"   "` and `null` all give `null`
- `categoryImageUrl(url)` → the trimmed URL, or `null`
- `normalizeCategories(list, language)` → always an array. Entries that are
  not objects are dropped; each remaining one keeps its fields and gains
  `key` (React key only), `name` (never empty), `description`, `emoji`,
  `imageUrl` and `products` (always an array)

## `src/lib/contact.js`

```js
import { buildContactItems, contactDisplayMode, CONTACT_DISPLAY_MODES, safeExternalUrl,
         linkHostname, telHref, instagramHandle, instagramUrl, linkIcon, trimSpace,
         linkLabelProblem, linkUrlProblem, linkListProblem, keptLinkIds, completeLinkUrl,
         LINK_MESSAGES, LINK_ID_PATTERN, MAX_LINKS, MAX_LINK_LABEL_RUNES,
         MAX_LINK_URL_BYTES } from '../lib/contact'
```

Pure functions, no React, and none of them throws, whatever it is handed.

This file is the only client implementation of the **link rules** that
`PUT /api/menus/:id` applies (`backend/internal/utils/links.go`, section 8 of
`docs/API.md`). The settings page reports its inline errors with it, and
`buildContactItems` draws a link only when it passes. Every control or invisible
character in the file is written as an escape sequence, and no regular
expression uses a Unicode property escape: the letter test reads a range table
generated from Go's `unicode.IsLetter`, and digits are ASCII only, so they need
no table.

The link rules:

- `trimSpace(value)` → the string trimmed exactly like Go's `strings.TrimSpace`:
  Unicode White_Space (tab to carriage return, space, U+0085, U+00A0, U+1680,
  U+2000–U+200A, U+2028, U+2029, U+202F, U+205F, U+3000), but not U+FEFF or
  U+180E; `''` for anything but a string
- `LINK_MESSAGES` → `tooMany`, `listInvalid`, `labelInvalid`, `labelRequired`,
  `labelTooLong`, `urlTooLong`, `urlInvalid`: the server's Turkish messages,
  word for word. `MAX_LINKS` is 8, `MAX_LINK_LABEL_RUNES` 40,
  `MAX_LINK_URL_BYTES` 500
- `linkLabelProblem(label)` → the message or `''`, checked on the trimmed label
  in this order: a C0 or C1 control character (U+0000–U+001F, U+007F–U+009F) →
  `labelInvalid`; nothing visible → `labelRequired`, where nothing visible means
  that nothing is left once the invisible format characters (U+00AD, U+061C,
  U+180E, U+200B–U+200F, U+202A–U+202E, U+2060–U+2064, U+2066–U+206F, U+FEFF)
  are removed and the rest is trimmed again, so U+200B, a space and U+200B is
  `labelRequired`; more than 40 code points of the trimmed label, invisible
  characters included → `labelTooLong`
- `linkUrlProblem(url)` → the message or `''`, checked on the trimmed address:
  more than 500 UTF-8 bytes → `urlTooLong`; otherwise `urlInvalid` unless it
  has no white space, control or invisible character and none of `<` `>` `"`
  `` ` `` `{` `}` `|` `\` `^` `[` `]` anywhere (`https://[ornek.com]` is
  refused: Go's `url.URL.Hostname` would strip the brackets and pass the name
  inside, but a browser cannot open that address); starts with `http://` or
  `https://` in any letter case; parses under Go's `net/url` rules with no
  userinfo; has a hostname of 1–253 characters in at least two labels of 1–63
  letters (any Unicode letter),
  ASCII digits or `-`, no label starting or ending with `-` and a letter in the
  last one; and, when a `:` follows the host, a port of 1–5 digits worth
  1–65535 (`https://ornek.com:` is refused)
- `linkListProblem(links)` → the message for a whole `links` value, or `''`: not
  an array, an entry that is not an object, or an `id`, `label` or `url` that is
  not a string → `listInvalid`; more than 8 entries → `tooMany`; then the first
  entry that fails, its label before its address. `null` for the whole value is
  an empty list, and a save that sends `links: null` stores `[]`. A `null` entry
  counts as an entry with no members, and a `null` `id`, `label` or `url` as
  that member missing: a `null` label is `labelRequired`, a `null` url
  `urlInvalid`, and a `null` id is replaced like a missing one
- `keptLinkIds(links)` → per entry, the id the server keeps, or `null` where it
  assigns a new UUID: an id is kept when it matches `LINK_ID_PATTERN`
  (`^[A-Za-z0-9_-]{1,64}$`) and no earlier entry of the list has it
- `safeExternalUrl(value)` → the address as the server stores it (trimmed, only
  the scheme lowercased) when it passes, otherwise `null`
- `linkHostname(value)` → the lowercase hostname of such an address, with its
  percent escapes decoded, or `null`
- `completeLinkUrl(value)` → what the address field becomes when it loses focus:
  trimmed; `https//x` and `http//x` get the missing colon and `https:/x`
  becomes `https://x`; otherwise `https://` goes in front of an address with a
  dot, no spaces and no scheme

The contact items:

- `CONTACT_DISPLAY_MODES` → `[{ id, label }]`: `inline` "Yan yana", `list`
  "Açık liste", `footer` "Sadece alt bilgi", `hidden` "Hiç gösterme"
- `contactDisplayMode(value)` → one of those ids; anything unknown is `'inline'`
- `telHref(phone)` → `'tel:'`, a `+` when one comes before the first digit,
  then the ASCII digits; `null` when there is no digit.
  `'+90 (555) 000 00 00'` → `'tel:+905550000000'`
- `instagramHandle(value)` → the user name (letters, digits, `.` and `_`), or
  `null`. The value is trimmed, every leading `@` is dropped and the rest is
  trimmed again; what is left is read as `name`, `instagram.com/name`,
  `www.instagram.com/name` or `http(s)://(www.)instagram.com/name`, optionally
  followed by more path and a query. The first path segment is the name, and
  `p`, `reel`, `reels`, `stories`, `explore` and `tv` are not names. So `@@name`,
  `@ name`, `@instagram.com/name` and `@https://www.instagram.com/name/` give
  `name`; `@` and `kahve duragi` give `null`. It is the one rule for the menu
  and the settings page: the Instagram field shows a stored value as
  `instagramHandle(value)` when that is a name and as the trimmed stored text
  otherwise, so a value the menu shows without a link stays visible in the
  field and can be cleared, and the page's unsaved-changes check reads the
  stored value the same way. `instagramUrl(value)` →
  `'https://www.instagram.com/<name>/'` or `null`
- `linkIcon(url)` → `'whatsapp'`, `'maps'`, `'instagram'`, `'facebook'`,
  `'youtube'`, `'twitter'`, `'linkedin'` or `'globe'`, decided by hostname
- `buildContactItems(business)` → always an array, in this order: `wifi`
  `{ ssid, password }` when either is non-blank; `instagram`
  `{ handle, url, text }` whenever the field is non-blank — `handle` and `url`
  are `null` when it holds no user name, and `text` is then the field as typed,
  so the value is shown without a link instead of disappearing; `phone`
  `{ phone, href }`; then one `link` `{ id, label, url, hostname, icon }` per
  `links` entry whose label and address pass the link rules. Every item carries
  `type` and a `key` unique within the list, even when stored ids repeat or are
  blank. The display mode is NOT applied here: the caller decides where the
  items go.

`node frontend/tests/contact.test.mjs` asserts every rule above, including
the examples `https://<svg/onload=alert(1)>`, a host holding U+202E, `:99999`,
`https://.`, `https://-`, `https://https//ornek.com` and `https://[ornek.com]`,
and labels of U+200B, of U+200B, a space and U+200B, and of a newline, BEL,
U+0085 and NUL.

## `src/lib/clipboard.js`

- `copyToClipboard(value)` → `Promise<boolean>`; falls back to a hidden
  textarea where `navigator.clipboard` is unavailable (plain HTTP, older in-app
  browsers)

## `src/lib/sortableRows.js`

```js
import { closestRowSlot, rowSlotKeyboardCoordinates } from '../lib/sortableRows'

const sensors = useSensors(
  useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
  useSensor(KeyboardSensor, { coordinateGetter: rowSlotKeyboardCoordinates }),
)
<DndContext sensors={sensors} collisionDetection={closestRowSlot} onDragEnd={...}>
```

For a vertical `@dnd-kit/sortable` list whose rows differ in height: the menu
editor's categories, where an open category is many rows tall. Every row defines
a slot, the top the dragged row would have if it were dropped at that row's
index. `closestRowSlot` picks the slot nearest to the dragged row's top, and
`rowSlotKeyboardCoordinates` moves the dragged row to the next slot up or down,
so one arrow press is one place whatever the heights. With dnd-kit's
`closestCenter` or `closestCorners` and `sortableKeyboardCoordinates`, the arrow
keys cannot move an open category taller than about three closed rows. On
rows of one height `closestRowSlot` picks the same row as `closestCenter`, except
exactly halfway between two slots. The product lists keep dnd-kit's own pair.

`node frontend/tests/sortableRows.test.mjs` runs the keyboard loop on lists of
mixed heights.

`npm test` in `frontend/` runs both test files, and the CI frontend job runs it
right after `npm ci`. Nothing imports either file, so neither reaches the bundle.

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
// a URL that no longer loads shows the placeholder and "Görsel yüklenemedi. Lütfen yeniden yükleyin."

import { useToast } from '../ui/Toast.jsx'
const toast = useToast()   // toast.success(msg) / .error(msg) / .info(msg)
// the stack sits at the TOP: top right below the dashboard top bar, full width
// between side gutters on narrow screens, above modals (z-[100]), with
// role="status" and aria-live="polite". At most 2 toasts are shown: a new one
// removes the oldest. While a toast is on screen the stack's whole box takes
// pointer events: a click on a card dismisses that card, and a click in the gap
// between two cards or in a card's rounded corner does nothing, so no click on
// the stack reaches what lies underneath - a modal's backdrop would close the
// dialog and lose what was typed into it. With no toast the stack takes no
// pointer events. A mousedown on the stack (a card, its close button or a gap)
// is cancelled, so clicking a toast never moves focus: the focused field keeps
// it. The close button is the keyboard's way in; dismissing a toast from it
// (Enter or Space) returns focus to the element that had it before focus
// entered the stack, when that element is still in the document. In the DOM the
// message comes before the close button, so the live region reads the message
// first; CSS `order` draws the button at the START (left) edge of the card, away
// from a modal's top-right close button. Not at the bottom, where a toast would
// cover the sticky "Kaydet" bar and the footer buttons of the modals.

import ErrorBoundary from '../ui/ErrorBoundary.jsx'
<ErrorBoundary fallback={node | ((error, reset) => node)} resetKeys={[a, b]}>...</ErrorBoundary>
// resetKeys are compared element by element, so an inline array literal is safe;
// the fallback must not read anything that could throw again
```

## Customer menu contact components (`src/components/menu/ContactInfo.jsx`)

```jsx
import { ContactBar, ContactList } from '../menu/ContactInfo.jsx'
const items = buildContactItems(business)                 // src/lib/contact.js
<ContactBar items={items} language="tr" onAccentText="#ffffff" className="mt-4" />
<ContactList items={items} language="tr" className="mt-4" />
<ContactList items={items} language="tr" compact />
```

- `ContactBar` (`contact_display: 'inline'`): a centred, wrapping row of chips —
  buttons with `aria-expanded` / `aria-controls` — and one detail panel under
  it; at most one chip is open, and tapping the open chip closes it. When the
  open item leaves `items`, the selection is cleared, so the panel does not open
  again by itself when that item comes back.
- `ContactList` (`'list'`): every item with its details always open, on one
  card. With `compact` it is the product screens' footer variant.
- Both render **nothing at all** for an empty list (`className` included), and
  give their rows unique React keys even when item keys repeat or are missing.
  The Wi-Fi password is masked — at most 12 dots — until its reveal toggle is
  pressed; every web link opens with `target="_blank"` and
  `rel="noopener noreferrer"`.
- A value's action pills sit beside it while both fit and wrap onto a line below
  it when they do not. A value that is never truncated — a phone number, a Wi-Fi
  password, an Instagram value that holds no user name, and every value of the
  compact footer, network name, link label and hostname included — carries the
  `break-anywhere` class (see Shared CSS classes): one longer than its line
  breaks inside itself instead of running past the edge of a narrow screen, and
  only where the line has no other break, so a phone number still wraps at its
  spaces. In the compact footer a revealed Wi-Fi password starts on its own line
  under its caption. In the panel and the list a network name, an Instagram user
  name and a link label may truncate, with the full text in `title`. An
  Instagram value that holds no user name is shown whole, as plain text with no
  "Instagram'da aç" button.
- `MenuContent` draws them on the home view only (`inline` / `list`);
  `MenuFooter` draws the compact list on product screens for every mode but
  `hidden`.

## Customer menu footer (`src/components/menu/MenuFooter.jsx`)

```jsx
<MenuFooter business={menu.business} footer={menu.footer} language="tr" scope="products" />
```

- `scope="home"` draws only the "Karecik ile hazırlandı" signature;
  `scope="products"` adds the compact contact list, the price date
  (`footer.price_note`), the VAT sentence and the "Yerli Üretim" badge.
- **VAT.** The menu's own toggle governs every VAT sentence on the page:
  `business.show_vat_note === false` → none at all; `true` →
  `business.vat_note_text` trimmed like Go's `strings.TrimSpace`, or
  `"Fiyatlarımıza KDV dahildir."` when it is blank. The sentence is read from
  the business fields rather than `footer.vat_note`, so the dashboard live
  preview shows an unsaved edit of the text at once; on the customer menu both
  carry the same sentence. `footer.vat_note` is only the fallback for a business
  without those fields. The settings page keeps a blank `vat_note_text` blank:
  the default sentence is only the placeholder of its field, and the help text
  under the field says that a blank text prints that sentence.
- The "Yerli Üretim" block is the badge alone, with no VAT sentence of its own.

---

## Shared CSS classes (`src/index.css`)

`btn`, `btn-primary`, `btn-secondary`, `btn-ghost`, `btn-danger`, `btn-sm`,
`input`, `label`, `help-text`, `error-text`, `card`, `badge`,
`dragging`, `no-scrollbar`, `break-anywhere`

`break-anywhere` is for a value that is never truncated and must never run past
the edge of its line (a Wi-Fi password, a phone number): `overflow-wrap:
anywhere`, which breaks inside the value only where its line has no other break,
and `word-break: break-all` only inside `@supports not (overflow-wrap: anywhere)`,
for a browser without it. It is plain CSS after the Tailwind layers.

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
| `/demo` | `CustomerMenu` (`businessSlug={VITE_DEMO_BUSINESS} embedded`) |
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
