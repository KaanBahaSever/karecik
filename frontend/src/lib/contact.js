// The contact details of the customer menu - which of them exist, how they are
// laid out and where each one may link to - and the rules the owner's custom
// links must pass.
//
// Every value here comes from an owner - typed into a form, or stored by a
// route that never checked it - and several of them end up in an href. So each
// helper checks what it is handed instead of trusting it, answers null, '' or an
// empty list for anything it cannot use, and never throws. An empty phone, Wi-Fi
// or Instagram field is an ordinary state, not an error: it simply produces no
// item.
//
// The link rules (linkLabelProblem, linkUrlProblem, linkListProblem and
// keptLinkIds) are the ones PUT /api/menus/:id holds `links` to, written out in
// docs/FRONTEND-CONTRACT.md. This file is their only client copy: the settings
// page reports its inline errors with it, and buildContactItems draws a link
// only when it passes, so the menu never links anywhere the rules refuse.
//
// Pure functions only - no React, no DOM. No regular expression here uses a
// Unicode property escape: the letter test reads a range table generated from
// Go's unicode package instead, which also makes its answer the server's own.
// Digits need no table, because the rules accept ASCII digits only. And every
// control or invisible character is written as an escape sequence: a raw one
// cannot be seen in review, and a raw NUL would make git treat this file as
// binary.

/* ---------------------------------------------------------- display modes */

/**
 * How the contact items are laid out, bound to `contact_display`. The ids and
 * the labels are the ones the shared API contract gives GET /api/meta
 * (`contact_display_modes`), in the same order.
 *
 *   inline  one row of chips on the home view; a tap opens that item's details
 *   list    the same items as an always-open list on the home view
 *   footer  nothing on the home view, only the footer of the product screens
 *   hidden  nowhere at all
 *
 * The first three also repeat the compact list in the product screens' footer.
 */
export const CONTACT_DISPLAY_MODES = [
  { id: 'inline', label: 'Yan yana' },
  { id: 'list', label: 'Açık liste' },
  { id: 'footer', label: 'Sadece alt bilgi' },
  { id: 'hidden', label: 'Hiç gösterme' },
]

/**
 * One of the four display mode ids. Anything else - a missing field, a payload
 * from before the column existed, a typo - is 'inline', the column default.
 *
 * @param {unknown} value
 * @returns {'inline'|'list'|'footer'|'hidden'}
 */
export function contactDisplayMode(value) {
  return CONTACT_DISPLAY_MODES.some((mode) => mode.id === value) ? value : 'inline'
}

/* ------------------------------------------------------------- characters */

/**
 * Go's unicode.IsSpace, which is what strings.TrimSpace trims: the Unicode
 * White_Space characters. String.prototype.trim is a different set - it trims
 * U+FEFF, which Go keeps, and keeps U+0085, which Go trims - so it cannot stand
 * in for the server's trim.
 */
function isSpaceCode(code) {
  return (
    (code >= 0x09 && code <= 0x0d) ||
    code === 0x20 ||
    code === 0x85 ||
    code === 0xa0 ||
    code === 0x1680 ||
    (code >= 0x2000 && code <= 0x200a) ||
    code === 0x2028 ||
    code === 0x2029 ||
    code === 0x202f ||
    code === 0x205f ||
    code === 0x3000
  )
}

/** The C0 and C1 control characters, DEL included: U+0000-U+001F and U+007F-U+009F. */
function isControlCode(code) {
  return code <= 0x1f || (code >= 0x7f && code <= 0x9f)
}

/**
 * Formatting characters that are normally invisible: the soft hyphen, the
 * Arabic letter mark, the Mongolian vowel separator, the zero-width characters
 * and direction marks, the bidirectional embeddings, overrides and isolates, the
 * invisible operators and the byte order mark. A label made of nothing else -
 * or of nothing else and white space - reads as empty, and inside an address
 * they make the text a customer reads differ from where the link goes.
 */
function isInvisibleCode(code) {
  return (
    code === 0xad ||
    code === 0x61c ||
    code === 0x180e ||
    (code >= 0x200b && code <= 0x200f) ||
    (code >= 0x202a && code <= 0x202e) ||
    (code >= 0x2060 && code <= 0x2064) ||
    (code >= 0x2066 && code <= 0x206f) ||
    code === 0xfeff
  )
}

/*
  Every test above is on a single UTF-16 unit. That is exact: none of those
  characters is a surrogate, and a surrogate is none of them.
*/
function someCode(text, test) {
  for (let index = 0; index < text.length; index += 1) {
    if (test(text.charCodeAt(index))) return true
  }
  return false
}

/** `text` with every invisible formatting character removed. */
function withoutInvisible(text) {
  let kept = ''
  for (let index = 0; index < text.length; index += 1) {
    if (!isInvisibleCode(text.charCodeAt(index))) kept += text[index]
  }
  return kept
}

/**
 * The code points of a string. A lone surrogate becomes U+FFFD, the character
 * the server's JSON decoder puts in its place, so the counts below match the
 * server's for every string a browser can send.
 */
function codePoints(text) {
  const points = []
  for (let index = 0; index < text.length; index += 1) {
    const code = text.charCodeAt(index)
    if (code >= 0xd800 && code <= 0xdbff && index + 1 < text.length) {
      const next = text.charCodeAt(index + 1)
      if (next >= 0xdc00 && next <= 0xdfff) {
        points.push((code - 0xd800) * 0x400 + (next - 0xdc00) + 0x10000)
        index += 1
        continue
      }
    }
    points.push(code >= 0xd800 && code <= 0xdfff ? 0xfffd : code)
  }
  return points
}

