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
| `api.previewMenu(lang, { menu })` | PublicMenu |
| `api.publicMenu(businessSlug, menuSlug, lang)` | PublicMenu — leave `lang` empty to let the server negotiate `Accept-Language` |
| `api.publicMenuByHost(menuSlug, lang)` | PublicMenu (tenant from the request host) |
| `api.analyticsSummary({ menu_id, from, to })` | Analytics summary (see "Analytics and audit endpoints") |
| `api.analyticsEvents({ menu_id, type, from, to, ip, limit, offset })` | `{ items, total, limit, offset }` |
| `api.auditLogs({ entity_type, action, limit, offset })` | `{ items, total, limit, offset }` |

Empty parameters are left out of the query string. The visitor events of the
customer menu do NOT go through `api.js`: `src/lib/analytics.js` sends them with
`navigator.sendBeacon` (see there).

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
              contact_display, contact_in_footer, links: [{ id, label, url }] },
  categories: [{ id, name, description, icon, image_url, is_active,
                 products: [{ id, name, description, ingredients, price, compare_price,
                              image_url, allergens, is_featured, is_active }] }],
  footer: { price_note, price_date, vat_note, powered_by },
  language,          // the language every text above was resolved in
  menus, menu_resolved,
}
```

> **Language.** The public menu endpoints negotiate the language when the
> request names none: an explicit `lang` the menu offers wins, then the first
> language of the browser's `Accept-Language` the menu offers, then the menu's
> `default_language`. The browser sends `Accept-Language` on its own, so a
> client that has no stored choice simply leaves `lang` out and reads the
> result from `language` — it never has to guess from `navigator.language`
> itself. Once the visitor picks a language, send it as `lang`. The directory
> payload (`menu_resolved: false`) carries `language` too, matched against every
> supported language. The dashboard preview reports `language` but ignores the
> owner's `Accept-Language` (explicit `lang`, else the menu default).
>
> **Price date.** `footer.price_date` is `"YYYY-MM-DD"` (the Istanbul calendar
> day of the menu's last price change), or `""` when the menu hides it. Build
> the localized sentence from it; `footer.price_note` is the same day as a
> ready-made Turkish sentence, kept for older clients.

> **Contact details.** A dashboard menu (`/api/menus`) and the public `business`
> both carry `contact_display` — `'inline'`, `'list'` or `'hidden'`, which
> governs the **home view only** — `contact_in_footer` (boolean, default
> `false`: whether the compact list is repeated in the menu's footer, which is
> the same on every view) and `links`, always an array of `{ id, label, url }` in the owner's
> order. A save may still send the legacy `contact_display: 'footer'`; it is
> stored as `'hidden'` + `contact_in_footer: true`, so a read never returns it. A
> save has to pass the link rules (see `src/lib/contact.js` below, and section 8
> of `docs/API.md`). The public `business.links` carries only the stored entries
> that pass them, at most 8, each with an id unique within the payload; the
> owner's own endpoints return the stored entries as they are, so a stored link
> may break the rules there. When `contact_display` is `'hidden'` **and**
> `contact_in_footer` is `false` — the block is drawn nowhere — the public
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

### Analytics and audit endpoints

Full rules in sections 10 and 11 of `docs/API.md`; the shapes a client reads:

```js
// public, no session; body is JSON even when sent as text/plain (sendBeacon).
// 204 also when the event is not stored: a repeat of the same view within 10 s,
// or past the business's daily cap — the page does nothing differently
POST /api/public/events
  { business_slug, menu_slug, type: 'menu_view' | 'category_view' | 'product_view',
    category_id?, product_id?, visitor_id?, language? }      -> 204, no body

// 🔒 from/to are YYYY-MM-DD, inclusive, Istanbul days; default the last 30 days
GET /api/analytics/summary?menu_id=&from=&to=
  -> { from, to, total_visits, unique_visitors, menu_views, category_views, product_views,
       daily: [{ date, visits, unique_visitors, category_views, product_views }],
       top_categories: [{ id, name, views }],
       top_products: [{ id, name, category_name, views }] }

// 🔒 newest first; limit default 50, max 200; ip is a prefix match
GET /api/analytics/events?menu_id=&type=&from=&to=&ip=&limit=&offset=
  -> { items: [{ id, created_at, type, menu_id, menu_name, category_id, category_name,
                 product_id, product_name, ip, port, ip_source, visitor_id, language,
                 user_agent }], total, limit, offset }

