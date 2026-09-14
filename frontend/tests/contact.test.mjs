// Assertions for the link rules and the contact helpers of src/lib/contact.js.
//
//   node frontend/tests/contact.test.mjs
//
// Nothing imports this file, so it never reaches the app bundle. Every control
// or invisible character below is written as an escape sequence.

import assert from 'node:assert/strict'

import {
  buildContactItems,
  completeLinkUrl,
  contactDisplayMode,
  instagramHandle,
  instagramUrl,
  keptLinkIds,
  LINK_ID_PATTERN,
  LINK_MESSAGES,
  linkHostname,
  linkIcon,
  linkLabelProblem,
  linkListProblem,
  linkUrlProblem,
  MAX_LINK_LABEL_RUNES,
  MAX_LINK_URL_BYTES,
  MAX_LINKS,
  safeExternalUrl,
  telHref,
  trimSpace,
} from '../src/lib/contact.js'

let passed = 0
const failures = []

function check(name, run) {
  try {
    run()
    passed += 1
  } catch (error) {
    failures.push(`${name}\n    ${String(error.message).split('\n').join('\n    ')}`)
  }
}

const M = LINK_MESSAGES
const OK = ''
const hex = (code) => `U+${code.toString(16).padStart(4, '0')}`
const valid = (url) => assert.equal(linkUrlProblem(url), OK, JSON.stringify(url))
const invalid = (url) => assert.equal(linkUrlProblem(url), M.urlInvalid, JSON.stringify(url))
const link = (label, url, id) => (id === undefined ? { label, url } : { id, label, url })

/* The White_Space characters Go's strings.TrimSpace trims. */
const GO_SPACES = [
  0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x20, 0x85, 0xa0, 0x1680, 0x2000, 0x2001, 0x2002, 0x2003, 0x2004,
  0x2005, 0x2006, 0x2007, 0x2008, 0x2009, 0x200a, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000,
]

/* The invisible formatting characters of the link rules. */
const INVISIBLE = [
  0xad, 0x61c, 0x180e, 0x200b, 0x200c, 0x200d, 0x200e, 0x200f, 0x202a, 0x202b, 0x202c, 0x202d,
  0x202e, 0x2060, 0x2061, 0x2062, 0x2063, 0x2064, 0x2066, 0x2067, 0x2068, 0x2069, 0x206a, 0x206b,
  0x206c, 0x206d, 0x206e, 0x206f, 0xfeff,
]

/* ------------------------------------------------------------ constants */

check('messages are the server wording, limits are 8 / 40 / 500', () => {
  assert.deepEqual(M, {
    tooMany: 'En fazla 8 link ekleyebilirsiniz.',
    listInvalid: 'Link listesi geçersiz.',
    labelInvalid: 'Link adı geçersiz karakter içeriyor.',
    labelRequired: 'Link adı zorunludur.',
    labelTooLong: 'Link adı en fazla 40 karakter olabilir.',
    urlTooLong: 'Link adresi en fazla 500 karakter olabilir.',
    urlInvalid: 'Link adresi http:// veya https:// ile başlayan geçerli bir adres olmalıdır.',
  })
  assert.equal(MAX_LINKS, 8)
  assert.equal(MAX_LINK_LABEL_RUNES, 40)
  assert.equal(MAX_LINK_URL_BYTES, 500)
})

/* ------------------------------------------------------------------ trim */

check('trimSpace trims every Go White_Space character at both ends', () => {
  for (const code of GO_SPACES) {
    const space = String.fromCharCode(code)
    assert.equal(trimSpace(`${space}a b${space}${space}`), 'a b', hex(code))
  }
})

check('trimSpace keeps U+FEFF, U+180E and U+200B, which Go does not trim', () => {
  assert.equal(trimSpace('\ufeffa\ufeff'), '\ufeffa\ufeff')
  assert.equal(trimSpace('\u180ea\u180e'), '\u180ea\u180e')
  assert.equal(trimSpace(' \u200ba '), '\u200ba')
})

check('trimSpace of anything but a string is empty', () => {
  for (const value of [null, undefined, 42, {}, []]) assert.equal(trimSpace(value), '')
})

/* ----------------------------------------------------------------- label */