/** The length in UTF-8 bytes, which is how the server measures an address. */
function utf8Length(text) {
  return codePoints(text).reduce(
    (bytes, point) => bytes + (point < 0x80 ? 1 : point < 0x800 ? 2 : point < 0x10000 ? 3 : 4),
    0,
  )
}

/** The hex digit a UTF-16 unit stands for, or -1. */
function hexDigit(code) {
  if (code >= 0x30 && code <= 0x39) return code - 0x30
  if (code >= 0x41 && code <= 0x46) return code - 0x37
  if (code >= 0x61 && code <= 0x66) return code - 0x57
  return -1
}

/**
 * `value` with its leading and trailing white space removed exactly the way Go's
 * strings.TrimSpace removes it - the trim the server applies to a link's label
 * and address, and to the VAT note. Anything but a string gives ''.
 *
 *   ' Web '             -> 'Web'
 *   '\u0085Web\u3000'   -> 'Web'
 *   '\ufeffWeb'         -> '\ufeffWeb'   (U+FEFF is not white space)
 *
 * @param {unknown} value
 * @returns {string}
 */
export function trimSpace(value) {
  if (typeof value !== 'string') return ''

  let start = 0
  let end = value.length
  while (start < end && isSpaceCode(value.charCodeAt(start))) start += 1
  while (end > start && isSpaceCode(value.charCodeAt(end - 1))) end -= 1
  return value.slice(start, end)
}

/* ---------------------------------------------------------------- letters */

/*
  Go's unicode.IsLetter, generated from Go 1.24's own tables (Unicode 15.0.0) by
  iterating every code point through that function. It is the test the server's
  hostname rule applies to every character that is not an ASCII digit or "-".

  The table is a list of ranges packed as comma-separated base-36 numbers, two
  per range: the gap after the end of the previous range, then the length of the
  range minus one. unpackRanges turns that into [start, end, start, end, ...].
*/
const LETTER_RANGES =
  '1t,p,6,p,1b,0,a,0,4,0,5,m,1,u,1,cp,4,b,e,4,7,0,1,0,3l,4,1,1,2,3,1,0,6,0,1,2,1,0,1,j,1,2a,1,' +
  '3u,8,4l,1,11,2,0,6,14,1z,q,4,3,19,16,z,1,1,2q,1,0,f,1,7,1,a,2,2,0,g,0,1,t,t,2g,b,0,o,w,9,1,' +
  '4,0,5,l,4,0,9,0,3,0,n,o,7,a,5,n,1,5,h,15,1m,1h,3,0,i,0,7,9,f,f,4,7,2,1,2,l,1,6,1,0,3,3,3,0,' +
  'g,0,d,1,1,2,e,1,a,0,8,5,4,1,2,l,1,6,1,1,1,1,1,1,v,3,1,0,j,2,g,8,1,2,1,l,1,6,1,1,1,4,3,0,i,0,' +
  'f,1,n,0,b,7,2,1,2,l,1,6,1,1,1,4,3,0,u,1,1,2,f,0,h,0,1,5,3,2,1,3,3,1,1,0,1,1,3,1,3,2,3,b,m,0,' +
  '1g,7,1,2,1,m,1,f,3,0,q,2,2,0,2,1,u,0,4,7,1,2,1,m,1,9,1,4,3,0,v,1,1,1,f,1,h,8,1,2,1,14,2,0,g,' +
  '0,5,2,8,2,o,5,5,h,3,n,1,8,1,0,2,6,1m,1b,1,1,c,6,1m,1,1,0,1,4,1,n,1,0,1,9,1,1,9,0,2,4,1,0,l,' +
  '3,w,0,1r,7,1,z,r,4,37,16,k,0,g,5,4,3,3,0,3,1,7,2,4,c,c,0,h,11,1,0,5,0,2,16,1,98,1,3,2,6,1,0,' +
  '1,3,2,14,1,3,2,w,1,3,2,6,1,0,1,3,2,e,1,1k,1,3,2,1u,11,f,g,2d,2,5,3,h7,2,g,1,p,5,22,6,7,7,h,' +
  'd,i,e,h,e,c,1,2,f,1f,z,0,4,0,1v,2g,7,4,2,x,1,0,5,1x,a,u,1d,t,2,4,b,17,4,p,1i,m,9,1g,2a,0,2l,' +
  '1a,h,7,1i,t,d,1,a,17,q,z,15,2,a,z,2,8,7,16,2,2,15,3,1,5,1,1,3,0,5,5b,1s,7p,2,5,2,11,2,5,2,7,' +
  '1,0,1,0,1,0,1,u,2,1g,1,6,1,0,3,2,1,6,3,3,2,5,4,c,5,2,1,6,38,0,d,0,g,c,2t,0,4,0,2,9,1,0,3,4,' +
  '6,0,1,0,1,0,1,3,1,a,2,3,5,4,4,0,1g,1,22j,6c,6,3,3,1,c,11,1,0,5,0,2,1j,7,0,g,m,9,6,1,6,1,6,1,' +
  '6,1,6,1,6,1,6,1,6,28,0,d1,1,16,4,5,1,4,2d,6,2,1,2h,1,3,5,16,1,2l,h,v,1c,f,e8,533,1s,h3g,1v,' +
  '19,2,7g,3,f,a,1,k,1a,g,u,2,1x,1d,8,2,2u,2,1r,5,1,1,0,1,4,o,f,1,2,1,3,1,m,t,1f,e,1d,1q,5,3,0,' +
  '1,1,b,r,a,m,p,s,7,1a,s,0,g,4,1,9,a,4,1,14,n,2,1,7,k,m,3,0,3,1d,1,0,3,1,2,4,2,0,1,0,o,2,2,a,' +
  '7,2,c,5,2,5,2,5,9,6,1,6,1,16,1,d,6,36,t,8mb,c,m,4,1c,6is,a5,2,2x,12,6,c,4,5,0,1,9,1,c,1,4,1,' +
  '0,1,1,1,1,1,2z,x,a2,i,1r,2,1h,14,b,38,4,1,3q,10,p,6,p,b,2g,3,5,2,5,2,5,2,2,z,b,1,p,1,i,1,1,' +
  '1,e,2,d,y,3e,at,s,3,1c,1b,v,d,j,1,7,6,11,a,t,2,z,4,7,1c,4d,i,z,4,z,4,13,8,1f,c,a,1,e,1,6,1,' +
  '1,1,a,1,e,1,6,1,1,1v,8m,9,l,a,7,o,5,1,15,1,8,1x,5,2,0,1,17,1,1,3,0,2,m,a,m,9,u,1t,i,1,1,a,l,' +
  'a,p,1y,1j,6,1,1s,0,f,3,1,2,1,s,16,s,3,s,z,7,1,r,r,1h,a,l,a,i,d,h,32,20,1j,1e,d,1e,d,z,9o,15,' +
  '6,1,26,s,a,0,8,l,16,h,1a,k,r,m,c,1g,1l,1,2,0,d,18,w,o,q,z,t,0,2,0,8,y,3,0,c,1b,e,3,l,0,1,0,' +
  'z,h,1,o,j,1,1r,6,1,0,1,3,1,e,1,9,7,1a,12,7,2,1,2,l,1,6,1,1,1,4,3,0,i,0,c,4,4e,1g,i,3,k,2,u,' +
  '1b,k,1,1,0,54,1a,15,3,10,1b,k,0,1n,16,d,0,1z,q,11,6,55,17,38,1r,v,7,2,0,2,7,1,1,1,n,f,0,1,0,' +
  '2m,7,2,12,g,0,1,0,s,0,a,13,7,0,l,0,b,19,j,0,i,20,7b,8,1,10,h,0,1d,t,34,6,1,1,1,11,l,0,p,5,1,' +
  '1,1,v,e,0,93,i,f,0,1,c,1,x,3g,0,27,pl,6e,5f,218,2o,f,tr,h,5,33t,g6,6nt,fs,7,u,h,26,h,t,i,1b,' +
  'g,3,v,k,5,i,j4,1r,3k,22,5,0,1u,c,1s,1,1,0,s,4qf,8,yd,16,8,6w7,3,1,6,1,1,1,82,f,0,t,2,2,0,e,' +
  '3,8,az,1s4,2y,5,c,3,8,7,9,4me,2c,1,1y,1,1,2,0,2,1,2,3,1,b,1,0,1,6,1,1s,1,3,2,7,1,6,1,r,1,3,' +
  '1,4,1,0,3,6,1,9f,2,o,1,o,1,u,1,o,1,u,1,o,1,u,1,o,1,u,1,o,1,7,1f8,u,6,5,79,1p,42,18,a,6,g,0,' +
  '8x,t,i,17,dg,r,l0,6,1,3,1,1,1,e,1,5g,1n,1v,7,0,xg,3,1,q,1,1,1,0,2,0,1,9,1,3,1,0,1,0,6,0,4,0,' +
  '1,0,1,0,1,2,1,1,1,0,2,0,1,0,1,0,1,0,1,0,1,1,1,0,2,3,1,6,1,3,1,3,1,0,1,9,1,g,5,2,1,4,1,g,3es,' +
  'wyn,w,37d,6,65,2,4g1,e,5rk,2e7,f1,15u,3t6,5,38f'

