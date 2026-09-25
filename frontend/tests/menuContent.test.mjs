// Assertions for the pure helpers behind what the customer menu prints: the
// footer's price and VAT sentences (src/lib/format.js), a product's description
// and ingredients (productTexts), the search's text folding (searchFold) and
// SVG image detection (isSvgUrl, all three in src/lib/category.js).
//
//   node frontend/tests/menuContent.test.mjs
//
// Nothing imports this file, so it never reaches the app bundle.

import assert from 'node:assert/strict'

import { comparableText, isSvgUrl, productTexts, searchFold } from '../src/lib/category.js'
import {
  DEFAULT_VAT_NOTE,
  footerPriceSentence,
  footerVatNote,
  formatDay,
  istanbulDay,
  parseIsoDay,
} from '../src/lib/format.js'

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

/* ------------------------------------------------------------- days */

check('parseIsoDay accepts real calendar days only', () => {
  assert.deepEqual(parseIsoDay('2026-08-24'), { year: 2026, month: 8, day: 24 })
  assert.deepEqual(parseIsoDay(' 2028-02-29 '), { year: 2028, month: 2, day: 29 })
  for (const value of ['2026-02-30', '2027-02-29', '2026-8-24', '24.08.2026', '', null, 20260824]) {
    assert.equal(parseIsoDay(value), null, JSON.stringify(value))
  }
})

check('istanbulDay is the calendar day in Europe/Istanbul', () => {
  // 22:30 UTC on 23 August is already the 24th in Istanbul (UTC+3).
  assert.equal(istanbulDay('2026-08-23T22:30:00Z'), '2026-08-24')
  assert.equal(istanbulDay('2026-08-23T20:59:59Z'), '2026-08-23')
  assert.equal(istanbulDay(''), '')
  assert.equal(istanbulDay('not a date'), '')
  assert.equal(istanbulDay(undefined), '')
})

check('formatDay spells the month out in the visitor language', () => {
  assert.equal(formatDay('2026-08-24', 'tr'), '24 Ağustos 2026')
  assert.equal(formatDay('2026-08-24', 'en'), '24 August 2026')
  assert.equal(formatDay('2026-08-24', 'de'), '24. August 2026')
  assert.equal(formatDay('2026-08-24', 'fr'), '24 août 2026')
  assert.match(formatDay('2026-08-24', 'ru'), /^24 августа 2026/)
  // Arabic keeps Latin digits and the Gregorian calendar.
  const arabic = formatDay('2026-08-24', 'ar')
  assert.match(arabic, /24/)
  assert.match(arabic, /2026/)
  // An unknown language is written the English way; a bad day is nothing.
  assert.equal(formatDay('2026-08-24', 'xx'), '24 August 2026')
  assert.equal(formatDay('2026-02-30', 'en'), '')
  // The day never moves to its neighbour, whatever the zone.
  assert.equal(formatDay('2026-01-01', 'en'), '1 January 2026')
  assert.equal(formatDay('2026-12-31', 'en'), '31 December 2026')
})

check('formatDay writes the first of the month as "1er" in French only', () => {
  assert.equal(formatDay('2026-09-01', 'fr'), '1er septembre 2026')
  assert.equal(formatDay('2026-01-01', 'fr'), '1er janvier 2026')
  assert.equal(formatDay('2026-09-02', 'fr'), '2 septembre 2026')
  assert.equal(formatDay('2026-09-11', 'fr'), '11 septembre 2026')
  assert.equal(formatDay('2026-09-21', 'fr'), '21 septembre 2026')
  assert.equal(formatDay('2026-09-01', 'en'), '1 September 2026')
  assert.equal(formatDay('2026-09-01', 'de'), '1. September 2026')
  assert.equal(formatDay('2026-09-01', 'tr'), '1 Eylül 2026')
})

/* --------------------------------------------------- price sentence */

check('the price sentence is built from footer.price_date in every language', () => {
  const footer = { price_date: '2026-08-24', price_note: 'Fiyatlarımız 24.08.2026 tarihinden itibaren geçerlidir.' }
  const business = { show_price_date: true }
  assert.equal(
    footerPriceSentence(business, footer, 'tr'),
    'Fiyatlarımız 24 Ağustos 2026 tarihinden itibaren geçerlidir.',
  )
  assert.equal(footerPriceSentence(business, footer, 'en'), 'Prices valid from 24 August 2026.')
  assert.equal(footerPriceSentence(business, footer, 'de'), 'Preise gültig ab 24. August 2026.')
  assert.equal(footerPriceSentence(business, footer, 'fr'), 'Prix en vigueur à partir du 24 août 2026.')
  // "2026 г." must not be followed by a second full stop.
  const russian = footerPriceSentence(business, footer, 'ru')
  assert.match(russian, /^Цены действительны с 24 августа 2026/)
  assert.ok(!russian.endsWith('..'), russian)
  assert.match(footerPriceSentence(business, footer, 'ar'), /^الأسعار سارية اعتبارًا من .*24.*2026/)
  assert.equal(
    footerPriceSentence(business, { price_date: '2026-09-01' }, 'fr'),
    'Prix en vigueur à partir du 1er septembre 2026.',
  )
})

