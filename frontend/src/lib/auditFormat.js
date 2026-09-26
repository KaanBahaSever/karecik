// How the owner panel's change history ("Değişiklik Geçmişi") reads one entry
// of GET /api/audit-logs: the Turkish names of the actions, of the record types
// and of the fields, and the way a changed value is printed.
//
// Pure functions only - no React, no DOM - so frontend/tests/
// dashboardAnalytics.test.mjs runs them with plain node. An entry is written by
// the server, but its `changes` hold whatever the owner typed, so nothing here
// trusts a shape: an unknown action, record type or field prints as it came,
// and a value of any type prints as text.
//
// NOTE: the copy is Turkish on purpose - it is panel UI, read by the owner.

import { displayCidr } from './ipExclusion.js'

/* ---------------------------------------------------------------- actions */

/**
 * Every action the server records, with the sentence the "İşlem" column
 * prints. `tone` colours the badge: 'create' green, 'delete' red, 'price'
 * amber, 'neutral' grey.
 */
export const AUDIT_ACTIONS = {
  'product.create': { label: 'Ürün eklendi', tone: 'create' },
  'product.update': { label: 'Ürün düzenlendi', tone: 'neutral' },
  'product.delete': { label: 'Ürün silindi', tone: 'delete' },
  'product.price': { label: 'Fiyat güncellendi', tone: 'price' },
  'product.bulk_price': { label: 'Toplu fiyat güncellemesi', tone: 'price' },
  'product.reorder': { label: 'Ürünler yeniden sıralandı', tone: 'neutral' },
  'category.create': { label: 'Kategori eklendi', tone: 'create' },
  'category.update': { label: 'Kategori düzenlendi', tone: 'neutral' },
  'category.delete': { label: 'Kategori silindi', tone: 'delete' },
  'category.reorder': { label: 'Kategoriler yeniden sıralandı', tone: 'neutral' },
  'menu.create': { label: 'Menü oluşturuldu', tone: 'create' },
  'menu.update': { label: 'Menü ayarları değişti', tone: 'neutral' },
  'menu.delete': { label: 'Menü silindi', tone: 'delete' },
  'business.update': { label: 'İşletme bilgileri değişti', tone: 'neutral' },
  'account.password_change': { label: 'Şifre değiştirildi', tone: 'neutral' },
  'upload.create': { label: 'Dosya yüklendi', tone: 'create' },
  'analytics.exclude_ip.add': { label: 'IP hariç tutuldu', tone: 'create' },
  'analytics.exclude_ip.remove': { label: 'IP listeden çıkarıldı', tone: 'delete' },
}

const has = (object, key) => Object.prototype.hasOwnProperty.call(object, key)
const textOf = (value) => (typeof value === 'string' ? value.trim() : '')

/**
 * The Turkish sentence of an action; an unknown action prints as it came, a
 * missing one as '—'.
 */
export function auditActionLabel(action) {
  const id = textOf(action)
  return has(AUDIT_ACTIONS, id) ? AUDIT_ACTIONS[id].label : id || '—'
}

/** 'create' | 'delete' | 'price' | 'neutral' - the colour of an action's badge. */
export function auditActionTone(action) {
  const id = textOf(action)
  return has(AUDIT_ACTIONS, id) ? AUDIT_ACTIONS[id].tone : 'neutral'
}

/* ----------------------------------------------------------- record types */

/** The record types an entry can be about, in the order the filter lists them. */
export const AUDIT_ENTITY_TYPES = [
  { id: 'product', label: 'Ürün' },
  { id: 'category', label: 'Kategori' },
  { id: 'menu', label: 'Menü' },
  { id: 'business', label: 'İşletme' },
  { id: 'account', label: 'Hesap' },
  { id: 'upload', label: 'Dosya' },
  { id: 'analytics_exclusion', label: 'Hariç tutulan IP' },
]

/** The Turkish name of a record type; an unknown one prints as it came, a missing one as ''. */
export function auditEntityLabel(type) {
  const id = textOf(type)
  return AUDIT_ENTITY_TYPES.find((entity) => entity.id === id)?.label || id
}

