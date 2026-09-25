// Assertions for the landing-page dictionary, src/locales/landing.js.
//
//   node frontend/tests/landingLocales.test.mjs
//
// The landing page, its header and the sign-up dialog speak tr / en / de. The
// failure this file exists to catch is the half-translated page: a key added to
// Turkish and forgotten in German renders `undefined`, and a string typed
// straight into the JSX stays Turkish whatever the visitor picked. So besides
// the dictionary itself, the landing sources are read as text and every key
// they use is checked against every language.
//
// Nothing imports this file, so it never reaches the app bundle.

import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

import {
  DEFAULT_LANDING_LANGUAGE,
  LANDING_LANGUAGES,
  LANDING_STRINGS,
  landingText,
  signUpErrorKey,
  signUpErrorMessage,
  validateSignUp,
} from '../src/locales/landing.js'

const SRC = join(dirname(fileURLToPath(import.meta.url)), '..', 'src')

// Every file that renders landing copy. A new component under
// components/landing belongs in this list.
const LANDING_SOURCES = [
  'pages/Landing.jsx',
  'components/landing/Header.jsx',
  'components/landing/PhoneFrame.jsx',
  'components/landing/SignUpModal.jsx',
]

const CODES = LANDING_LANGUAGES.map((language) => language.code)
const REFERENCE = LANDING_STRINGS[DEFAULT_LANDING_LANGUAGE]
const REFERENCE_KEYS = Object.keys(REFERENCE).sort()

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

/** The source text with /* *\/ and // comments blanked out. Rough, but the
 *  landing files hold no string that contains a comment marker. */
