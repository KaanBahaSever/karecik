// Assertions for the analytics and change-history helpers of the owner panel:
// src/lib/analyticsFormat.js and src/lib/auditFormat.js.
//
//   node frontend/tests/dashboardAnalytics.test.mjs
//
// Nothing imports this file, so it never reaches the app bundle.

import assert from 'node:assert/strict'

import {
  addDays,
  checkCustomRange,
  DATE_RANGE_PRESETS,
  dayInZone,
  daysInRange,
  eventTypeLabel,
  EVENT_TYPES,
  formatCount,
  formatDateTime,
  formatDay,
  formatIp,
  formatPort,
  formatRange,
  ipSourceInfo,
  isValidDay,
  labelInterval,
  listDays,
  matchingPreset,
  MAX_RANGE_DAYS,
  niceAxis,
  normalizeRange,
  pageInfo,
  presetRange,
  shortUserAgent,
} from '../src/lib/analyticsFormat.js'
import {
  AUDIT_ACTIONS,
  AUDIT_ENTITY_TYPES,
  auditActionLabel,
  auditActionTone,
  auditChangeRows,
  auditEntityLabel,
  auditFieldLabel,
  auditNameLookup,
  EMPTY_VALUE,
  formatAuditValue,
  truncateText,
} from '../src/lib/auditFormat.js'

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

/* ----------------------------------------------------------------- calendar */

check('days are counted in Europe/Istanbul, not in the zone of the machine', () => {
  // 22:30 UTC is already the next day in Istanbul (UTC+3).
  assert.equal(dayInZone(new Date('2026-09-24T22:30:00Z')), '2026-09-25')
  assert.equal(dayInZone(new Date('2026-09-24T20:59:59Z')), '2026-09-24')
  assert.equal(dayInZone('2026-01-01T00:00:00+03:00'), '2026-01-01')
  assert.equal(dayInZone('not a date'), '')
})

check('only real calendar days are days', () => {
  assert.equal(isValidDay('2026-09-25'), true)
  assert.equal(isValidDay('2028-02-29'), true)
  assert.equal(isValidDay('2026-02-29'), false)
  assert.equal(isValidDay('2026-13-01'), false)
  assert.equal(isValidDay('2026-9-25'), false)
  assert.equal(isValidDay(''), false)
  assert.equal(isValidDay(null), false)
  assert.equal(isValidDay(20260925), false)
})

check('day arithmetic crosses months, years and leap days', () => {
  assert.equal(addDays('2026-09-25', -6), '2026-09-19')
  assert.equal(addDays('2026-03-01', -1), '2026-02-28')
  assert.equal(addDays('2028-03-01', -1), '2028-02-29')
  assert.equal(addDays('2026-12-31', 1), '2027-01-01')
  assert.equal(addDays('2026-10-25', 1), '2026-10-26') // no DST drift: the arithmetic is in UTC
  assert.equal(addDays('bad', 1), '')
  assert.equal(daysInRange('2026-09-01', '2026-09-30'), 30)
  assert.equal(daysInRange('2026-09-25', '2026-09-25'), 1)
  assert.equal(daysInRange('2026-09-26', '2026-09-25'), 0)
  assert.deepEqual(listDays('2026-12-30', '2027-01-02'), [
    '2026-12-30',
    '2026-12-31',
    '2027-01-01',
    '2027-01-02',
  ])
  assert.equal(listDays('1900-01-01', '2100-01-01', 10).length, 10)
})

check('presets end today and count today in', () => {
  const now = new Date('2026-09-25T09:00:00Z')
  assert.deepEqual(presetRange('today', now), { from: '2026-09-25', to: '2026-09-25' })
  assert.deepEqual(presetRange('7d', now), { from: '2026-09-19', to: '2026-09-25' })
  assert.deepEqual(presetRange('30d', now), { from: '2026-08-27', to: '2026-09-25' })
  assert.deepEqual(presetRange('90d', now), { from: '2026-06-28', to: '2026-09-25' })
  assert.deepEqual(presetRange('nope', now), presetRange('30d', now))
  assert.deepEqual(
    DATE_RANGE_PRESETS.map((preset) => preset.label),
    ['Bugün', 'Son 7 gün', 'Son 30 gün', 'Son 90 gün'],
  )
  // Late evening UTC is already tomorrow in Istanbul, and "today" follows it.
  assert.deepEqual(presetRange('today', new Date('2026-09-25T21:30:00Z')), {
    from: '2026-09-26',
    to: '2026-09-26',
  })
})