check('label: ordinary labels pass, trimmed as Go trims', () => {
  assert.equal(linkLabelProblem('Web sitemiz'), OK)
  assert.equal(linkLabelProblem('  Web  '), OK)
  assert.equal(linkLabelProblem('\u0085Web\u3000'), OK)
  assert.equal(linkLabelProblem('Rezervasyon (WhatsApp)'), OK)
})

check('label: every C0 and C1 control character is an invalid character', () => {
  for (let code = 0; code <= 0x9f; code += 1) {
    if (code > 0x1f && code < 0x7f) continue
    assert.equal(linkLabelProblem(`Web${String.fromCharCode(code)}site`), M.labelInvalid, hex(code))
  }
})

check('label examples: NUL, BEL, newline and U+0085', () => {
  assert.equal(linkLabelProblem('\u0000'), M.labelInvalid)
  assert.equal(linkLabelProblem('\u0007'), M.labelInvalid)
  assert.equal(linkLabelProblem('Web\u0007'), M.labelInvalid)
  assert.equal(linkLabelProblem('Web\u000asite'), M.labelInvalid)
  assert.equal(linkLabelProblem('Web\u0085site'), M.labelInvalid)
  // Alone they are white space the trim removes, which leaves nothing.
  assert.equal(linkLabelProblem('\u000a'), M.labelRequired)
  assert.equal(linkLabelProblem('\u0085'), M.labelRequired)
})

check('label example: U+200B alone, and any label of invisible characters only, is required', () => {
  assert.equal(linkLabelProblem('\u200b'), M.labelRequired)
  for (const code of INVISIBLE) {
    const character = String.fromCharCode(code)
    assert.equal(linkLabelProblem(character), M.labelRequired, hex(code))
    assert.equal(linkLabelProblem(` ${character}${character} `), M.labelRequired, `${hex(code)} twice`)
    assert.equal(linkLabelProblem(`${character}W`), OK, `${hex(code)} before a letter`)
  }
  assert.equal(linkLabelProblem(''), M.labelRequired)
  assert.equal(linkLabelProblem('   '), M.labelRequired)
  assert.equal(linkLabelProblem(null), M.labelRequired)
})

check('label: U+2065 and U+FFF9 are not on the invisible list', () => {
  assert.equal(linkLabelProblem('\u2065'), OK)
  assert.equal(linkLabelProblem('\ufff9'), OK)
})

check('label example: U+200B, a space and U+200B is required - white space among invisible characters shows nothing either', () => {
  assert.equal(linkLabelProblem('\u200b \u200b'), M.labelRequired)
  assert.equal(linkLabelProblem(' \u200b \u200b '), M.labelRequired)
  assert.equal(linkLabelProblem('\u200b\u3000\ufeff\u00a0\u2060'), M.labelRequired)
  for (const code of GO_SPACES) {
    const space = String.fromCharCode(code)
    // Tab to carriage return and U+0085 are control characters as well, and a
    // control character inside the label is reported before anything else.
    const expected = code <= 0x1f || code === 0x85 ? M.labelInvalid : M.labelRequired
    assert.equal(linkLabelProblem(`\u200b${space}\u200b`), expected, hex(code))
    assert.equal(linkLabelProblem(`\ufeff${space}${space}\u2066`), expected, `${hex(code)} twice`)
  }
  // Anything visible left over keeps the label, and the invisible characters
  // stay part of it: they are left out of the blank test only.
  assert.equal(linkLabelProblem('\u200b a \u200b'), OK)
  assert.equal(linkLabelProblem('\u200b\u2065\u200b'), OK)
  assert.equal(linkLabelProblem(`\u200b ${'a'.repeat(38)} \u200b`), M.labelTooLong)
})

check('label: at most 40 code points of the trimmed label', () => {
  assert.equal(linkLabelProblem('a'.repeat(40)), OK)
  assert.equal(linkLabelProblem('a'.repeat(41)), M.labelTooLong)
  assert.equal(linkLabelProblem(`  ${'a'.repeat(40)}\u0085`), OK)
  assert.equal(linkLabelProblem('ş'.repeat(40)), OK)
  assert.equal(linkLabelProblem('\ud83d\ude00'.repeat(40)), OK)
  assert.equal(linkLabelProblem('\ud83d\ude00'.repeat(41)), M.labelTooLong)
  // A lone surrogate reaches the server as one U+FFFD.
  assert.equal(linkLabelProblem('\ud800'.repeat(40)), OK)
  assert.equal(linkLabelProblem('\ud800'.repeat(41)), M.labelTooLong)
})

