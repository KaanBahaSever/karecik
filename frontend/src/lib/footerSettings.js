// The "İletişim ve alt bilgi" section of the menu settings page: where the
// contact details appear, and the one-line summary of what the footer at the
// bottom of the customer menu will hold.
//
// The live preview needs nothing from here: it lays the whole draft over the
// saved payload's `business`, and the customer footer reads the switches
// (contact_in_footer, show_price_date, show_vat_note, ...) from there, so an
// unsaved switch shows in the phone at once.
//
// Pure functions only - no React, no DOM - so frontend/tests/
// dashboardSettings.test.mjs runs them with plain node.
//
// NOTE: the copy is Turkish on purpose - it is panel UI, read by the owner.

/* ------------------------------------------------------ contact placement */

/**
 * Where the contact details sit on the menu's home view, bound to
 * `contact_display`. The footer is a separate switch now (`contact_in_footer`),
 * so these three say nothing about it - see the shared API contract (C3).
 *
 * The ids mirror the values PUT /api/menus/:id stores. The labels and the help
 * lines are the settings page's own: they sit beside the one control that
 * shows them.
 */
export const HOME_CONTACT_MODES = [
  {
    id: 'inline',
    label: 'Yan yana',
    help: 'Ana ekranda küçük düğmeler halinde yan yana dizilir; dokunulan düğmenin bilgisi açılır.',
  },
  {
    id: 'list',
    label: 'Açık liste',
    help: 'Ana ekranda tüm bilgiler her zaman açık bir liste halinde görünür.',
  },
  {
    id: 'hidden',
    label: 'Gösterme',
    help: 'Ana ekranda gösterilmez. İsterseniz aşağıdan yalnızca alt bilgide gösterebilirsiniz.',
  },
]

/**
 * The two contact settings of a stored menu, read the way the settings page
 * edits them.
 *
 * A menu saved before the footer became its own switch may still hold the
 * legacy `contact_display: 'footer'` - "nothing on the home view, the footer
 * only". That is exactly 'hidden' with the footer switch on, which is how the
 * server's migration converts it too, so the page shows it that way and a
 * save writes the new pair. Anything else unknown or missing is 'inline', the
 * column default, and a missing `contact_in_footer` is off, its default.
 *
 * @param {unknown} menu
 * @returns {{ contact_display: 'inline'|'list'|'hidden', contact_in_footer: boolean }}
 */
export function contactPlacement(menu) {
  const record = menu && typeof menu === 'object' ? menu : {}
  const stored = record.contact_display

  if (stored === 'footer') return { contact_display: 'hidden', contact_in_footer: true }

  return {
    contact_display: HOME_CONTACT_MODES.some((mode) => mode.id === stored) ? stored : 'inline',
    contact_in_footer: record.contact_in_footer === true,
  }
}

/* ---------------------------------------------------------- footer summary */

/** "a", "a ve b", "a, b ve c" - the Turkish list. */
export function joinTurkish(items) {
  const list = (Array.isArray(items) ? items : []).filter(Boolean)
  if (list.length <= 1) return list.join('')
  return `${list.slice(0, -1).join(', ')} ve ${list[list.length - 1]}`
}

/**
 * The one line under the footer switches that says what a customer will see
 * at the bottom of the menu once the draft is saved.
 *
 * @param {object}  options
 * @param {boolean} options.contactInFooter - the "İletişim bilgileri" switch
 * @param {boolean} options.hasContact      - whether any contact detail is filled in;
 *                                            the switch alone shows nothing without one
 * @param {boolean} options.showPriceDate
 * @param {string}  options.priceDate       - "25.09.2026", printed beside the notice
 * @param {boolean} options.showVatNote
 * @param {boolean} options.showYerliUretim
 * @returns {string}
 */
export function footerSummary({
  contactInFooter = false,
  hasContact = false,
  showPriceDate = false,
  priceDate = '',
  showVatNote = false,
  showYerliUretim = false,
} = {}) {
  const parts = []
  if (contactInFooter && hasContact) parts.push('iletişim bilgileri')
  if (showPriceDate) {
    parts.push(priceDate ? `fiyat geçerlilik tarihi (${priceDate})` : 'fiyat geçerlilik tarihi')
  }
  if (showVatNote) parts.push('KDV notu')
  if (showYerliUretim) parts.push('Yerli Üretim rozeti')

  if (parts.length === 0) {
    return 'Alt bilgide başka bir şey görünmeyecek. Karecik imzası her zaman görünür.'
  }
  return `Alt bilgide ${joinTurkish(parts)} görünecek. Karecik imzası her zaman görünür.`
}