// 🔒 newest first; limit default 50, max 200
GET /api/audit-logs?entity_type=&action=&limit=&offset=
  -> { items: [{ id, created_at, user_id, user_email, action, entity_type, entity_id,
                 entity_label, changes: { <field>: { old, new } }, ip, port, ip_source }],
       total, limit, offset }
```

`ip` and `port` may be `null` (not established); `category_name` /
`product_name` are `null` for a record deleted since. `changes` keys are field
names, translations as `translations.<lang>.<field>`; a Wi-Fi password is always
`"••••"`. The actions are `product.create|update|delete|price|bulk_price|reorder`,
`category.create|update|delete|reorder`, `menu.create|update|delete`,
`business.update`, `account.password_change` and `upload.create`; the entity
types `product`, `category`, `menu`, `business`, `account`, `upload`.

`POST /api/uploads` accepts SVG (`image/svg+xml`) as well as JPEG, PNG, WebP and
GIF. An SVG with script, external references or embedded HTML — or a raster file
whose bytes are not an image — is a `422` with a Turkish message to show as is.
`components/ui/ImageUploader.jsx` checks the type (by MIME type or by extension:
`.svg .jpg .jpeg .png .webp .gif`), refuses an empty file and applies the 5 MB
limit before uploading, with the rules in `src/lib/imageUpload.js`; the server
stays the authority on the content. Its previews use `object-contain`, so an SVG
without `width`/`height` still shows.

The dashboard reads these endpoints on two pages of their own:
`/panel/analitik` (`pages/dashboard/Analytics.jsx` — menu picker and day range
over the summary, the daily chart, the top lists and the paged visit log with
IP, port and IP source) and `/panel/gecmis` (`pages/dashboard/AuditLog.jsx` —
the business-wide change history, filtered by record type). The Turkish labels
and value formatting live in `src/lib/analyticsFormat.js` and
`src/lib/auditFormat.js`.

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
- `parseIsoDay('2026-08-24')` → `{ year, month, day }` or `null`;
  `istanbulDay(iso)` → the instant's Istanbul day as `"YYYY-MM-DD"`;
  `formatDay('2026-08-24', 'de')` → `"24. August 2026"` — the month spelled out
  in the visitor's language, Latin digits and the Gregorian calendar in Arabic too
- `footerPriceSentence(business, footer, language)` → the localized "prices
  valid from …" sentence, or `''`: built from `footer.price_date`; the business's
  `show_price_date` (and `price_updated_at`) win over the saved footer so the
  live preview shows an unsaved toggle; `footer.price_note` is used only when
  `price_date` is absent altogether (an older server)
- `footerVatNote(business, footer, language)` → the VAT sentence, or `''` (see
  the footer section below); `DEFAULT_VAT_NOTE` is the server's Turkish default

## `src/lib/category.js`

```js
import { normalizeCategories, categoryEmoji, categoryImageUrl, isSvgUrl,
         productTexts, comparableText } from '../lib/category'