check('label: control first, then required, then length', () => {
  assert.equal(linkLabelProblem(`${'a'.repeat(50)}\u0007`), M.labelInvalid)
  assert.equal(linkLabelProblem('\u200b\u0000'), M.labelInvalid)
  assert.equal(linkLabelProblem('\u200b'.repeat(50)), M.labelRequired)
})

/* ------------------------------------------------------------------- url */

check('url: ordinary addresses pass', () => {
  for (const url of [
    'https://ornek.com',
    'http://ornek.com',
    'HTTPS://ORNEK.COM',
    'https://www.ornek.com/menu',
    'https://ornek.com/menu?masa=5#tatlilar',
    'https://ornek.com/',
    'https://ornek.com?',
    'https://ornek.com#',
    'https://wa.me/905550000000',
    'https://maps.app.goo.gl/AbC123',
    'https://a.b',
    'https://sub-domain.ornek.com.tr',
    'https://ornek.com/a@b',
    'https://ornek.com/?x=@y',
    'https://ornek.com/#@y',
    'https://ornek.com/a:b',
    'https://ornek.com/%41',
    'https://1.2.3.a4',
    'https://123.com',
    'https://xn--rnek-zoa.com',
    'https://o--k.com',
  ]) {
    valid(url)
  }
})

check('url: http:// or https:// in any letter case, and nothing else', () => {
  for (const url of [
    '',
    'ftp://ornek.com',
    'javascript:alert(1)',
    'data:text/html,x',
    'https:ornek.com',
    'https:/ornek.com',
    'https//ornek.com',
    'https:\\\\ornek.com',
    '//ornek.com',
    'ornek.com',
    '/menu',
    'mailto:a@ornek.com',
    'httpss://ornek.com',
  ]) {
    invalid(url)
  }
  assert.equal(linkUrlProblem(null), M.urlInvalid)
  assert.equal(linkUrlProblem(42), M.urlInvalid)
})

check('url example: https://<svg/onload=alert(1)> is refused', () => {
  invalid('https://<svg/onload=alert(1)>')
})

check('url: each of < > " ` { } | \\ ^ [ ] is refused anywhere', () => {
  for (const character of ['<', '>', '"', '`', '{', '}', '|', '\\', '^', '[', ']']) {
    invalid(`https://ornek.com/a${character}b`)
    invalid(`https://ornek.com/?q=${character}`)
    invalid(`https://ornek.com/#${character}`)
    invalid(`https://orn${character}ek.com`)
  }
})

check('url: white space, control and invisible characters are refused anywhere', () => {
  const codes = [...GO_SPACES, 0x00, 0x01, 0x07, 0x1f, 0x7f, 0x80, 0x9f, ...INVISIBLE]
  for (const code of codes) {
    const character = String.fromCharCode(code)
    invalid(`https://ornek.com/a${character}b`)
    invalid(`https://ornek.com/?q=a${character}b`)
    invalid(`https://orn${character}ek.com`)
  }
})

check('url example: a host carrying U+202E is refused', () => {
  invalid('https://ornek\u202emoc.evil.com')
  invalid('https://\u202eornek.com')
})

check('url: U+2065 and U+FFF9-U+FFFB are on no list - fine in a path, not letters in a host', () => {
  valid('https://ornek.com/\u2065')
  valid('https://ornek.com/\ufff9\ufffb')
  invalid('https://orn\ufff9ek.com')
})

check('url: trimmed as Go trims', () => {
  valid('\u0085https://ornek.com\u2029')
  invalid('\ufeffhttps://ornek.com')
})

check('url: credentials before the host are refused, an empty one included', () => {
  for (const url of [
    'https://user@ornek.com',
    'https://user:pass@ornek.com',
    'https://@ornek.com',
    'https://ornek.com@evil.com',
    'https://a@b@ornek.com',
  ]) {
    invalid(url)
  }
})

check('url examples: https://. and https://- are refused', () => {
  invalid('https://.')
  invalid('https://-')
})