/* ----------------------------------------------------------------- fields */

/**
 * The Turkish label of every field an entry can carry: a product's and a
 * category's columns, every setting of the menu settings page (MENU_FIELDS in
 * pages/dashboard/MenuSettings.jsx) and the business record. A field that is
 * not here prints under its own name, so a column added later is still shown.
 */
export const AUDIT_FIELD_LABELS = {
  /* products and categories */
  name: 'Ad',
  description: 'Açıklama',
  ingredients: 'İçindekiler',
  price: 'Fiyat',
  compare_price: 'Karşılaştırma fiyatı',
  old_price: 'Eski fiyat',
  new_price: 'Yeni fiyat',
  calories: 'Kalori',
  image_url: 'Görsel',
  allergens: 'Alerjenler',
  badges: 'Rozetler',
  options: 'Opsiyonlar',
  is_featured: 'Öne çıkan',
  category_id: 'Kategori',
  menu_id: 'Menü',
  icon: 'İkon',
  position: 'Sıra',
  ids: 'Sıralama',
  order: 'Yeni sıralama',
  translations: 'Çeviriler',
  /* a category delete: how many of its products went with it */
  deleted_products: 'Silinen ürün sayısı',
  /* a bulk price update */
  percentage: 'Oran',
  rounding: 'Yuvarlama',
  affected: 'Güncellenen ürün',
  prices: 'Fiyatlar',
  category_ids: 'Kategoriler',
  amount: 'Tutar',
  /* menu identity */
  slogan: 'Slogan',
  slug: 'Adres (URL)',
  logo_url: 'Logo',
  cover_url: 'Kapak görseli',
  header_display: 'Menü başlığı',
  logo_fade_in: 'Logo animasyonu',
  /* contact */
  phone: 'Telefon',
  address: 'Adres',
  instagram: 'Instagram',
  wifi_ssid: 'Wi-Fi ağ adı',
  wifi_password: 'Wi-Fi şifresi',
  links: 'Linkler',
  contact_display: 'Ana ekranda iletişim',
  contact_in_footer: 'Alt bilgide iletişim',
  /* currency and languages */
  currency: 'Para birimi',
  languages: 'Menü dilleri',
  default_language: 'Varsayılan dil',
  /* appearance */
  theme: 'Tasarım tarzı',
  font_family: 'Yazı tipi',
  primary_color: 'Ana renk',
  text_color: 'Metin rengi',
  background_type: 'Arka plan türü',
  background_color: 'Arka plan rengi',
  background_image_url: 'Arka plan görseli',
  background_overlay_opacity: 'Karartma',
  /* splash screen */
  splash_enabled: 'Karşılama ekranı',
  splash_logo_url: 'Karşılama logosu',
  splash_headline: 'Karşılama başlığı',
  splash_text: 'Karşılama alt başlığı',
  splash_bg_color: 'Karşılama arka planı',
  splash_duration: 'Karşılama süresi',
  splash_entrance: 'Giriş animasyonu',
  splash_exit_animation: 'Çıkış animasyonu',
  splash_exit_easing: 'Animasyon eğrisi',
  splash_exit_duration: 'Çıkış süresi',
  splash_display: 'Karşılama görünümü',
  splash_slide_fade: 'Kayma biçimi',
  /* footer notices */
  show_price_date: 'Fiyat tarihi',
  show_vat_note: 'KDV notu',
  vat_note_text: 'KDV notu metni',
  show_yerli_uretim: 'Yerli Üretim rozeti',
  yerli_uretim_logo_url: 'Yerli Üretim logosu',
  /* status */
  is_active: 'Yayında',
  /* uploads and the account */
  url: 'Dosya adresi',
  size: 'Boyut',
  content_type: 'Dosya türü',
  filename: 'Dosya adı',
  email: 'E-posta',
  password: 'Şifre',
  method: 'Yöntem',
  /* an analytics exclusion (analytics.exclude_ip.add / .remove) */
  cidr: 'IP / aralık',
  label: 'Not',
  deleted_events: 'Silinen ziyaret kaydı',
}

