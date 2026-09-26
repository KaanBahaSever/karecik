// Assertions for src/lib/ipExclusion.js: how the analytics page reads,
// normalises, checks and matches the IP addresses and ranges whose visits are
// never recorded ("Hariç tutulan IP'ler").
//
//   node frontend/tests/ipExclusion.test.mjs
//
// Nothing imports this file, so it never reaches the app bundle.

import assert from 'node:assert/strict'

import {
  addResultText,
  alreadyRemovedText,
  checkExclusion,
  checkLabel,
  cidrContains,
  currentIpCovered,
  displayCidr,
  entryName,
  excludableAddress,
  EXCLUSION_MESSAGES,
  exclusionMatcher,
  findCoveringEntry,
  futureVisitsText,
  isSingleEntry,
  listFullMessage,
  MAX_EXCLUDED_IPS,
  MAX_LABEL_LENGTH,
  MIN_PREFIX,
  parseExclusion,
  parseNetwork,
  pastRecordsText,
  readExclusionState,
  removeEntryLabel,
  removeEntryText,
  withAddedEntry,
  withoutEntry,
} from '../src/lib/ipExclusion.js'

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

/** The fields of parseNetwork a test cares about. */
function summary(value) {
  const network = parseNetwork(value)
  return network
    ? { family: network.family, cidr: network.cidr, display: network.display, single: network.single }
    : null
}

/* ------------------------------------------------------------- constants */

check('the limits are the server contract', () => {
  assert.equal(MAX_EXCLUDED_IPS, 50)
  assert.equal(MAX_LABEL_LENGTH, 60)
  assert.deepEqual(MIN_PREFIX, { 4: 16, 6: 48 })
  assert.equal(EXCLUSION_MESSAGES.duplicate, 'Bu IP zaten listede.')
  assert.equal(listFullMessage(), listFullMessage(50))
  assert.match(listFullMessage(), /en fazla 50 IP/)
})

/* --------------------------------------------------------------- parsing */

check('an IPv4 address is a single-host /32 shown bare', () => {
  assert.deepEqual(summary('198.18.139.87'), {
    family: 4,
    cidr: '198.18.139.87/32',
    display: '198.18.139.87',
    single: true,
  })
  assert.deepEqual(summary('  198.18.139.87  '), summary('198.18.139.87'))
  assert.deepEqual(summary('198.18.139.87/32'), summary('198.18.139.87'))
  assert.equal(summary('0.0.0.0').cidr, '0.0.0.0/32')
  assert.equal(summary('255.255.255.255').cidr, '255.255.255.255/32')
})

check('a range has its host bits masked off and is shown as a range', () => {
  assert.deepEqual(summary('198.18.139.87/24'), {
    family: 4,
    cidr: '198.18.139.0/24',
    display: '198.18.139.0/24',
    single: false,
  })
  assert.equal(summary('198.18.139.87/16').cidr, '198.18.0.0/16')
  assert.equal(summary('198.18.139.87/20').cidr, '198.18.128.0/20')
  assert.equal(summary('198.18.139.87/31').cidr, '198.18.139.86/31')
  assert.equal(summary('10.1.2.3/0').cidr, '0.0.0.0/0') // read; the size rule is separate
})

check('IPv4 text netip refuses is refused', () => {
  for (const bad of [
    '',
    '   ',
    '198.18.139',
    '198.18.139.87.1',
    '198.18.139.256',
    '198.018.139.87', // a leading zero: octal to some parsers, refused by netip
    '198.18.139.-1',
    '198.18.139.+1',
    '198.18..87',
    '198.18.139.87/33',
    '198.18.139.87/',
    '198.18.139.87/024',
    '198.18.139.87/-1',
    '198.18.139.87/ 24',
    '198.18.139.87/24/8',
    'localhost',
    'unknown',
    '-',
    '1e3.0.0.1',
  ]) {
    assert.equal(parseNetwork(bad), null, `${JSON.stringify(bad)} should not parse`)
  }
  assert.equal(parseNetwork(null), null)
  assert.equal(parseNetwork(undefined), null)
  assert.equal(parseNetwork(3232235777), null)
})

