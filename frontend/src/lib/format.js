// Currency, price and date formatting helpers.
// The currency codes mirror backend/internal/utils/currency.go exactly.
//
// NOTE: the dashboard's copy stays Turkish on purpose — Karecik serves Turkish
// businesses. The customer menu's footer sentences at the bottom of this file
// are the exception: they go through the menu's locale dictionary. Only the
// code (identifiers and comments) is English.

import { comparableText } from './category.js'
import { trimSpace } from './contact.js'
import { t } from '../locales/index.js'

export const CURRENCIES = {
  TRY: { code: 'TRY', symbol: '₺', label: 'Türk Lirası', position: 'suffix' },
  USD: { code: 'USD', symbol: '$', label: 'Amerikan Doları', position: 'prefix' },
  EUR: { code: 'EUR', symbol: '€', label: 'Euro', position: 'prefix' },
  GBP: { code: 'GBP', symbol: '£', label: 'İngiliz Sterlini', position: 'prefix' },
  AZN: { code: 'AZN', symbol: '₼', label: 'Azerbaycan Manatı', position: 'suffix' },
  RUB: { code: 'RUB', symbol: '₽', label: 'Rus Rublesi', position: 'suffix' },
  SAR: { code: 'SAR', symbol: '﷼', label: 'Suudi Riyali', position: 'suffix' },
  AED: { code: 'AED', symbol: 'د.إ', label: 'BAE Dirhemi', position: 'suffix' },
}

export const CURRENCY_LIST = Object.values(CURRENCIES)

/** 145 -> "145,00 ₺"  |  145 (USD) -> "$145.00" */
export function formatPrice(value, currencyCode = 'TRY') {
  const currency = CURRENCIES[currencyCode] || CURRENCIES.TRY
  const parsed = Number(value)
  const safe = Number.isFinite(parsed) ? parsed : 0

  const locale = currency.position === 'prefix' ? 'en-US' : 'tr-TR'
  const text = safe.toLocaleString(locale, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })

  return currency.position === 'prefix' ? `${currency.symbol}${text}` : `${text} ${currency.symbol}`
}

/** Returns the symbol of a currency code. */
export function currencySymbol(currencyCode = 'TRY') {
  return (CURRENCIES[currencyCode] || CURRENCIES.TRY).symbol
}

/** The zone the price date is shown in; see formatDate. */
const PRICE_DATE_TIME_ZONE = 'Europe/Istanbul'

/**
 * ISO date -> "24.08.2026", as the calendar day in Europe/Istanbul.
 *
 * The zone is pinned instead of taken from the browser, so the day an owner
 * sees does not depend on the time zone their computer is set to: 22:30 UTC on
 * 23 August is already 24.08 in Istanbul, while a browser in London would have
 * printed 23.08. The same date is printed on the customer menu by the backend
 * (repository/menu.go -> BuildFooter), and the two must agree on the zone.
 *
 * The parts are joined by hand rather than taken from format(), so the order
 * and the separator stay "dd.mm.yyyy" whatever pattern the browser's locale
 * data prescribes. Should Intl or the zone be unavailable altogether, the
 * browser's local calendar day is the fallback — what this function always did.
 */
export function formatDate(isoDate) {
  if (!isoDate) return ''
  const date = new Date(isoDate)
  if (Number.isNaN(date.getTime())) return ''

  try {
    const parts = {}
    new Intl.DateTimeFormat('tr-TR', {
      timeZone: PRICE_DATE_TIME_ZONE,
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
    })
      .formatToParts(date)
      .forEach((part) => {
        parts[part.type] = part.value
      })

    if (parts.day && parts.month && parts.year) {
      return `${parts.day}.${parts.month}.${parts.year}`
    }
  } catch {
    /* no usable Intl time zone support: the local day below */
  }

  const day = String(date.getDate()).padStart(2, '0')
  const month = String(date.getMonth() + 1).padStart(2, '0')
  return `${day}.${month}.${date.getFullYear()}`
}

/** Parses typed input into a number: accepts both "12,50" and "12.50". */
export function parsePrice(text) {
  if (typeof text === 'number') return text
  const cleaned = String(text ?? '')
    .replace(/\s/g, '')
    .replace(/\./g, '')
    .replace(',', '.')
  const parsed = Number.parseFloat(cleaned)
  return Number.isFinite(parsed) ? parsed : 0
}

