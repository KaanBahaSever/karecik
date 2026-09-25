// Assertions for src/lib/language.js: which language the customer menu opens
// in, and the visitor's remembered pick.
//
//   node frontend/tests/menuLanguage.test.mjs
//
// Nothing imports this file, so it never reaches the app bundle.

import assert from 'node:assert/strict'

import {
  baseLanguage,
  browserLanguages,
  LANGUAGE_STORAGE_PREFIX,
  languageStorageKey,
  matchPreferredLanguage,
  readRememberedLanguage,
  rememberLanguage,
  resolveMenuLanguage,
} from '../src/lib/language.js'

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

/** A Storage stand-in; `failing` makes every call throw, like a blocked one. */
function memoryStorage(initial = {}, failing = false) {
  const data = new Map(Object.entries(initial))
  const guard = () => {
    if (failing) throw new Error('SecurityError')
  }
  return {
    data,
    getItem(key) {
      guard()
      return data.has(key) ? data.get(key) : null
    },
    setItem(key, value) {
      guard()
      data.set(key, String(value))
    },
    removeItem(key) {
      guard()
      data.delete(key)
    },
  }
}

/* ------------------------------------------------------------- matching */

check('baseLanguage strips the region and lowercases', () => {
  assert.equal(baseLanguage('de-DE'), 'de')
  assert.equal(baseLanguage('pt_BR'), 'pt')
  assert.equal(baseLanguage(' EN '), 'en')
  assert.equal(baseLanguage('zh-Hant-TW'), 'zh')
  assert.equal(baseLanguage('ar'), 'ar')
  assert.equal(baseLanguage(''), '')
  assert.equal(baseLanguage(null), '')
  assert.equal(baseLanguage(42), '')
})

check('matchPreferredLanguage: the first preference the menu offers', () => {
  assert.equal(matchPreferredLanguage(['de-DE', 'en-US'], ['tr', 'en', 'de']), 'de')
  assert.equal(matchPreferredLanguage(['ja', 'en-GB', 'de'], ['tr', 'en', 'de']), 'en')
  assert.equal(matchPreferredLanguage(['de-AT'], ['de']), 'de')
  assert.equal(matchPreferredLanguage(['ja', 'ko'], ['tr', 'en']), '')
  assert.equal(matchPreferredLanguage([], ['tr']), '')
  assert.equal(matchPreferredLanguage(['tr'], []), '')
  assert.equal(matchPreferredLanguage('tr', ['tr']), '')
  assert.equal(matchPreferredLanguage([null, 7, 'fr-CA'], ['fr']), 'fr')
  assert.equal(matchPreferredLanguage(['en'], ['tr', null, 'en']), 'en')
})

check('browserLanguages reads navigator.languages, else navigator.language', () => {
  assert.deepEqual(browserLanguages({ languages: ['de-DE', 'en'], language: 'fr' }), ['de-DE', 'en'])
  assert.deepEqual(browserLanguages({ languages: [], language: 'fr-FR' }), ['fr-FR'])
  assert.deepEqual(browserLanguages({ language: 'ru' }), ['ru'])
  assert.deepEqual(browserLanguages({}), [])
  assert.deepEqual(browserLanguages(null), [])
  const hostile = {
    get languages() {
      throw new Error('boom')
    },
  }
  assert.deepEqual(browserLanguages(hostile), [])
})

/* -------------------------------------------------------------- resolving */

const MENU = { available: ['tr', 'en', 'de', 'ar'], fallback: 'tr' }

check('1. an explicit pick wins while the menu offers it', () => {
  assert.equal(
    resolveMenuLanguage({ ...MENU, choice: 'ar', served: 'de', preferred: ['en'] }),
    'ar',
  )
  // A pick the menu no longer offers is passed over.
  assert.equal(
    resolveMenuLanguage({ ...MENU, choice: 'fr', served: 'de', preferred: ['en'] }),
    'de',
  )
  assert.equal(resolveMenuLanguage({ ...MENU, choice: '', served: 'de' }), 'de')
})

check('2. then the language the server resolved the texts in', () => {
  assert.equal(resolveMenuLanguage({ ...MENU, served: 'de', preferred: ['en-US'] }), 'de')
  // Anything the menu does not offer is not a served language.
  assert.equal(resolveMenuLanguage({ ...MENU, served: 'ru', preferred: ['en-US'] }), 'en')
})

check('3. then the browser preference, matched against the menu', () => {
  assert.equal(resolveMenuLanguage({ ...MENU, preferred: ['fr-FR', 'de-DE'] }), 'de')
  assert.equal(resolveMenuLanguage({ ...MENU, preferred: ['ar-SA'] }), 'ar')
})