check('IPv6 is written the way Go netip writes it', () => {
  assert.deepEqual(summary('2001:DB8:0:0:0:0:0:1'), {
    family: 6,
    cidr: '2001:db8::1/128',
    display: '2001:db8::1',
    single: true,
  })
  assert.equal(summary('2001:0db8:0000:0000:0001:0000:0000:0001').display, '2001:db8::1:0:0:1')
  // A lone zero group is not compressed; of two equal runs the first is.
  assert.equal(summary('2001:db8:0:1:1:1:1:1').display, '2001:db8:0:1:1:1:1:1')
  assert.equal(summary('2001:0:0:1:0:0:1:1').display, '2001::1:0:0:1:1')
  assert.equal(summary('::').display, '::')
  assert.equal(summary('::1').display, '::1')
  assert.equal(summary('fe80::').display, 'fe80::')
  assert.equal(summary('1:2:3:4:5:6:7:8').display, '1:2:3:4:5:6:7:8')
  assert.equal(summary('2a02:e0:1234:5678:9abc:def0:1:2/48').cidr, '2a02:e0:1234::/48')
  assert.equal(summary('2a02:e0:1234:5678::/64').display, '2a02:e0:1234:5678::/64')
  assert.equal(summary('2a02:e0:1234:5678::1/52').cidr, '2a02:e0:1234:5000::/52')
})

check('IPv6 text netip refuses is refused', () => {
  for (const bad of [
    '1:2:3:4:5:6:7',
    '1:2:3:4:5:6:7:8:9',
    '1:2:3:4:5:6:7:8::', // "::" must stand for at least one zero group
    '1::2::3',
    ':1:2:3:4:5:6:7',
    '1:2:3:4:5:6:7:',
    '12345::1',
    'g::1',
    'fe80::1%eth0',
    '::1/129',
    '::1.2.3',
    '::1.2.3.256',
    ':::1',
  ]) {
    assert.equal(parseNetwork(bad), null, `${JSON.stringify(bad)} should not parse`)
  }
})

check('IPv4-mapped IPv6 is unmapped, as the server unmaps the visitor address', () => {
  assert.deepEqual(summary('::ffff:198.18.139.87'), summary('198.18.139.87'))
  assert.deepEqual(summary('::FFFF:c612:8b57'), summary('198.18.139.87'))
  assert.equal(summary('::ffff:198.18.139.87/120').cidr, '198.18.139.0/24')
  assert.equal(summary('::ffff:198.18.0.0/112').cidr, '198.18.0.0/16')
  // A mapped range shorter than /96 reaches past the IPv4 block: it stays IPv6.
  assert.equal(summary('::ffff:198.18.0.0/80').family, 6)
  // An embedded IPv4 tail on anything else is just IPv6 text.
  assert.equal(summary('64:ff9b::198.18.139.87').display, '64:ff9b::c612:8b57')
})

check('displayCidr drops the prefix of a single host only', () => {
  assert.equal(displayCidr('198.18.139.87/32'), '198.18.139.87')
  assert.equal(displayCidr('198.18.139.0/24'), '198.18.139.0/24')
  assert.equal(displayCidr('2001:db8::1/128'), '2001:db8::1')
  // "/32" of IPv6 is a range, not a host.
  assert.equal(displayCidr('2001:db8::/32'), '2001:db8::/32')
  assert.equal(displayCidr(' not an ip '), 'not an ip')
  assert.equal(displayCidr(null), '')
})

/* ------------------------------------------------------------- size rule */

