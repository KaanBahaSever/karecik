// The small, pure pieces of the "Hariç tutulan IP'ler" section of the owner
// panel's analytics page: reading an address or a CIDR range the way the
// server does, the rules an entry must pass before it is sent, matching the
// addresses of the visit log against the list, the page's own edits of its
// copy of the list, and the sentences the add dialog and the list print -
// each of which says "adres" of a single address and "aralık" of a range.
//
// WHY THE RULES ARE REPEATED HERE. The server is the authority - POST
// /api/analytics/excluded-ips checks every one of them again - but every add
// first asks the server how many past visits the entry would cover
// (GET .../match-count) and then opens a dialog about them. Refusing a typo,
// a range that is far too broad or a duplicate here means the owner hears
// about it next to the field, before that round trip and that dialog, rather
// than inside a dialog that can then only be closed. Where this file and the
// server could disagree, this file is the more lenient one, so it never
// refuses what the server would take.
//
// The rules mirror handlers.AddExcludedIP (net/netip on the Go side):
//
//   - an IPv4 or IPv6 address, or either one with a /prefix (CIDR);
//   - normalised: host bits masked off ("203.0.113.7/24" is 203.0.113.0/24)
//     and IPv4-mapped IPv6 unmapped ("::ffff:203.0.113.7" is 203.0.113.7),
//     because that is how the server writes and matches the visitor address;
//   - no range broader than /16 for IPv4 or /48 for IPv6 - wider than that
//     hides a whole operator's customers, not the owner;
//   - no entry the list already covers, and at most MAX_EXCLUDED_IPS entries;
//   - a label of at most MAX_LABEL_LENGTH characters.
//
// Pure functions only - no React, no DOM - so frontend/tests/
// ipExclusion.test.mjs runs them with plain node. Nothing here throws: input
// that cannot be read answers null, '' or a Turkish `problem`.
//
// Bytes rather than BigInt: an address is a 4- or 16-byte array and a prefix
// compares whole bytes, then the bits of the one byte it ends in.
//
// NOTE: the copy is Turkish on purpose - it is panel UI, read by the owner.

import { formatCount } from './analyticsFormat.js'

/** How many entries one business may keep (the server's limit, also sent as `max`). */
export const MAX_EXCLUDED_IPS = 50

/** The longest label, in characters (code points, as the server counts them). */
export const MAX_LABEL_LENGTH = 60

/**
 * The broadest range the server accepts, per address family. /16 is 65 536
 * IPv4 addresses - an office block at most; /48 is one site's IPv6 allocation.
 */
export const MIN_PREFIX = { 4: 16, 6: 48 }

/** The Turkish problems the checks answer; exported so a test can pin them. */
export const EXCLUSION_MESSAGES = {
  empty: 'Bir IP adresi ya da aralık yazın.',
  invalid: 'Geçerli bir IP adresi ya da aralık yazın (örn. 203.0.113.7 ya da 203.0.113.0/24).',
  prefix: 'Aralık uzunluğu IPv4 için 0–32, IPv6 için 0–128 arasında bir sayı olmalı.',
  tooBroad4:
    "Bu aralık çok geniş: /16'dan geniş bir IPv4 aralığı çok sayıda gerçek müşterinizin ziyaretini de gizler. En geniş /16 yazabilirsiniz.",
  tooBroad6:
    "Bu aralık çok geniş: /48'den geniş bir IPv6 aralığı çok sayıda gerçek müşterinizin ziyaretini de gizler. En geniş /48 yazabilirsiniz.",
  duplicate: 'Bu IP zaten listede.',
  label: `Not en fazla ${MAX_LABEL_LENGTH} karakter olabilir.`,
}

/** The problem of a full list; `limit` is the server's `max`. */
export function listFullMessage(limit = MAX_EXCLUDED_IPS) {
  return `Listeye en fazla ${limit} IP eklenebilir. Yeni bir IP için önce listeden birini çıkarın.`
}

/* --------------------------------------------------------------- parsing */

/** One decimal octet as Go's netip reads it: 0-255, no leading zero, no sign. */
const OCTET_PATTERN = /^(0|[1-9]\d{0,2})$/

/** A prefix length: digits only, no leading zero (Go's ParsePrefix refuses "/024"). */
const BITS_PATTERN = /^(0|[1-9]\d{0,2})$/

const HEX_GROUP_PATTERN = /^[0-9a-fA-F]{1,4}$/