/** Formats a price for an editable input: 145 -> "145,00" */
export function priceToInput(value) {
  const parsed = Number(value)
  return (Number.isFinite(parsed) ? parsed : 0).toFixed(2).replace('.', ',')
}

/* ------------------------------------------------ customer menu footer copy */

/*
  The footer's two legal sentences, in the visitor's language.

  The server still sends its Turkish renderings (footer.price_note and
  footer.vat_note) for older clients, but a sentence can only be localized from
  its parts: `footer.price_date` ("YYYY-MM-DD", or "" while the menu hides the
  date) and the menu's own VAT fields. So these helpers build the sentence
  here, and the Turkish strings are only a last resort for a payload that
  carries nothing better.
*/

/**
 * The server's defaultVatNote (repository/menu.go): what footer.vat_note holds
 * when the VAT toggle is on and the owner's text is blank. It is also the
 * column default of menus.vat_note_text, so every menu whose owner never
 * touched the field carries exactly this sentence as its own text. Recognised
 * so that default, and only the default, can be shown in the visitor's
 * language.
 */
export const DEFAULT_VAT_NOTE = 'Fiyatlarımıza KDV dahildir.'

/**
 * Whether a VAT sentence is the stock Turkish one — compared the way product
 * texts are (comparableText), so a changed letter case, extra spaces or a
 * missing full stop do not make it the owner's own words.
 *
 * @param {string} text - already trimmed
 * @returns {boolean}
 */
function isDefaultVatNote(text) {
  return text !== '' && comparableText(text) === comparableText(DEFAULT_VAT_NOTE)
}

/**
 * The locale each menu language formats a day in. English is en-GB, because
 * "24 August 2026" reads the same everywhere while en-US would print the month
 * first; Arabic keeps the Gregorian calendar and Latin digits, matching the
 * prices beside it.
 */
const DAY_LOCALES = {
  tr: 'tr-TR',
  en: 'en-GB',
  de: 'de-DE',
  ru: 'ru-RU',
  ar: 'ar-u-ca-gregory-nu-latn',
  fr: 'fr-FR',
}

const ISO_DAY = /^(\d{4})-(\d{2})-(\d{2})$/

/**
 * { year, month, day } of a "YYYY-MM-DD" string naming a real calendar day,
 * otherwise null: '2026-02-30', '2026-8-4' and '' are all null.
 *
 * @param {unknown} value
 * @returns {{year: number, month: number, day: number}|null}
 */
export function parseIsoDay(value) {
  const match = typeof value === 'string' ? ISO_DAY.exec(value.trim()) : null
  if (!match) return null

  const year = Number(match[1])
  const month = Number(match[2])
  const day = Number(match[3])
  const check = new Date(Date.UTC(year, month - 1, day))
  if (
    check.getUTCFullYear() !== year ||
    check.getUTCMonth() !== month - 1 ||
    check.getUTCDate() !== day
  ) {
    return null
  }
  return { year, month, day }
}

/**
 * The calendar day of an instant in Europe/Istanbul, as "YYYY-MM-DD", or ''.
 * The same zone the server writes footer.price_date in (see formatDate above
 * for why it is pinned); Turkey has kept UTC+3 all year since 2016, so a
 * browser without zone data falls back to that fixed offset.
 *
 * @param {unknown} isoDate - e.g. business.price_updated_at
 * @returns {string}
 */
export function istanbulDay(isoDate) {
  if (!isoDate) return ''
  const date = new Date(isoDate)
  if (Number.isNaN(date.getTime())) return ''

  try {
    const parts = {}
    new Intl.DateTimeFormat('en-US', {
      timeZone: PRICE_DATE_TIME_ZONE,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      numberingSystem: 'latn',
    })
      .formatToParts(date)
      .forEach((part) => {
        parts[part.type] = part.value
      })
    const day = `${parts.year}-${parts.month}-${parts.day}`
    if (parseIsoDay(day)) return day
  } catch {
    /* no zone data: the fixed offset below */
  }

  const shifted = new Date(date.getTime() + 3 * 60 * 60 * 1000)
  return shifted.toISOString().slice(0, 10)
}

/**
 * A "YYYY-MM-DD" day written out for `language`: "24 Ağustos 2026",
 * "24 August 2026", "24. August 2026", "24 августа 2026 г.", "24 août 2026".
 * The month is spelled out on purpose — 05.08.2026 is the fifth of August in
 * Istanbul and the eighth of May to an American visitor.
 *
 * French writes the first of a month as an ordinal — "1er septembre 2026" —
 * which Intl does not do, so that one day is patched in by hand.
 *
 * Without Intl support the day falls back to "24.08.2026". An invalid day is ''.
 *
 * @param {unknown} isoDay
 * @param {string}  language
 * @returns {string}
 */
