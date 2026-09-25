// Which language the customer menu opens in, and the visitor's remembered pick.
//
// HOW THE LANGUAGE IS CHOSEN
//
// The backend negotiates first. A request without ?lang= is answered in the
// first language of the browser's Accept-Language header that the menu offers
// ('de-DE' counts as 'de'), else in the menu's default language, and the
// payload says which one it used in `language`. That header is exactly what
// navigator.languages lists, so the first paint is already in the visitor's
// language: no second request, no flash of Turkish before German.
//
// The client then settles on one language for the whole screen — the texts AND
// the interface around them — in this order:
//
//   1. the visitor's explicit pick: tapped on this visit, named by the
//      address's ?lang= (the landing page's demo iframe passes its own
//      language that way), or remembered from an earlier visit — and only
//      while the menu still offers it
//   2. payload.language, the language the server resolved the texts in
//   3. navigator.languages matched against the menu's languages — a payload
//      from a server that does not report `language` yet
//   4. the menu's default_language
//   5. 'tr'
//
// Without a menu to ask — the loading screen, "menu not found", the tenant
// directory — the pool is every language the interface is written in, so a
// German visitor who mistyped an address is told so in German.
//
// Pure functions only, apart from the storage helpers at the bottom, which take
// the storage as an argument and swallow every error it can throw.

import { LANGUAGE_CODES } from '../locales/index.js'

/** localStorage key prefix; the tenant slug completes it. */
export const LANGUAGE_STORAGE_PREFIX = 'karecik_lang_'

/**
 * The primary subtag of a BCP 47 tag, lowercased: 'de-DE' -> 'de',
 * 'pt_BR' -> 'pt', ' EN ' -> 'en'. Anything but a string gives ''.
 *
 * @param {unknown} tag
 * @returns {string}
 */
export function baseLanguage(tag) {
  if (typeof tag !== 'string') return ''
  return tag.trim().split(/[-_]/)[0].toLowerCase()
}

/** The language codes of a list, as plain strings; anything else is dropped. */
function codesOf(list) {
  return Array.isArray(list)
    ? list.filter((code) => typeof code === 'string' && code !== '')
    : []
}

/**
 * The first of the visitor's preferred languages that `available` offers, or ''.
 * Preferences are compared by their primary subtag, the way the server
 * compares Accept-Language, so 'de-AT' picks a menu's 'de'.
 *
 * @param {unknown} preferred - navigator.languages, most preferred first
 * @param {unknown} available - the language codes on offer
 * @returns {string}
 */
export function matchPreferredLanguage(preferred, available) {
  const offered = codesOf(available)
  if (!Array.isArray(preferred) || offered.length === 0) return ''

  for (const tag of preferred) {
    const base = baseLanguage(tag)
    if (base && offered.includes(base)) return base
  }
  return ''
}

/**
 * The browser's preferred languages, most preferred first — navigator.languages,
 * or navigator.language alone where the list is missing. [] outside a browser.
 *
 * @param {object} [nav] - defaults to the global navigator
 * @returns {string[]}
 */
export function browserLanguages(nav = globalThis.navigator) {
  try {
    if (!nav) return []
    if (Array.isArray(nav.languages) && nav.languages.length > 0) return codesOf(nav.languages)
    return codesOf([nav.language])
  } catch {
    return []
  }
}

/**
 * The language the customer menu renders in. See the order at the top of this
 * file.
 *
 * @param {object}   input
 * @param {string}   input.choice    - The visitor's explicit or remembered pick
 * @param {string}   input.served    - payload.language
 * @param {string[]} input.preferred - browserLanguages()
 * @param {string[]} input.available - business.languages; empty without a menu
 * @param {string}   input.fallback  - business.default_language
 * @returns {string}
 */
export function resolveMenuLanguage({ choice, served, preferred, available, fallback } = {}) {
  const menuLanguages = codesOf(available)
  const pool = menuLanguages.length > 0 ? menuLanguages : LANGUAGE_CODES
  const offered = (code) => typeof code === 'string' && pool.includes(code)

  if (offered(choice)) return choice
  if (offered(served)) return served

  const matched = matchPreferredLanguage(preferred, pool)
  if (matched) return matched

  if (typeof fallback === 'string' && fallback !== '') return fallback
  return 'tr'
}

/* ----------------------------------------------------- the remembered pick */

/*
  One key per TENANT, not per menu: a visitor who reads one of a café's menus in
  English wants its other menus in English too, while another café's menus are
  none of that choice's business. Slugs are compared lowercased, because the
  address a visitor typed may not be the case the tenant was stored in.

  A pick is never forgotten because the menu on screen does not offer it. The
  tenant's menus need not offer the same languages — one in six, its sibling
  in two — and a visitor who picked German on the first must find German there
  again after a look at the second. The menu that lacks it passes it over
  anyway: resolveMenuLanguage only accepts a pick the menu offers, and the
  server treats a ?lang= the menu does not offer as absent.
*/

/**
 * @param {unknown} tenant - the business slug
 * @returns {string} the storage key, or '' when there is no tenant to key it on
 */
export function languageStorageKey(tenant) {
  const slug = typeof tenant === 'string' ? tenant.trim().toLowerCase() : ''
  return slug ? `${LANGUAGE_STORAGE_PREFIX}${slug}` : ''
}

/** window.localStorage, or null where reading it throws (sandboxes, blocked site data). */
export function browserStorage() {
  try {
    return typeof window !== 'undefined' ? window.localStorage : null
  } catch {
    return null
  }
}

/**
 * The language remembered for a tenant, or ''. Only one of the interface's own
 * languages counts; whether the menu still OFFERS it is resolveMenuLanguage's
 * question, answered once the payload is in.
 *
 * @param {unknown} tenant
 * @param {Storage|null} [storage]
 * @returns {string}
 */
export function readRememberedLanguage(tenant, storage = browserStorage()) {
  const key = languageStorageKey(tenant)
  if (!key || !storage) return ''
  try {
    const code = storage.getItem(key)
    return LANGUAGE_CODES.includes(code) ? code : ''
  } catch {
    return ''
  }
}

/**
 * Remembers an explicit pick for the tenant. Failing to store it is not an
 * error — the pick simply lasts for this visit only.
 *
 * @param {unknown} tenant
 * @param {string} code
 * @param {Storage|null} [storage]
 */
export function rememberLanguage(tenant, code, storage = browserStorage()) {
  const key = languageStorageKey(tenant)
  if (!key || !storage || !LANGUAGE_CODES.includes(code)) return
  try {
    storage.setItem(key, code)
  } catch {
    /* private windows and full quotas refuse writes; the pick lasts this visit */
  }
}