/**
 * The label of a field path as the server writes it.
 *
 *   name                    -> "Ad"
 *   translations.en.name    -> "Ad (EN)"
 *   translations.de         -> "Çeviriler (DE)"
 *   links.0.url             -> "Linkler › 0.url" - the parent's label and the
 *                              rest of the path as it came, since the leaf's
 *                              own label ("url" is "Dosya adresi" for an
 *                              upload) would name the wrong thing
 *   some_new_column         -> "some_new_column"
 */
export function auditFieldLabel(path) {
  const field = textOf(path)
  if (!field) return '—'
  if (has(AUDIT_FIELD_LABELS, field)) return AUDIT_FIELD_LABELS[field]

  const parts = field.split('.')
  if (parts[0] === 'translations' && parts.length >= 2) {
    const language = parts[1].toUpperCase()
    const inner = parts.slice(2).join('.')
    const base = inner ? auditFieldLabel(inner) : AUDIT_FIELD_LABELS.translations
    return `${base} (${language})`
  }
  if (parts.length > 1 && has(AUDIT_FIELD_LABELS, parts[0])) {
    return `${AUDIT_FIELD_LABELS[parts[0]]} › ${parts.slice(1).join('.')}`
  }
  return field
}

/* ----------------------------------------------------------------- values */

/**
 * Stored value -> the word the settings page shows for it, for the few fields
 * whose values are codes.
 */
const VALUE_LABELS = {
  contact_display: {
    inline: 'Yan yana',
    list: 'Açık liste',
    hidden: 'Gösterme',
    footer: 'Sadece alt bilgi',
  },
  background_type: { color: 'Renk', image: 'Görsel' },
  /* the bulk price dialog's options (BulkPriceModal ROUNDING_OPTIONS) */
  rounding: {
    none: 'Yuvarlama yok',
    integer: 'Tam sayıya',
    nearest_5: "5'in katına",
    nearest_10: "10'un katına",
    ends_50: "0,50'nin katına",
    ends_95: '…,95 ile bitir',
    ends_99: '…,99 ile bitir',
  },
  /* how a password was changed (passwordChangeRecord) */
  method: { dashboard: 'Panelden', reset_link: 'Sıfırlama bağlantısıyla' },
}

/** What an empty value prints as - quoted text could be mistaken for a value typed in. */
export const EMPTY_VALUE = '(boş)'

/* ----------------------------------------------------------- record names */

/**
 * The fields whose values are ids of other records, and the kind of record
 * each one points at. The server records the id alone, so the page prints the
 * name that record has now, from a lookup the page loads with the entries
 * (auditNameLookup) - never the id, which means nothing to the owner.
 */
const ID_FIELDS = { category_id: 'category', category_ids: 'category', menu_id: 'menu' }

/** An id the loaded list does not have: the record was deleted since. */
const MISSING_RECORD = { category: 'Silinmiş kategori', menu: 'Silinmiş menü' }

/** An id with no list to look it up in: the list could not be loaded. */
const UNKNOWN_RECORD = { category: 'Bilinmeyen kategori', menu: 'Bilinmeyen menü' }

/** A record that exists but has no name in any language. */
const UNNAMED_RECORD = { category: 'İsimsiz kategori', menu: 'İsimsiz menü' }

/** The name in `translations` for `language`, else Turkish, else the first one there is. */
function translatedName(translations, language) {
  if (!translations || typeof translations !== 'object') return ''
  const nameIn = (code) =>
    code && has(translations, code) ? textOf(translations[code]?.name) : ''
  return (
    nameIn(language) || nameIn('tr') || Object.keys(translations).map(nameIn).find(Boolean) || ''
  )
}

/**
 * The names the id fields print with: every menu and category of the business
 * (GET /api/menus, GET /api/categories), by id. A category is named in the
 * default language of its menu - the language the owner writes that menu in,
 * and the one the server names records in for the "Kayıt" column
 * (repository.CategoryNames).
 *
 * A list passed as anything but an array is one the page could not load: its
 * ids print as "Bilinmeyen …" rather than as deleted.
 *
 * @returns {{ menu: Map<string,string>|null, category: Map<string,string>|null }}
 */