check('ranges broader than /16 (IPv4) or /48 (IPv6) are refused with the reason', () => {
  assert.equal(parseExclusion('198.18.0.0/16').problem, '')
  assert.equal(parseExclusion('198.18.0.0/16').network.cidr, '198.18.0.0/16')
  assert.equal(parseExclusion('198.18.0.0/15').problem, EXCLUSION_MESSAGES.tooBroad4)
  assert.equal(parseExclusion('0.0.0.0/0').problem, EXCLUSION_MESSAGES.tooBroad4)
  assert.equal(parseExclusion('2a02:e0:1234::/48').problem, '')
  assert.equal(parseExclusion('2a02:e0::/47').problem, EXCLUSION_MESSAGES.tooBroad6)
  assert.equal(parseExclusion('::/0').problem, EXCLUSION_MESSAGES.tooBroad6)
  // Judged after unmapping: ::ffff:0:0/100 is 0.0.0.0/4.
  assert.equal(parseExclusion('::ffff:0.0.0.0/100').problem, EXCLUSION_MESSAGES.tooBroad4)
  assert.match(EXCLUSION_MESSAGES.tooBroad4, /gerçek müşteri/)
})

check('an empty, unreadable or out-of-range entry gets the fitting message', () => {
  assert.deepEqual(parseExclusion('  '), { network: null, problem: EXCLUSION_MESSAGES.empty })
  assert.deepEqual(parseExclusion(undefined), { network: null, problem: EXCLUSION_MESSAGES.empty })
  assert.equal(parseExclusion('198.18.139').problem, EXCLUSION_MESSAGES.invalid)
  assert.equal(parseExclusion('ev bilgisayarım').problem, EXCLUSION_MESSAGES.invalid)
  assert.equal(parseExclusion('198.18.139.87/40').problem, EXCLUSION_MESSAGES.prefix)
  assert.equal(parseExclusion('::1/200').problem, EXCLUSION_MESSAGES.prefix)
  assert.equal(parseExclusion('198.18.139.87/abc').problem, EXCLUSION_MESSAGES.prefix)
  assert.equal(parseExclusion('nope/24').problem, EXCLUSION_MESSAGES.invalid)
})

/* -------------------------------------------------------------- matching */

const LIST = [
  { id: 'a', cidr: '198.18.139.87/32', display: '198.18.139.87', label: 'Ev' },
  { id: 'b', cidr: '85.105.0.0/16', display: '85.105.0.0/16', label: 'Ofis' },
  { id: 'c', cidr: '2a02:e0:1234::/48', display: '2a02:e0:1234::/48', label: '' },
]

check('cidrContains: family, prefix and every masked bit', () => {
  assert.equal(cidrContains('85.105.0.0/16', '85.105.200.7'), true)
  assert.equal(cidrContains('85.105.0.0/16', '85.106.0.1'), false)
  assert.equal(cidrContains('198.18.139.87/32', '198.18.139.87'), true)
  assert.equal(cidrContains('198.18.139.87/32', '198.18.139.88'), false)
  assert.equal(cidrContains('198.18.128.0/20', '198.18.143.255'), true)
  assert.equal(cidrContains('198.18.128.0/20', '198.18.144.0'), false)
  assert.equal(cidrContains('2a02:e0:1234::/48', '2a02:e0:1234:ffff::1'), true)
  assert.equal(cidrContains('2a02:e0:1234::/48', '2a02:e0:1235::1'), false)
  assert.equal(cidrContains('85.105.0.0/16', '::ffff:85.105.1.1'), true)
  // A range contains its narrower ranges, never a broader one.
  assert.equal(cidrContains('85.105.0.0/16', '85.105.3.0/24'), true)
  assert.equal(cidrContains('85.105.3.0/24', '85.105.0.0/16'), false)
  // Across families, and for what the log writes when it has no address.
  assert.equal(cidrContains('0.0.0.0/0', '::1'), false)
  for (const unknown of ['-', 'unknown', '', null, undefined]) {
    assert.equal(cidrContains('0.0.0.0/0', unknown), false)
  }
})