check('a custom range is validated and a reversed one swapped', () => {
  assert.deepEqual(normalizeRange('2026-09-01', '2026-09-10'), { from: '2026-09-01', to: '2026-09-10' })
  assert.deepEqual(normalizeRange('2026-09-10', '2026-09-01'), { from: '2026-09-01', to: '2026-09-10' })
  assert.equal(normalizeRange('', '2026-09-01'), null)
  assert.equal(normalizeRange('2026-02-30', '2026-03-01'), null)

  const now = new Date('2026-09-25T09:00:00Z')
  assert.equal(matchingPreset({ from: '2026-09-19', to: '2026-09-25' }, now), '7d')
  assert.equal(matchingPreset({ from: '2026-09-18', to: '2026-09-25' }, now), 'custom')
  assert.equal(matchingPreset(null, now), 'custom')
})

check('a custom range the summary would refuse is stopped with its reason', () => {
  // maxAnalyticsDays of handlers/analytics.go.
  assert.equal(MAX_RANGE_DAYS, 366)

  assert.deepEqual(checkCustomRange('2026-09-10', '2026-09-01'), {
    range: { from: '2026-09-01', to: '2026-09-10' },
    problem: '',
  })
  // 366 days, both ends counted, is the longest range there is...
  assert.deepEqual(checkCustomRange('2025-09-25', '2026-09-25').range, {
    from: '2025-09-25',
    to: '2026-09-25',
  })
  // ...and one more day is refused, reversed or not, with the API's own words.
  for (const [from, to] of [
    ['2025-09-24', '2026-09-25'],
    ['2026-09-25', '2025-09-24'],
    ['2025-01-01', '2026-09-25'],
  ]) {
    assert.deepEqual(checkCustomRange(from, to), {
      range: null,
      problem: 'Tarih aralığı en fazla 366 gün olabilir.',
    })
  }
  assert.deepEqual(checkCustomRange('', '2026-09-25'), {
    range: null,
    problem: 'Başlangıç ve bitiş tarihini seçin.',
  })
})

check('days and timestamps print in Turkish, in Istanbul time', () => {
  assert.equal(formatDay('2026-09-25'), '25 Eyl')
  assert.equal(formatDay('2026-09-25', 'long'), '25 Eylül 2026, Cuma')
  assert.equal(formatDay('2026-02-01', 'numeric'), '01.02.2026')
  assert.equal(formatDay('2026-02-30'), '')
  assert.equal(formatRange('2026-09-01', '2026-09-30'), '01.09.2026 – 30.09.2026')
  assert.equal(formatRange('2026-09-25', '2026-09-25'), '25.09.2026')
  assert.equal(formatRange('x', '2026-09-25'), '')

  assert.equal(formatDateTime('2026-09-25T11:03:09Z'), '25.09.2026 14:03:09')
  assert.equal(formatDateTime('2026-09-24T21:00:00Z'), '25.09.2026 00:00:00')
  assert.equal(formatDateTime(null), '—')
  assert.equal(formatDateTime('garbage'), '—')
})

/* ------------------------------------------------------------------ numbers */

check('counts use the Turkish thousands separator', () => {
  assert.equal(formatCount(0), '0')
  assert.equal(formatCount(999), '999')
  assert.equal(formatCount(1234), '1.234')
  assert.equal(formatCount(1234567), '1.234.567')
  assert.equal(formatCount('42'), '42')
  assert.equal(formatCount(undefined), '0')
  assert.equal(formatCount(NaN), '0')
})