check('the price sentence: switched off, hidden, or from an older server', () => {
  const footer = { price_date: '2026-08-24', price_note: 'Fiyatlarımız 24.08.2026 tarihinden itibaren geçerlidir.' }
  // The toggle wins over a saved payload that still carries a date.
  assert.equal(footerPriceSentence({ show_price_date: false }, footer, 'en'), '')
  // "" is how the server says the date is hidden; its Turkish note is not used.
  assert.equal(footerPriceSentence({}, { price_date: '', price_note: 'x' }, 'en'), '')
  // A server without price_date: its own sentence is the last resort.
  assert.equal(
    footerPriceSentence({}, { price_note: '  Fiyatlarımız 24.08.2026 tarihinden itibaren geçerlidir. ' }, 'en'),
    'Fiyatlarımız 24.08.2026 tarihinden itibaren geçerlidir.',
  )
  assert.equal(footerPriceSentence({}, null, 'en'), '')
  assert.equal(footerPriceSentence(undefined, undefined, 'en'), '')
  // A malformed date is no date.
  assert.equal(footerPriceSentence({}, { price_date: '24.08.2026' }, 'en'), '')
})

check('the price sentence in the preview: toggle on over a payload saved without it', () => {
  const business = { show_price_date: true, price_updated_at: '2026-08-23T22:30:00Z' }
  assert.equal(
    footerPriceSentence(business, { price_date: '', price_note: '' }, 'en'),
    'Prices valid from 24 August 2026.',
  )
})

/* -------------------------------------------------------- VAT note */

check('the VAT note: the owner text as typed, or the localized default', () => {
  assert.equal(DEFAULT_VAT_NOTE, 'Fiyatlarımıza KDV dahildir.')
  const own = { show_vat_note: true, vat_note_text: '  Fiyatlara KDV ve servis dahildir. ' }
  assert.equal(footerVatNote(own, {}, 'en'), 'Fiyatlara KDV ve servis dahildir.')

  const blank = { show_vat_note: true, vat_note_text: '   ' }
  assert.equal(footerVatNote(blank, { vat_note: DEFAULT_VAT_NOTE }, 'tr'), 'Fiyatlarımıza KDV dahildir.')
  assert.equal(footerVatNote(blank, {}, 'en'), 'All prices include VAT.')
  assert.equal(footerVatNote(blank, {}, 'de'), 'Alle Preise inkl. MwSt.')
  assert.equal(footerVatNote(blank, {}, 'ar'), 'جميع الأسعار شاملة ضريبة القيمة المضافة.')

  assert.equal(footerVatNote({ show_vat_note: false, vat_note_text: 'x' }, { vat_note: 'y' }, 'en'), '')
})

check('the VAT note: the stock Turkish text is localized like a blank one', () => {
  // The column default of menus.vat_note_text: what every menu carries whose
  // owner never touched the field, and what the public payload sends.
  const stock = { show_vat_note: true, vat_note_text: DEFAULT_VAT_NOTE }
  const footer = { vat_note: DEFAULT_VAT_NOTE }
  assert.equal(footerVatNote(stock, footer, 'tr'), 'Fiyatlarımıza KDV dahildir.')
  assert.equal(footerVatNote(stock, footer, 'en'), 'All prices include VAT.')
  assert.equal(footerVatNote(stock, footer, 'de'), 'Alle Preise inkl. MwSt.')
  assert.equal(footerVatNote(stock, footer, 'ru'), 'Все цены указаны с учётом НДС.')
  assert.equal(footerVatNote(stock, footer, 'ar'), 'جميع الأسعار شاملة ضريبة القيمة المضافة.')
  assert.equal(footerVatNote(stock, footer, 'fr'), 'Tous nos prix sont TTC.')

  // Case, spacing and the full stop do not make it the owner's own words.
  for (const text of ['  fiyatlarımıza kdv dahildir ', 'FİYATLARIMIZA  KDV DAHİLDİR.', 'Fiyatlarımıza KDV dahildir']) {
    assert.equal(footerVatNote({ show_vat_note: true, vat_note_text: text }, {}, 'en'), 'All prices include VAT.', text)
  }
  // Anything else the owner typed is still shown as typed.
  assert.equal(
    footerVatNote({ show_vat_note: true, vat_note_text: 'Fiyatlarımıza KDV ve servis dahildir.' }, footer, 'en'),
    'Fiyatlarımıza KDV ve servis dahildir.',
  )
})

check('the VAT note from the footer alone', () => {
  // A payload whose business lacks the fields: footer.vat_note is all there is,
  // and the server's Turkish default is swapped for the localized one.
  assert.equal(footerVatNote({}, { vat_note: DEFAULT_VAT_NOTE }, 'fr'), 'Tous nos prix sont TTC.')
  assert.equal(footerVatNote({}, { vat_note: ' Servis dahil. ' }, 'fr'), 'Servis dahil.')
  assert.equal(footerVatNote({}, { vat_note: '' }, 'fr'), '')
  assert.equal(footerVatNote(null, null, 'fr'), '')
  // Toggle on without a text field: the server's rendering, localized when it is the default.
  assert.equal(footerVatNote({ show_vat_note: true }, { vat_note: DEFAULT_VAT_NOTE }, 'en'), 'All prices include VAT.')
  assert.equal(footerVatNote({ show_vat_note: true }, {}, 'en'), 'All prices include VAT.')
})

