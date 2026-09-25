// Assertions for the settings-side helpers of the owner panel:
// src/lib/footerSettings.js (contact placement and the footer summary),
// src/lib/imageUpload.js (the upload rules every image field
// shares) and src/lib/productText.js (description vs. ingredients).
//
//   node frontend/tests/dashboardSettings.test.mjs
//
// Nothing imports this file, so it never reaches the app bundle.

import assert from 'node:assert/strict'

import {
  contactPlacement,
  footerSummary,
  HOME_CONTACT_MODES,
  joinTurkish,
} from '../src/lib/footerSettings.js'
import {
  fileExtension,
  formatMegabytes,
  IMAGE_ACCEPT,
  imageUploadProblem,
  isAcceptedImage,
  MAX_UPLOAD_BYTES,
  UPLOAD_MESSAGES,
} from '../src/lib/imageUpload.js'
import { descriptionRepeatsIngredients } from '../src/lib/productText.js'

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

/* ------------------------------------------------------- contact placement */

check('the home view offers exactly inline, list and hidden', () => {
  assert.deepEqual(
    HOME_CONTACT_MODES.map((mode) => [mode.id, mode.label]),
    [
      ['inline', 'Yan yana'],
      ['list', 'Açık liste'],
      ['hidden', 'Gösterme'],
    ],
  )
  for (const mode of HOME_CONTACT_MODES) assert.ok(mode.help.length > 20)
})

check("a legacy 'footer' menu reads as hidden with the footer switch on", () => {
  assert.deepEqual(contactPlacement({ contact_display: 'footer' }), {
    contact_display: 'hidden',
    contact_in_footer: true,
  })
  // Even when a stale payload says the switch is off: 'footer' always meant "in the footer".
  assert.deepEqual(contactPlacement({ contact_display: 'footer', contact_in_footer: false }), {
    contact_display: 'hidden',
    contact_in_footer: true,
  })
})

check('the new pair is read as stored, with the column defaults for anything else', () => {
  assert.deepEqual(contactPlacement({ contact_display: 'list', contact_in_footer: true }), {
    contact_display: 'list',
    contact_in_footer: true,
  })
  assert.deepEqual(contactPlacement({ contact_display: 'hidden' }), {
    contact_display: 'hidden',
    contact_in_footer: false,
  })
  assert.deepEqual(contactPlacement({ contact_display: 'sideways', contact_in_footer: 'yes' }), {
    contact_display: 'inline',
    contact_in_footer: false,
  })
  assert.deepEqual(contactPlacement(null), { contact_display: 'inline', contact_in_footer: false })
  assert.deepEqual(contactPlacement({ contact_display: 'toString' }).contact_display, 'inline')
})

/* ------------------------------------------------------------ the summary */

check('Turkish lists join with commas and a final "ve"', () => {
  assert.equal(joinTurkish([]), '')
  assert.equal(joinTurkish(['a']), 'a')
  assert.equal(joinTurkish(['a', 'b']), 'a ve b')
  assert.equal(joinTurkish(['a', 'b', 'c']), 'a, b ve c')
  assert.equal(joinTurkish(['a', '', 'c']), 'a ve c')
  assert.equal(joinTurkish(null), '')
})

check('the footer summary names what will show, and the signature always', () => {
  assert.equal(
    footerSummary({
      contactInFooter: true,
      hasContact: true,
      showPriceDate: true,
      priceDate: '25.09.2026',
      showVatNote: true,
      showYerliUretim: true,
    }),
    'Alt bilgide iletişim bilgileri, fiyat geçerlilik tarihi (25.09.2026), KDV notu ve Yerli Üretim rozeti görünecek. Karecik imzası her zaman görünür.',
  )
  assert.equal(
    footerSummary({ showVatNote: true }),
    'Alt bilgide KDV notu görünecek. Karecik imzası her zaman görünür.',
  )
  assert.equal(
    footerSummary({}),
    'Alt bilgide başka bir şey görünmeyecek. Karecik imzası her zaman görünür.',
  )
  // The contact switch shows nothing when there is nothing to show.
  assert.equal(footerSummary({ contactInFooter: true, hasContact: false }), footerSummary({}))
  assert.ok(footerSummary().includes('Karecik imzası her zaman görünür'))
})

/* ---------------------------------------------------------------- uploads */