export function auditNameLookup({ menus, categories } = {}) {
  const menuNames = new Map()
  const menuLanguages = new Map()
  for (const menu of Array.isArray(menus) ? menus : []) {
    if (!menu || typeof menu.id !== 'string') continue
    menuNames.set(menu.id, textOf(menu.name))
    menuLanguages.set(menu.id, textOf(menu.default_language))
  }

  const categoryNames = new Map()
  for (const category of Array.isArray(categories) ? categories : []) {
    if (!category || typeof category.id !== 'string') continue
    const language = menuLanguages.get(category.menu_id) || ''
    categoryNames.set(category.id, translatedName(category.translations, language))
  }

  return {
    menu: Array.isArray(menus) ? menuNames : null,
    category: Array.isArray(categories) ? categoryNames : null,
  }
}

/** One id of an ID_FIELDS field as the name of the record it points at. */
function recordName(kind, id, names) {
  const known = names && names[kind] instanceof Map ? names[kind] : null
  if (!known) return UNKNOWN_RECORD[kind]
  if (!known.has(id)) return MISSING_RECORD[kind]
  return known.get(id) || UNNAMED_RECORD[kind]
}

/**
 * The contact links of a menu are recorded as the whole list of
 * { id, label, url }. The id is internal - printed, it alone filled the
 * preview - so a link reads as "Web sitemiz (https://example.com)".
 */
function isLinkList(value) {
  return value.every(
    (item) => item !== null && typeof item === 'object' && typeof item.url === 'string',
  )
}

function formatLink(link) {
  const label = textOf(link.label)
  const url = textOf(link.url)
  if (label && url) return `${label} (${url})`
  return label || url || EMPTY_VALUE
}

/** The last segment of a field path: "translations.en.price" -> "price". */
function leafOf(field) {
  const parts = textOf(field).split('.')
  return parts[parts.length - 1]
}

/** A number in the Turkish style, with a fixed or a flexible number of decimals. */
function turkishNumber(value, minimum, maximum) {
  try {
    return value.toLocaleString('tr-TR', {
      minimumFractionDigits: minimum,
      maximumFractionDigits: maximum,
    })
  } catch {
    return String(value)
  }
}

/**
 * Prices print with two decimals, a percentage with its sign in front (the
 * Turkish "%10"), a file size in kilobytes; every other number as it is.
 */
function formatNumber(value, field) {
  const leaf = leafOf(field)
  if (/price|amount/.test(leaf)) return turkishNumber(value, 2, 2)
  if (leaf === 'percentage') return `%${turkishNumber(value, 0, 2)}`
  if (leaf === 'size') return `${turkishNumber(value / 1024, 0, 1)} KB`
  return turkishNumber(value, 0, 4)
}

/**
 * A bulk price update records every product it moved as { id, name, price };
 * such a list reads as "Latte 145,00, Mocha 160,00" rather than as JSON.
 */
function isPricedList(value) {
  return value.every(
    (item) =>
      item !== null &&
      typeof item === 'object' &&
      typeof item.name === 'string' &&
      typeof item.price === 'number',
  )
}

/**
 * One changed value as text.
 *
 *   null, undefined, '' or []  -> "(boş)"
 *   true / false               -> "Açık" / "Kapalı"
 *   145.5 (a price field)      -> "145,50"
 *   ['tr', 'en']               -> "tr, en"
 *   'inline' (contact_display) -> "Yan yana"
 *   a category or menu id      -> its name, looked up in `names`
 *                                 (auditNameLookup); "Silinmiş kategori" when
 *                                 it is gone, "Bilinmeyen kategori" without
 *                                 a lookup
 *   the links of a menu        -> "Web sitemiz (https://example.com), …"
 *   '203.0.113.7/32' (cidr)    -> "203.0.113.7" - a single host without its
 *                                 prefix, as the exclusion list shows it
 *   any other object, or a list of them -> compact JSON
 *
 * A masked value ("••••" for a Wi-Fi password) is text like any other and
 * prints as the server stored it.
 */