check('url example: https://https//ornek.com is refused - its host is "https"', () => {
  invalid('https://https//ornek.com')
})

check('url: the hostname is at least two labels of letters, digits and inner hyphens', () => {
  for (const url of [
    'https://',
    'https:///ornek.com',
    'https://:80',
    'https://localhost',
    'https://ornek',
    'https://ornek.',
    'https://ornek.com.',
    'https://.ornek.com',
    'https://ornek..com',
    'https://-ornek.com',
    'https://ornek-.com',
    'https://ornek.-com',
    'https://ornek.com-',
    'https://orn_ek.com',
    'https://orn~ek.com',
    "https://orn'ek.com",
    'https://orn!ek.com',
    'https://orn*ek.com',
    'https://orn%ek.com',
    'https://[::1]',
    'https://[::1]:8080',
  ]) {
    invalid(url)
  }
  valid('https://orn-ek.com')
  valid('https://a1.b2c')
  valid('https://ornek.c-1')
})

check('url: the last label needs a letter, which keeps bare IPv4 addresses out', () => {
  invalid('https://1.2.3.4')
  invalid('https://192.168.1.1:80/menu')
  invalid('https://ornek.123')
})

check('url: a label is at most 63 characters, the hostname at most 253', () => {
  valid(`https://${'a'.repeat(63)}.com`)
  invalid(`https://${'a'.repeat(64)}.com`)
  const label = 'a'.repeat(49)
  valid(`https://${label}.${label}.${label}.${label}.${label}.com`) // 253
  invalid(`https://${label}a.${label}.${label}.${label}.${label}.com`) // 254
  valid(`https://${'ş'.repeat(63)}.com`)
  invalid(`https://${'ş'.repeat(64)}.com`)
})

check('url: any Unicode letter in a label, but not a combining mark', () => {
  valid('https://örnek.com')
  valid('https://\u4f8b\u3048.jp')
  valid('https://\u0645\u0637\u0639\u0645.com')
  valid('https://\ud835\udc9c.com')
  invalid('https://e\u0301.com')
  assert.equal(linkHostname('https://ÖRNEK.com.tr/menu'), 'örnek.com.tr')
})

check('url: a label holds ASCII digits only - the digits of other scripts are refused', () => {
  valid('https://ornek1.com')
  invalid('https://\u0663\u0664.com')
  invalid('https://ornek\uff11.com')
  invalid('https://ornek.\u0663')
})

check('url: host escapes follow Go - only bytes of 0x80 and above, forming valid UTF-8', () => {
  valid('https://%C3%BCr%C3%BCn.com')
  assert.equal(linkHostname('https://%C3%BCr%C3%BCn.com'), 'ürün.com')
  assert.equal(safeExternalUrl('https://%C3%BCr%C3%BCn.com'), 'https://%C3%BCr%C3%BCn.com')
  for (const url of [
    'https://%41.com',
    'https://orn%65k.com',
    'https://%25.com',
    'https://%zz.com',
    'https://%C3.com',
    'https://%E2%80%8B.com',
    'https://%ED%A0%80.com',
    'https://%C0%AF.com',
    'https://%F4%90%80%80.com',
    'https://[ornek.com%25eth0]',
  ]) {
    invalid(url)
  }
})

check('url example: https://[ornek.com] is refused - no square brackets anywhere, whatever Go makes of them', () => {
  for (const url of [
    'https://[ornek.com]',
    'https://[ornek.com]:8080/menu',
    'HTTPS://[ORNEK.COM]',
    'https://[ornek.com',
    'https://[ornek.com]:',
    'https://[ornek.com]x',
    'https://ornek.com]',
    'https://[::1]/menu',
    'https://ornek.com/[menu]',
    'https://ornek.com/menu?masa[]=5',
    'https://ornek.com/#[tatlilar]',
  ]) {
    invalid(url)
  }
  assert.equal(linkHostname('https://[ornek.com]'), null)
  assert.equal(safeExternalUrl('https://[ornek.com]:8080/menu'), null)
  assert.equal(linkIcon('https://[wa.me]/905550000000'), 'globe')
  // Escaped brackets are ordinary text in a path or a query. In a host they are
  // escapes of ASCII bytes, which Go refuses there.
  valid('https://ornek.com/%5Bmenu%5D')
  valid('https://ornek.com/menu?masa%5B%5D=5')
  invalid('https://%5Bornek.com%5D')
})