check('findCoveringEntry is the 409 rule: the same entry, or a range around it', () => {
  assert.equal(findCoveringEntry(LIST, '198.18.139.87')?.id, 'a')
  assert.equal(findCoveringEntry(LIST, '198.18.139.87/32')?.id, 'a')
  assert.equal(findCoveringEntry(LIST, '::ffff:198.18.139.87')?.id, 'a')
  assert.equal(findCoveringEntry(LIST, '85.105.9.0/24')?.id, 'b')
  assert.equal(findCoveringEntry(LIST, '85.105.0.0/16')?.id, 'b')
  // A broader range around an entry is new: it covers more than the entry.
  assert.equal(findCoveringEntry(LIST, '198.18.139.0/24'), null)
  assert.equal(findCoveringEntry(LIST, '198.18.139.86'), null)
  assert.equal(findCoveringEntry([], '198.18.139.87'), null)
  assert.equal(findCoveringEntry(null, '198.18.139.87'), null)
  assert.equal(findCoveringEntry(LIST, 'garbage'), null)
  // An entry the server sent without `cidr` is read from its `display`.
  assert.equal(findCoveringEntry([{ id: 'x', display: '10.0.0.1' }], '10.0.0.1')?.id, 'x')
})

check('exclusionMatcher answers the covering entry of each log address', () => {
  const match = exclusionMatcher(LIST)
  assert.equal(match('198.18.139.87')?.id, 'a')
  assert.equal(match('85.105.77.1')?.id, 'b')
  assert.equal(match('2a02:e0:1234:1::9')?.id, 'c')
  assert.equal(match('198.18.139.86'), null)
  assert.equal(match('-'), null)
  assert.equal(match('unknown'), null)
  assert.equal(match(''), null)
  assert.equal(match(null), null)
  // The log holds addresses, never ranges.
  assert.equal(match('85.105.0.0/16'), null)
  assert.equal(exclusionMatcher([])('198.18.139.87'), null)
  assert.equal(exclusionMatcher(undefined)('198.18.139.87'), null)
  assert.equal(exclusionMatcher([{ id: 'bad', cidr: 'nope' }])('198.18.139.87'), null)
})

check('excludableAddress: a real log address normalised, anything else no button', () => {
  assert.equal(excludableAddress('198.18.139.87'), '198.18.139.87')
  assert.equal(excludableAddress(' 198.18.139.87 '), '198.18.139.87')
  assert.equal(excludableAddress('::ffff:198.18.139.87'), '198.18.139.87')
  assert.equal(excludableAddress('2A02:E0::1'), '2a02:e0::1')
  for (const unknown of ['-', 'unknown', '', '   ', null, undefined, 42, '85.105.0.0/16']) {
    assert.equal(excludableAddress(unknown), '', JSON.stringify(unknown))
  }
})

/* ---------------------------------------------------------------- payload */

check('readExclusionState: the E2 payload, with safe defaults', () => {
  assert.deepEqual(
    readExclusionState({
      items: [LIST[0], null, { cidr: 'no id' }, LIST[1]],
      current_ip: ' 198.18.139.87 ',
      current_ip_source: 'cloudflare',
      current_ip_excluded: true,
      optout: true,
      max: 50,
    }),
    {
      items: [LIST[0], LIST[1]],
      currentIp: '198.18.139.87',
      currentIpSource: 'cloudflare',
      currentIpExcluded: true,
      optout: true,
      max: 50,
    },
  )
  const empty = {
    items: [],
    currentIp: null,
    currentIpSource: '',
    currentIpExcluded: false,
    optout: false,
    max: MAX_EXCLUDED_IPS,
  }
  assert.deepEqual(readExclusionState(null), empty)
  assert.deepEqual(readExclusionState([]), empty)
  assert.deepEqual(
    readExclusionState({ items: 'x', current_ip: '-', optout: 'yes', max: -1, current_ip_excluded: 1 }),
    empty,
  )
  assert.equal(readExclusionState({ current_ip: null }).currentIp, null)
})