function unpackRanges(packed) {
  const ranges = []
  const numbers = packed.split(',')
  let end = -1
  for (let index = 0; index + 1 < numbers.length; index += 2) {
    const start = end + 1 + parseInt(numbers[index], 36)
    end = start + parseInt(numbers[index + 1], 36)
    ranges.push(start, end)
  }
  return ranges
}

function inRanges(ranges, point) {
  let low = 0
  let high = ranges.length / 2 - 1
  while (low <= high) {
    const middle = (low + high) >> 1
    if (point < ranges[middle * 2]) high = middle - 1
    else if (point > ranges[middle * 2 + 1]) low = middle + 1
    else return true
  }
  return false
}

// Unpacked on first use rather than when the module loads.
let letterRanges = null

function isLetter(point) {
  if (point < 0x80) return (point >= 0x41 && point <= 0x5a) || (point >= 0x61 && point <= 0x7a)
  if (!letterRanges) letterRanges = unpackRanges(LETTER_RANGES)
  return inRanges(letterRanges, point)
}

/* ------------------------------------------------------------- link rules */

export const MAX_LINKS = 8
export const MAX_LINK_LABEL_RUNES = 40 // code points of the trimmed label
export const MAX_LINK_URL_BYTES = 500 // UTF-8 bytes of the trimmed address

/** An id the server keeps as it is, unless an earlier link of the list already has it. */
export const LINK_ID_PATTERN = /^[A-Za-z0-9_-]{1,64}$/