function withoutComments(source) {
  return source.replace(/\/\*[\s\S]*?\*\//g, ' ').replace(/(^|[^:'"])\/\/.*$/gm, '$1')
}

function readSource(path) {
  return readFileSync(join(SRC, path), 'utf8')
}

/* ------------------------------------------------------------ dictionary */

check('every picker language has a dictionary, and nothing else does', () => {
  assert.ok(CODES.includes(DEFAULT_LANDING_LANGUAGE), 'default language is not in the picker')
  assert.deepEqual(Object.keys(LANDING_STRINGS).sort(), [...CODES].sort())
  assert.equal(new Set(CODES).size, CODES.length, 'a language code is listed twice')
  for (const language of LANDING_LANGUAGES) {
    assert.ok(language.short && language.name, `${language.code}: short code or name missing`)
  }
})

check('all languages carry exactly the same keys', () => {
  const problems = []
  for (const code of CODES) {
    const keys = Object.keys(LANDING_STRINGS[code])
    const missing = REFERENCE_KEYS.filter((key) => !keys.includes(key))
    const extra = keys.filter((key) => !(key in REFERENCE))
    if (missing.length) problems.push(`${code} lacks: ${missing.join(', ')}`)
    if (extra.length) problems.push(`${code} has extra: ${extra.join(', ')}`)
  }
  assert.deepEqual(problems, [])
})

check('no value is empty, padded or anything but a string', () => {
  const problems = []
  for (const code of CODES) {
    for (const [key, value] of Object.entries(LANDING_STRINGS[code])) {
      if (typeof value !== 'string') problems.push(`${code}.${key} is ${typeof value}`)
      else if (!value.trim()) problems.push(`${code}.${key} is empty`)
      else if (value !== value.trim()) problems.push(`${code}.${key} has surrounding spaces`)
    }
  }
  assert.deepEqual(problems, [])
})

check('the numbers in a sentence agree across languages', () => {
  // "at least 8 characters", "100", "72 bytes", "2026": the limits are spelled
  // out in every language, so a change made in one must be made in all.
  const digits = (text) => (text.match(/\d+/g) || []).sort().join(',')
  const problems = []
  for (const key of REFERENCE_KEYS) {
    for (const code of CODES) {
      if (digits(LANDING_STRINGS[code][key]) !== digits(REFERENCE[key])) {
        problems.push(`${code}.${key}: [${digits(LANDING_STRINGS[code][key])}] vs [${digits(REFERENCE[key])}]`)
      }
    }
  }
  assert.deepEqual(problems, [])
})

check('the brand name is kept, never translated', () => {
  const problems = []
  for (const key of REFERENCE_KEYS) {
    if (!REFERENCE[key].includes('Karecik')) continue
    for (const code of CODES) {
      if (!LANDING_STRINGS[code][key].includes('Karecik')) problems.push(`${code}.${key}`)
    }
  }
  assert.deepEqual(problems, [])
})

check('landingText returns the language asked for, Turkish otherwise', () => {
  for (const code of CODES) assert.equal(landingText(code), LANDING_STRINGS[code])
  for (const unknown of ['ar', 'xx', '', undefined, null]) {
    assert.equal(landingText(unknown), REFERENCE)
  }
})

/* --------------------------------------------------------------- sources */

check('every key the landing sources use exists in every language', () => {
  const used = new Set()
  for (const path of LANDING_SOURCES) {
    const source = withoutComments(readSource(path))
    for (const match of source.matchAll(/\bt\.([A-Za-z][A-Za-z0-9]*)/g)) used.add(match[1])
  }
  // Keys handed around as values (validateSignUp, signUpErrorKey) rather than
  // written as t.key.
  const landingModule = withoutComments(readFileSync(join(SRC, 'locales/landing.js'), 'utf8'))
  for (const match of landingModule.matchAll(/'(signUpError[A-Za-z]+)'/g)) used.add(match[1])

  assert.ok(used.size > 20, `only ${used.size} keys found; the scan has stopped matching`)
  const problems = []
  for (const key of used) {
    for (const code of CODES) {
      if (typeof LANDING_STRINGS[code][key] !== 'string') problems.push(`${code}.${key}`)
    }
  }
  assert.deepEqual(problems, [])
})

check('no Turkish copy is hardcoded in the landing sources', () => {
  // Letters only Turkish uses among the three languages. ö and ü are left out
  // on purpose: German shares them, and a German word in the JSX is caught by
  // review, not by this. Comments may say anything.
  const turkishOnly = /[çğıışÇĞİŞ]/
  const problems = []
  for (const path of LANDING_SOURCES) {
    withoutComments(readSource(path))
      .split('\n')
      .forEach((line, index) => {
        if (turkishOnly.test(line)) problems.push(`${path}:${index + 1}: ${line.trim()}`)
      })
  }
  assert.deepEqual(problems, [])
})

/* -------------------------------------------------------- sign-up checks */

check('validateSignUp names the first problem of each field', () => {
  assert.deepEqual(validateSignUp({}), {
    businessName: 'signUpErrorBusinessNameRequired',
    email: 'signUpErrorEmailRequired',
    password: 'signUpErrorPasswordRequired',
  })
  assert.deepEqual(validateSignUp({ businessName: '   ', email: ' ', password: '' }), {
    businessName: 'signUpErrorBusinessNameRequired',
    email: 'signUpErrorEmailRequired',
    password: 'signUpErrorPasswordRequired',
  })
  assert.deepEqual(validateSignUp({ businessName: ' K ', email: 'a@b', password: '1234567' }), {
    businessName: 'signUpErrorBusinessNameShort',
    email: 'signUpErrorEmailInvalid',
    password: 'signUpErrorPasswordShort',
  })
  assert.deepEqual(
    validateSignUp({ businessName: 'Kahve Durağı', email: ' kahve@ornek.com ', password: '12345678' }),
    {},
  )
})

check('validateSignUp counts the name in characters, the password in bytes', () => {
  const valid = { email: 'kahve@ornek.com', password: '12345678' }
  // 100 characters is the server's ceiling; an emoji is ONE character to it
  // (len([]rune)), though two UTF-16 units to JavaScript's .length.
  assert.deepEqual(validateSignUp({ ...valid, businessName: 'a'.repeat(100) }), {})
  assert.deepEqual(validateSignUp({ ...valid, businessName: '☕'.repeat(50) + '\u{1F370}'.repeat(50) }), {})
  assert.equal(
    validateSignUp({ ...valid, businessName: 'a'.repeat(101) }).businessName,
    'signUpErrorBusinessNameLong',
  )
  // "ş" is two bytes: 36 of them fit bcrypt's 72, 37 do not.
  const base = { businessName: 'Kahve', email: 'kahve@ornek.com' }
  assert.deepEqual(validateSignUp({ ...base, password: 'ş'.repeat(36) }), {})
  assert.equal(validateSignUp({ ...base, password: 'ş'.repeat(37) }).password, 'signUpErrorPasswordLong')
  assert.deepEqual(validateSignUp({ ...base, password: 'a'.repeat(72) }), {})
  assert.equal(validateSignUp({ ...base, password: 'a'.repeat(73) }).password, 'signUpErrorPasswordLong')
})

check('a failed request is described in the visitor language', () => {
  const apiError = (message, status, code) => Object.assign(new Error(message), { status, code })

  const taken = apiError('Bu e-posta adresi zaten kayıtlı.', 409, 'CONFLICT')
  const offline = apiError('Sunucuya ulaşılamadı. Backend çalışıyor mu?', 0, 'NETWORK_ERROR')
  const crashed = apiError('Sunucuda beklenmeyen bir hata olustu.', 500, 'INTERNAL_ERROR')
  const limited = apiError('Çok fazla istek.', 429, 'TOO_MANY_REQUESTS')
  const refused = apiError('İşletme adı metin olmalıdır.', 422, 'VALIDATION_ERROR')
  const bug = new TypeError("Cannot read properties of undefined (reading 'id')")

  assert.equal(signUpErrorKey(taken), 'signUpErrorEmailTaken')
  assert.equal(signUpErrorKey(offline), 'signUpErrorNetwork')
  assert.equal(signUpErrorKey(crashed), 'signUpErrorServer')
  assert.equal(signUpErrorKey(limited), 'signUpErrorRateLimited')
  assert.equal(signUpErrorKey(refused), 'signUpErrorGeneric')
  assert.equal(signUpErrorKey(bug), 'signUpErrorGeneric')
  assert.equal(signUpErrorKey(undefined), 'signUpErrorGeneric')

  for (const code of CODES) {
    const t = LANDING_STRINGS[code]
    assert.equal(signUpErrorMessage(taken, code), t.signUpErrorEmailTaken)
    assert.equal(signUpErrorMessage(offline, code), t.signUpErrorNetwork)
    assert.equal(signUpErrorMessage(crashed, code), t.signUpErrorServer)
    // A JavaScript error's text is never shown, in any language.
    assert.equal(signUpErrorMessage(bug, code), t.signUpErrorGeneric)
  }
  // An unforeseen 4xx keeps the server's own Turkish sentence for a Turkish
  // visitor, and becomes the translated fallback for everyone else.
  assert.equal(signUpErrorMessage(refused, 'tr'), refused.message)
  assert.equal(signUpErrorMessage(refused, 'en'), LANDING_STRINGS.en.signUpErrorGeneric)
  assert.equal(signUpErrorMessage(refused, 'de'), LANDING_STRINGS.de.signUpErrorGeneric)
  assert.equal(signUpErrorMessage(apiError('  ', 422), 'tr'), REFERENCE.signUpErrorGeneric)
})

/* --------------------------------------------------------------- summary */

if (failures.length > 0) {
  console.error(`${failures.length} failed, ${passed} passed\n\n${failures.join('\n\n')}`)
  process.exit(1)
}
console.log(`landingLocales.test.mjs: all ${passed} checks passed`)