check('4. then the default language, 5. then tr', () => {
  assert.equal(resolveMenuLanguage({ ...MENU, preferred: ['ja'] }), 'tr')
  assert.equal(
    resolveMenuLanguage({ available: ['en', 'de'], fallback: 'en', preferred: ['ja'] }),
    'en',
  )
  assert.equal(resolveMenuLanguage({ available: ['en', 'de'], preferred: [] }), 'tr')
  assert.equal(resolveMenuLanguage(), 'tr')
  assert.equal(resolveMenuLanguage({}), 'tr')
})

check('without a menu, every interface language is on offer', () => {
  // The loading screen, "menu not found", the tenant directory.
  assert.equal(resolveMenuLanguage({ available: [], preferred: ['ru-RU'] }), 'ru')
  assert.equal(resolveMenuLanguage({ preferred: ['fr-CA', 'en'] }), 'fr')
  assert.equal(resolveMenuLanguage({ available: [], preferred: ['ja'] }), 'tr')
  // A remembered pick counts there too.
  assert.equal(resolveMenuLanguage({ available: [], choice: 'ar', preferred: ['en'] }), 'ar')
  // But never a language the interface is not written in.
  assert.equal(resolveMenuLanguage({ available: [], choice: 'ja', preferred: ['ja'] }), 'tr')
  assert.equal(resolveMenuLanguage({ available: null, served: 'de' }), 'de')
})

/* ------------------------------------------------------ the remembered pick */

check('the storage key is per tenant and case-insensitive', () => {
  assert.equal(LANGUAGE_STORAGE_PREFIX, 'karecik_lang_')
  assert.equal(languageStorageKey('melly-coffee'), 'karecik_lang_melly-coffee')
  assert.equal(languageStorageKey(' Melly-Coffee '), 'karecik_lang_melly-coffee')
  assert.equal(languageStorageKey(''), '')
  assert.equal(languageStorageKey(null), '')
})

check('remember and read a pick', () => {
  const storage = memoryStorage()
  assert.equal(readRememberedLanguage('melly', storage), '')

  rememberLanguage('melly', 'de', storage)
  assert.equal(storage.data.get('karecik_lang_melly'), 'de')
  assert.equal(readRememberedLanguage('melly', storage), 'de')
  assert.equal(readRememberedLanguage('MELLY', storage), 'de')
  // Another tenant's pick is its own.
  assert.equal(readRememberedLanguage('other', storage), '')

  // A later pick replaces the earlier one.
  rememberLanguage('melly', 'ar', storage)
  assert.equal(readRememberedLanguage('melly', storage), 'ar')
})

check('a pick survives a sibling menu that does not offer it', () => {
  // Suadiye offers six languages, Cihangir two. German picked on Suadiye is
  // passed over on Cihangir, and is still there back on Suadiye.
  const storage = memoryStorage({ karecik_lang_melly: 'de' })
  const choice = readRememberedLanguage('melly', storage)
  assert.equal(
    resolveMenuLanguage({ choice, served: 'en', preferred: ['en-US'], available: ['tr', 'en'], fallback: 'tr' }),
    'en',
  )
  assert.equal(readRememberedLanguage('melly', storage), 'de')
  assert.equal(
    resolveMenuLanguage({
      choice: readRememberedLanguage('melly', storage),
      served: 'en',
      preferred: ['en-US'],
      available: ['tr', 'en', 'de', 'ru', 'ar', 'fr'],
      fallback: 'tr',
    }),
    'de',
  )
})

check('only interface languages are remembered or read back', () => {
  const storage = memoryStorage({ karecik_lang_melly: 'xx' })
  assert.equal(readRememberedLanguage('melly', storage), '')
  rememberLanguage('melly', 'ja', storage)
  assert.equal(storage.data.get('karecik_lang_melly'), 'xx')
  rememberLanguage('', 'de', storage)
  assert.equal(storage.data.size, 1)
})

check('a storage that throws, or none at all, is tolerated', () => {
  const blocked = memoryStorage({ karecik_lang_melly: 'de' }, true)
  assert.equal(readRememberedLanguage('melly', blocked), '')
  assert.doesNotThrow(() => rememberLanguage('melly', 'en', blocked))

  assert.equal(readRememberedLanguage('melly', null), '')
  assert.doesNotThrow(() => rememberLanguage('melly', 'en', null))
  // Outside a browser the default storage is simply absent.
  assert.equal(readRememberedLanguage('melly'), '')
})

/* --------------------------------------------------------------- summary */

if (failures.length > 0) {
  console.error(`${failures.length} failed, ${passed} passed\n\n${failures.join('\n\n')}`)
  process.exit(1)
}
console.log(`menuLanguage.test.mjs: all ${passed} checks passed`)