/** The messages, word for word, that the server answers a refused `links` list with. */
export const LINK_MESSAGES = {
  tooMany: 'En fazla 8 link ekleyebilirsiniz.',
  listInvalid: 'Link listesi geçersiz.',
  labelInvalid: 'Link adı geçersiz karakter içeriyor.',
  labelRequired: 'Link adı zorunludur.',
  labelTooLong: 'Link adı en fazla 40 karakter olabilir.',
  urlTooLong: 'Link adresi en fazla 500 karakter olabilir.',
  urlInvalid: 'Link adresi http:// veya https:// ile başlayan geçerli bir adres olmalıdır.',
}

/**
 * The problem with a link label, as the server words it, or ''. The label is
 * trimmed first, as the server trims it, and checked in this order:
 *
 *   a control character anywhere  -> labelInvalid   ('Web\u0007', '\u0000')
 *   nothing visible               -> labelRequired  ('', '  ', '\u200b', '\u200b \u200b')
 *   more than 40 code points      -> labelTooLong
 *
 * "Nothing visible" means nothing is left once the invisible characters are
 * removed and what remains is trimmed again. Removing them without that second
 * trim would accept '\u200b \u200b': the first trim stops at the invisible
 * character at either end, so the space between them outlives the removal, and
 * a label of one space still shows nothing.
 * The length is counted on the trimmed label as it is stored, invisible
 * characters included.
 *
 * @param {unknown} label - anything but a string counts as ''
 * @returns {string}
 */
export function linkLabelProblem(label) {
  const trimmed = trimSpace(label)
  if (someCode(trimmed, isControlCode)) return LINK_MESSAGES.labelInvalid
  if (trimSpace(withoutInvisible(trimmed)) === '') return LINK_MESSAGES.labelRequired
  if (codePoints(trimmed).length > MAX_LINK_LABEL_RUNES) return LINK_MESSAGES.labelTooLong
  return ''
}

/* Refused anywhere in an address, besides white space, controls and the
   invisible characters.

   "[" and "]" are among them. Go's url.Parse reads "https://[ornek.com]" as a
   bracketed host and url.URL.Hostname strips the brackets, so the name inside
   would pass the hostname rule, while a browser opens nothing at that address:
   only an IPv6 address may stand inside brackets. Refusing both characters
   everywhere keeps the answer from resting on how a Go release parses a
   bracketed host. */
const FORBIDDEN_URL_CHARACTERS = '<>"`{}|\\^[]'

const HTTP_PREFIX = /^(https?):\/\//i

/** Go's validOptionalPort: nothing, or a colon followed by digits only. */
function validOptionalPort(text) {
  return text === '' || (text[0] === ':' && /^[0-9]*$/.test(text.slice(1)))
}

/** False when Go's url.Parse refuses a path or fragment: a "%" not followed by two hex digits. */
function validEscapes(text) {
  for (let index = text.indexOf('%'); index !== -1; index = text.indexOf('%', index + 3)) {
    if (hexDigit(text.charCodeAt(index + 1)) === -1 || hexDigit(text.charCodeAt(index + 2)) === -1) {
      return false
    }
  }
  return true
}

function pushUtf8(bytes, point) {
  if (point < 0x80) {
    bytes.push(point)
  } else if (point < 0x800) {
    bytes.push(0xc0 | (point >> 6), 0x80 | (point & 0x3f))
  } else if (point < 0x10000) {
    bytes.push(0xe0 | (point >> 12), 0x80 | ((point >> 6) & 0x3f), 0x80 | (point & 0x3f))
  } else {
    bytes.push(
      0xf0 | (point >> 18),
      0x80 | ((point >> 12) & 0x3f),
      0x80 | ((point >> 6) & 0x3f),
      0x80 | (point & 0x3f),
    )
  }
}

/** The text of a UTF-8 byte sequence, or null when the sequence is not valid UTF-8. */
function decodeUtf8(bytes) {
  let text = ''
  for (let index = 0; index < bytes.length; ) {
    const first = bytes[index]
    if (first < 0x80) {
      text += String.fromCharCode(first)
      index += 1
      continue
    }

    let length
    let point
    let smallest
    if (first >= 0xc2 && first <= 0xdf) {
      length = 2
      point = first & 0x1f
      smallest = 0x80
    } else if (first >= 0xe0 && first <= 0xef) {
      length = 3
      point = first & 0x0f
      smallest = 0x800
    } else if (first >= 0xf0 && first <= 0xf4) {
      length = 4
      point = first & 0x07
      smallest = 0x10000
    } else {
      return null
    }

    for (let offset = 1; offset < length; offset += 1) {
      const next = bytes[index + offset]
      if (next === undefined || (next & 0xc0) !== 0x80) return null
      point = (point << 6) | (next & 0x3f)
    }
    // Overlong forms, surrogates and anything past U+10FFFF are not UTF-8.
    if (point < smallest || point > 0x10ffff || (point >= 0xd800 && point <= 0xdfff)) return null

    text += String.fromCodePoint(point)
    index += length
  }
  return text
}

/**
 * The host of an authority as Go's url.Parse returns it (url.URL.Host: still
 * with its port, and with its %-escapes decoded), or null where Go refuses it -
 * or where the decoded bytes are not valid UTF-8, which Go accepts but which
 * leaves a U+FFFD in the hostname that validHostname refuses anyway.
 *
 * The authority holds no "[" or "]", which parseLinkUrl has refused by now, so
 * Go's separate path for a bracketed host has no counterpart here. Nor can an
 * escape bring a bracket in: "%5B" and "%5D" stand for bytes below 0x80.
 *
 * In a host Go decodes an escape only when it stands for a byte of 0x80 or
 * above, or is "%25", the percent sign itself; any other escape is an error.
 */