check('the count axis ends on a clean number and never steps below 1', () => {
  assert.deepEqual(niceAxis(0), { max: 4, step: 1 })
  assert.deepEqual(niceAxis(-5), { max: 4, step: 1 })
  assert.deepEqual(niceAxis(3), { max: 3, step: 1 })
  assert.deepEqual(niceAxis(7), { max: 8, step: 2 })
  assert.deepEqual(niceAxis(10), { max: 10, step: 5 })
  assert.deepEqual(niceAxis(37), { max: 40, step: 10 })
  assert.deepEqual(niceAxis(1234), { max: 1500, step: 500 })
  for (const top of [1, 2, 9, 11, 99, 101, 999, 4321]) {
    const axis = niceAxis(top)
    assert.ok(axis.max >= top, `max ${axis.max} < ${top}`)
    assert.ok(axis.step >= 1)
    assert.equal(axis.max % axis.step, 0)
    assert.ok(axis.max / axis.step <= 5, `too many ticks for ${top}`)
  }
})

check('axis labels thin out as the days outgrow the width', () => {
  assert.equal(labelInterval(7, 700), 1)
  assert.equal(labelInterval(30, 300), 6)
  assert.equal(labelInterval(90, 700), 8)
  assert.equal(labelInterval(1, 700), 1)
  assert.equal(labelInterval(30, 0), 1)
})

check('pagination reports the visible slice and both directions', () => {
  assert.deepEqual(pageInfo(312, 50, 0), {
    start: 1,
    end: 50,
    total: 312,
    page: 1,
    pages: 7,
    hasPrevious: false,
    hasNext: true,
    previousOffset: 0,
    nextOffset: 50,
  })
  const last = pageInfo(312, 50, 300)
  assert.equal(last.start, 301)
  assert.equal(last.end, 312)
  assert.equal(last.hasNext, false)
  assert.equal(last.hasPrevious, true)
  assert.equal(last.previousOffset, 250)
  const empty = pageInfo(0, 50, 0)
  assert.equal(empty.start, 0)
  assert.equal(empty.end, 0)
  assert.equal(empty.pages, 1)
  assert.equal(empty.hasNext, false)
  // An offset past the end (rows deleted meanwhile) shows nothing but can go back.
  const past = pageInfo(10, 50, 50)
  assert.equal(past.start, 0)
  assert.equal(past.hasPrevious, true)
})

/* ----------------------------------------------------------- events, sources */

check('every event type the API sends has a Turkish label', () => {
  assert.deepEqual(
    EVENT_TYPES.map((type) => type.id),
    ['menu_view', 'category_view', 'product_view'],
  )
  assert.equal(eventTypeLabel('menu_view'), 'Menü açılışı')
  assert.equal(eventTypeLabel('category_view'), 'Kategori görüntüleme')
  assert.equal(eventTypeLabel('product_view'), 'Ürün detayı açılışı')
  assert.equal(eventTypeLabel('something_new'), 'something_new')
  assert.equal(eventTypeLabel(''), '—')
  assert.equal(eventTypeLabel(null), '—')
})

check('an unknown address or port prints a dash, a real one as it is', () => {
  assert.equal(formatIp('203.0.113.7'), '203.0.113.7')
  assert.equal(formatIp('2001:db8::1'), '2001:db8::1')
  assert.equal(formatIp('-'), '—')
  assert.equal(formatIp(''), '—')
  assert.equal(formatIp(null), '—')
  assert.equal(formatPort(51234), '51234')
  assert.equal(formatPort('443'), '443')
  assert.equal(formatPort(null), '—')
  assert.equal(formatPort(undefined), '—')
  assert.equal(formatPort('-'), '—')
  assert.equal(formatPort(0), '—')
  assert.equal(formatPort(70000), '—')
})