export function formatAuditValue(value, field = '', names = null) {
  if (value === null || value === undefined) return EMPTY_VALUE
  if (typeof value === 'boolean') return value ? 'Açık' : 'Kapalı'
  if (typeof value === 'number') {
    return Number.isFinite(value) ? formatNumber(value, field) : String(value)
  }

  const leaf = leafOf(field)

  if (typeof value === 'string') {
    if (value.trim() === '') return EMPTY_VALUE
    if (has(ID_FIELDS, leaf)) return recordName(ID_FIELDS[leaf], value.trim(), names)
    if (leaf === 'cidr') return displayCidr(value)
    const labels = VALUE_LABELS[leaf]
    return labels && has(labels, value) ? labels[value] : value
  }

  if (Array.isArray(value)) {
    if (value.length === 0) return EMPTY_VALUE
    const primitives = value.every(
      (item) => item === null || ['string', 'number', 'boolean'].includes(typeof item),
    )
    if (primitives) return value.map((item) => formatAuditValue(item, field, names)).join(', ')
    if (leaf === 'links' && isLinkList(value)) return value.map(formatLink).join(', ')
    if (isPricedList(value)) {
      return value
        .map((item) => `${item.name.trim() || '—'} ${turkishNumber(item.price, 2, 2)}`)
        .join(', ')
    }
  }

  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

/** How many characters of a value show before "Tümünü göster". */
export const AUDIT_VALUE_PREVIEW = 80

/** `text` cut to `max` characters with an ellipsis, and whether it was cut. */
export function truncateText(text, max = AUDIT_VALUE_PREVIEW) {
  const value = typeof text === 'string' ? text : String(text ?? '')
  const limit = Math.max(1, Math.trunc(max) || AUDIT_VALUE_PREVIEW)
  // Array.from counts code points, so an emoji is never split in half.
  const characters = Array.from(value)
  if (characters.length <= limit) return { text: value, truncated: false }
  return { text: `${characters.slice(0, limit - 1).join('')}…`, truncated: true }
}

/**
 * The rows of the "Değişiklikler" cell, in the order the server wrote them:
 * one per field, with its label and both values already printed.
 *
 * `changes` is `{ "<field>": { "old": any, "new": any } }`. The server
 * writes both keys every time, with null on the side that has nothing: a
 * created record's fields have no old value, a deleted record's no new one,
 * and a logo set for the first time has no old value either. A null side is
 * therefore left out rather than printed as "(boş)", so a creation reads as
 * the values it was created with. (An empty string is a value, and still
 * prints as "(boş)": "the slogan was cleared" is worth saying.)
 *
 * A field whose record is not such an object is shown with the whole record
 * as its new value rather than dropped: the history must never hide that
 * something changed. Anything that is not an object at all gives no rows.
 *
 * @param {object} [options]
 * @param {object} [options.names] - auditNameLookup's result, for the fields
 *   that hold a category or menu id
 * @param {number} [options.preview] - the length `long` is measured against
 * @returns {{ field, label, before, after, hasBefore, hasAfter, long }[]}
 *   `hasBefore` / `hasAfter` say which sides carry a value; at least one
 *   always does. `long` is true when either printed value is longer than
 *   the preview.
 */
export function auditChangeRows(changes, { names = null, preview = AUDIT_VALUE_PREVIEW } = {}) {
  if (!changes || typeof changes !== 'object' || Array.isArray(changes)) return []

  return Object.keys(changes).map((field) => {
    const entry = changes[field]
    const shaped = entry && typeof entry === 'object' && !Array.isArray(entry)
    const present = (key) =>
      shaped && has(entry, key) && entry[key] !== null && entry[key] !== undefined
    const hasBefore = present('old')
    // Nothing on either side still gets a row - "(boş)" - so the field shows.
    const hasAfter = shaped ? present('new') || !hasBefore : true

    const before = hasBefore ? formatAuditValue(entry.old, field, names) : ''
    const after = hasAfter ? formatAuditValue(shaped ? entry.new : entry, field, names) : ''
    const tooLong = (text) => Array.from(text).length > preview

    return {
      field,
      label: auditFieldLabel(field),
      before,
      after,
      hasBefore,
      hasAfter,
      long: tooLong(before) || tooLong(after),
    }
  })
}
