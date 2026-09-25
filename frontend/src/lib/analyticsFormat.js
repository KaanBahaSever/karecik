// The small, pure pieces of the owner panel's analytics page ("Analitik"): the
// date ranges its filters offer, the Turkish labels of the event types and of
// the address sources, and the formatters its tables and its chart print with.
//
// Pure functions only - no React, no DOM - so frontend/tests/
// dashboardAnalytics.test.mjs runs them with plain node. Every helper takes
// whatever the API or the owner hands it and answers '' / '—' / a safe default
// for anything it cannot use; none of them throws.
//
// NOTE: the copy is Turkish on purpose - it is panel UI, read by the owner.

/* ------------------------------------------------------------- the calendar */

/**
 * The zone every analytics day is counted in. The API reads `from` and `to` as
 * inclusive days in Europe/Istanbul (GET /api/analytics/summary), and the price
 * date of the customer menu uses the same zone (lib/format.js), so a day means
 * the same thing wherever the owner's computer happens to be.
 */
export const ANALYTICS_TIME_ZONE = 'Europe/Istanbul'

const YMD_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/

/* Written out rather than asked of Intl: the axis and the tooltips must read the
   same on every browser, and a test must be able to pin them down exactly. */
const MONTHS = [
  'Ocak',
  'Şubat',
  'Mart',
  'Nisan',
  'Mayıs',
  'Haziran',
  'Temmuz',
  'Ağustos',
  'Eylül',
  'Ekim',
  'Kasım',
  'Aralık',
]
const MONTHS_SHORT = [
  'Oca',
  'Şub',
  'Mar',
  'Nis',
  'May',
  'Haz',
  'Tem',
  'Ağu',
  'Eyl',
  'Eki',
  'Kas',
  'Ara',
]
const WEEKDAYS = ['Pazar', 'Pazartesi', 'Salı', 'Çarşamba', 'Perşembe', 'Cuma', 'Cumartesi']

/** The UTC midnight of a 'YYYY-MM-DD' day, or null when it is not a real day. */
function utcDay(value) {
  const match = YMD_PATTERN.exec(typeof value === 'string' ? value : '')
  if (!match) return null

  const year = Number(match[1])
  const month = Number(match[2])
  const day = Number(match[3])
  const date = new Date(Date.UTC(year, month - 1, day))

  // Date.UTC rolls 2026-02-31 over into March; a day that rolled is not a day.
  if (
    date.getUTCFullYear() !== year ||
    date.getUTCMonth() !== month - 1 ||
    date.getUTCDate() !== day
  ) {
    return null
  }
  return date
}

/** Whether `value` is a real calendar day written as 'YYYY-MM-DD'. */
export function isValidDay(value) {
  return utcDay(value) !== null
}

/** Reads the calendar parts of `date` in `timeZone`; null when Intl cannot. */
function zonedParts(date, timeZone, withTime) {
  try {
    const options = { timeZone, year: 'numeric', month: '2-digit', day: '2-digit' }
    if (withTime) {
      Object.assign(options, {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hourCycle: 'h23',
      })
    }

    const parts = {}
    new Intl.DateTimeFormat('en-GB', options).formatToParts(date).forEach((part) => {
      parts[part.type] = part.value
    })
    if (!parts.year || !parts.month || !parts.day) return null
    // Some engines print midnight as "24" even with h23; the day is already right.
    if (parts.hour === '24') parts.hour = '00'
    return parts
  } catch {
    return null
  }
}

/**
 * The calendar day of `date` in Europe/Istanbul, as 'YYYY-MM-DD'.
 *
 * Should Intl or the zone be unavailable, the browser's own calendar day is
 * the fallback - off by one around midnight at worst, never empty.
 *
 * @param {Date|string|number} date - defaults to now
 * @returns {string} '' for an invalid date
 */
export function dayInZone(date = new Date(), timeZone = ANALYTICS_TIME_ZONE) {
  const value = date instanceof Date ? date : new Date(date)
  if (Number.isNaN(value.getTime())) return ''

  const parts = zonedParts(value, timeZone, false)
  if (parts) return `${parts.year}-${parts.month}-${parts.day}`

  const month = String(value.getMonth() + 1).padStart(2, '0')
  const day = String(value.getDate()).padStart(2, '0')
  return `${value.getFullYear()}-${month}-${day}`
}