check('currentIpCovered follows the items, so an add or a remove shows at once', () => {
  const state = readExclusionState({ items: LIST, current_ip: '85.105.9.9', current_ip_excluded: false })
  assert.equal(currentIpCovered(state), true)
  assert.equal(currentIpCovered({ ...state, items: [LIST[0]] }), false)
  // An address this file cannot read is left to the server's answer.
  assert.equal(currentIpCovered({ ...state, currentIp: 'weird', currentIpExcluded: true }), true)
  assert.equal(currentIpCovered({ ...state, currentIp: null, currentIpExcluded: true }), false)
  assert.equal(currentIpCovered(null), false)
})

check('withAddedEntry puts a new entry last, where the oldest-first list will have it', () => {
  // The server lists oldest first (ORDER BY created_at), and the page reloads
  // the list right after an add: the optimistic copy must already agree, or
  // the new row shows first and then jumps to the end.
  const state = readExclusionState({ items: LIST.slice(0, 2), current_ip: '198.18.139.87' })
  const added = withAddedEntry(state, LIST[2])
  assert.deepEqual(added.items.map((item) => item.id), ['a', 'b', 'c'])
  const reloaded = readExclusionState({ items: LIST, current_ip: '198.18.139.87' })
  assert.deepEqual(added.items, reloaded.items)
  // Everything else of the state is kept, and the old state is not touched.
  assert.equal(added.currentIp, '198.18.139.87')
  assert.deepEqual(state.items.map((item) => item.id), ['a', 'b'])
  // An entry already on the list (a double answer) moves, never doubles.
  const moved = withAddedEntry(reloaded, { ...LIST[0], label: 'Yeni' })
  assert.deepEqual(moved.items.map((item) => item.id), ['b', 'c', 'a'])
  // Nothing to add to, or nothing usable to add: unchanged.
  assert.equal(withAddedEntry(null, LIST[0]), null)
  assert.equal(withAddedEntry(state, null), state)
  assert.equal(withAddedEntry(state, { cidr: '10.0.0.1/32' }), state)
  assert.deepEqual(withAddedEntry({ items: 'x' }, LIST[0]).items, [LIST[0]])
})

check('withoutEntry removes one entry and keeps the order of the rest', () => {
  const state = readExclusionState({ items: LIST })
  assert.deepEqual(withoutEntry(state, 'b').items.map((item) => item.id), ['a', 'c'])
  assert.deepEqual(withoutEntry(state, 'nope').items.map((item) => item.id), ['a', 'b', 'c'])
  assert.equal(state.items.length, 3)
  assert.equal(withoutEntry(null, 'a'), null)
  assert.equal(withoutEntry(state, undefined), state)
})

/* ----------------------------------------------------------------- checks */

check('checkLabel trims and counts characters, not UTF-16 units', () => {
  assert.deepEqual(checkLabel('  Ev  '), { label: 'Ev', problem: '' })
  assert.deepEqual(checkLabel(undefined), { label: '', problem: '' })
  assert.equal(checkLabel('ş'.repeat(60)).problem, '')
  assert.equal(checkLabel('ş'.repeat(61)).problem, EXCLUSION_MESSAGES.label)
  // 60 house emoji are 120 UTF-16 units and still 60 characters (code points,
  // as Go's utf8.RuneCountInString counts them); "☕️" is two code points.
  assert.equal(checkLabel('🏠'.repeat(60)).problem, '')
  assert.equal(checkLabel('☕️'.repeat(30)).problem, '')
  assert.equal(checkLabel('☕️'.repeat(31)).problem, EXCLUSION_MESSAGES.label)
  assert.equal(checkLabel('🏠'.repeat(61)).problem, EXCLUSION_MESSAGES.label)
})

check('checkExclusion: the value the dialog and the API get', () => {
  assert.deepEqual(checkExclusion({ cidr: ' 198.18.139.87 ', label: ' Ev ' }, { items: [], max: 50 }), {
    value: { cidr: '198.18.139.87/32', display: '198.18.139.87', label: 'Ev', single: true },
    problem: '',
    field: '',
  })
  assert.deepEqual(checkExclusion({ cidr: '85.105.3.9/24' }).value, {
    cidr: '85.105.3.0/24',
    display: '85.105.3.0/24',
    label: '',
    single: false,
  })
})

