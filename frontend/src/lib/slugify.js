// Turkish-aware slugification, mirroring backend/internal/utils/slug.go.
//
// The dashboard shows a live public-URL preview while the user types a menu
// name, so the two implementations must agree character for character —
// otherwise the preview promises one address and the server stores another.
// Verified against the Go function:
//
//   "Kadıköy Kahve Barı"  -> kadikoy-kahve-bari
//   "Bahçe Menüsü"        -> bahce-menusu
//   "İstanbul Işık"       -> istanbul-isik      (dotted and dotless I both -> i)
//   "Bar & Kokteyl"       -> bar-ve-kokteyl
//   "ÇĞİIÖŞÜ"             -> cgiiosu
//
// NOTE: the server is still the authority. This module exists to preview and to
// pre-format typed input; the final slug always comes back from the API, which
// resolves collisions by appending -2, -3 … before inserting.

/* Turkish (and common circumflex) letters mapped to ASCII, plus the ampersand
   spelled out the way Turkish reads it. Uppercase forms are listed explicitly
   rather than relying on toLowerCase(): JavaScript lowercases "I" to "i" using
   the invariant rule, which is what we want here, but being explicit keeps this
   table readable next to its Go counterpart. */
const TURKISH_MAP = {
  ç: 'c', Ç: 'c',
  ğ: 'g', Ğ: 'g',
  ı: 'i', I: 'i', İ: 'i',
  ö: 'o', Ö: 'o',
  ş: 's', Ş: 's',
  ü: 'u', Ü: 'u',
  â: 'a', Â: 'a',
  î: 'i', Î: 'i',
  û: 'u', Û: 'u',
  '&': '-ve-',
}

const TURKISH_PATTERN = /[çÇğĞıIİöÖşŞüÜâÂîÎûÛ&]/g

/** Longest slug the backend accepts (utils.IsValidSlug). */
export const MAX_SLUG_LENGTH = 60

/** Shortest slug the backend accepts. */
export const MIN_SLUG_LENGTH = 2

/**
 * Turns free text into a URL-safe slug.
 *
 * @param {string} input
 * @param {string} fallback - Used when the input reduces to nothing at all.
 *                            Menus use "menu"; the Go side takes the same
 *                            argument so both ends agree on the empty case.
 */
export function slugify(input, fallback = 'menu') {
  let value = String(input ?? '').trim()

  // Map the Turkish letters BEFORE lowercasing, so "İ" and "I" both land on a
  // plain "i" instead of going through JavaScript's locale-invariant rules.
  value = value.replace(TURKISH_PATTERN, (letter) => TURKISH_MAP[letter] ?? letter)

  value = value
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, '-')
    .replace(/-{2,}/g, '-')
    .replace(/^-+|-+$/g, '')

  if (!value) return fallback
  if (value.length > MAX_SLUG_LENGTH) {
    value = value.slice(0, MAX_SLUG_LENGTH).replace(/-+$/, '')
  }
  return value
}

/**
 * Cleans a slug the user is typing by hand.
 *
 * Unlike slugify() this keeps a trailing hyphen, because removing it mid-word
 * makes the field fight the user ("bar-" would collapse to "bar" before they
 * finish typing "bar-kokteyl"). Call slugify() again on submit.
 */
export function cleanSlugInput(input) {
  return String(input ?? '')
    .replace(TURKISH_PATTERN, (letter) => TURKISH_MAP[letter] ?? letter)
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, '-')
    .replace(/-{2,}/g, '-')
    .replace(/^-+/, '')
    .slice(0, MAX_SLUG_LENGTH)
}

/** Mirrors utils.IsValidSlug — length, charset, and no leading/trailing hyphen. */
export function isValidSlug(slug) {
  const value = String(slug ?? '')
  if (value.length < MIN_SLUG_LENGTH || value.length > MAX_SLUG_LENGTH) return false
  if (value.startsWith('-') || value.endsWith('-')) return false
  return /^[a-z0-9-]+$/.test(value)
}

/** Mirrors utils.reservedSlugs — kept in sync with backend/internal/utils/slug.go. */
export const RESERVED_SLUGS = new Set([
  'www', 'api', 'admin', 'app', 'panel', 'mail', 'ftp', 'blog', 'help',
  'destek', 'karecik', 'static', 'cdn', 'assets', 'dashboard', 'login',
  'register', 'demo',
])

export function isReservedSlug(slug) {
  return RESERVED_SLUGS.has(String(slug ?? '').toLowerCase())
}