check('url example: a port is 1-5 digits worth 1-65535, so :99999 is refused', () => {
  invalid('https://ornek.com:99999')
  for (const url of [
    'https://ornek.com:0',
    'https://ornek.com:00000',
    'https://ornek.com:65536',
    'https://ornek.com:000080',
    'https://ornek.com:8a',
    'https://ornek.com:80:90',
    'https://ornek.com:-1',
  ]) {
    invalid(url)
  }
  for (const url of [
    'https://ornek.com:1',
    'https://ornek.com:8080/menu',
    'https://ornek.com:65535',
    'https://ornek.com:00080',
  ]) {
    valid(url)
  }
  // A colon with nothing after it is refused, not read as "no port".
  invalid('https://ornek.com:')
  invalid('https://ornek.com:/menu')
})

check('url: a broken % escape in the path or the fragment is refused, in the query it is not', () => {
  for (const url of [
    'https://ornek.com/%zz',
    'https://ornek.com/%4',
    'https://ornek.com/a%',
    'https://ornek.com/#%zz',
    'https://ornek.com/#x%',
  ]) {
    invalid(url)
  }
  valid('https://ornek.com/?q=%zz')
  valid('https://ornek.com/?q=%')
  valid('https://ornek.com/%E2%82%AC')
})

check('url: at most 500 UTF-8 bytes of the trimmed address, checked before anything else', () => {
  const base = 'https://ornek.com/' // 18 bytes
  valid(base + 'a'.repeat(482))
  assert.equal(linkUrlProblem(base + 'a'.repeat(483)), M.urlTooLong)
  valid(base + 'ş'.repeat(241))
  assert.equal(linkUrlProblem(`${base}${'ş'.repeat(241)}a`), M.urlTooLong)
  valid(`   ${base}${'a'.repeat(482)}\u3000`)
  assert.equal(linkUrlProblem(`javascript:${'a'.repeat(490)}`), M.urlTooLong)
  // A lone surrogate reaches the server as U+FFFD: three bytes.
  valid(`${base}${'\ud800'.repeat(160)}aa`)
  assert.equal(linkUrlProblem(`${base}${'\ud800'.repeat(161)}`), M.urlTooLong)
})

check('url: stored with its scheme lowercased and nothing else changed', () => {
  assert.equal(safeExternalUrl('HtTpS://Ornek.COM/Menu?A=B#C'), 'https://Ornek.COM/Menu?A=B#C')
  assert.equal(safeExternalUrl('  HTTP://ornek.com  '), 'http://ornek.com')
  assert.equal(safeExternalUrl('javascript:alert(1)'), null)
  assert.equal(safeExternalUrl(null), null)
  assert.equal(linkHostname('ftp://ornek.com'), null)
})

/* ------------------------------------------------------------------ list */

check('list: not an array, or an entry of the wrong type, is an invalid list', () => {
  for (const value of [{}, 'x', 42, true]) assert.equal(linkListProblem(value), M.listInvalid)
  for (const entry of [42, 'x', true, [], [link('Web', 'https://a.com')]]) {
    assert.equal(linkListProblem([entry]), M.listInvalid, JSON.stringify(entry))
  }
  for (const fields of [
    { label: 5, url: 'https://a.com' },
    { label: 'Web', url: {} },
    { id: 7, label: 'Web', url: 'https://a.com' },
    { label: [], url: 'https://a.com' },
    { label: 'Web', url: true },
  ]) {
    assert.equal(linkListProblem([fields]), M.listInvalid, JSON.stringify(fields))
  }
})