/** 'YYYY-MM-DD' plus `days` (negative goes back); '' for an invalid day. */
export function addDays(day, days) {
  const date = utcDay(day)
  if (!date || !Number.isFinite(days)) return ''
  date.setUTCDate(date.getUTCDate() + Math.trunc(days))
  return date.toISOString().slice(0, 10)
}

/**
 * How many days `from`..`to` covers, both ends included; 0 when either is
 * invalid or they are reversed.
 */
export function daysInRange(from, to) {
  const start = utcDay(from)
  const end = utcDay(to)
  if (!start || !end || end < start) return 0
  return Math.round((end - start) / 86400000) + 1
}

/**
 * Every day of `from`..`to`, both ends included. At most `limit` days, so a
 * typo cannot allocate a century.
 */
export function listDays(from, to, limit = 3660) {
  const count = Math.min(daysInRange(from, to), limit)
  const days = []
  for (let index = 0; index < count; index += 1) days.push(addDays(from, index))
  return days
}

/* ----------------------------------------------------------- range presets */

/**
 * The quick ranges of the filter row, shortest first. `days` counts today, so
 * "Son 7 gün" is today and the six days before it - the same inclusive days
 * the API counts.
 */
export const DATE_RANGE_PRESETS = [
  { id: 'today', label: 'Bugün', days: 1 },
  { id: '7d', label: 'Son 7 gün', days: 7 },
  { id: '30d', label: 'Son 30 gün', days: 30 },
  { id: '90d', label: 'Son 90 gün', days: 90 },
]

/** The API's own default range is the last 30 days; the page opens on the same. */
export const DEFAULT_RANGE_PRESET = '30d'

/**
 * The { from, to } of a preset, ending today in Europe/Istanbul. An unknown id
 * answers the default preset's range.
 */
export function presetRange(id, now = new Date()) {
  const preset =
    DATE_RANGE_PRESETS.find((candidate) => candidate.id === id) ||
    DATE_RANGE_PRESETS.find((candidate) => candidate.id === DEFAULT_RANGE_PRESET)
  const to = dayInZone(now)
  return { from: addDays(to, -(preset.days - 1)), to }
}

/**
 * A custom range the API can take: both days valid, and in order - a reversed
 * pair is swapped rather than refused, since the owner obviously meant the
 * days between them. null when either day is missing or not a real day.
 */
export function normalizeRange(from, to) {
  if (!isValidDay(from) || !isValidDay(to)) return null
  return from <= to ? { from, to } : { from: to, to: from }
}

/**
 * The longest range the summary answers, both ends included: maxAnalyticsDays
 * of backend/internal/handlers/analytics.go, which refuses anything longer
 * with a 422.
 */
export const MAX_RANGE_DAYS = 366

/**
 * The custom range of the "Özel" form, checked before it reaches the API:
 * normalizeRange's rules, and no longer than MAX_RANGE_DAYS - a range the
 * summary refuses would leave the cards and the chart empty under a retry
 * button that can never succeed.
 *
 * @returns {{ range: {from: string, to: string}|null, problem: string }}
 *   exactly one of the two is set; `problem` is the Turkish message to show.
 */
export function checkCustomRange(from, to) {
  const range = normalizeRange(from, to)
  if (!range) return { range: null, problem: 'Başlangıç ve bitiş tarihini seçin.' }
  if (daysInRange(range.from, range.to) > MAX_RANGE_DAYS) {
    return { range: null, problem: `Tarih aralığı en fazla ${MAX_RANGE_DAYS} gün olabilir.` }
  }
  return { range, problem: '' }
}

/** The preset a range is, or 'custom' when it matches none of them today. */
export function matchingPreset(range, now = new Date()) {
  if (!range) return 'custom'
  const found = DATE_RANGE_PRESETS.find((preset) => {
    const candidate = presetRange(preset.id, now)
    return candidate.from === range.from && candidate.to === range.to
  })
  return found ? found.id : 'custom'
}

/* ------------------------------------------------------------ date printing */

/**
 * 'YYYY-MM-DD' -> "25 Eyl" (short, for the chart axis) or
 * "25 Eylül 2026, Perşembe" (long, for tooltips and screen readers).
 * '' for an invalid day.
 */