/** "203.0.113.7" -> [203, 0, 113, 7], or null. */
function parseIPv4(text) {
  const parts = text.split('.')
  if (parts.length !== 4) return null
  const bytes = []
  for (const part of parts) {
    if (!OCTET_PATTERN.test(part)) return null
    const value = Number(part)
    if (value > 255) return null
    bytes.push(value)
  }
  return bytes
}

/**
 * "2001:db8::1" -> its 16 bytes, or null. Takes "::" once (for at least one
 * zero group, as netip does) and a dotted IPv4 tail ("::ffff:203.0.113.7").
 * A zone ("fe80::1%eth0") is refused: no visitor address carries one, and a
 * prefix may not.
 */
function parseIPv6(text) {
  if (!text.includes(':') || text.includes('%')) return null

  let body = text
  const lastColon = body.lastIndexOf(':')
  const tail = body.slice(lastColon + 1)
  if (tail.includes('.')) {
    const v4 = parseIPv4(tail)
    if (!v4) return null
    const high = ((v4[0] << 8) | v4[1]).toString(16)
    const low = ((v4[2] << 8) | v4[3]).toString(16)
    body = `${body.slice(0, lastColon + 1)}${high}:${low}`
  }

  const halves = body.split('::')
  if (halves.length > 2) return null
  const groupsOf = (part) => (part === '' ? [] : part.split(':'))
  const head = groupsOf(halves[0])
  const rest = halves.length === 2 ? groupsOf(halves[1]) : []
  if (![...head, ...rest].every((group) => HEX_GROUP_PATTERN.test(group))) return null

  if (halves.length === 1 && head.length !== 8) return null
  if (halves.length === 2 && head.length + rest.length > 7) return null

  const groups = [
    ...head,
    ...Array(8 - head.length - rest.length).fill('0'),
    ...rest,
  ].map((group) => parseInt(group, 16))

  const bytes = []
  for (const group of groups) bytes.push(group >> 8, group & 0xff)
  return bytes
}

/** { family: 4|6, bytes } of a bare address, or null. */
function parseAddress(text) {
  if (text.includes(':')) {
    const bytes = parseIPv6(text)
    return bytes ? { family: 6, bytes } : null
  }
  const bytes = parseIPv4(text)
  return bytes ? { family: 4, bytes } : null
}

/** ::ffff:a.b.c.d - an IPv4 address written as IPv6. */
function isMapped(bytes) {
  return (
    bytes.length === 16 &&
    bytes.slice(0, 10).every((byte) => byte === 0) &&
    bytes[10] === 0xff &&
    bytes[11] === 0xff
  )
}

/** `bytes` with every bit after the first `bits` cleared. */
function maskBytes(bytes, bits) {
  return bytes.map((byte, index) => {
    const kept = bits - index * 8
    if (kept >= 8) return byte
    if (kept <= 0) return 0
    return byte & (0xff << (8 - kept)) & 0xff
  })
}

/**
 * The text of an address: dotted IPv4, or IPv6 the way Go's netip (and RFC
 * 5952) writes it - lowercase, no leading zeros, the longest run of two or
 * more zero groups (the first of equal runs) as "::".
 */
function formatAddress(family, bytes) {
  if (family === 4) return bytes.join('.')

  const groups = []
  for (let index = 0; index < 16; index += 2) groups.push((bytes[index] << 8) | bytes[index + 1])

  let bestStart = -1
  let bestLength = 0
  for (let start = 0; start < 8; start += 1) {
    let end = start
    while (end < 8 && groups[end] === 0) end += 1
    if (end - start >= 2 && end - start > bestLength) {
      bestStart = start
      bestLength = end - start
    }
  }

  const hex = (list) => list.map((group) => group.toString(16)).join(':')
  if (bestStart < 0) return hex(groups)
  return `${hex(groups.slice(0, bestStart))}::${hex(groups.slice(bestStart + bestLength))}`
}

const FULL_BITS = { 4: 32, 6: 128 }

/**
 * An address or a CIDR range read and normalised, with no judgement about its
 * size: what the server stored, what the visit log shows, what the owner
 * typed. null when it is neither.
 *
 * @returns {{ family: 4|6, bits: number, bytes: number[], address: string,
 *             cidr: string, display: string, single: boolean }|null}
 */
