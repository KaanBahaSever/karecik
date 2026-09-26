// Visitor analytics of the customer menu: what the owner's dashboard counts.
//
// Three events, sent to POST /api/public/events:
//
//   menu_view      a resolved menu was shown — once per menu per page load,
//                  never again for the refetch a language switch makes
//   category_view  a category was opened, from the grid or from the chip row
//   product_view   a product's detail sheet was opened
//
// The caller decides WHETHER to send (never from the landing page's demo
// iframe or the dashboard's live preview, never for the tenant directory);
// this file decides HOW, and the answer to that is "without the visitor ever
// noticing":
//
//   - navigator.sendBeacon first. It queues the request with the browser and
//     returns at once, and the browser delivers it even when the visitor
//     closes the tab straight after tapping. The body is a plain string, which
//     the beacon sends as text/plain — a type that needs no CORS preflight.
//     The server parses the body as JSON whatever its Content-Type says.
//   - fetch with keepalive where there is no beacon, or where the beacon's
//     queue refused the request (it returns false when full).
//   - nothing is awaited, every failure is swallowed, and nothing here ever
//     throws. A lost event costs a number on a chart; an exception here would
//     cost the menu.
//
// OPTING OUT. A browser whose owner switched on "Bu tarayıcıdan yapılan
// ziyaretleri sayma" in the panel carries the karecik_analytics_optout=1
// cookie (POST /api/analytics/optout sets it for the whole platform domain,
// so every tenant subdomain and the path-form menu see it). Such a browser
// sends nothing at all: no beacon, and no visitor id is even created. The
// server drops the event anyway when the cookie reaches it; this is only the
// courtesy of not sending what would be thrown away.
//
// The URL is built exactly as lib/api.js builds its own: VITE_API_URL (empty
// means same origin) followed by the /api path.

/*
  `import.meta.env` is read defensively. Vite replaces it at build time, as it
  does in api.js, but the plain-Node test runner has no such object, and
  `import.meta.env.VITE_API_URL` would throw there while the module loads.
*/
const API_BASE = (import.meta.env && import.meta.env.VITE_API_URL) || ''

/** Where every event goes. */
export const EVENTS_URL = `${API_BASE}/api/public/events`

export const MENU_EVENT_TYPES = ['menu_view', 'category_view', 'product_view']

/** localStorage key of the anonymous visitor id. */
export const VISITOR_ID_KEY = 'karecik_visitor_id'

/**
 * The cookie of a browser whose visits are never counted. Written by the
 * server (POST /api/analytics/optout) and deliberately not HttpOnly, so this
 * file and the panel's switch can read it.
 */
export const OPTOUT_COOKIE = 'karecik_analytics_optout'

/**
 * Whether a cookie header - `document.cookie` - carries the opt-out marker,
 * `karecik_analytics_optout=1`. Only that exact value counts: an expired or
 * emptied cookie is how the switch is turned off.
 *
 * @param {string} cookieText
 * @returns {boolean}
 */
export function hasAnalyticsOptout(cookieText) {
  if (typeof cookieText !== 'string' || !cookieText) return false
  return cookieText.split(';').some((pair) => {
    const separator = pair.indexOf('=')
    if (separator < 0) return false
    const name = pair.slice(0, separator).trim()
    return name === OPTOUT_COOKIE && pair.slice(separator + 1).trim() === '1'
  })
}

/** document.cookie, or '' where there is no document or reading it throws (sandboxed frames). */
export function browserCookies() {
  try {
    if (typeof document === 'undefined') return ''
    return typeof document.cookie === 'string' ? document.cookie : ''
  } catch {
    return ''
  }
}

/** What the server accepts as a visitor id: 1-64 of [A-Za-z0-9_-]. */
const VISITOR_ID_PATTERN = /^[A-Za-z0-9_-]{1,64}$/

/**
 * @param {unknown} value
 * @returns {boolean}
 */
export function isValidVisitorId(value) {
  return typeof value === 'string' && VISITOR_ID_PATTERN.test(value)
}

/**
 * A fresh random visitor id: a UUID where the browser can make one (only in a
 * secure context — a menu opened over plain http on a LAN address cannot),
 * else 32 hex digits from getRandomValues, else a Math.random fallback that is
 * unique enough to tell visitors apart, which is all the id is for.
 *
 * @param {Crypto} [cryptoImpl]
 * @returns {string}
 */