function parseHost(authority) {
  const colon = authority.lastIndexOf(':')
  if (colon !== -1 && !validOptionalPort(authority.slice(colon))) return null

  const bytes = []
  for (let index = 0; index < authority.length; ) {
    const code = authority.charCodeAt(index)

    if (code === 0x25) {
      const high = hexDigit(authority.charCodeAt(index + 1))
      const low = hexDigit(authority.charCodeAt(index + 2))
      if (high === -1 || low === -1) return null
      if (high < 8 && !(high === 2 && low === 5)) return null
      bytes.push(high * 16 + low)
      index += 3
      continue
    }

    let point = code
    let width = 1
    if (code >= 0xd800 && code <= 0xdfff) {
      const next = authority.charCodeAt(index + 1)
      if (code <= 0xdbff && next >= 0xdc00 && next <= 0xdfff) {
        point = (code - 0xd800) * 0x400 + (next - 0xdc00) + 0x10000
        width = 2
      } else {
        point = 0xfffd
      }
    }
    pushUtf8(bytes, point)
    index += width
  }

  return decodeUtf8(bytes)
}

/**
 * Go's url.URL.Hostname, read off a host from parseHost: the host without its
 * port. Hostname would also strip the brackets of a bracketed host, which a
 * host from parseHost never is.
 */
function hostnameOf(host) {
  const colon = host.lastIndexOf(':')
  return colon !== -1 && validOptionalPort(host.slice(colon)) ? host.slice(0, colon) : host
}

/**
 * 1-253 code points of dot-separated labels, at least two of them. Each label
 * is 1-63 letters (any Unicode letter), ASCII digits or hyphens, neither
 * starting nor ending with a hyphen, and the last one holds a letter - which is
 * what keeps a bare IPv4 address such as 192.168.1.1 out.
 */
function validHostname(hostname) {
  const length = codePoints(hostname).length
  if (length < 1 || length > 253) return false

  const labels = hostname.split('.')
  if (labels.length < 2) return false

  return labels.every((label, index) => {
    const points = codePoints(label)
    if (points.length < 1 || points.length > 63) return false
    if (points[0] === 0x2d || points[points.length - 1] === 0x2d) return false
    const allowed = (point) => isLetter(point) || (point >= 0x30 && point <= 0x39) || point === 0x2d
    if (!points.every(allowed)) return false
    return index < labels.length - 1 || points.some((point) => isLetter(point))
  })
}

/**
 * The port of a host from parseHost. No ":" means no port, which passes;
 * otherwise what follows the last ":" is 1-5 ASCII digits worth 1-65535, so a
 * ":" with nothing after it is refused rather than read as no port. Called
 * after validHostname, which refuses a ":" inside the hostname, so the last ":"
 * is the port's.
 */
function validPort(host) {
  const colon = host.lastIndexOf(':')
  if (colon === -1) return true
  const port = host.slice(colon + 1)
  return /^[0-9]{1,5}$/.test(port) && Number(port) >= 1 && Number(port) <= 65535
}

/**
 * The pieces of an already trimmed address this file needs, or null when the
 * address breaks a rule: { href, hostname, path }.
 *
 * Hand-written rather than `new URL()`: the WHATWG parser is lenient on
 * purpose - it reads `https:example.com` and `https:\\example.com` as ordinary
 * addresses - while the rules follow Go's url.Parse, and the answer has to be
 * the server's, not the browser's.
 */
function parseLinkUrl(address) {
  const refused = someCode(
    address,
    (code) =>
      isSpaceCode(code) ||
      isControlCode(code) ||
      isInvisibleCode(code) ||
      FORBIDDEN_URL_CHARACTERS.includes(String.fromCharCode(code)),
  )
  if (refused) return null

  const prefix = HTTP_PREFIX.exec(address)
  if (!prefix) return null

  // Split the way url.Parse splits: the fragment off at the first "#", the
  // query off at the first "?" of what is left, then the authority from "//"
  // to the first "/".
  const rest = address.slice(prefix[0].length)
  const hash = rest.indexOf('#')
  const beforeHash = hash === -1 ? rest : rest.slice(0, hash)
  const fragment = hash === -1 ? '' : rest.slice(hash + 1)
  const question = beforeHash.indexOf('?')
  const beforeQuery = question === -1 ? beforeHash : beforeHash.slice(0, question)
  const slash = beforeQuery.indexOf('/')
  const authority = slash === -1 ? beforeQuery : beforeQuery.slice(0, slash)
  const path = slash === -1 ? '' : beforeQuery.slice(slash)

  // Credentials before an "@" are refused, an empty one included.
  if (authority.includes('@')) return null
  // url.Parse checks the escapes of the path and the fragment, not the query.
  if (!validEscapes(path) || !validEscapes(fragment)) return null

  const host = parseHost(authority)
  if (host === null) return null

  const hostname = hostnameOf(host)
  if (!validHostname(hostname) || !validPort(host)) return null

  const schemeLength = prefix[1].length
  return {
    // Only the scheme is lowercased, exactly as the server stores it; the rest
    // of the address may be case-sensitive.
    href: address.slice(0, schemeLength).toLowerCase() + address.slice(schemeLength),
    hostname: hostname.toLowerCase(),
    path,
  }
}

/** { problem, link }: the message for an address (or '') and, when it passes, parseLinkUrl's pieces. */
function inspectLinkUrl(value) {
  const address = trimSpace(value)
  if (utf8Length(address) > MAX_LINK_URL_BYTES) {
    return { problem: LINK_MESSAGES.urlTooLong, link: null }
  }
  const link = parseLinkUrl(address)
  return { problem: link ? '' : LINK_MESSAGES.urlInvalid, link }
}