check('checkExclusion: each problem, and where it belongs', () => {
  const list = { items: LIST, max: 50 }
  assert.deepEqual(checkExclusion({ cidr: '' }, list), {
    value: null,
    problem: EXCLUSION_MESSAGES.empty,
    field: 'cidr',
  })
  assert.equal(checkExclusion({ cidr: '1.2.0.0/8' }, list).problem, EXCLUSION_MESSAGES.tooBroad4)
  assert.deepEqual(checkExclusion({ cidr: '198.18.139.87' }, list), {
    value: null,
    problem: 'Bu IP zaten listede.',
    field: 'cidr',
  })
  const inside = checkExclusion({ cidr: '85.105.1.2' }, list)
  assert.equal(inside.field, 'cidr')
  assert.equal(inside.problem, 'Bu IP zaten listede. (85.105.0.0/16 aralığının içinde)')

  assert.deepEqual(checkExclusion({ cidr: '9.9.9.9', label: 'x'.repeat(61) }, list), {
    value: null,
    problem: EXCLUSION_MESSAGES.label,
    field: 'label',
  })

  // A full list is said first: nothing typed into the fields can fix it.
  const full = Array.from({ length: 50 }, (_, index) => ({
    id: String(index),
    cidr: `10.0.${index}.1/32`,
  }))
  assert.deepEqual(checkExclusion({ cidr: 'nonsense' }, { items: full, max: 50 }), {
    value: null,
    problem: listFullMessage(50),
    field: '',
  })
  assert.equal(checkExclusion({ cidr: '9.9.9.9' }, { items: full.slice(0, 3), max: 3 }).problem, listFullMessage(3))
  // A list that could not be loaded leaves the duplicate and limit checks to the server.
  assert.equal(checkExclusion({ cidr: '198.18.139.87' }, { items: null }).problem, '')
  assert.equal(checkExclusion({ cidr: '198.18.139.87' }).problem, '')
})

/* -------------------------------------------------------------- sentences */

check('the dialog says how many past visits the entry covers', () => {
  assert.equal(pastRecordsText(12), "Bu IP'den 12 ziyaret kaydı var.")
  assert.equal(pastRecordsText(1234), "Bu IP'den 1.234 ziyaret kaydı var.")
  assert.equal(pastRecordsText(0), "Bu IP'den kaydedilmiş bir ziyaret yok.")
  assert.equal(pastRecordsText(undefined), "Bu IP'den kaydedilmiş bir ziyaret yok.")
  assert.equal(pastRecordsText(-3), "Bu IP'den kaydedilmiş bir ziyaret yok.")
  assert.equal(pastRecordsText(5, { single: false }), 'Bu IP aralığından 5 ziyaret kaydı var.')
  assert.equal(pastRecordsText(0, { single: false }), 'Bu IP aralığından kaydedilmiş bir ziyaret yok.')
})

check('the result says what happened to the past records', () => {
  assert.equal(
    addResultText({ display: '198.18.139.87', deleteHistory: true, deletedEvents: 12 }),
    '198.18.139.87 hariç tutuldu. 12 kayıt silindi.',
  )
  assert.equal(
    addResultText({ display: '198.18.139.87', deleteHistory: true, deletedEvents: 1500 }),
    '198.18.139.87 hariç tutuldu. 1.500 kayıt silindi.',
  )
  assert.equal(
    addResultText({ display: '85.105.0.0/16', deleteHistory: false, deletedEvents: 0, pastCount: 7 }),
    '85.105.0.0/16 hariç tutuldu. Geçmiş 7 kayıt silinmedi.',
  )
  assert.equal(
    addResultText({ display: '198.18.139.87', deleteHistory: false, deletedEvents: 0, pastCount: 0 }),
    '198.18.139.87 hariç tutuldu. Bundan sonraki ziyaretler kaydedilmeyecek.',
  )
  assert.equal(addResultText({ deleteHistory: true }), 'IP hariç tutuldu. 0 kayıt silindi.')
})