/* ------------------------------------------------- product texts */

check('comparableText ignores case, spacing and a closing full stop', () => {
  assert.equal(comparableText(' Espresso,  SÜT.  '), 'espresso, süt')
  assert.equal(comparableText('İÇİNDEKİLER'), 'içindekiler')
  assert.equal(comparableText('Kahve!!'), 'kahve')
  assert.equal(comparableText('قهوة، حليب،'), 'قهوة، حليب')
  assert.equal(comparableText(null), '')
})

check('productTexts: the card subtitle is the description, else the ingredients', () => {
  assert.equal(productTexts({ description: 'Günlük kavrulan çekirdek.', ingredients: 'espresso, süt' }).subtitle, 'Günlük kavrulan çekirdek.')
  assert.equal(productTexts({ description: '  ', ingredients: ' espresso, süt ' }).subtitle, 'espresso, süt')
  assert.equal(productTexts({ ingredients: 'espresso' }).subtitle, 'espresso')
  assert.equal(productTexts({}).subtitle, '')
})

check('productTexts: a description that repeats the ingredients is dropped', () => {
  assert.deepEqual(productTexts({ description: 'Espresso, süt.', ingredients: 'espresso,  Süt' }), {
    subtitle: 'Espresso, süt.',
    description: '',
    ingredients: 'espresso,  Süt',
  })
  // Different texts both stay.
  assert.deepEqual(
    productTexts({ description: 'Sıcak servis edilir.', ingredients: 'espresso, süt' }),
    { subtitle: 'Sıcak servis edilir.', description: 'Sıcak servis edilir.', ingredients: 'espresso, süt' },
  )
  // Word order matters: that is a different text.
  assert.equal(productTexts({ description: 'süt, espresso', ingredients: 'espresso, süt' }).description, 'süt, espresso')
  // Only a description: it stays.
  assert.equal(productTexts({ description: 'Tek başına.' }).description, 'Tek başına.')
})

check('productTexts never throws on odd records', () => {
  for (const value of [null, undefined, 42, 'x', [], { description: 7, ingredients: {} }]) {
    assert.deepEqual(productTexts(value), { subtitle: '', description: '', ingredients: '' })
  }
})

/* ---------------------------------------------------------- search */

check('searchFold makes I, İ, ı and i one letter, whatever the case', () => {
  assert.equal(searchFold('Ice Latte'), 'ice latte')
  assert.equal(searchFold('ICE LATTE'), 'ice latte')
  assert.equal(searchFold('İÇECEK'), 'içecek')
  assert.equal(searchFold('Irmak'), 'irmak')
  assert.equal(searchFold('ıhlamur'), 'ihlamur')
  // What the menu's search does: fold both sides, then look for the needle.
  assert.ok(searchFold('Ice Americano').includes(searchFold('ice')))
  assert.ok(searchFold('Ice Americano').includes(searchFold('ICE')))
  assert.ok(searchFold('Ihlamur').includes(searchFold('ıhla')))
  assert.ok(searchFold('Iced Tea').includes(searchFold('İced')))
  // Other scripts lowercase as usual.
  assert.equal(searchFold('Молоко'), 'молоко')
  assert.equal(searchFold('Crème Brûlée'), 'crème brûlée')
  assert.equal(searchFold('قهوة'), 'قهوة')
  // Numbers are their digits; anything else is nothing.
  assert.equal(searchFold(42), '42')
  for (const value of [null, undefined, {}, [], Number.NaN]) {
    assert.equal(searchFold(value), '', String(value))
  }
})

/* ------------------------------------------------------------- SVG */

check('isSvgUrl recognises SVG files and data URLs', () => {
  assert.ok(isSvgUrl('/uploads/1726000000-ab12cd34.svg'))
  assert.ok(isSvgUrl('https://cdn.example.com/logo.SVG?v=2'))
  assert.ok(isSvgUrl('/uploads/logo.svg#mark'))
  assert.ok(isSvgUrl('/uploads/logo.svgz'))
  assert.ok(isSvgUrl(' data:image/svg+xml;base64,PHN2Zz4= '))
  assert.ok(isSvgUrl('data:image/svg+xml,%3Csvg%3E'))

  assert.ok(!isSvgUrl('/uploads/photo.png'))
  assert.ok(!isSvgUrl('/svg/photo.jpg'))
  assert.ok(!isSvgUrl('/uploads/photo.png?format=svg'))
  assert.ok(!isSvgUrl('data:image/png;base64,iVBOR'))
  assert.ok(!isSvgUrl(''))
  assert.ok(!isSvgUrl(null))
})

/* --------------------------------------------------------------- summary */

if (failures.length > 0) {
  console.error(`${failures.length} failed, ${passed} passed\n\n${failures.join('\n\n')}`)
  process.exit(1)
}
console.log(`menuContent.test.mjs: all ${passed} checks passed`)