/**
 * The problem with a link address, as the server words it, or ''. The address
 * is trimmed first; more than 500 UTF-8 bytes is urlTooLong, and anything else
 * is urlInvalid unless all of these hold:
 *
 *   - no white space, control or invisible character, and none of < > " ` { } | \ ^ [ ]
 *   - it starts with http:// or https://, in any letter case
 *   - url.Parse accepts it, with no credentials ("user@") before the host
 *   - the hostname: see validHostname
 *   - a ":" after the hostname is followed by a port of 1-5 digits worth 1-65535
 *
 *   'https://ornek.com/menu'           -> ''
 *   'https://<svg/onload=alert(1)>'    -> urlInvalid
 *   'https://https//ornek.com'         -> urlInvalid   (the host is "https")
 *   'https://ornek.com:99999'          -> urlInvalid
 *   'https://[ornek.com]'              -> urlInvalid   (no brackets anywhere)
 *
 * @param {unknown} url - anything but a string counts as ''
 * @returns {string}
 */
export function linkUrlProblem(url) {
  return inspectLinkUrl(url).problem
}

/**
 * The address as the server stores it - trimmed, scheme lowercased, nothing
 * else changed - when it passes linkUrlProblem; otherwise null.
 *
 *   ' HTTPS://Ornek.com/Menu '  -> 'https://Ornek.com/Menu'
 *   'javascript:alert(1)'      -> null
 *   'ornek.com'                -> null   (not absolute)
 *
 * @param {unknown} value
 * @returns {string|null}
 */
export function safeExternalUrl(value) {
  const { link } = inspectLinkUrl(value)
  return link ? link.href : null
}

/**
 * The hostname of an address safeExternalUrl accepts, lowercased and with its
 * %-escapes decoded; otherwise null. Shown under a custom link, so the customer
 * sees where it really goes.
 *
 * @param {unknown} value
 * @returns {string|null}
 */
export function linkHostname(value) {
  const { link } = inspectLinkUrl(value)
  return link ? link.hostname : null
}

/** True for `{...}` records; false for null, arrays, strings, numbers and the like. */
function isPlainObject(value) {
  return Object.prototype.toString.call(value) === '[object Object]'
}

/**
 * One `links` entry as the server's JSON decoder reads it into { id, label, url },
 * or null when the decoder refuses it.
 *
 * Go's encoding/json decodes null (which is what JSON.stringify writes for an
 * undefined entry) into an entry with every field empty, matches the three keys
 * in any letter case - a later key overwriting an earlier one - ignores other
 * keys, leaves a field alone when its value is null, and refuses any other
 * value that is not a string.
 */
function linkFields(entry) {
  const fields = { id: '', label: '', url: '' }
  if (entry === null || entry === undefined) return fields
  if (!isPlainObject(entry)) return null

  for (const key of Object.keys(entry)) {
    const name = key.replace(/[A-Z]/g, (letter) => letter.toLowerCase())
    if (name !== 'id' && name !== 'label' && name !== 'url') continue

    const value = entry[key]
    if (value === null || value === undefined) continue
    if (typeof value !== 'string') return null
    fields[name] = value
  }
  return fields
}

/**
 * The problem with a whole `links` value, as the server words it, or ''.
 *
 *   not an array, or an entry that is not an object -> listInvalid
 *   more than 8 entries                              -> tooMany
 *   otherwise the first entry that breaks a rule, in the owner's order, its
 *   label checked before its address
 *
 * null is an empty list: the server reads `links: null` as [] and stores [].
 * Inside the list a null entry counts as an entry with none of its members, and
 * a null id, label or url as that member missing (see linkFields) - so a null
 * label is labelRequired, a null url urlInvalid, and a null id is replaced.
 *
 * @param {unknown} links
 * @returns {string}
 */
export function linkListProblem(links) {
  if (links === null || links === undefined) return ''
  if (!Array.isArray(links)) return LINK_MESSAGES.listInvalid

  const entries = links.map(linkFields)
  if (entries.includes(null)) return LINK_MESSAGES.listInvalid
  if (entries.length > MAX_LINKS) return LINK_MESSAGES.tooMany

  for (const entry of entries) {
    const problem = linkLabelProblem(entry.label) || linkUrlProblem(entry.url)
    if (problem) return problem
  }
  return ''
}

/**
 * Per entry of `links`, the id the server keeps, or null where it assigns a new
 * UUID: an id is kept when it matches LINK_ID_PATTERN and no earlier entry of
 * the same list already has it. A missing, invalid or repeated id gets null.
 *
 *   [{ id: 'a' }, { id: 'a' }, { id: 'x y' }, {}] -> ['a', null, null, null]
 *
 * @param {unknown} links
 * @returns {(string|null)[]}
 */
export function keptLinkIds(links) {
  if (!Array.isArray(links)) return []

  const seen = new Set()
  return links.map((entry) => {
    const fields = linkFields(entry)
    const id = fields ? fields.id : ''
    if (!LINK_ID_PATTERN.test(id) || seen.has(id)) return null
    seen.add(id)
    return id
  })
}

/* A scheme is a letter, then letters, digits, "+", "-" or ".", then a colon. A
   digit straight after the colon is a port instead: "ornek.com:8080" has none. */
const URL_SCHEME = /^[a-z][a-z0-9+.-]*:(?!\d)/i
const MISSING_COLON = /^(https?)\/\//i // "https//ornek.com"
const MISSING_SLASH = /^(https?):\/(?!\/)/i // "https:/ornek.com"

