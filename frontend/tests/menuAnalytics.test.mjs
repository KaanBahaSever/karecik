// Assertions for src/lib/analytics.js: the visitor events the customer menu
// sends to POST /api/public/events.
//
//   node frontend/tests/menuAnalytics.test.mjs
//
// The transport is injected (navigator, fetch, storage), so nothing here sends
// a request. Nothing imports this file, so it never reaches the app bundle.

import assert from 'node:assert/strict'

import {
  browserCookies,
  buildEventBody,
  createVisitorId,
  EVENTS_URL,
  getVisitorId,
  hasAnalyticsOptout,
  isValidVisitorId,
  MENU_EVENT_TYPES,
  OPTOUT_COOKIE,
  trackMenuEvent,
  VISITOR_ID_KEY,
} from '../src/lib/analytics.js'

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
  }
}

/** A navigator whose beacon records what it was handed and answers `accept`. */
function beaconNavigator(accept = true) {
  const sent = []
  return {
    sent,
    sendBeacon(url, body) {
      sent.push({ url, body })
      return accept
    },
  }
}

/** A fetch that records its calls and returns `result`. */
function recordingFetch(result = Promise.resolve()) {
  const calls = []
  const fetchImpl = (url, init) => {
    calls.push({ url, init })
    return result
  }
  fetchImpl.calls = calls
  return fetchImpl
}

const EVENT = {
  businessSlug: 'melly-coffee',
  menuSlug: 'suadiye',
  type: 'category_view',
  categoryId: '0b6f0f3e-1d2a-4c55-9a51-0d8a3a9f2c11',
  language: 'en',
}

/* ------------------------------------------------------------ constants */

check('the endpoint and the event types', () => {
  // VITE_API_URL is unset here, so the base is the page's own origin.
  assert.equal(EVENTS_URL, '/api/public/events')
  assert.deepEqual(MENU_EVENT_TYPES, ['menu_view', 'category_view', 'product_view'])
  assert.equal(VISITOR_ID_KEY, 'karecik_visitor_id')
})

/* ------------------------------------------------------------ visitor id */

check('isValidVisitorId: 1-64 of [A-Za-z0-9_-]', () => {
  assert.ok(isValidVisitorId('abc'))
  assert.ok(isValidVisitorId('0b6f0f3e-1d2a-4c55-9a51-0d8a3a9f2c11'))
  assert.ok(isValidVisitorId('a'.repeat(64)))
  assert.ok(!isValidVisitorId('a'.repeat(65)))
  assert.ok(!isValidVisitorId(''))
  assert.ok(!isValidVisitorId('has space'))
  assert.ok(!isValidVisitorId('<script>'))
  assert.ok(!isValidVisitorId(null))
  assert.ok(!isValidVisitorId(42))
})

check('createVisitorId: a UUID, else random hex, else a fallback — always valid', () => {
  const uuid = createVisitorId({ randomUUID: () => '0b6f0f3e-1d2a-4c55-9a51-0d8a3a9f2c11' })
  assert.equal(uuid, '0b6f0f3e-1d2a-4c55-9a51-0d8a3a9f2c11')

  const insecure = {
    randomUUID() {
      throw new Error('insecure context')
    },
    getRandomValues(array) {
      array.fill(171)
      return array
    },
  }
  assert.equal(createVisitorId(insecure), 'ab'.repeat(16))

  const fallback = createVisitorId(null)
  assert.ok(isValidVisitorId(fallback), fallback)
  assert.notEqual(createVisitorId(null), fallback)

  assert.ok(isValidVisitorId(createVisitorId()))
})

check('getVisitorId: stored once, then read back', () => {
  const storage = memoryStorage()
  const first = getVisitorId(storage)
  assert.ok(isValidVisitorId(first))
  assert.equal(storage.data.get(VISITOR_ID_KEY), first)
  assert.equal(getVisitorId(storage), first)

  const returning = memoryStorage({ [VISITOR_ID_KEY]: 'returning-visitor_1' })
  assert.equal(getVisitorId(returning), 'returning-visitor_1')
})

check('getVisitorId: a corrupt stored id is replaced', () => {
  const storage = memoryStorage({ [VISITOR_ID_KEY]: 'not valid!' })
  const id = getVisitorId(storage)
  assert.ok(isValidVisitorId(id))
  assert.equal(storage.data.get(VISITOR_ID_KEY), id)
})

check('getVisitorId: without storage, one id for the whole page load', () => {
  const blocked = memoryStorage({}, true)
  const first = getVisitorId(blocked)
  assert.ok(isValidVisitorId(first))
  assert.equal(getVisitorId(blocked), first)
  assert.equal(getVisitorId(null), first)
})