export function parseNetwork(value) {
  const text = typeof value === 'string' ? value.trim() : ''
  if (!text) return null

  const slash = text.indexOf('/')
  const addressText = slash >= 0 ? text.slice(0, slash) : text
  let bits = null
  if (slash >= 0) {
    const bitsText = text.slice(slash + 1)
    if (!BITS_PATTERN.test(bitsText)) return null
    bits = Number(bitsText)
  }

  const parsed = parseAddress(addressText)
  if (!parsed) return null

  let { family, bytes } = parsed
  if (bits === null) bits = FULL_BITS[family]
  if (bits > FULL_BITS[family]) return null

  // Unmapped the way the server unmaps the visitor address, so
  // "::ffff:203.0.113.7" and "203.0.113.7" are the same entry. A mapped range
  // shorter than /96 reaches past the IPv4 block and stays IPv6.
  if (family === 6 && isMapped(bytes) && bits >= 96) {
    family = 4
    bytes = bytes.slice(12)
    bits -= 96
  }

  const masked = maskBytes(bytes, bits)
  const address = formatAddress(family, masked)
  const single = bits === FULL_BITS[family]
  const cidr = `${address}/${bits}`
  return { family, bits, bytes: masked, address, cidr, display: single ? address : cidr, single }
}

/**
 * The text of an entry as the list shows it: the bare address for a single
 * host (/32, /128), the range otherwise. The server sends `display` for the
 * same thing; this is for values it does not (an audit entry's `cidr`).
 * Unreadable text comes back trimmed, as it was.
 */
export function displayCidr(value) {
  const network = parseNetwork(value)
  if (network) return network.display
  return typeof value === 'string' ? value.trim() : ''
}

/**
 * What the owner typed, read and checked against the size rule.
 *
 * @returns {{ network: object|null, problem: string }} exactly one is set;
 *   `network` is parseNetwork's result.
 */
export function parseExclusion(value) {
  const text = typeof value === 'string' ? value.trim() : ''
  if (!text) return { network: null, problem: EXCLUSION_MESSAGES.empty }

  const network = parseNetwork(text)
  if (!network) {
    // A readable address with an impossible prefix gets the more useful message.
    const slash = text.indexOf('/')
    const readable = slash > 0 && parseNetwork(text.slice(0, slash))
    return { network: null, problem: readable ? EXCLUSION_MESSAGES.prefix : EXCLUSION_MESSAGES.invalid }
  }

  if (network.bits < MIN_PREFIX[network.family]) {
    return {
      network: null,
      problem: network.family === 4 ? EXCLUSION_MESSAGES.tooBroad4 : EXCLUSION_MESSAGES.tooBroad6,
    }
  }
  return { network, problem: '' }
}

/* -------------------------------------------------------------- matching */

/** Whether network `outer` contains network `inner` (an address is a /32 or /128). */
function covers(outer, inner) {
  if (!outer || !inner || outer.family !== inner.family || inner.bits < outer.bits) return false
  const masked = maskBytes(inner.bytes, outer.bits)
  return masked.every((byte, index) => byte === outer.bytes[index])
}

/**
 * Whether the range `cidr` contains `value` (an address or a range). false
 * for anything unreadable - the "-" the log writes for an unknown address
 * never matches, just as it never matches on the server.
 */
export function cidrContains(cidr, value) {
  return covers(parseNetwork(cidr), parseNetwork(value))
}

/**
 * A log address the owner can exclude with one click, normalised - or '' for
 * a missing, unknown ("-", "unknown") or unreadable one, which gets no button.
 */
export function excludableAddress(ip) {
  const network = parseNetwork(typeof ip === 'string' && !ip.includes('/') ? ip : '')
  return network ? network.address : ''
}

/** The entries' networks, read once. Unreadable entries are left out. */
function readEntries(items) {
  const list = []
  for (const item of Array.isArray(items) ? items : []) {
    const network = parseNetwork(item?.cidr) || parseNetwork(item?.display)
    if (network) list.push({ item, network })
  }
  return list
}

/**
 * The entry of `items` that already covers `value` - the same range, or a
 * broader one around it - or null. This is the server's 409 rule.
 */
export function findCoveringEntry(items, value) {
  const target = parseNetwork(value)
  if (!target) return null
  return readEntries(items).find((entry) => covers(entry.network, target))?.item || null
}

/**
 * A function answering which entry covers an address, for every row of the
 * visit log: the list is read once, not once per row.
 *
 * @returns {(ip: string) => object|null}
 */
