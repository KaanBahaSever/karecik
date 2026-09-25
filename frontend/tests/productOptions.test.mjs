// Assertions for the per-language option names of src/lib/productOptions.js,
// which ProductModal.jsx edits and saves, and for textDir() of
// src/locales/index.js, which ProductDetailModal.jsx puts on every option name.
//
//   node frontend/tests/productOptions.test.mjs
//
// Nothing imports this file, so it never reaches the app bundle.

import assert from 'node:assert/strict'

import {
  buildOptionsPayload,
  hasAnyOptionName,
  OPTION_NAME_MESSAGES,
  optionNamePayload,
  optionNamesOf,
} from '../src/lib/productOptions.js'
import { textDir } from '../src/locales/index.js'

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

const LANGUAGES = ['tr', 'en', 'de', 'ar']

/** A dialog row as ProductModal keeps it. */
function group(uid, names, items, extra = {}) {
  return { uid, names, type: 'single', required: false, items, ...extra }
}
function item(uid, names, price = '0,00') {
  return { uid, names, price }
}

/* ------------------------------------------------------------ loading */

check('a stored option opens with a name per menu language', () => {
  const stored = {
    name: 'Süt Tercihi',
    translations: {
      tr: { name: 'Süt Tercihi' },
      en: { name: 'Milk' },
      fr: { name: 'Lait' }, // not one of this menu's languages
    },
  }
  assert.deepEqual(optionNamesOf(stored, LANGUAGES, 'tr'), {
    tr: 'Süt Tercihi',
    en: 'Milk',
    de: '',
    ar: '',
  })
})

check('an option saved before translations existed opens on its name', () => {
  assert.deepEqual(optionNamesOf({ name: 'Boy', price: 10 }, LANGUAGES, 'tr'), {
    tr: 'Boy',
    en: '',
    de: '',
    ar: '',
  })
  assert.deepEqual(optionNamesOf({ name: 'Boy', translations: null }, ['tr'], 'tr'), { tr: 'Boy' })
  assert.deepEqual(optionNamesOf(null, ['tr', 'en'], 'tr'), { tr: '', en: '' })
})

check('after a default-language switch the new default tab shows its own text', () => {
  // Saved while Turkish was the default; the menu then switched to English.
  const stored = {
    name: 'Süt Tercihi',
    translations: { tr: { name: 'Süt Tercihi' }, en: { name: 'Milk' } },
  }
  assert.deepEqual(optionNamesOf(stored, ['en', 'tr'], 'en'), { en: 'Milk', tr: 'Süt Tercihi' })
})

/* ------------------------------------------------------------- saving */

check('the primary language becomes name, the others translations', () => {
  assert.deepEqual(
    optionNamePayload({ tr: ' Boy ', en: ' Size ', de: '   ', ar: 'الحجم' }, LANGUAGES, 'tr'),
    { name: 'Boy', translations: { en: { name: 'Size' }, ar: { name: 'الحجم' } } },
  )
  // Nothing but the primary language: no translations key at all.
  assert.deepEqual(optionNamePayload({ tr: 'Boy', en: '' }, LANGUAGES, 'tr'), { name: 'Boy' })
  // A language the menu no longer offers is not sent.
  assert.deepEqual(optionNamePayload({ tr: 'Boy', fr: 'Taille' }, LANGUAGES, 'tr'), { name: 'Boy' })
})

check('hasAnyOptionName ignores blank languages', () => {
  assert.equal(hasAnyOptionName({ tr: ' ', en: '' }), false)
  assert.equal(hasAnyOptionName({}), false)
  assert.equal(hasAnyOptionName(undefined), false)
  assert.equal(hasAnyOptionName({ tr: '', de: 'Größe' }), true)
})

check('buildOptionsPayload sends names per language with parsed surcharges', () => {
  const { options, problem } = buildOptionsPayload(
    [
      group(
        'g1',
        { tr: 'Süt Tercihi', en: 'Milk', de: '', ar: 'الحليب' },
        [
          item('i1', { tr: 'Yulaf Sütü', en: 'Oat milk' }, '25,00'),
          item('i2', { tr: 'Laktozsuz' }, '1.250,5'),
        ],
        { type: 'multiple', required: true },
      ),
    ],
    LANGUAGES,
    'tr',
  )
  assert.equal(problem, null)
  assert.deepEqual(options, [
    {
      name: 'Süt Tercihi',
      translations: { en: { name: 'Milk' }, ar: { name: 'الحليب' } },
      type: 'multiple',
      required: true,
      items: [
        { name: 'Yulaf Sütü', translations: { en: { name: 'Oat milk' } }, price: 25 },
        { name: 'Laktozsuz', price: 1250.5 },
      ],
    },
  ])
})