export function formatDay(isoDay, language = 'tr') {
  const parsed = parseIsoDay(isoDay)
  if (!parsed) return ''

  const { year, month, day } = parsed
  try {
    const formatter = new Intl.DateTimeFormat(DAY_LOCALES[language] || DAY_LOCALES.en, {
      day: 'numeric',
      month: 'long',
      year: 'numeric',
      timeZone: 'UTC',
    })
    // Midnight UTC, read in UTC: the day is a calendar day, not an instant,
    // and no zone may move it to its neighbour.
    const date = new Date(Date.UTC(year, month - 1, day))

    if (language === 'fr' && day === 1 && typeof formatter.formatToParts === 'function') {
      return formatter
        .formatToParts(date)
        .map((part) => (part.type === 'day' ? '1er' : part.value))
        .join('')
    }
    return formatter.format(date)
  } catch {
    return `${String(day).padStart(2, '0')}.${String(month).padStart(2, '0')}.${year}`
  }
}

/**
 * The footer's price-validity sentence, or '' for none.
 *
 *   show_price_date false          nothing — also over a saved payload that
 *                                  still carries a date, because the dashboard
 *                                  preview lays the unsaved toggle over it
 *   footer.price_date "YYYY-MM-DD" "Prices valid from 24 August 2026."
 *   show_price_date true, no date  the day of business.price_updated_at: the
 *                                  preview again, with the toggle just
 *                                  switched on over a payload saved without it
 *   price_date absent altogether   footer.price_note, the Turkish sentence of a
 *                                  server that predates price_date
 *
 * @param {object} business - PublicMenu.business, or the preview's draft over it
 * @param {object} footer   - PublicMenu.footer
 * @param {string} language
 * @returns {string}
 */
export function footerPriceSentence(business, footer, language = 'tr') {
  if (business?.show_price_date === false) return ''

  let day = parseIsoDay(footer?.price_date) ? footer.price_date.trim() : ''
  if (!day && business?.show_price_date === true) day = istanbulDay(business.price_updated_at)

  if (day) {
    const date = formatDay(day, language)
    // Russian dates end in the abbreviation "г.", and the sentence's own full
    // stop right after it would print "2026 г..".
    if (date) return t('priceValidFrom', language, { date }).replace(/\.\.$/, '.')
  }

  // hasOwnProperty rather than Object.hasOwn: QR menus open in old in-app
  // browsers, and the latter is too recent for some of them.
  const hasPriceDate =
    footer !== null &&
    typeof footer === 'object' &&
    Object.prototype.hasOwnProperty.call(footer, 'price_date')
  return hasPriceDate ? '' : trimSpace(footer?.price_note)
}

/**
 * The footer's VAT sentence, or '' for none.
 *
 *   show_vat_note false  nothing
 *   show_vat_note true   the owner's vat_note_text as typed (trimmed, as the
 *                        server trims it), or the localized "prices include
 *                        VAT" when that is blank
 *   no toggle at all     footer.vat_note
 *
 * Wherever it comes from, the stock Turkish sentence (DEFAULT_VAT_NOTE) is
 * swapped for the localized one. Most menus never change the field, so their
 * vat_note_text IS that sentence, and printing it as the owner's words left
 * the German, Arabic and every other footer with one Turkish line in it. Text
 * the owner actually wrote is shown as typed: nobody translated it.
 *
 * The business fields win over footer.vat_note for the same reason as the
 * price date: the dashboard preview's draft is only on the business.
 *
 * @param {object} business
 * @param {object} footer
 * @param {string} language
 * @returns {string}
 */
export function footerVatNote(business, footer, language = 'tr') {
  const enabled = business?.show_vat_note
  if (enabled === false) return ''

  const localized = (text) => (isDefaultVatNote(text) ? t('vatIncluded', language) : text)
  const serverNote = localized(trimSpace(footer?.vat_note))

  if (enabled === true) {
    if (typeof business.vat_note_text === 'string') {
      return localized(trimSpace(business.vat_note_text)) || t('vatIncluded', language)
    }
    return serverNote || t('vatIncluded', language)
  }

  return serverNote
}