export function createVisitorId(cryptoImpl = globalThis.crypto) {
  try {
    if (cryptoImpl && typeof cryptoImpl.randomUUID === 'function') {
      const id = cryptoImpl.randomUUID()
      if (isValidVisitorId(id)) return id
    }
  } catch {
    /* insecure context: fall through */
  }

  try {
    if (cryptoImpl && typeof cryptoImpl.getRandomValues === 'function') {
      const bytes = cryptoImpl.getRandomValues(new Uint8Array(16))
      return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
    }
  } catch {
    /* fall through */
  }

  return `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 12)}`
}

/** window.localStorage, or null where reading it throws (sandboxes, blocked site data). */
function browserStorage() {
  try {
    return typeof window !== 'undefined' ? window.localStorage : null
  } catch {
    return null
  }
}

/* The id of this page load when storage cannot keep one. The events of one
   visit then still count as one visitor, which is the part that matters. */
let pageVisitorId = ''

/**
 * The anonymous id the server counts unique visitors by: read from storage,
 * created and stored on the first visit. A storage that refuses to read or to
 * write is tolerated — the id then lives for this page load only.
 *
 * It identifies a browser, not a person, and it is never sent anywhere but
 * the events endpoint.
 *
 * @param {Storage|null} [storage]
 * @returns {string}
 */
export function getVisitorId(storage = browserStorage()) {
  let stored = ''
  try {
    stored = storage ? storage.getItem(VISITOR_ID_KEY) : ''
  } catch {
    stored = ''
  }
  if (isValidVisitorId(stored)) return stored

  if (!isValidVisitorId(pageVisitorId)) pageVisitorId = createVisitorId()
  try {
    if (storage) storage.setItem(VISITOR_ID_KEY, pageVisitorId)
  } catch {
    /* private window or full quota: this page load's id it is */
  }
  return pageVisitorId
}

/** A non-empty trimmed string, or ''. */
function text(value) {
  return typeof value === 'string' ? value.trim() : ''
}

/**
 * The JSON body of one event, or null when the event cannot be sent at all:
 * an unknown type, or no business or menu slug to attribute it to. Optional
 * fields are left out rather than sent empty.
 *
 * @param {object} event
 * @returns {object|null}
 */
export function buildEventBody({
  businessSlug,
  menuSlug,
  type,
  categoryId,
  productId,
  language,
  visitorId,
} = {}) {
  const business = text(businessSlug)
  const menu = text(menuSlug)
  if (!business || !menu || !MENU_EVENT_TYPES.includes(type)) return null

  const body = { business_slug: business, menu_slug: menu, type }

  const category = typeof categoryId === 'number' ? String(categoryId) : text(categoryId)
  const product = typeof productId === 'number' ? String(productId) : text(productId)
  if (category) body.category_id = category
  if (product) body.product_id = product
  if (isValidVisitorId(visitorId)) body.visitor_id = visitorId
  if (text(language)) body.language = text(language)

  return body
}

/**
 * Sends one event and returns whether it was handed to the browser. Never
 * throws and never waits. A browser that opted out (OPTOUT_COOKIE) sends
 * nothing and answers false.
 *
 * @param {object} event - { businessSlug, menuSlug, type, categoryId?, productId?, language? }
 * @param {object} [transport] - test seams: { navigator, fetch, storage, cookie }
 * @returns {boolean}
 */
export function trackMenuEvent(event, transport = {}) {
  try {
    // Asked first: an opted-out browser should not even get a visitor id.
    const cookie = 'cookie' in transport ? transport.cookie : browserCookies()
    if (hasAnalyticsOptout(cookie)) return false

    const storage = 'storage' in transport ? transport.storage : browserStorage()
    const body = buildEventBody({ ...event, visitorId: getVisitorId(storage) })
    if (!body) return false

    const payload = JSON.stringify(body)
    const nav = 'navigator' in transport ? transport.navigator : globalThis.navigator
    const fetchImpl = 'fetch' in transport ? transport.fetch : globalThis.fetch

    if (nav && typeof nav.sendBeacon === 'function') {
      try {
        if (nav.sendBeacon(EVENTS_URL, payload)) return true
      } catch {
        /* a beacon refused outright: try fetch */
      }
    }

    if (typeof fetchImpl === 'function') {
      // A string body goes out as text/plain, like the beacon's: no preflight.
      const pending = fetchImpl(EVENTS_URL, { method: 'POST', body: payload, keepalive: true })
      if (pending && typeof pending.catch === 'function') pending.catch(() => {})
      return true
    }
  } catch {
    /* analytics never breaks the menu */
  }
  return false
}