export function exclusionMatcher(items) {
  const entries = readEntries(items)
  return (ip) => {
    if (entries.length === 0) return null
    const target = parseNetwork(typeof ip === 'string' && !ip.includes('/') ? ip : '')
    if (!target) return null
    return entries.find((entry) => covers(entry.network, target))?.item || null
  }
}

/* ---------------------------------------------------------------- payload */

/**
 * GET /api/analytics/excluded-ips read into the shape the page keeps, with a
 * safe default for anything missing or malformed:
 *
 *   items              the entries, as the server sent them (each with an id)
 *   currentIp          the address a menu visit from here is logged under,
 *                      or null when the server could not establish one
 *   currentIpSource    where that address came from (a clientip source id)
 *   currentIpExcluded  whether the list covers it, as the server judged
 *   optout             whether this browser carries the opt-out cookie
 *   max                how many entries the list may hold
 */
export function readExclusionState(data) {
  const source = data && typeof data === 'object' && !Array.isArray(data) ? data : {}
  const items = Array.isArray(source.items)
    ? source.items.filter((item) => item && typeof item === 'object' && item.id)
    : []
  const ip = typeof source.current_ip === 'string' ? source.current_ip.trim() : ''
  return {
    items,
    // "-" is how the server writes an address it could not establish.
    currentIp: ip && ip !== '-' ? ip : null,
    currentIpSource: typeof source.current_ip_source === 'string' ? source.current_ip_source : '',
    currentIpExcluded: source.current_ip_excluded === true,
    optout: source.optout === true,
    max: Number.isInteger(source.max) && source.max > 0 ? source.max : MAX_EXCLUDED_IPS,
  }
}

/**
 * The page's copy of the list with `item` - an entry the server has just
 * created - on it, in the place GET /api/analytics/excluded-ips will give it:
 * LAST, because the server lists oldest first (repository.ListExcludedIPs,
 * ORDER BY created_at). The page reloads the list right after an add; were the
 * two orders to disagree, the new entry would show in one place and then jump
 * to the other a moment later. An item already present moves rather than
 * doubles. A missing state or an item without an id changes nothing.
 */
export function withAddedEntry(state, item) {
  if (!state || !item || typeof item !== 'object' || !item.id) return state
  const items = Array.isArray(state.items) ? state.items : []
  return { ...state, items: [...items.filter((row) => row.id !== item.id), item] }
}

/** The page's copy of the list without the entry `id`; the order is kept. */
export function withoutEntry(state, id) {
  if (!state || !id) return state
  const items = Array.isArray(state.items) ? state.items : []
  return { ...state, items: items.filter((row) => row.id !== id) }
}

/**
 * Whether the current address is on the list. Judged here from the items when
 * the address can be read, so an entry just added or removed shows at once;
 * the server's own answer otherwise.
 */
export function currentIpCovered(state) {
  if (!state || !state.currentIp) return false
  if (!parseNetwork(state.currentIp)) return state.currentIpExcluded === true
  return findCoveringEntry(state.items, state.currentIp) !== null
}

/* ----------------------------------------------------------------- checks */

/** A label trimmed and measured in characters, so an emoji counts once. */
export function checkLabel(value) {
  const label = typeof value === 'string' ? value.trim() : ''
  if (Array.from(label).length > MAX_LABEL_LENGTH) {
    return { label, problem: EXCLUSION_MESSAGES.label }
  }
  return { label, problem: '' }
}

/**
 * Every rule an entry must pass before the add dialog opens, in the order the
 * owner should hear about them: the list being full first (nothing typed can
 * fix it), then the address, then the label.
 *
 * @param {{ cidr: string, label?: string }} input - what was typed
 * @param {{ items?: object[], max?: number }} [list] - the current list; a list
 *   that could not be loaded is passed as nothing, and the server decides
 * @returns {{ value: { cidr, display, label, single }|null, problem: string,
 *             field: 'cidr'|'label'|'' }} `field` is where the problem belongs
 */
export function checkExclusion({ cidr, label } = {}, { items, max } = {}) {
  const limit = Number.isInteger(max) && max > 0 ? max : MAX_EXCLUDED_IPS
  if (Array.isArray(items) && items.length >= limit) {
    return { value: null, problem: listFullMessage(limit), field: '' }
  }

  const { network, problem } = parseExclusion(cidr)
  if (!network) return { value: null, problem, field: 'cidr' }

  const covering = findCoveringEntry(items, network.cidr)
  if (covering) {
    const around = parseNetwork(covering.cidr)
    const broader = around && around.bits < network.bits
    return {
      value: null,
      problem: broader
        ? `${EXCLUSION_MESSAGES.duplicate} (${covering.display || around.display} aralığının içinde)`
        : EXCLUSION_MESSAGES.duplicate,
      field: 'cidr',
    }
  }

  const checked = checkLabel(label)
  if (checked.problem) return { value: null, problem: checked.problem, field: 'label' }

  return {
    value: { cidr: network.cidr, display: network.display, label: checked.label, single: network.single },
    problem: '',
    field: '',
  }
}