/* ------------------------------------------------------------------ body */

check('buildEventBody: the contract fields, optional ones left out', () => {
  assert.deepEqual(buildEventBody({ ...EVENT, visitorId: 'v-1' }), {
    business_slug: 'melly-coffee',
    menu_slug: 'suadiye',
    type: 'category_view',
    category_id: '0b6f0f3e-1d2a-4c55-9a51-0d8a3a9f2c11',
    visitor_id: 'v-1',
    language: 'en',
  })

  assert.deepEqual(
    buildEventBody({ businessSlug: ' melly-coffee ', menuSlug: 'suadiye', type: 'menu_view' }),
    { business_slug: 'melly-coffee', menu_slug: 'suadiye', type: 'menu_view' },
  )

  assert.deepEqual(
    buildEventBody({
      businessSlug: 'b',
      menuSlug: 'm',
      type: 'product_view',
      productId: 'p-1',
      categoryId: '',
      visitorId: 'bad id',
      language: '  ',
    }),
    { business_slug: 'b', menu_slug: 'm', type: 'product_view', product_id: 'p-1' },
  )
})

check('buildEventBody: nothing to send without slugs or a known type', () => {
  assert.equal(buildEventBody({ ...EVENT, businessSlug: '' }), null)
  assert.equal(buildEventBody({ ...EVENT, menuSlug: undefined }), null)
  assert.equal(buildEventBody({ ...EVENT, type: 'page_view' }), null)
  assert.equal(buildEventBody({}), null)
  assert.equal(buildEventBody(), null)
})

/* ------------------------------------------------------------- transport */

check('trackMenuEvent sends a text beacon of the JSON body', () => {
  const nav = beaconNavigator(true)
  const fetchImpl = recordingFetch()
  const storage = memoryStorage({ [VISITOR_ID_KEY]: 'visitor-1' })

  assert.equal(trackMenuEvent(EVENT, { navigator: nav, fetch: fetchImpl, storage }), true)
  assert.equal(nav.sent.length, 1)
  assert.equal(fetchImpl.calls.length, 0)
  assert.equal(nav.sent[0].url, '/api/public/events')
  // A plain string, which the beacon sends as text/plain: no CORS preflight.
  assert.equal(typeof nav.sent[0].body, 'string')
  assert.deepEqual(JSON.parse(nav.sent[0].body), {
    business_slug: 'melly-coffee',
    menu_slug: 'suadiye',
    type: 'category_view',
    category_id: '0b6f0f3e-1d2a-4c55-9a51-0d8a3a9f2c11',
    visitor_id: 'visitor-1',
    language: 'en',
  })
})

check('trackMenuEvent falls back to a keepalive fetch', () => {
  for (const nav of [beaconNavigator(false), {}, null]) {
    const fetchImpl = recordingFetch()
    const storage = memoryStorage({ [VISITOR_ID_KEY]: 'visitor-1' })
    assert.equal(trackMenuEvent(EVENT, { navigator: nav, fetch: fetchImpl, storage }), true)
    assert.equal(fetchImpl.calls.length, 1)
    const { url, init } = fetchImpl.calls[0]
    assert.equal(url, '/api/public/events')
    assert.equal(init.method, 'POST')
    assert.equal(init.keepalive, true)
    assert.equal(JSON.parse(init.body).type, 'category_view')
  }
})

check('trackMenuEvent never throws and never rejects', () => {
  const throwingBeacon = {
    sendBeacon() {
      throw new TypeError('Illegal invocation')
    },
  }
  const fetchImpl = recordingFetch()
  assert.equal(trackMenuEvent(EVENT, { navigator: throwingBeacon, fetch: fetchImpl, storage: null }), true)
  assert.equal(fetchImpl.calls.length, 1)

  // A rejected fetch is swallowed rather than left unhandled.
  const rejected = Promise.reject(new Error('offline'))
  assert.equal(
    trackMenuEvent(EVENT, { navigator: null, fetch: recordingFetch(rejected), storage: null }),
    true,
  )

  const throwingFetch = () => {
    throw new Error('boom')
  }
  assert.equal(trackMenuEvent(EVENT, { navigator: null, fetch: throwingFetch, storage: null }), false)
  assert.equal(trackMenuEvent(EVENT, { navigator: null, fetch: null, storage: null }), false)
  assert.equal(trackMenuEvent(null, { navigator: null, fetch: null, storage: null }), false)
  assert.equal(trackMenuEvent(undefined), false)
})