check('list: null is an empty list; a null entry has no members, and a null member is a missing one', () => {
  assert.equal(linkListProblem(null), OK)
  assert.equal(linkListProblem(undefined), OK)
  assert.equal(linkListProblem([]), OK)
  assert.equal(linkListProblem([null]), M.labelRequired)
  assert.equal(linkListProblem([link('Web', 'https://a.com'), null]), M.labelRequired)
  assert.equal(linkListProblem([{ label: null, url: null }]), M.labelRequired)
  assert.equal(linkListProblem([{ label: 'Web', url: null }]), M.urlInvalid)
  assert.equal(linkListProblem([{ label: 'Web' }]), M.urlInvalid)
  assert.equal(linkListProblem([{ id: null, label: 'Web', url: 'https://a.com' }]), OK)
  // A null member is skipped like an absent one, so it does not undo a key in
  // another letter case that did carry a string.
  assert.equal(linkListProblem([{ label: 'Web', LABEL: null, url: 'https://a.com' }]), OK)
  assert.deepEqual(keptLinkIds([{ id: null }, null, { id: 'a', ID: null }]), [null, null, 'a'])
})

check('list: more than 8 entries; a wrong type is reported before the count', () => {
  const eight = Array.from({ length: 8 }, (_, index) => link(`Link ${index}`, 'https://a.com'))
  assert.equal(linkListProblem(eight), OK)
  assert.equal(linkListProblem([...eight, link('Link 8', 'https://a.com')]), M.tooMany)
  assert.equal(linkListProblem([...eight, link('', '')]), M.tooMany)
  assert.equal(linkListProblem([...eight, 42]), M.listInvalid)
})

check('list: the first failing entry is reported, its label before its address', () => {
  assert.equal(
    linkListProblem([link('Web', 'https://a.com'), link('Web', 'ftp://a.com'), link('', 'https://a.com')]),
    M.urlInvalid,
  )
  assert.equal(linkListProblem([link('Web', 'https://a.com'), link('', 'ftp://a.com')]), M.labelRequired)
  assert.equal(linkListProblem([link('a'.repeat(41), 'x'.repeat(600))]), M.labelTooLong)
  assert.equal(linkListProblem([link('Web', 'x'.repeat(600)), link('\u0000', 'ftp://a')]), M.urlTooLong)
})

check('list: keys match in any letter case, a later one winning; other keys are ignored', () => {
  assert.equal(linkListProblem([{ LABEL: 'Web', Url: 'https://a.com', extra: 5 }]), OK)
  assert.equal(linkListProblem([{ label: '', Label: 'Web', url: 'https://a.com' }]), OK)
  assert.equal(linkListProblem([{ label: 'Web', LABEL: '', url: 'https://a.com' }]), M.labelRequired)
  assert.equal(linkListProblem([{ label: 'Web', LABEL: 5, url: 'https://a.com' }]), M.listInvalid)
})

/* ------------------------------------------------------------------- ids */

check('ids: kept when they match the pattern and no earlier entry has them', () => {
  assert.deepEqual(
    keptLinkIds([
      { id: 'a' },
      { id: 'a' },
      { id: 'x y' },
      {},
      { id: '' },
      { id: 'A_b-9' },
      { id: 'q'.repeat(64) },
      { id: 'q'.repeat(65) },
      null,
      { id: 5 },
      { ID: 'upper' },
      { id: 'ş' },
    ]),
    ['a', null, null, null, null, 'A_b-9', 'q'.repeat(64), null, null, null, 'upper', null],
  )
  assert.deepEqual(keptLinkIds([{ id: 'bad id' }, { id: 'b' }, { id: 'b' }]), [null, 'b', null])
  assert.deepEqual(keptLinkIds('x'), [])
  assert.ok(LINK_ID_PATTERN.test('link-1'))
  assert.ok(!LINK_ID_PATTERN.test('link:1'))
})

/* ------------------------------------------------------- completeLinkUrl */

check('completeLinkUrl: repairs a mistyped scheme, otherwise prepends https://', () => {
  const cases = [
    ['https//x', 'https://x'],
    ['http//x', 'http://x'],
    ['https:/x', 'https://x'],
    ['http:/x', 'http://x'],
    ['HTTPS//ornek.com', 'HTTPS://ornek.com'],
    ['https//ornek.com/menu', 'https://ornek.com/menu'],
    ['ornek.com', 'https://ornek.com'],
    ['  ornek.com/menu  ', 'https://ornek.com/menu'],
    ['wa.me/905550000000', 'https://wa.me/905550000000'],
    ['//ornek.com', 'https://ornek.com'],
    ['ornek.com:8080', 'https://ornek.com:8080'],
    ['https://ornek.com', 'https://ornek.com'],
    ['mailto:a@ornek.com', 'mailto:a@ornek.com'],
    ['ornek', 'ornek'],
    ['ornek .com', 'ornek .com'],
    ['', ''],
    [null, ''],
  ]
  for (const [input, expected] of cases) {
    assert.equal(completeLinkUrl(input), expected, JSON.stringify(input))
  }
  assert.notEqual(completeLinkUrl('https//ornek.com'), 'https://https//ornek.com')
  valid(completeLinkUrl('https//ornek.com'))
})