check('every panel image format is accepted, by type or by extension', () => {
  const accepted = [
    { name: 'logo.svg', type: 'image/svg+xml' },
    { name: 'logo.svg', type: '' }, // some browsers send no type for SVG
    { name: 'LOGO.SVG', type: 'application/octet-stream' },
    { name: 'logo', type: 'image/svg+xml; charset=utf-8' },
    { name: 'photo.jpeg', type: 'image/jpeg' },
    { name: 'photo.jpg', type: '' },
    { name: 'photo.jpeg', type: '' },
    { name: 'shot.png', type: 'image/png' },
    { name: 'pic.webp', type: 'image/webp' },
    { name: 'anim.gif', type: 'image/gif' },
    { name: 'weird', type: 'image/jpg' },
  ]
  for (const file of accepted) {
    assert.equal(isAcceptedImage(file), true, JSON.stringify(file))
    assert.equal(imageUploadProblem({ ...file, size: 1024 }), '', JSON.stringify(file))
  }

  const refused = [
    { name: 'menu.pdf', type: 'application/pdf' },
    { name: 'photo.heic', type: 'image/heic' },
    { name: 'logo.svg.exe', type: '' },
    { name: 'noextension', type: '' },
    { name: '.svg', type: '' },
  ]
  for (const file of refused) {
    assert.equal(imageUploadProblem({ ...file, size: 1024 }), UPLOAD_MESSAGES.type, JSON.stringify(file))
  }
  assert.equal(imageUploadProblem(null), UPLOAD_MESSAGES.type)
})

check('the size limit is the server default, with a readable message', () => {
  assert.equal(MAX_UPLOAD_BYTES, 5 * 1024 * 1024)
  const file = { name: 'big.png', type: 'image/png' }
  assert.equal(imageUploadProblem({ ...file, size: MAX_UPLOAD_BYTES }), '')
  assert.equal(
    imageUploadProblem({ ...file, size: MAX_UPLOAD_BYTES + 1 }),
    'Dosya çok büyük (5 MB). En fazla 5 MB yükleyebilirsiniz.',
  )
  assert.equal(
    imageUploadProblem({ ...file, size: 7.2 * 1024 * 1024 }),
    'Dosya çok büyük (7,2 MB). En fazla 5 MB yükleyebilirsiniz.',
  )
  assert.equal(imageUploadProblem({ ...file, size: 0 }), UPLOAD_MESSAGES.empty)
  assert.equal(formatMegabytes(512 * 1024), '0,5 MB')
  assert.equal(formatMegabytes(10 * 1024 * 1024), '10 MB')
})

check('the file picker lists every type and extension, SVG included', () => {
  for (const token of ['image/svg+xml', '.svg', 'image/png', '.jpeg', '.webp', '.gif']) {
    assert.ok(IMAGE_ACCEPT.split(',').includes(token), token)
  }
  assert.equal(fileExtension('a.b.PNG'), '.png')
  assert.equal(fileExtension('noext'), '')
  assert.equal(fileExtension('trailing.'), '')
})

/* ------------------------------------------------------------ product text */

check('description and ingredients are compared the way a reader would', () => {
  assert.equal(descriptionRepeatsIngredients('Espresso, süt, kakao.', 'espresso, süt, kakao'), true)
  assert.equal(descriptionRepeatsIngredients('ESPRESSO, SÜT', 'espresso, süt'), true)
  assert.equal(descriptionRepeatsIngredients('  Espresso,   SÜT. ', 'espresso, süt'), true)
  assert.equal(descriptionRepeatsIngredients('İÇİNDEKİLER', 'içindekiler'), true)
  assert.equal(
    descriptionRepeatsIngredients('Günlük kavrulan çekirdeklerle.', 'espresso, süt'),
    false,
  )
  // Two blank fields are fine; one blank field is fine.
  assert.equal(descriptionRepeatsIngredients('', ''), false)
  assert.equal(descriptionRepeatsIngredients('  ', ' '), false)
  assert.equal(descriptionRepeatsIngredients('espresso', ''), false)
  assert.equal(descriptionRepeatsIngredients(null, undefined), false)
})

if (failures.length > 0) {
  console.error(`${failures.length} failed, ${passed} passed\n\n${failures.join('\n\n')}`)
  process.exit(1)
}
console.log(`dashboardSettings.test.mjs: all ${passed} checks passed`)
