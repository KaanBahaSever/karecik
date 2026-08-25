// Para birimi, fiyat ve tarih biçimlendirme yardımcıları.
// Kodlar backend'deki internal/utils/currency.go ile birebir aynıdır.

export const PARA_BIRIMLERI = {
  TRY: { code: 'TRY', symbol: '₺', label: 'Türk Lirası', position: 'suffix' },
  USD: { code: 'USD', symbol: '$', label: 'Amerikan Doları', position: 'prefix' },
  EUR: { code: 'EUR', symbol: '€', label: 'Euro', position: 'prefix' },
  GBP: { code: 'GBP', symbol: '£', label: 'İngiliz Sterlini', position: 'prefix' },
  AZN: { code: 'AZN', symbol: '₼', label: 'Azerbaycan Manatı', position: 'suffix' },
  RUB: { code: 'RUB', symbol: '₽', label: 'Rus Rublesi', position: 'suffix' },
  SAR: { code: 'SAR', symbol: '﷼', label: 'Suudi Riyali', position: 'suffix' },
  AED: { code: 'AED', symbol: 'د.إ', label: 'BAE Dirhemi', position: 'suffix' },
}

export const PARA_BIRIMI_LISTESI = Object.values(PARA_BIRIMLERI)

/** "145" -> "145,00 ₺"  |  "145" (USD) -> "$145.00" */
export function fiyatBicimle(deger, paraBirimi = 'TRY') {
  const birim = PARA_BIRIMLERI[paraBirimi] || PARA_BIRIMLERI.TRY
  const sayi = Number(deger)
  const guvenli = Number.isFinite(sayi) ? sayi : 0

  const yerel = birim.position === 'prefix' ? 'en-US' : 'tr-TR'
  const metin = guvenli.toLocaleString(yerel, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })

  return birim.position === 'prefix' ? `${birim.symbol}${metin}` : `${metin} ${birim.symbol}`
}

/** Para birimi simgesini döndürür. */
export function paraSimgesi(paraBirimi = 'TRY') {
  return (PARA_BIRIMLERI[paraBirimi] || PARA_BIRIMLERI.TRY).symbol
}

/** ISO tarih -> "24.08.2026" */
export function tarihBicimle(isoTarih) {
  if (!isoTarih) return ''
  const tarih = new Date(isoTarih)
  if (Number.isNaN(tarih.getTime())) return ''

  const gun = String(tarih.getDate()).padStart(2, '0')
  const ay = String(tarih.getMonth() + 1).padStart(2, '0')
  return `${gun}.${ay}.${tarih.getFullYear()}`
}

/** Girdi metnini sayıya çevirir: "12,50" ve "12.50" ikisini de kabul eder. */
export function fiyatCozumle(metin) {
  if (typeof metin === 'number') return metin
  const temiz = String(metin ?? '')
    .replace(/\s/g, '')
    .replace(/\./g, '')
    .replace(',', '.')
  const sayi = Number.parseFloat(temiz)
  return Number.isFinite(sayi) ? sayi : 0
}

/** Fiyatı düzenlenebilir girdi biçimine çevirir: 145 -> "145,00" */
export function fiyatGirdiye(deger) {
  const sayi = Number(deger)
  return (Number.isFinite(sayi) ? sayi : 0).toFixed(2).replace('.', ',')
}