check('trackMenuEvent sends nothing for an event it cannot attribute', () => {
  const nav = beaconNavigator(true)
  const fetchImpl = recordingFetch()
  const transport = { navigator: nav, fetch: fetchImpl, storage: null }
  assert.equal(trackMenuEvent({ ...EVENT, businessSlug: '' }, transport), false)
  assert.equal(trackMenuEvent({ ...EVENT, type: 'click' }, transport), false)
  assert.equal(nav.sent.length + fetchImpl.calls.length, 0)
})

/* ---------------------------------------------------------------- opt-out */

check('hasAnalyticsOptout reads exactly karecik_analytics_optout=1 from a cookie header', () => {
  assert.equal(OPTOUT_COOKIE, 'karecik_analytics_optout')
  assert.equal(hasAnalyticsOptout('karecik_analytics_optout=1'), true)
  assert.equal(hasAnalyticsOptout('a=b; karecik_analytics_optout=1; c=d'), true)
  assert.equal(hasAnalyticsOptout('a=b;karecik_analytics_optout = 1 '), true)
  assert.equal(hasAnalyticsOptout('karecik_analytics_optout=0'), false)
  assert.equal(hasAnalyticsOptout('karecik_analytics_optout='), false)
  assert.equal(hasAnalyticsOptout('karecik_analytics_optout'), false)
  // A cookie whose name merely contains the marker's is another cookie.
  assert.equal(hasAnalyticsOptout('x_karecik_analytics_optout=1'), false)
  assert.equal(hasAnalyticsOptout('karecik_analytics_optout_old=1'), false)
  assert.equal(hasAnalyticsOptout('session=karecik_analytics_optout=1'), false)
  assert.equal(hasAnalyticsOptout(''), false)
  assert.equal(hasAnalyticsOptout(null), false)
  assert.equal(hasAnalyticsOptout(undefined), false)
})

check('an opted-out browser sends nothing and gets no visitor id', () => {
  const nav = beaconNavigator(true)
  const fetchImpl = recordingFetch()
  const storage = memoryStorage()
  const cookie = 'lang=tr; karecik_analytics_optout=1'
  const transport = { navigator: nav, fetch: fetchImpl, storage, cookie }

  for (const type of MENU_EVENT_TYPES) {
    assert.equal(trackMenuEvent({ ...EVENT, type }, transport), false)
  }
  assert.equal(nav.sent.length, 0)
  assert.equal(fetchImpl.calls.length, 0)
  assert.equal(storage.data.has(VISITOR_ID_KEY), false)

  // Switched off again (the cookie expired): events flow as before.
  assert.equal(trackMenuEvent(EVENT, { ...transport, cookie: 'lang=tr' }), true)
  assert.equal(nav.sent.length, 1)
})

check('without a document there is no cookie, and tracking goes on', () => {
  // Plain node has no `document`: the default cookie source answers ''.
  assert.equal(browserCookies(), '')
  const nav = beaconNavigator(true)
  assert.equal(trackMenuEvent(EVENT, { navigator: nav, fetch: null, storage: null }), true)
  assert.equal(nav.sent.length, 1)

  // A document whose cookie getter throws (a sandboxed frame) is no document.
  const previous = Object.getOwnPropertyDescriptor(globalThis, 'document')
  Object.defineProperty(globalThis, 'document', {
    configurable: true,
    value: {
      get cookie() {
        throw new Error('SecurityError')
      },
    },
  })
  try {
    assert.equal(browserCookies(), '')
    const nav = beaconNavigator(true)
    assert.equal(trackMenuEvent(EVENT, { navigator: nav, fetch: null, storage: null }), true)
    assert.equal(nav.sent.length, 1)
  } finally {
    if (previous) Object.defineProperty(globalThis, 'document', previous)
    else delete globalThis.document
  }

  // And one that carries the marker is read by default.
  Object.defineProperty(globalThis, 'document', {
    configurable: true,
    value: { cookie: 'karecik_analytics_optout=1' },
  })
  try {
    assert.equal(browserCookies(), 'karecik_analytics_optout=1')
    const quiet = beaconNavigator(true)
    assert.equal(trackMenuEvent(EVENT, { navigator: quiet, fetch: null, storage: null }), false)
    assert.equal(quiet.sent.length, 0)
  } finally {
    if (previous) Object.defineProperty(globalThis, 'document', previous)
    else delete globalThis.document
  }
})

/* --------------------------------------------------------------- summary */

// Let the swallowed rejection above settle before reporting: an unhandled one
// would crash the process here.
await new Promise((resolve) => setTimeout(resolve, 0))

if (failures.length > 0) {
  console.error(`${failures.length} failed, ${passed} passed\n\n${failures.join('\n\n')}`)
  process.exit(1)
}
console.log(`menuAnalytics.test.mjs: all ${passed} checks passed`)