```

- `isSvgUrl(url)` → whether an image address is an SVG (by its path's
  extension, or a `data:image/svg+xml` URI). The customer menu draws an SVG with
  `object-contain` and an explicit height, a photo with `object-cover`
- `productTexts(product)` → `{ subtitle, description, ingredients }`: the card's
  grey line is the description, else the ingredients; the detail sheet drops a
  description that only repeats the ingredients
- `comparableText(value)` → the text as that repeat check compares it: NFC,
  Turkish lowercase, whitespace collapsed, trailing punctuation dropped. The
  product dialog's "same text in both fields" warning (`src/lib/productText.js`)
  uses this same function, so the two can never disagree

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

- `CONTACT_DISPLAY_MODES` → `[{ id, label }]`, the ids of
  `contact_display_modes` in `/api/meta`: `inline` "Yan yana", `list`
  "Açık liste", `hidden` "Gösterme" (the settings page shows them under the
  heading "Ana ekranda iletişim bilgileri", so the short label suffices; the
  server's own label is "Ana sayfada gösterme"). The footer list is the separate
  `contact_in_footer` switch, not a mode
- `contactDisplayMode(value)` → one of those ids; the legacy `'footer'` is
  `'hidden'` and anything else unknown is `'inline'`
- `contactInFooter(business)` → `true` when `contact_in_footer === true` or the
  business still carries the legacy `contact_display: 'footer'`; never throws
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

- `LANGUAGES`, `LANGUAGE_CODES` (`tr en de ru ar fr`), `findLanguage(code)`,
  `languageShort(code)`, `isRtl(code)`, `languageDir(code)` → `'rtl'` | `'ltr'`
- `ALLERGENS` → `[{ code, emoji, tr, en, de, ru, ar, fr }]` (one label per
  language; the dashboard reads `tr`), `findAllergen(code)`,
  `allergenLabel(code, language)`
- `STRINGS` → one dictionary per language, all with the same keys
- `t(key, language, values)` → a customer menu interface string with
  `{placeholders}` filled from `values`; an unknown language reads English, a
  missing key English, then Turkish, then the key itself

Every customer-facing string of the menu goes through `t` — no literal text in
`components/menu/*` or `pages/menu/*`. "Karecik", "Instagram" and "Yerli Üretim"
are never translated, and neither are the owner's own product names.
`node frontend/tests/locales.test.mjs` checks the key sets, the placeholders and
that every `t('…')` used in the menu source exists.

> **Right to left.** An Arabic menu sets `dir="rtl"` and `lang` on its roots
> (and, standalone, on `<html>`), uses logical Tailwind classes only
> (`text-start`, `ps-*`, `start-3` — never `left`/`right`), wraps prices in
> `<bdi dir="ltr">` and owner-typed text in `<bdi dir={textDir(text, language)}>`
> — `rtl` when the menu is RTL and the text holds an RTL letter (so "Frozen
> بالفراولة" keeps the Latin term on the right), `auto` otherwise. The language
> picker itself is `dir="ltr"`, so its order (TR … FR) never flips.

> Never render flag emoji in the interface: Windows cannot draw them and prints
> the country code instead, which made English show up as "GB". Use
> `language.short` (TR / EN / DE) instead.

## `src/lib/language.js`

Which language the customer menu renders in, in this order: the visitor's pick
(tapped now, named by the address's `?lang=`, or remembered for this tenant in
`localStorage` `karecik_lang_<tenant>`) while the menu offers it → the payload's
`language` (the server's `Accept-Language` negotiation) → `navigator.languages`
matched against the menu → `default_language` → `'tr'`. The first request sends
the pick when there is one and no `lang` otherwise, so the first paint is
already in the visitor's language. `?lang=` is how the landing page's demo
iframe follows the landing language; it is never remembered. A tap on an
address that already has `?lang=` rewrites it in place (history `replace`); an
address without one never gains it. A remembered pick is never forgotten: a
sibling menu that does not offer it simply ignores it. Directory links keep the
address's query string.
`resolveMenuLanguage`, `matchPreferredLanguage`, `baseLanguage`,
`browserLanguages`, `readRememberedLanguage`, `rememberLanguage`; tested by
`tests/menuLanguage.test.mjs`.

## `src/lib/analytics.js`

`trackMenuEvent({ businessSlug, menuSlug, type, categoryId?, productId?, language })`
sends one `POST /api/public/events` with `navigator.sendBeacon` (a `text/plain`
JSON body, no preflight), falling back to `fetch` with `keepalive`; it never
throws and never waits. The anonymous `visitor_id` lives in `localStorage`
`karecik_visitor_id`. The customer menu sends `menu_view` once per menu per page
load, `category_view` when a category opens and `product_view` when a product's
detail sheet opens — never when `embedded` (the landing demo iframe, the
dashboard live preview) and never for the tenant directory.

## `src/locales/landing.js`

- `LANDING_LANGUAGES` (tr / en / de), `DEFAULT_LANDING_LANGUAGE`, `LANDING_STRINGS`,
  `landingText(language)`, `readSavedLanguage()`, `saveLanguage(language)`
- `validateSignUp({ businessName, email, password })` → `{ field: messageKey }`,
  the server's limits included (name ≤ 100 characters, password ≤ 72 bytes)
- `signUpErrorKey(error)` / `signUpErrorMessage(error, language)` → a request
  error in the visitor's language, decided by status and code (409, network,
  429, 5xx, other)

The landing page sets `<html lang>` to its language and pins `dir="ltr"` while
it is mounted. `tests/landingLocales.test.mjs` checks the three dictionaries.

---

## Shared components (`src/components/ui/`)

```jsx
import Modal from '../ui/Modal.jsx'
<Modal open onClose={fn} title="" description="" width="max-w-lg" footer={<>...</>}
       closeLabel="Kapat">body</Modal>
// closeLabel is the X button's accessible name; the landing sign-up dialog
// passes the visitor's language

import ConfirmModal from '../ui/ConfirmModal.jsx'
<ConfirmModal open onClose={fn} onConfirm={fn} title="" message="" confirmText="Sil" busy={false} />

import Loading from '../ui/Loading.jsx'
<Loading fullScreen text="..." />

import EmptyState from '../ui/EmptyState.jsx'
<EmptyState icon={LucideIcon} title="" description="" action={<button/>} />

import ImageUploader from '../ui/ImageUploader.jsx'
<ImageUploader value={url|null} onChange={(url)=>{}} label="" hint="" round={false} />
// a URL that no longer loads shows the placeholder and "Görsel yüklenemedi. Lütfen yeniden yükleyin."
// accepts SVG as well; "JPG, PNG, WEBP, GIF veya SVG · en fazla 5 MB" is always
// printed, and `hint` (default '') is field-specific advice only

import { useToast } from '../ui/Toast.jsx'
const toast = useToast()   // toast.success(msg) / .error(msg) / .info(msg)
// second argument: a duration in ms, or { duration, closeLabel } — closeLabel
// names the close button (default "Bildirimi kapat"; the landing page passes
// the visitor's language)
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
  `MenuFooter` draws the compact list only when `contactInFooter(business)` is
  true, whatever `contact_display` says.

## Customer menu footer (`src/components/menu/MenuFooter.jsx`)

```jsx
<MenuFooter business={menu.business} footer={menu.footer} language="tr" />
<MenuFooter language="tr" scope="directory" />
```

One footer, identical on every view of a menu (category grid, product list,
search results). Top to bottom, each only when it has something to say:

- the compact contact list, only when `contactInFooter(business)` — the owner
  turned `contact_in_footer` on (it is off by default);
- the price sentence in the visitor's language, from `footerPriceSentence`
  (`footer.price_date`);
- the VAT sentence (below);
- the "Yerli Üretim" badge when `show_yerli_uretim` is on — printed as-is in
  every language, never translated;
- the "Powered by Karecik" signature, always last, from `t('poweredBy')`.
  `footer.powered_by` is not read.

Nothing else belongs in the footer: no loose Instagram link, Wi-Fi field or
button of its own. `scope="directory"` (the tenant directory, which has no
menu) draws the signature alone. The legacy values `"home"` / `"products"` draw
the full footer.
- **VAT.** The menu's own toggle governs every VAT sentence on the page
  (`footerVatNote`): `business.show_vat_note === false` → none at all; `true` →
  `business.vat_note_text` as the owner typed it, trimmed like Go's
  `strings.TrimSpace`, or the localized `t('vatIncluded')` ("Fiyatlarımıza KDV
  dahildir." in Turkish) when it is blank. The sentence is read from the
  business fields rather than `footer.vat_note`, so the dashboard live preview
  shows an unsaved edit of the text at once. `footer.vat_note` is only the
  fallback for a business without those fields, and the server's Turkish
  default sentence there is swapped for the localized one. The settings page keeps a blank `vat_note_text` blank:
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
| `/sifremi-unuttum` | `pages/ForgotPassword.jsx` |
| `/sifre-sifirla` | `pages/ResetPassword.jsx` |
| `/m/:businessSlug` | `pages/menu/CustomerMenu.jsx` (tenant address) |
| `/m/:businessSlug/:menuSlug` | `pages/menu/CustomerMenu.jsx` |
| `/demo` | `CustomerMenu` (`businessSlug={VITE_DEMO_BUSINESS} embedded`); the landing page loads it as `/demo?lang=<landing language>` |
| `/panel` | `pages/dashboard/DashboardLayout.jsx` (Outlet) |
| `/panel` (index) | `pages/dashboard/MenuEditor.jsx` |
| `/panel/ayarlar` | `pages/dashboard/MenuSettings.jsx` |
| `/panel/qr` | `pages/dashboard/QrHub.jsx` |
| `/panel/analitik` | `pages/dashboard/Analytics.jsx` — visitor analytics and the visit log |
| `/panel/gecmis` | `pages/dashboard/AuditLog.jsx` — change history (audit trail) |
| `/panel/hesap` | `pages/dashboard/Account.jsx` |

Any customer menu address also accepts `?lang=<code>` to open in that language
when the menu offers it.

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