check('every clientip source reads in Turkish, with a tooltip', () => {
  const sources = ['cloudflare', 'cloudflare-unverified', 'edge', 'forwarded-untrusted', 'peer', 'unknown']
  for (const source of sources) {
    const info = ipSourceInfo(source)
    assert.ok(info.text && info.text !== source, `no label for ${source}`)
    assert.ok(info.description.length > 20, `no explanation for ${source}`)
  }
  assert.equal(ipSourceInfo('cloudflare').text, 'Cloudflare · doğrulanmış')
  assert.equal(ipSourceInfo('cloudflare').tone, 'good')
  assert.equal(ipSourceInfo('cloudflare-unverified').text, 'Cloudflare · doğrulanmamış')
  assert.equal(ipSourceInfo('cloudflare-unverified').label, 'Cloudflare')
  assert.equal(ipSourceInfo('cloudflare-unverified').detail, 'doğrulanmamış')
  assert.equal(ipSourceInfo('peer').detail, '')
  // The two sources that carry a port say whose port it is; neither claims
  // to be the only one (a direct connection records its port too).
  assert.match(ipSourceInfo('cloudflare').description, /port/)
  assert.doesNotMatch(ipSourceInfo('cloudflare').description, /yalnızca/)
  assert.match(ipSourceInfo('peer').description, /port/)
  assert.match(ipSourceInfo('peer').description, /vekil/)
  assert.equal(ipSourceInfo('cloudflare-unverified').tone, 'warn')
  assert.equal(ipSourceInfo('forwarded-untrusted').tone, 'warn')
  assert.equal(ipSourceInfo('mystery').text, 'mystery')
  assert.equal(ipSourceInfo('').text, '—')
  assert.equal(ipSourceInfo(undefined).tone, 'neutral')
  // A prototype key is not a source.
  assert.equal(ipSourceInfo('constructor').text, 'constructor')
})

check('user agents shrink to browser and system', () => {
  const cases = [
    [
      'Mozilla/5.0 (Linux; Android 14; SM-S911B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.6613.127 Mobile Safari/537.36',
      'Chrome 128 · Android',
    ],
    [
      'Mozilla/5.0 (iPhone; CPU iPhone OS 17_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Mobile/15E148 Safari/604.1',
      'Safari 17 · iPhone',
    ],
    [
      'Mozilla/5.0 (iPhone; CPU iPhone OS 17_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/128.0.6613.98 Mobile/15E148 Safari/604.1',
      'Chrome 128 · iPhone',
    ],
    [
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36 Edg/129.0.2792.52',
      'Edge 129 · Windows',
    ],
    [
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:130.0) Gecko/20100101 Firefox/130.0',
      'Firefox 130 · macOS',
    ],
    [
      'Mozilla/5.0 (Linux; Android 14; SAMSUNG SM-A546B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/25.0 Chrome/121.0.0.0 Mobile Safari/537.36',
      'Samsung Internet 25 · Android',
    ],
    [
      'Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Instagram 345.0.0.34.94 (iPhone15,2; iOS 17_5; tr_TR)',
      'Instagram · iPhone',
    ],
    ['Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)', 'Bot · Googlebot'],
    ['facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)', 'Bot · facebookexternalhit'],
    // "CUBOT" is a phone brand, not a crawler.
    [
      'Mozilla/5.0 (Linux; Android 12; CUBOT X50) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36',
      'Chrome 120 · Android',
    ],
  ]
  for (const [agent, expected] of cases) assert.equal(shortUserAgent(agent), expected, agent)

  assert.equal(shortUserAgent(''), '—')
  assert.equal(shortUserAgent(null), '—')
  const odd = 'SomeKiosk-Agent-Without-Any-Known-Token-At-All-v1'
  assert.equal(shortUserAgent(odd), `${odd.slice(0, 39)}…`)
  assert.equal(shortUserAgent('tiny'), 'tiny')
})

/* -------------------------------------------------------------- audit trail */