/* ------------------------------------------------------------- instagram */

check('instagramHandle: a name, @name and the profile address in every accepted form', () => {
  for (const form of [
    '@kahve.duragi',
    '@@kahve.duragi',
    'kahve.duragi',
    '  @kahve.duragi  ',
    '  @@@kahve.duragi  ',
    'instagram.com/kahve.duragi',
    'www.instagram.com/kahve.duragi',
    'http://instagram.com/kahve.duragi',
    'https://instagram.com/kahve.duragi',
    'http://www.instagram.com/kahve.duragi',
    'https://www.instagram.com/kahve.duragi',
    'https://www.instagram.com/kahve.duragi/',
    'https://www.instagram.com/kahve.duragi?igsh=MWx0',
    'https://www.instagram.com/kahve.duragi/?igsh=MWx0',
    'HTTPS://WWW.INSTAGRAM.COM/kahve.duragi',
  ]) {
    assert.equal(instagramHandle(form), 'kahve.duragi', form)
  }
  assert.equal(instagramUrl('instagram.com/kahve_duragi'), 'https://www.instagram.com/kahve_duragi/')
})

check('instagramHandle: trims, drops every leading @, trims again, then reads a name or a profile address', () => {
  for (const [value, expected] of [
    ['@ kahveduragi', 'kahveduragi'],
    [' @@ kahveduragi ', 'kahveduragi'],
    ['@instagram.com/kahveduragi', 'kahveduragi'],
    ['@ www.instagram.com/kahveduragi', 'kahveduragi'],
    ['@https://www.instagram.com/kahveduragi/', 'kahveduragi'],
    ['@@https://instagram.com/kahveduragi?igsh=MWx0', 'kahveduragi'],
    ['@', null],
    ['@ ', null],
    ['@ @kahveduragi', null],
    ['@ kahve duragi', null],
    ['@https://www.instagram.com/p/C0d3/', null],
  ]) {
    assert.equal(instagramHandle(value), expected, JSON.stringify(value))
  }
})

check('instagramHandle: p, reel, reels, stories, explore and tv are not user names', () => {
  for (const page of ['p', 'reel', 'reels', 'stories', 'explore', 'tv', 'P', 'Reels']) {
    assert.equal(instagramHandle(`https://www.instagram.com/${page}/C0d3/`), null, page)
    assert.equal(instagramHandle(`instagram.com/${page}`), null, page)
  }
})

check('instagramHandle: no user name', () => {
  for (const value of [
    'kahve duragi',
    'kahve-duragi',
    '@',
    '@@',
    'kahve@duragi',
    '@kahve@',
    '',
    'instagram.com/',
    'https://www.facebook.com/kahve',
    'https://m.instagram.com/kahve',
    null,
    42,
  ]) {
    assert.equal(instagramHandle(value), null, String(value))
  }
})

/* ----------------------------------------------------------------- items */

check('items: an Instagram value with no user name is still an item - text, no link', () => {
  assert.deepEqual(buildContactItems({ instagram: 'Kahve Durağı resmi hesap' }), [
    { type: 'instagram', key: 'instagram', handle: null, url: null, text: 'Kahve Durağı resmi hesap' },
  ])
  assert.deepEqual(buildContactItems({ instagram: 'https://www.instagram.com/p/C0d3/' }), [
    { type: 'instagram', key: 'instagram', handle: null, url: null, text: 'https://www.instagram.com/p/C0d3/' },
  ])
  assert.deepEqual(buildContactItems({ instagram: ' https://www.instagram.com/kahve/ ' }), [
    { type: 'instagram', key: 'instagram', handle: 'kahve', url: 'https://www.instagram.com/kahve/', text: '@kahve' },
  ])
  // Every leading "@" goes and the rest is trimmed again, as instagramHandle reads it.
  for (const stored of ['@@kahve', '@ kahve', '@instagram.com/kahve', '@https://www.instagram.com/kahve/']) {
    assert.deepEqual(
      buildContactItems({ instagram: stored }),
      [{ type: 'instagram', key: 'instagram', handle: 'kahve', url: 'https://www.instagram.com/kahve/', text: '@kahve' }],
      stored,
    )
  }
  assert.deepEqual(buildContactItems({ instagram: '@' }), [
    { type: 'instagram', key: 'instagram', handle: null, url: null, text: '@' },
  ])
  assert.deepEqual(buildContactItems({ instagram: '   ' }), [])
})