/* -------------------------------------------------------------- sentences */

/** A count the API sent, as a whole number of at least 0. */
function countOf(value) {
  const number = Math.trunc(Number(value))
  return Number.isFinite(number) && number > 0 ? number : 0
}

/**
 * How an entry is named in a sentence: the server's `display`, else its range
 * read the same way, else plain "IP".
 */
export function entryName(entry) {
  const display = typeof entry?.display === 'string' ? entry.display.trim() : ''
  return display || displayCidr(entry?.cidr) || 'IP'
}

/**
 * Whether an entry is one address rather than a range - the switch every
 * sentence about an entry turns on ("bu adresten" / "bu aralıktan"). A checked
 * entry (checkExclusion's value) says so itself; a list item is read from its
 * range, and text that cannot be read at all is a range when it has a prefix.
 */
export function isSingleEntry(entry) {
  if (typeof entry?.single === 'boolean') return entry.single
  const network = parseNetwork(entry?.cidr) || parseNetwork(entry?.display)
  if (network) return network.single
  return !entryName(entry).includes('/')
}

/** The add dialog's subtitle: what the entry changes from now on. */
export function futureVisitsText(entry) {
  return isSingleEntry(entry)
    ? 'Bundan sonra bu adresten gelen menü ziyaretleri kaydedilmez.'
    : 'Bundan sonra bu aralıktan gelen menü ziyaretleri kaydedilmez.'
}

/** The accessible name of a list row's "Çıkar" button. */
export function removeEntryLabel(entry) {
  return `${entryName(entry)} ${isSingleEntry(entry) ? 'adresini' : 'aralığını'} listeden çıkar`
}

/** The question of the removal dialog: what removing the entry brings back. */
export function removeEntryText(entry) {
  const label = typeof entry?.label === 'string' ? entry.label.trim() : ''
  const source = isSingleEntry(entry) ? 'bu adresten' : 'bu aralıktan'
  return (
    `${entryName(entry)}${label ? ` (${label})` : ''} listeden çıkarılınca ${source} gelen ` +
    'ziyaretler yeniden kaydedilmeye başlar. Daha önce silinen kayıtlar geri gelmez.'
  )
}

/** A removal the server answered 404: the entry was already gone. */
export function alreadyRemovedText(entry) {
  return isSingleEntry(entry) ? 'Bu IP zaten listede değildi.' : 'Bu IP aralığı zaten listede değildi.'
}

/**
 * The first line of the add dialog: "Bu IP'den 12 ziyaret kaydı var." - or,
 * with nothing to decide about, that there is nothing.
 */
export function pastRecordsText(count, { single = true } = {}) {
  const subject = single ? "Bu IP'den" : 'Bu IP aralığından'
  const total = countOf(count)
  if (total === 0) return `${subject} kaydedilmiş bir ziyaret yok.`
  return `${subject} ${formatCount(total)} ziyaret kaydı var.`
}

/**
 * The line shown once an entry was added:
 *
 *   history deleted        "203.0.113.7 hariç tutuldu. 12 kayıt silindi."
 *   history kept           "203.0.113.7 hariç tutuldu. Geçmiş 12 kayıt silinmedi."
 *   there was no history   "203.0.113.7 hariç tutuldu. Bundan sonraki ziyaretler kaydedilmeyecek."
 *
 * `deletedEvents` is the server's count (deleted_events); `pastCount` is what
 * match-count answered before the owner chose.
 */
export function addResultText({ display, deletedEvents, deleteHistory, pastCount } = {}) {
  const name = typeof display === 'string' && display.trim() ? display.trim() : 'IP'
  const head = `${name} hariç tutuldu.`
  if (deleteHistory) return `${head} ${formatCount(countOf(deletedEvents))} kayıt silindi.`
  if (countOf(pastCount) > 0) return `${head} Geçmiş ${formatCount(countOf(pastCount))} kayıt silinmedi.`
  return `${head} Bundan sonraki ziyaretler kaydedilmeyecek.`
}