check('every action and record type of the contract has a Turkish label', () => {
  const actions = [
    'product.create',
    'product.update',
    'product.delete',
    'product.price',
    'product.bulk_price',
    'product.reorder',
    'category.create',
    'category.update',
    'category.delete',
    'category.reorder',
    'menu.create',
    'menu.update',
    'menu.delete',
    'business.update',
    'account.password_change',
    'upload.create',
  ]
  assert.deepEqual(Object.keys(AUDIT_ACTIONS).sort(), actions.slice().sort())
  for (const action of actions) assert.notEqual(auditActionLabel(action), action)

  assert.equal(auditActionLabel('product.price'), 'Fiyat güncellendi')
  assert.equal(auditActionTone('product.delete'), 'delete')
  assert.equal(auditActionTone('menu.create'), 'create')
  assert.equal(auditActionTone('product.bulk_price'), 'price')
  assert.equal(auditActionLabel('future.thing'), 'future.thing')
  assert.equal(auditActionTone('future.thing'), 'neutral')
  assert.equal(auditActionLabel(null), '—')
  assert.equal(auditActionLabel('toString'), 'toString')

  assert.deepEqual(
    AUDIT_ENTITY_TYPES.map((entity) => entity.id),
    ['product', 'category', 'menu', 'business', 'account', 'upload'],
  )
  assert.equal(auditEntityLabel('product'), 'Ürün')
  assert.equal(auditEntityLabel('upload'), 'Dosya')
  assert.equal(auditEntityLabel('widget'), 'widget')
  assert.equal(auditEntityLabel(undefined), '')
})

check('field paths read as Turkish labels, translations with their language', () => {
  assert.equal(auditFieldLabel('price'), 'Fiyat')
  assert.equal(auditFieldLabel('phone'), 'Telefon')
  assert.equal(auditFieldLabel('logo_url'), 'Logo')
  assert.equal(auditFieldLabel('contact_in_footer'), 'Alt bilgide iletişim')
  assert.equal(auditFieldLabel('translations.en.name'), 'Ad (EN)')
  assert.equal(auditFieldLabel('translations.de.ingredients'), 'İçindekiler (DE)')
  assert.equal(auditFieldLabel('translations.ar.description'), 'Açıklama (AR)')
  assert.equal(auditFieldLabel('translations.fr'), 'Çeviriler (FR)')
  assert.equal(auditFieldLabel('links.0.url'), 'Linkler › 0.url')
  assert.equal(auditFieldLabel('brand_new_column'), 'brand_new_column')
  assert.equal(auditFieldLabel(''), '—')
})

check('values print as the owner would say them', () => {
  assert.equal(formatAuditValue(null), EMPTY_VALUE)
  assert.equal(formatAuditValue(undefined), EMPTY_VALUE)
  assert.equal(formatAuditValue('   '), EMPTY_VALUE)
  assert.equal(formatAuditValue([]), EMPTY_VALUE)
  assert.equal(formatAuditValue(true), 'Açık')
  assert.equal(formatAuditValue(false), 'Kapalı')
  assert.equal(formatAuditValue(145, 'price'), '145,00')
  assert.equal(formatAuditValue(1450.5, 'compare_price'), '1.450,50')
  assert.equal(formatAuditValue(3, 'position'), '3')
  assert.equal(formatAuditValue(0.45, 'background_overlay_opacity'), '0,45')
  assert.equal(formatAuditValue(['tr', 'en']), 'tr, en')
  assert.equal(formatAuditValue('inline', 'contact_display'), 'Yan yana')
  assert.equal(formatAuditValue('footer', 'contact_display'), 'Sadece alt bilgi')
  assert.equal(formatAuditValue('inline', 'name'), 'inline')
  assert.equal(formatAuditValue('••••', 'wifi_password'), '••••')
  assert.equal(formatAuditValue({ a: 1 }), '{"a":1}')
  assert.equal(formatAuditValue([{ id: 'x' }]), '[{"id":"x"}]')
})