check('items: a link is drawn only when its label and address pass the rules', () => {
  const items = buildContactItems({
    links: [
      link('Web', 'https://ornek.com', 'web'),
      link('', 'https://ornek.com', 'blank'),
      link('Web\u0007', 'https://ornek.com', 'bel'),
      link('\u200b', 'https://ornek.com', 'zwsp'),
      link('a'.repeat(41), 'https://ornek.com', 'long'),
      link('Evil', 'https://<svg/onload=alert(1)>', 'svg'),
      link('Bidi', 'https://ornek\u202emoc.com', 'bidi'),
      link('Port', 'https://ornek.com:99999', 'port'),
      link('Scheme', 'https://https//ornek.com', 'scheme'),
      link('Bracket', 'https://[ornek.com]', 'bracket'),
      link('\u200b \u200b', 'https://ornek.com', 'blank-zwsp'),
      link(' Harita ', ' HTTPS://maps.app.goo.gl/x ', 'map'),
      { id: 'typed', label: 5, url: 'https://ornek.com' },
      42,
      null,
    ],
  })
  assert.deepEqual(
    items.map((item) => item.key),
    ['link:web', 'link:map'],
  )
  assert.deepEqual(items[1], {
    type: 'link',
    key: 'link:map',
    id: 'map',
    label: 'Harita',
    url: 'https://maps.app.goo.gl/x',
    hostname: 'maps.app.goo.gl',
    icon: 'maps',
  })
})

check('items: keys stay unique when stored ids repeat, are blank or are invalid', () => {
  const items = buildContactItems({
    wifi_ssid: 'KahveNet',
    instagram: 'kahve',
    phone: '+90 555 010 20 30',
    links: [
      link('A', 'https://a.com', 'same'),
      link('B', 'https://b.com', 'same'),
      link('C', 'https://c.com', ''),
      link('D', 'https://d.com'),
      link('E', 'https://e.com', 'bad id'),
      link('F', 'https://f.com', 'link-index:4'),
      link('G', 'https://g.com', 'wifi'),
    ],
  })
  const keys = items.map((item) => item.key)
  assert.equal(items.length, 10)
  assert.equal(new Set(keys).size, keys.length, keys.join(', '))
})

check('items: always an array, never a throw', () => {
  for (const value of [null, undefined, 42, 'x', [], { links: 'x' }]) {
    assert.ok(Array.isArray(buildContactItems(value)))
  }
  const hostile = {
    get wifi_ssid() {
      throw new Error('boom')
    },
  }
  assert.deepEqual(buildContactItems(hostile), [])
})

check('linkIcon, telHref and contactDisplayMode', () => {
  assert.equal(linkIcon('https://wa.me/905550000000'), 'whatsapp')
  assert.equal(linkIcon('https://www.google.com/maps/place/x'), 'maps')
  assert.equal(linkIcon('https://www.instagram.com/kahve'), 'instagram')
  assert.equal(linkIcon('https://evilinstagram.com/'), 'globe')
  assert.equal(linkIcon('not an address'), 'globe')
  assert.equal(telHref('+90 (555) 000 00 00'), 'tel:+905550000000')
  assert.equal(telHref('yok'), null)
  assert.equal(contactDisplayMode('list'), 'list')
  assert.equal(contactDisplayMode('x'), 'inline')
})

/* --------------------------------------------------------------- summary */

if (failures.length > 0) {
  console.error(`${failures.length} failed, ${passed} passed\n\n${failures.join('\n\n')}`)
  process.exit(1)
}
console.log(`contact.test.mjs: all ${passed} checks passed`)