/**
 * What the address field becomes when it loses focus: trimmed, with an
 * obviously mistyped scheme repaired, and with "https://" in front of an
 * address typed without one. Anything else is left as typed, for the inline
 * error to explain.
 *
 *   'https//ornek.com', 'http//ornek.com'  -> the missing colon is added
 *   'https:/ornek.com'                     -> 'https://ornek.com'
 *   'ornek.com/menu', 'wa.me/905550000000' -> 'https://' in front (a dot, no spaces, no scheme)
 *   'ornek', 'mailto:a@ornek.com'          -> unchanged
 *
 * Repairing the scheme first is what keeps "https//ornek.com" from turning into
 * "https://https//ornek.com", an address whose host is "https".
 *
 * @param {unknown} value
 * @returns {string}
 */
export function completeLinkUrl(value) {
  const trimmed = trimSpace(value)
  if (MISSING_COLON.test(trimmed)) return trimmed.replace(MISSING_COLON, '$1://')
  if (MISSING_SLASH.test(trimmed)) return trimmed.replace(MISSING_SLASH, '$1://')
  if (trimmed === '' || /\s/.test(trimmed) || !trimmed.includes('.')) return trimmed
  if (URL_SCHEME.test(trimmed)) return trimmed
  return `https://${trimmed.replace(/^\/+/, '')}`
}

/* ------------------------------------------------------ phone, instagram */

/**
 * A `tel:` href for a phone number as the owner typed it, or null when it holds
 * no digit at all.
 *
 * Only ASCII digits are kept, plus a single `+` when one comes before the first
 * digit: `+90 (555) 000 00 00` -> `tel:+905550000000`, `(0212) 555 00 00` ->
 * `tel:02125550000`. Spaces, brackets and dashes mean nothing to a dialler.
 *
 * @param {unknown} phone
 * @returns {string|null}
 */
export function telHref(phone) {
  if (typeof phone !== 'string') return null

  const firstDigit = phone.search(/[0-9]/)
  if (firstDigit === -1) return null

  const plus = phone.slice(0, firstDigit).includes('+') ? '+' : ''
  return `tel:${plus}${phone.replace(/[^0-9]/g, '')}`
}

const INSTAGRAM_HANDLE = /^[A-Za-z0-9._]+$/

/* A profile address, with or without the scheme and "www.": the first path
   segment, then optionally more path and a query. */