check('unfinished rows are dropped, as before translations', () => {
  const { options, problem } = buildOptionsPayload(
    [
      // No name in any language: dropped with its items.
      group('g1', { tr: '', en: ' ' }, [item('i1', { tr: 'Büyük' }, '10')]),
      // Named, but nobody can answer it: dropped.
      group('g2', { tr: 'Boy' }, [item('i2', { tr: '', en: '' }, '5')]),
      // Kept, minus its unnamed item.
      group('g3', { tr: 'Şurup' }, [item('i3', {}, '3'), item('i4', { tr: 'Vanilya' }, '5')]),
    ],
    LANGUAGES,
    'tr',
  )
  assert.equal(problem, null)
  assert.deepEqual(options, [
    {
      name: 'Şurup',
      type: 'single',
      required: false,
      items: [{ name: 'Vanilya', price: 5 }],
    },
  ])
})

check('a row named only in another language is refused, not dropped', () => {
  const groupProblem = buildOptionsPayload(
    [group('g1', { tr: '', en: 'Size' }, [item('i1', { tr: 'Büyük' })])],
    LANGUAGES,
    'tr',
  )
  assert.deepEqual(groupProblem, {
    options: [],
    problem: { uid: 'g1', message: OPTION_NAME_MESSAGES.groupMissingPrimary },
  })

  const itemProblem = buildOptionsPayload(
    [group('g1', { tr: 'Boy' }, [item('i1', { tr: 'Küçük' }), item('i2', { tr: ' ', de: 'Groß' })])],
    LANGUAGES,
    'tr',
  )
  assert.deepEqual(itemProblem, {
    options: [],
    problem: { uid: 'i2', message: OPTION_NAME_MESSAGES.itemMissingPrimary },
  })
})

check('loading and saving again sends back what was stored', () => {
  const stored = [
    {
      name: 'Boy',
      translations: { tr: { name: 'Boy' }, en: { name: 'Size' } },
      type: 'single',
      required: true,
      items: [
        { name: 'Küçük', price: 0 },
        { name: 'Büyük', translations: { tr: { name: 'Büyük' }, ar: { name: 'كبير' } }, price: 12.5 },
      ],
    },
  ]
  const rows = stored.map((entry, gi) =>
    group(
      `g${gi}`,
      optionNamesOf(entry, LANGUAGES, 'tr'),
      entry.items.map((option, ii) => item(`i${ii}`, optionNamesOf(option, LANGUAGES, 'tr'), option.price)),
      { type: entry.type, required: entry.required },
    ),
  )
  const { options, problem } = buildOptionsPayload(rows, LANGUAGES, 'tr')
  assert.equal(problem, null)
  // The default language is not sent as a translation: the backend stores its
  // copy of the name itself.
  assert.deepEqual(options, [
    {
      name: 'Boy',
      translations: { en: { name: 'Size' } },
      type: 'single',
      required: true,
      items: [
        { name: 'Küçük', price: 0 },
        { name: 'Büyük', translations: { ar: { name: 'كبير' } }, price: 12.5 },
      ],
    },
  ])
})

/* -------------------------------------------------------------- textDir */

check('textDir: Arabic reads right to left in the Arabic menu only', () => {
  assert.equal(textDir('حليب كامل الدسم', 'ar'), 'rtl')
  // A translation that opens with a Latin loanword still reads right to left.
  assert.equal(textDir('Frozen بالفراولة', 'ar'), 'rtl')
  // An untranslated Turkish fallback in the Arabic menu keeps its own direction.
  assert.equal(textDir('Yulaf Sütü', 'ar'), 'auto')
  assert.equal(textDir('حليب', 'tr'), 'auto')
  assert.equal(textDir('Oat milk', 'en'), 'auto')
  assert.equal(textDir(undefined, 'ar'), 'auto')
  assert.equal(textDir(42, 'ar'), 'auto')
})

/* --------------------------------------------------------------- summary */

if (failures.length > 0) {
  console.error(`${failures.length} failed, ${passed} passed\n\n${failures.join('\n\n')}`)
  process.exit(1)
}
console.log(`productOptions.test.mjs: all ${passed} checks passed`)