check('category and menu ids print as names, contact links without their ids', () => {
  const MENU = '543bac88-6504-4eb1-8582-73a382dc8098'
  const MENU_EN = '7c1f0000-0000-4000-8000-000000000002'
  const HOT = '19ce36b4-c446-4bf5-88a1-9b78f760648f'
  const COLD = '0e957ad3-870b-4272-9112-2f4bd2cd528a'
  const BLANK = '0e957ad3-870b-4272-9112-2f4bd2cd5299'
  const ENGLISH = '0e957ad3-870b-4272-9112-2f4bd2cd5300'
  const GONE = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'

  const names = auditNameLookup({
    menus: [
      { id: MENU, name: 'Ana Menü', default_language: 'tr' },
      { id: MENU_EN, name: 'Brunch', default_language: 'en' },
    ],
    categories: [
      { id: HOT, menu_id: MENU, translations: { en: { name: 'Hot Drinks' }, tr: { name: 'Sıcak İçecekler' } } },
      { id: COLD, menu_id: MENU, translations: { tr: { name: 'Soğuk İçecekler' } } },
      { id: BLANK, menu_id: MENU, translations: { tr: { name: '  ' } } },
      // Named in its menu's default language; Turkish only as the fallback.
      { id: ENGLISH, menu_id: MENU_EN, translations: { tr: { name: 'Tatlılar' }, en: { name: 'Desserts' } } },
    ],
  })

  // A product moved to another category (PUT /api/products/:id).
  const moved = auditChangeRows({ category_id: { old: HOT, new: COLD } }, { names })
  assert.equal(moved[0].label, 'Kategori')
  assert.equal(moved[0].before, 'Sıcak İçecekler')
  assert.equal(moved[0].after, 'Soğuk İçecekler')

  // A category created on a menu, and a bulk price update scoped to categories.
  assert.equal(auditChangeRows({ menu_id: { old: null, new: MENU } }, { names })[0].after, 'Ana Menü')
  assert.equal(
    auditChangeRows({ category_ids: { old: null, new: [HOT, ENGLISH] } }, { names })[0].after,
    'Sıcak İçecekler, Desserts',
  )

  // Gone since, nameless, or no list to look in: never the raw id.
  assert.equal(formatAuditValue(GONE, 'category_id', names), 'Silinmiş kategori')
  assert.equal(formatAuditValue(GONE, 'menu_id', names), 'Silinmiş menü')
  assert.equal(formatAuditValue(BLANK, 'category_id', names), 'İsimsiz kategori')
  assert.equal(formatAuditValue(HOT, 'category_id'), 'Bilinmeyen kategori')
  const noCategories = auditNameLookup({ menus: [{ id: MENU, name: 'Ana Menü' }], categories: null })
  assert.equal(formatAuditValue(HOT, 'category_id', noCategories), 'Bilinmeyen kategori')
  assert.equal(formatAuditValue(MENU, 'menu_id', noCategories), 'Ana Menü')
  // Another field's uuid-looking text is still text.
  assert.equal(formatAuditValue(HOT, 'slug', names), HOT)

  // The contact links: label and address, the internal id left out, so the
  // preview shows what was added.
  const links = auditChangeRows({
    links: {
      old: [],
      new: [
        { id: '7e7b1b85-b33a-4e5c-a59b-e31491c9d877', label: 'Web sitemiz', url: 'https://example.com' },
        { id: 'x2', label: '', url: 'https://getir.com/melly' },
      ],
    },
  })
  assert.equal(links[0].label, 'Linkler')
  assert.equal(links[0].before, EMPTY_VALUE)
  assert.equal(links[0].after, 'Web sitemiz (https://example.com), https://getir.com/melly')
  assert.equal(links[0].long, false)
  // The same shape under another field is not a link list.
  assert.equal(formatAuditValue([{ id: 'x', url: 'u' }], 'badges'), '[{"id":"x","url":"u"}]')
})

check('long values are cut on a character boundary', () => {
  assert.deepEqual(truncateText('kısa', 10), { text: 'kısa', truncated: false })
  const cut = truncateText('a'.repeat(100), 10)
  assert.equal(cut.truncated, true)
  assert.equal(Array.from(cut.text).length, 10)
  assert.ok(cut.text.endsWith('…'))
  // An emoji is one character, never split into half a surrogate pair.
  const emoji = truncateText('☕'.repeat(5) + '🍰'.repeat(10), 6)
  assert.equal(emoji.text, '☕☕☕☕☕…')
})