const INSTAGRAM_PROFILE =
  /^(?:https?:\/\/)?(?:www\.)?instagram\.com\/([^/?#]+)(?:\/[^?#]*)?(?:\?[^#]*)?$/i

/* First path segments of instagram.com that are pages, not user names. */
const INSTAGRAM_PAGES = ['p', 'reel', 'reels', 'stories', 'explore', 'tv']

/**
 * The Instagram user name in what the owner typed - a name, "@name" or the
 * address of the profile - or null when there is none: a user name is letters,
 * digits, dots and underscores only.
 *
 * The value is trimmed, every "@" at its start is dropped - not just the first
 * - and what is left is trimmed again; only then is it read as a name or as a
 * profile address. So an "@" typed in front of either form, with or without a
 * space after it, changes nothing.
 *
 * This is the one rule for both sides: the customer menu links the name it
 * returns, and the settings page shows a stored value as that name (see
 * instagramFieldValue there), so the name the owner sees is the one the menu
 * links to.
 *
 *   'kahveduragi', '@kahveduragi', '@@kahveduragi'    -> 'kahveduragi'
 *   '@ kahveduragi'                                   -> 'kahveduragi'
 *   'instagram.com/kahveduragi'                       -> 'kahveduragi'
 *   '@instagram.com/kahveduragi'                      -> 'kahveduragi'
 *   'www.instagram.com/kahveduragi'                   -> 'kahveduragi'
 *   'https://www.instagram.com/kahveduragi/?igsh=abc' -> 'kahveduragi'
 *   '@https://www.instagram.com/kahveduragi/'         -> 'kahveduragi'
 *   'https://www.instagram.com/p/C0d3/'               -> null   (a post)
 *   '@', 'kahve duragi'                               -> null
 *
 * @param {unknown} value
 * @returns {string|null}
 */
export function instagramHandle(value) {
  if (typeof value !== 'string') return null

  const text = value.trim().replace(/^@+/, '').trim()
  const profile = INSTAGRAM_PROFILE.exec(text)
  if (profile && INSTAGRAM_PAGES.includes(profile[1].toLowerCase())) return null

  const handle = profile ? profile[1] : text
  return INSTAGRAM_HANDLE.test(handle) ? handle : null
}

/**
 * The profile URL for what instagramHandle accepts, or null.
 *
 *   'kahveduragi'     -> 'https://www.instagram.com/kahveduragi/'
 *   '@kahve.duragi'   -> 'https://www.instagram.com/kahve.duragi/'
 *   'kahve duragi'    -> null
 *
 * @param {unknown} value
 * @returns {string|null}
 */
export function instagramUrl(value) {
  const handle = instagramHandle(value)
  return handle ? `https://www.instagram.com/${handle}/` : null
}

/* --------------------------------------------------------------- link icons */

/* Country-code endings a Google or Yandex address may carry: .de, .com, .com.tr,
   .co.uk. Loose on purpose - at worst a lookalike domain gets the map pin. */
const GOOGLE_HOST = /^(?:www\.)?google\.(?:[a-z]{2,3}|com?\.[a-z]{2})$/
const GOOGLE_MAPS_HOST = /^maps\.google\.(?:[a-z]{2,3}|com?\.[a-z]{2})$/
const YANDEX_HOST = /^(?:www\.)?yandex\.(?:[a-z]{2,3}|com\.[a-z]{2})$/
const YANDEX_MAPS_HOST = /^maps\.yandex\.(?:[a-z]{2,3}|com\.[a-z]{2})$/
const MAPS_PATH = /^\/maps(?:\/|$)/i

/** `host` is `domain` itself or one of its subdomains - never merely ends in the same letters. */
function onDomain(host, domain) {
  return host === domain || host.endsWith(`.${domain}`)
}

function onAnyDomain(host, domains) {
  return domains.some((domain) => onDomain(host, domain))
}

function isMapsLink(host, path) {
  if (GOOGLE_MAPS_HOST.test(host) || YANDEX_MAPS_HOST.test(host)) return true
  if ((GOOGLE_HOST.test(host) || YANDEX_HOST.test(host)) && MAPS_PATH.test(path)) return true
  if (host === 'maps.app.goo.gl' || onAnyDomain(host, ['g.page', 'maps.apple.com'])) return true
  return host === 'goo.gl' && MAPS_PATH.test(path)
}

/** linkIcon on an already parsed address. */
function iconFor(link) {
  if (!link) return 'globe'

  const host = link.hostname
  if (onAnyDomain(host, ['wa.me', 'whatsapp.com'])) return 'whatsapp'
  if (isMapsLink(host, link.path)) return 'maps'
  if (onAnyDomain(host, ['instagram.com', 'instagr.am'])) return 'instagram'
  if (onAnyDomain(host, ['facebook.com', 'fb.com', 'fb.me'])) return 'facebook'
  if (onAnyDomain(host, ['youtube.com', 'youtu.be'])) return 'youtube'
  if (onAnyDomain(host, ['x.com', 'twitter.com'])) return 'twitter'
  if (onAnyDomain(host, ['linkedin.com', 'lnkd.in'])) return 'linkedin'
  return 'globe'
}

/**
 * Which icon a custom link gets, decided by its hostname (and, for the map
 * services that share a host with everything else they run, the path):
 *
 *   'whatsapp'   wa.me, whatsapp.com
 *   'maps'       Google, Yandex and Apple maps, goo.gl/maps, maps.app.goo.gl, g.page
 *   'instagram', 'facebook', 'youtube', 'twitter' (x.com too), 'linkedin'
 *   'globe'      everything else, and anything that is not a usable address
 *
 * @param {unknown} url
 * @returns {string}
 */
export function linkIcon(url) {
  return iconFor(inspectLinkUrl(url).link)
}

/* ------------------------------------------------------------------ items */

/** A value as display text: a string trimmed, anything else ''. */
function plainText(value) {
  return typeof value === 'string' ? value.trim() : ''
}

/**
 * The contact items to draw, in the order the customer menu shows them.
 *
 *   { type: 'wifi',      key, ssid, password }      ssid or password is non-blank
 *   { type: 'instagram', key, handle, url, text }   the field is non-blank; handle
 *                                                   and url are null when it holds
 *                                                   no user name, and `text` is
 *                                                   then the field as typed, so
 *                                                   nothing the owner wrote
 *                                                   silently disappears
 *   { type: 'phone',     key, phone, href }         the number has a digit
 *   { type: 'link',      key, id, label, url, hostname, icon }
 *                                                   one per `links` entry whose
 *                                                   label and address pass the
 *                                                   link rules, in the owner's
 *                                                   order; label trimmed, url as
 *                                                   the server stores it
 *
 * `key` is unique within the list and stable for the same payload, which is all
 * a React key and an "open chip" selection need: a link's key is its id where
 * keptLinkIds keeps one, otherwise its position, and the two prefixes cannot
 * collide. The display mode is NOT applied here: the caller decides where, or
 * whether, the items appear.
 *
 * Always returns an array and never throws, whatever `business` is.
 *
 * @param {unknown} business - PublicMenu.business, or the dashboard draft
 * @returns {object[]}
 */
export function buildContactItems(business) {
  try {
    if (!isPlainObject(business)) return []

    const items = []

    const ssid = plainText(business.wifi_ssid)
    const password = plainText(business.wifi_password)
    if (ssid || password) {
      items.push({ type: 'wifi', key: 'wifi', ssid, password })
    }

    const instagram = plainText(business.instagram)
    if (instagram) {
      const handle = instagramHandle(instagram)
      items.push({
        type: 'instagram',
        key: 'instagram',
        handle,
        url: handle ? instagramUrl(handle) : null,
        text: handle ? `@${handle}` : instagram,
      })
    }

    const phone = plainText(business.phone)
    const href = telHref(phone)
    if (href) {
      items.push({ type: 'phone', key: 'phone', phone, href })
    }

    const links = Array.isArray(business.links) ? business.links : []
    const ids = keptLinkIds(links)

    links.forEach((entry, index) => {
      const fields = linkFields(entry)
      if (!fields || linkLabelProblem(fields.label)) return

      const { link } = inspectLinkUrl(fields.url)
      if (!link) return

      const id = ids[index]
      items.push({
        type: 'link',
        key: id ? `link:${id}` : `link-index:${index}`,
        id: id || '',
        label: trimSpace(fields.label),
        url: link.href,
        hostname: link.hostname,
        icon: iconFor(link),
      })
    })

    return items
  } catch {
    /* Only a hostile object gets here - a getter that throws, say. No contact
       row is worth the menu it sits on. */
    return []
  }
}
