// Assertions for the customer menu's dictionaries in src/locales/index.js.
//
//   node frontend/tests/locales.test.mjs
//
// The six languages must stay in step: a key one of them lacks falls back to
// English on screen, which is exactly the half-translated menu the dictionary
// exists to prevent. So every language carries exactly the keys of 'tr', none of
// them empty, and every allergen has a label in all six.
//
// It also reads the customer-facing components and fails on a t('key') call
// whose key is in no dictionary — the one mistake the fallback would otherwise
// hide by printing the bare key.
//
// Nothing imports this file, so it never reaches the app bundle.

import assert from 'node:assert/strict'
import { readdirSync, readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

import {
  ALLERGENS,
  allergenLabel,
  findLanguage,
  isRtl,
  LANGUAGE_CODES,
  LANGUAGES,
  languageDir,
  STRINGS,
  t,
  textDir,
} from '../src/locales/index.js'

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

const SIX = ['tr', 'en', 'de', 'ru', 'ar', 'fr']
const trKeys = Object.keys(STRINGS.tr).sort()

/* ------------------------------------------------------------ languages */

check('the menu speaks exactly six languages, in picker order', () => {
  assert.deepEqual(LANGUAGE_CODES, SIX)
  assert.deepEqual(
    LANGUAGES.map((language) => language.code),
    SIX,
  )
  assert.deepEqual(Object.keys(STRINGS).sort(), [...SIX].sort())
  for (const language of LANGUAGES) {
    assert.equal(language.short, language.code.toUpperCase())
    assert.ok(language.label.trim(), language.code)
  }
  assert.equal(findLanguage('ar').label, 'العربية')
  assert.equal(findLanguage('xx').short, 'XX')
})

check('only Arabic is right to left', () => {
  for (const code of SIX) {
    assert.equal(isRtl(code), code === 'ar', code)
    assert.equal(languageDir(code), code === 'ar' ? 'rtl' : 'ltr', code)
  }
  assert.equal(languageDir(undefined), 'ltr')
})

check('textDir pins Arabic text right to left in the Arabic menu only', () => {
  // Arabic translations that open with a loanword kept in Latin letters: a
  // first-letter guess would lay them out left to right.
  for (const text of ['Frozen بالفراولة', 'Cheesecake بمربى الحليب', 'Latte مثلج', 'قهوة تركية']) {
    assert.equal(textDir(text, 'ar'), 'rtl', text)
  }
  // No right-to-left letter: an untranslated fallback keeps its own direction.
  for (const text of ['Latte', 'Espresso, süt.', 'Melly Beef', '145', '']) {
    assert.equal(textDir(text, 'ar'), 'auto', text)
  }
  // Every other menu leaves every text to its own script.
  for (const code of SIX.filter((language) => language !== 'ar')) {
    assert.equal(textDir('Frozen بالفراولة', code), 'auto', code)
    assert.equal(textDir('Latte', code), 'auto', code)
  }
  for (const value of [null, undefined, 42, {}]) {
    assert.equal(textDir(value, 'ar'), 'auto', String(value))
  }
})

/* ----------------------------------------------------------- dictionaries */

check('every language has exactly the keys of tr', () => {
  for (const code of SIX) {
    const keys = Object.keys(STRINGS[code]).sort()
    const missing = trKeys.filter((key) => !keys.includes(key))
    const extra = keys.filter((key) => !trKeys.includes(key))
    assert.deepEqual({ code, missing, extra }, { code, missing: [], extra: [] })
  }
})

check('no string is empty or only white space', () => {
  for (const code of SIX) {
    for (const [key, value] of Object.entries(STRINGS[code])) {
      assert.equal(typeof value, 'string', `${code}.${key}`)
      assert.ok(value.trim() !== '', `${code}.${key} is empty`)
      assert.equal(value, value.trim(), `${code}.${key} has stray white space`)
    }
  }
})

check('placeholders match across languages', () => {
  const placeholders = (text) => (text.match(/\{\w+\}/g) || []).sort()
  for (const key of trKeys) {
    const expected = placeholders(STRINGS.tr[key])
    for (const code of SIX) {
      assert.deepEqual(placeholders(STRINGS[code][key]), expected, `${code}.${key}`)
    }
  }
  assert.deepEqual(placeholders(STRINGS.tr.priceValidFrom), ['{date}'])
})

check('translated, not copied: the non-Turkish strings are not the Turkish ones', () => {
  // Keys that are the same word in several languages on purpose.
  // ("Telefon" is German as well as Turkish.)
  const shared = new Set(['kcal', 'wifiChip', 'instagram', 'phone'])
  for (const code of SIX.filter((language) => language !== 'tr')) {
    for (const key of trKeys) {
      if (shared.has(key)) continue
      assert.notEqual(STRINGS[code][key], STRINGS.tr[key], `${code}.${key} is still Turkish`)
    }
  }
})

check('brand words are never translated', () => {
  for (const code of SIX) {
    assert.ok(STRINGS[code].poweredBy.includes('Karecik'), `${code}.poweredBy`)
    assert.ok(STRINGS[code].openInstagram.includes('Instagram'), `${code}.openInstagram`)
    assert.equal(STRINGS[code].instagram, 'Instagram', code)
    // Yerli Üretim is the name of an official certification mark.
    assert.ok(STRINGS[code].yerliUretimTitle.includes('Yerli Üretim'), `${code}.yerliUretimTitle`)
  }
  assert.equal(t('poweredBy', 'en'), 'Powered by Karecik')
})

/* -------------------------------------------------------------------- t() */

check('t: the language, then English, then Turkish, then the key', () => {
  assert.equal(t('close', 'tr'), 'Kapat')
  assert.equal(t('close', 'de'), 'Schließen')
  assert.equal(t('close', 'ar'), 'إغلاق')
  // An unknown language reads English.
  assert.equal(t('close', 'xx'), 'Close')
  assert.equal(t('close', undefined), 'Kapat')
  assert.equal(t('close', '__proto__'), 'Close')
  // An unknown key reads as itself rather than as nothing.
  assert.equal(t('no-such-key', 'fr'), 'no-such-key')
})

check('t fills placeholders and leaves unknown ones visible', () => {
  assert.equal(t('priceValidFrom', 'en', { date: '24 August 2026' }), 'Prices valid from 24 August 2026.')
  assert.equal(
    t('priceValidFrom', 'tr', { date: '24 Ağustos 2026' }),
    'Fiyatlarımız 24 Ağustos 2026 tarihinden itibaren geçerlidir.',
  )
  assert.equal(t('priceValidFrom', 'en', {}), 'Prices valid from {date}.')
  assert.equal(t('priceValidFrom', 'en'), 'Prices valid from {date}.')
})

/* -------------------------------------------------------------- allergens */

check('every allergen has a label in all six languages', () => {
  const codes = new Set()
  for (const allergen of ALLERGENS) {
    assert.ok(!codes.has(allergen.code), `duplicate allergen ${allergen.code}`)
    codes.add(allergen.code)
    assert.ok(allergen.emoji, allergen.code)
    for (const code of SIX) {
      assert.equal(typeof allergen[code], 'string', `${allergen.code}.${code}`)
      assert.ok(allergen[code].trim() !== '', `${allergen.code}.${code} is empty`)
      assert.equal(allergenLabel(allergen.code, code), allergen[code])
    }
  }
  assert.equal(ALLERGENS.length, 17)
})

check('allergenLabel: English for an unknown language, the code for an unknown allergen', () => {
  assert.equal(allergenLabel('sut', 'tr'), 'Süt')
  assert.equal(allergenLabel('sut', 'ru'), 'Молоко')
  assert.equal(allergenLabel('sut', 'ar'), 'حليب')
  assert.equal(allergenLabel('sut', 'fr'), 'Lait')
  assert.equal(allergenLabel('sut', 'xx'), 'Milk')
  assert.equal(allergenLabel('gluten'), 'Gluten')
  assert.equal(allergenLabel('unknown', 'en'), 'unknown')
  // Vegetarian and vegan must not read the same anywhere.
  for (const code of SIX.filter((language) => language !== 'en' && language !== 'tr')) {
    assert.notEqual(allergenLabel('vegan', code), allergenLabel('vejetaryen', code), code)
  }
})

/* ------------------------------------------------ keys the components use */

check('every t() key used by the customer menu exists', () => {
  const here = dirname(fileURLToPath(import.meta.url))
  const sources = [
    ...['components/menu', 'pages/menu'].flatMap((folder) =>
      readdirSync(join(here, '..', 'src', folder))
        .filter((name) => name.endsWith('.jsx') || name.endsWith('.js'))
        .map((name) => join(here, '..', 'src', folder, name)),
    ),
    join(here, '..', 'src', 'lib', 'category.js'),
    join(here, '..', 'src', 'lib', 'format.js'),
  ]

  const used = new Set()
  for (const file of sources) {
    const text = readFileSync(file, 'utf8')
    for (const match of text.matchAll(/\bt\(\s*'([A-Za-z0-9_]+)'/g)) used.add(match[1])
    // t(notFound ? 'notFound' : 'loadFailed', ...) — both branches count.
    for (const match of text.matchAll(/\bt\(\s*\w+\s*\?\s*'(\w+)'\s*:\s*'(\w+)'/g)) {
      used.add(match[1])
      used.add(match[2])
    }
  }

  assert.ok(used.size > 20, `only ${used.size} keys found - is the scan broken?`)
  const unknown = [...used].filter((key) => !trKeys.includes(key))
  assert.deepEqual(unknown, [])
})

/* --------------------------------------------------------------- summary */

if (failures.length > 0) {
  console.error(`${failures.length} failed, ${passed} passed\n\n${failures.join('\n\n')}`)
  process.exit(1)
}
console.log(`locales.test.mjs: all ${passed} checks passed`)