check('isSingleEntry tells an address from a range, whatever form the entry has', () => {
  assert.equal(isSingleEntry(LIST[0]), true)
  assert.equal(isSingleEntry(LIST[1]), false)
  assert.equal(isSingleEntry(LIST[2]), false)
  assert.equal(isSingleEntry({ cidr: '2001:db8::9/128' }), true)
  assert.equal(isSingleEntry({ display: '203.0.113.7' }), true)
  // A checked entry (checkExclusion's value) says so itself.
  assert.equal(isSingleEntry(checkExclusion({ cidr: '2001:DB8:1234:5678::9/48' }).value), false)
  assert.equal(isSingleEntry(checkExclusion({ cidr: '203.0.113.7' }).value), true)
  // Unreadable text: a range when it has a prefix.
  assert.equal(isSingleEntry({ display: 'garbage/24' }), false)
  assert.equal(isSingleEntry({ display: 'garbage' }), true)
  assert.equal(isSingleEntry(null), true)
})

check('entryName: the server display, else the range read, else IP', () => {
  assert.equal(entryName(LIST[0]), '198.18.139.87')
  assert.equal(entryName({ cidr: '203.0.113.7/32' }), '203.0.113.7')
  assert.equal(entryName({ display: '  ', cidr: '203.0.113.0/24' }), '203.0.113.0/24')
  assert.equal(entryName({}), 'IP')
  assert.equal(entryName(null), 'IP')
})

check('the sentences about an entry say "adres" for an address and "aralık" for a range', () => {
  const single = checkExclusion({ cidr: '198.18.139.87' }).value
  const range = checkExclusion({ cidr: '2001:DB8:1234:5678::9/48' }).value
  assert.equal(futureVisitsText(single), 'Bundan sonra bu adresten gelen menü ziyaretleri kaydedilmez.')
  assert.equal(futureVisitsText(range), 'Bundan sonra bu aralıktan gelen menü ziyaretleri kaydedilmez.')

  assert.equal(removeEntryLabel(LIST[0]), '198.18.139.87 adresini listeden çıkar')
  assert.equal(
    removeEntryLabel({ id: 'r', cidr: '192.0.2.0/24', display: '192.0.2.0/24' }),
    '192.0.2.0/24 aralığını listeden çıkar',
  )
  assert.equal(removeEntryLabel(LIST[2]), '2a02:e0:1234::/48 aralığını listeden çıkar')

  assert.equal(
    removeEntryText(LIST[0]),
    '198.18.139.87 (Ev) listeden çıkarılınca bu adresten gelen ziyaretler yeniden kaydedilmeye başlar. Daha önce silinen kayıtlar geri gelmez.',
  )
  assert.equal(
    removeEntryText({ ...LIST[1], label: '  Ofis  ' }),
    '85.105.0.0/16 (Ofis) listeden çıkarılınca bu aralıktan gelen ziyaretler yeniden kaydedilmeye başlar. Daha önce silinen kayıtlar geri gelmez.',
  )
  assert.equal(
    removeEntryText(LIST[2]),
    '2a02:e0:1234::/48 listeden çıkarılınca bu aralıktan gelen ziyaretler yeniden kaydedilmeye başlar. Daha önce silinen kayıtlar geri gelmez.',
  )

  assert.equal(alreadyRemovedText(LIST[0]), 'Bu IP zaten listede değildi.')
  assert.equal(alreadyRemovedText(LIST[1]), 'Bu IP aralığı zaten listede değildi.')
})

/* --------------------------------------------------------------- summary */

if (failures.length > 0) {
  console.error(`${failures.length} failed, ${passed} passed\n\n${failures.join('\n\n')}`)
  process.exit(1)
}
console.log(`ipExclusion.test.mjs: all ${passed} checks passed`)