export function formatDay(day, style = 'short') {
  const date = utcDay(day)
  if (!date) return ''

  const dayOfMonth = date.getUTCDate()
  const month = date.getUTCMonth()
  if (style === 'long') {
    return `${dayOfMonth} ${MONTHS[month]} ${date.getUTCFullYear()}, ${WEEKDAYS[date.getUTCDay()]}`
  }
  if (style === 'numeric') {
    const pad = (number) => String(number).padStart(2, '0')
    return `${pad(dayOfMonth)}.${pad(month + 1)}.${date.getUTCFullYear()}`
  }
  return `${dayOfMonth} ${MONTHS_SHORT[month]}`
}

/** "1 Eyl – 30 Eyl 2026": the range under the page title. */
export function formatRange(from, to) {
  if (!isValidDay(from) || !isValidDay(to)) return ''
  if (from === to) return formatDay(from, 'numeric')
  return `${formatDay(from, 'numeric')} – ${formatDay(to, 'numeric')}`
}

/**
 * An RFC 3339 timestamp -> "25.09.2026 14:03:12", as the wall clock in
 * Europe/Istanbul - never the browser's zone, for the reason given at
 * ANALYTICS_TIME_ZONE. '—' for a missing or unreadable value.
 */
export function formatDateTime(value) {
  if (value === null || value === undefined || value === '') return '—'
  const date = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(date.getTime())) return '—'

  const parts = zonedParts(date, ANALYTICS_TIME_ZONE, true)
  if (parts) {
    return `${parts.day}.${parts.month}.${parts.year} ${parts.hour}:${parts.minute}:${parts.second}`
  }

  const pad = (number) => String(number).padStart(2, '0')
  return (
    `${pad(date.getDate())}.${pad(date.getMonth() + 1)}.${date.getFullYear()} ` +
    `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
  )
}

/* ---------------------------------------------------------------- numbers */

/**
 * 1234567 -> "1.234.567", the Turkish grouping. Written by hand so the output
 * does not depend on the locale data a browser ships. Non-numbers print "0":
 * a counter the API left out counted nothing.
 */
export function formatCount(value) {
  const number = Number(value)
  if (!Number.isFinite(number)) return '0'
  const rounded = Math.round(number)
  const digits = String(Math.abs(rounded)).replace(/\B(?=(\d{3})+(?!\d))/g, '.')
  return rounded < 0 ? `-${digits}` : digits
}

/**
 * A clean top for a count axis and its step: 1, 2 or 5 times a power of ten,
 * never below 1 because a visitor count has no fractions. An all-zero series
 * still gets a small axis (0..tickCount), so the chart keeps its shape.
 *
 * @returns {{ max: number, step: number }}
 */
export function niceAxis(maxValue, tickCount = 4) {
  const top = Number(maxValue)
  const ticks = Math.max(1, Math.trunc(tickCount) || 1)
  if (!Number.isFinite(top) || top <= 0) return { max: ticks, step: 1 }

  const rough = top / ticks
  const magnitude = 10 ** Math.floor(Math.log10(rough))
  const step = Math.max(
    1,
    [1, 2, 5, 10].map((factor) => factor * magnitude).find((candidate) => candidate >= rough),
  )
  return { max: Math.ceil(top / step) * step, step }
}

/**
 * How many days apart the x-axis labels sit so that neighbouring labels keep
 * at least `minSpacing` px between them: 1 labels every day, 7 every week.
 */
export function labelInterval(dayCount, plotWidth, minSpacing = 56) {
  const count = Math.trunc(Number(dayCount))
  const width = Number(plotWidth)
  if (!(count > 1) || !(width > 0)) return 1
  const fits = Math.max(1, Math.floor(width / minSpacing))
  return Math.max(1, Math.ceil(count / fits))
}

/* -------------------------------------------------------------- pagination */

/**
 * Where one page of a list sits, for the "1-50 / 312" line and the two
 * buttons under a table. `start` is 1-based; both are 0 for an empty list.
 */
export function pageInfo(total, limit, offset) {
  const count = Math.max(0, Math.trunc(Number(total)) || 0)
  const size = Math.max(1, Math.trunc(Number(limit)) || 1)
  const first = Math.max(0, Math.trunc(Number(offset)) || 0)

  const start = count === 0 || first >= count ? 0 : first + 1
  const end = start === 0 ? 0 : Math.min(first + size, count)

  return {
    start,
    end,
    total: count,
    page: Math.floor(first / size) + 1,
    pages: Math.max(1, Math.ceil(count / size)),
    hasPrevious: first > 0,
    hasNext: first + size < count,
    previousOffset: Math.max(0, first - size),
    nextOffset: first + size,
  }
}

/* ------------------------------------------------------------- event types */

/**
 * The three things the customer menu reports (POST /api/public/events), with
 * the labels the event filter and the visit log print. A menu_view is sent
 * once per opening of the menu and counts as one visit - the "Toplam ziyaret"
 * card is the number of them - so its label says "opening", like the hint
 * under that card.
 */
export const EVENT_TYPES = [
  { id: 'menu_view', label: 'Menü açılışı' },
  { id: 'category_view', label: 'Kategori görüntüleme' },
  { id: 'product_view', label: 'Ürün detayı açılışı' },
]

/**
 * The Turkish label of an event type; an unknown type prints as it came, a
 * missing one as '—'.
 */
export function eventTypeLabel(type) {
  const found = EVENT_TYPES.find((candidate) => candidate.id === type)
  if (found) return found.label
  return typeof type === 'string' && type.trim() ? type.trim() : '—'
}

/* --------------------------------------------------- addresses and sources */

/**
 * An address as the log prints it. The server writes "-" when it could not
 * establish one (clientip.IPUnknown); that and a missing value both print '—'.
 */
export function formatIp(ip) {
  const text = typeof ip === 'string' ? ip.trim() : ''
  return text && text !== '-' ? text : '—'
}

/**
 * A source port, or '—' when none was established: null, "-", or anything
 * that is not 1..65535.
 */
export function formatPort(port) {
  if (port === null || port === undefined || port === '' || port === '-') return '—'
  const number = Number(port)
  return Number.isInteger(number) && number > 0 && number <= 65535 ? String(number) : '—'
}

/**
 * Where a logged address came from, and so how far it can be trusted. The ids
 * are the clientip.Source values of the backend (internal/clientip); the
 * `description` is the one-line explanation the "Kaynak" column shows on hover.
 *
 * `tone` picks the badge colour: 'good' is proof, 'warn' a strong guess or a
 * value the visitor could have written, 'neutral' an address that identifies
 * nobody in particular.
 *
 * A port is recorded for two sources only (clientip.Resolve): 'cloudflare',
 * where it is the visitor's own source port as Cloudflare saw it, and 'peer',
 * where it is the port of the TCP connection - the visitor's only when nothing
 * stands between them and the server. The two descriptions say which.
 */
export const IP_SOURCES = {
  cloudflare: {
    label: 'Cloudflare',
    detail: 'doğrulanmış',
    tone: 'good',
    description:
      "Adres Cloudflare üzerinden geldi ve paylaşılan gizli anahtarla doğrulandı. En güvenilir kaynaktır; port, ziyaretçinin Cloudflare'e bağlandığı kaynak portudur.",
  },
  'cloudflare-unverified': {
    label: 'Cloudflare',
    detail: 'doğrulanmamış',
    tone: 'warn',
    description:
      "Adres Cloudflare başlığından okundu, ancak isteğin gerçekten Cloudflare'den geldiği doğrulanamadı. Büyük olasılıkla doğrudur, kesin kanıt değildir.",
  },
  edge: {
    label: 'Sunucu kenarı',
    detail: 'barındırma',
    tone: 'neutral',
    description:
      "Adres barındırma platformunun kenar sunucusundan (X-Real-IP) alındı. Cloudflare arkasında bu, ziyaretçinin değil Cloudflare'in adresi olabilir.",
  },
  'forwarded-untrusted': {
    label: 'Yönlendirme başlığı',
    detail: 'güvenilmez',
    tone: 'warn',
    description:
      'Adres X-Forwarded-For başlığından alındı. Bu başlığı ziyaretçi kendisi yazmış olabilir; kanıt olarak kullanmayın.',
  },
  peer: {
    label: 'Doğrudan bağlantı',
    detail: '',
    tone: 'neutral',
    description:
      'Adres ve port doğrudan TCP bağlantısından alındı. Sunucuya doğrudan bağlanan bir ziyaretçide ikisi de ziyaretçinindir; bir vekil sunucunun arkasında ise vekilin adresi ve bağlantı portudur, her ziyaretçi için aynı adrestir ve kimseyi tanımlamaz.',
  },
  unknown: {
    label: 'Bilinmiyor',
    detail: '',
    tone: 'neutral',
    description: 'Bu istek için kullanılabilir bir adres bulunamadı.',
  },
}

/**
 * Everything the "Kaynak" column shows for a source id: `label` and `detail`
 * for the cell's two lines (`text` joins them for one line), `description`
 * for its tooltip, `tone` for its colour. An id this file does not know is
 * shown as it came, with a neutral tone.
 */
export function ipSourceInfo(source) {
  const id = typeof source === 'string' ? source.trim() : ''
  const known = Object.prototype.hasOwnProperty.call(IP_SOURCES, id) ? IP_SOURCES[id] : null
  if (!known) {
    return {
      id,
      label: id || '—',
      detail: '',
      text: id || '—',
      tone: 'neutral',
      description: id ? 'Tanımlanmamış bir adres kaynağı.' : 'Kaynak bilgisi yok.',
    }
  }
  return {
    id,
    label: known.label,
    detail: known.detail,
    text: known.detail ? `${known.label} · ${known.detail}` : known.label,
    tone: known.tone,
    description: known.description,
  }
}

/* ------------------------------------------------------------ user agents */

/* Order matters: every Chromium browser also says "Chrome/", every browser on
   iOS says "Safari/", and the in-app browsers say both - so the more specific
   token is always asked first. */
const BROWSERS = [
  { name: 'Instagram', pattern: /\bInstagram\b/i, version: false },
  { name: 'Facebook', pattern: /\bFB(?:AN|AV|_IAB)\b/, version: false },
  { name: 'WhatsApp', pattern: /\bWhatsApp\b/i, version: false },
  { name: 'Edge', pattern: /\bEdg(?:e|A|iOS)?\/(\d+)/ },
  { name: 'Opera', pattern: /\b(?:OPR|OPT|Opera)\/(\d+)/ },
  { name: 'Samsung Internet', pattern: /\bSamsungBrowser\/(\d+)/ },
  { name: 'Yandex', pattern: /\bYaBrowser\/(\d+)/ },
  { name: 'Firefox', pattern: /\b(?:Firefox|FxiOS)\/(\d+)/ },
  { name: 'Chrome', pattern: /\b(?:CriOS|Chrome)\/(\d+)/ },
  { name: 'Safari', pattern: /\bVersion\/(\d+)[.\d]*.*\bSafari\//, requires: /\bSafari\// },
]

const SYSTEMS = [
  { name: 'iPhone', pattern: /\biPhone\b/ },
  { name: 'iPad', pattern: /\biPad\b/ },
  { name: 'Android', pattern: /\bAndroid\b/ },
  { name: 'Windows', pattern: /\bWindows\b/ },
  { name: 'ChromeOS', pattern: /\bCrOS\b/ },
  { name: 'macOS', pattern: /\bMac OS X\b|\bMacintosh\b/ },
  { name: 'Linux', pattern: /\bLinux\b/ },
]

/* A crawler names itself with "…bot/" and usually a "+https://" contact
   address; a bare "bot" is not enough - "CUBOT" is an Android phone brand. */
const BOT_PATTERN =
  /bot\/|bot;|crawler|spider|slurp|facebookexternalhit|headless|\+https?:\/\/|^curl\/|^wget\/|python-requests|go-http-client/i
const BOT_NAME_PATTERN = /([A-Za-z][\w-]*?(?:bot|crawler|spider|externalhit))\b/i

/**
 * A user agent reduced to what the visit log has room for:
 * "Chrome 128 · Android", "Safari 17 · iPhone", "Instagram · iPhone".
 * Crawlers and link previews say so ("Bot · Googlebot"); an agent this file
 * cannot read is cut to 40 characters; a missing one prints '—'. The full
 * string stays in the cell's title.
 */
export function shortUserAgent(userAgent) {
  const agent = typeof userAgent === 'string' ? userAgent.trim() : ''
  if (!agent) return '—'

  if (BOT_PATTERN.test(agent)) {
    const named = BOT_NAME_PATTERN.exec(agent)
    return named ? `Bot · ${named[1]}` : 'Bot'
  }

  let browser = ''
  for (const candidate of BROWSERS) {
    if (candidate.requires && !candidate.requires.test(agent)) continue
    const match = candidate.pattern.exec(agent)
    if (!match) continue
    browser =
      candidate.version === false || !match[1] ? candidate.name : `${candidate.name} ${match[1]}`
    break
  }

  const system = SYSTEMS.find((candidate) => candidate.pattern.test(agent))?.name || ''

  if (browser && system) return `${browser} · ${system}`
  if (browser || system) return browser || system
  return agent.length > 40 ? `${agent.slice(0, 39)}…` : agent
}