check('a change record becomes one row per field, in order', () => {
  const rows = auditChangeRows({
    price: { old: 120, new: 145 },
    'translations.en.name': { old: 'Latte', new: 'Iced Latte' },
    wifi_password: { old: '••••', new: '••••' },
    description: { new: 'x'.repeat(200) },
    image_url: { old: '/uploads/a.png' },
  })
  assert.deepEqual(
    rows.map((row) => row.label),
    ['Fiyat', 'Ad (EN)', 'Wi-Fi şifresi', 'Açıklama', 'Görsel'],
  )
  assert.equal(rows[0].before, '120,00')
  assert.equal(rows[0].after, '145,00')
  assert.equal(rows[0].long, false)
  assert.equal(rows[3].hasBefore, false)
  assert.equal(rows[3].long, true)
  assert.equal(rows[4].hasAfter, false)
  assert.equal(rows[4].before, '/uploads/a.png')

  // A bare value instead of { old, new } is still shown, as the new value.
  const bare = auditChangeRows({ is_active: false })
  assert.equal(bare[0].after, 'Kapalı')
  assert.equal(bare[0].hasBefore, false)

  assert.deepEqual(auditChangeRows(null), [])
  assert.deepEqual(auditChangeRows('nope'), [])
  assert.deepEqual(auditChangeRows([1, 2]), [])
})

check('the null side the server writes is left out, an empty string is not', () => {
  // audit.Change always marshals both keys; a creation has old: null.
  const created = auditChangeRows({
    name: { old: null, new: 'Latte' },
    logo_url: { old: null, new: '/uploads/logo.svg' },
  })
  assert.deepEqual(
    created.map((row) => [row.hasBefore, row.hasAfter, row.after]),
    [
      [false, true, 'Latte'],
      [false, true, '/uploads/logo.svg'],
    ],
  )

  const deleted = auditChangeRows({ name: { old: 'Kış Menüsü', new: null } })
  assert.equal(deleted[0].hasBefore, true)
  assert.equal(deleted[0].hasAfter, false)
  assert.equal(deleted[0].before, 'Kış Menüsü')

  const cleared = auditChangeRows({ slogan: { old: 'Kahvenin en iyi hali', new: '' } })
  assert.equal(cleared[0].hasAfter, true)
  assert.equal(cleared[0].after, EMPTY_VALUE)

  const nothing = auditChangeRows({ phone: { old: null, new: null } })
  assert.equal(nothing[0].hasBefore, false)
  assert.equal(nothing[0].hasAfter, true)
  assert.equal(nothing[0].after, EMPTY_VALUE)
})

check('bulk price, reorder, upload and password records read in Turkish', () => {
  const bulk = auditChangeRows({
    affected: { old: null, new: 2 },
    percentage: { old: null, new: 10 },
    prices: {
      old: [
        { id: 'p1', name: 'Latte', price: 120 },
        { id: 'p2', name: 'Melly Beef', price: 310.5 },
      ],
      new: [
        { id: 'p1', name: 'Latte', price: 132 },
        { id: 'p2', name: 'Melly Beef', price: 341.55 },
      ],
    },
    rounding: { old: null, new: 'nearest_5' },
  })
  assert.deepEqual(
    bulk.map((row) => [row.label, row.hasBefore ? row.before : null, row.after]),
    [
      ['Güncellenen ürün', null, '2'],
      ['Oran', null, '%10'],
      ['Fiyatlar', 'Latte 120,00, Melly Beef 310,50', 'Latte 132,00, Melly Beef 341,55'],
      ['Yuvarlama', null, "5'in katına"],
    ],
  )

  const reorder = auditChangeRows({ order: { old: null, new: ['Latte', 'Mocha'] } })
  assert.equal(reorder[0].label, 'Yeni sıralama')
  assert.equal(reorder[0].after, 'Latte, Mocha')

  const upload = auditChangeRows({
    content_type: { old: null, new: 'image/svg+xml' },
    size: { old: null, new: 2560 },
  })
  assert.equal(upload[0].label, 'Dosya türü')
  assert.equal(upload[1].after, '2,5 KB')

  const password = auditChangeRows({ method: { old: null, new: 'reset_link' } })
  assert.equal(password[0].label, 'Yöntem')
  assert.equal(password[0].after, 'Sıfırlama bağlantısıyla')
})

if (failures.length > 0) {
  console.error(`${failures.length} failed, ${passed} passed\n\n${failures.join('\n\n')}`)
  process.exit(1)
}
console.log(`dashboardAnalytics.test.mjs: all ${passed} checks passed`)
