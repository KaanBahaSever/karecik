// Currency, price and date formatting helpers.
// The currency codes mirror backend/internal/utils/currency.go exactly.
//
// NOTE: user-facing copy stays Turkish on purpose — Karecik serves Turkish
// businesses. Only the code (identifiers and comments) is English.

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

/** ISO date -> "24.08.2026" */
export function formatDate(isoDate) {
  if (!isoDate) return ''
  const date = new Date(isoDate)
  if (Number.isNaN(date.getTime())) return ''

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
