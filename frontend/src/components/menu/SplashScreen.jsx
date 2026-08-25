import { useEffect, useRef } from 'react'

/**
 * Müşteri menüsü açılırken bir kez gösterilen karşılama (splash) ekranı.
 *
 * @param {object}   business - PublicMenu.business (logo_url, name, splash_*)
 * @param {function} bitti    - Süre dolunca veya ekrana dokununca çağrılır
 */

/* Çok kısa ve tek seferlik açılış animasyonu — kütüphane kullanılmaz. */
const ANIMASYON_STILI = `
@keyframes kutuAcil {
  from { opacity: 0; transform: scale(0.94); }
  to   { opacity: 1; transform: scale(1); }
}
`

/** "#0f172a" veya "#fff" -> { r, g, b }; çözülemezse null. */
function hexRenkAyristir(hex) {
  const temiz = String(hex || '').trim().replace('#', '')

  if (temiz.length === 3) {
    const r = Number.parseInt(temiz[0] + temiz[0], 16)
    const g = Number.parseInt(temiz[1] + temiz[1], 16)
    const b = Number.parseInt(temiz[2] + temiz[2], 16)
    if ([r, g, b].some(Number.isNaN)) return null
    return { r, g, b }
  }

  if (temiz.length === 6) {
    const r = Number.parseInt(temiz.slice(0, 2), 16)
    const g = Number.parseInt(temiz.slice(2, 4), 16)
    const b = Number.parseInt(temiz.slice(4, 6), 16)
    if ([r, g, b].some(Number.isNaN)) return null
    return { r, g, b }
  }

  return null
}

/**
 * Arka planın parlaklığına göre okunaklı metin rengi seçer.
 * Logonun rengini bilmediğimiz için metin rengini zemine göre belirliyoruz.
 */
function okunakliMetinRengi(arkaPlan) {
  const rgb = hexRenkAyristir(arkaPlan)
  if (!rgb) return '#ffffff'

  const parlaklik = (0.299 * rgb.r + 0.587 * rgb.g + 0.114 * rgb.b) / 255
  return parlaklik > 0.6 ? '#111827' : '#ffffff'
}

export default function KarsilamaEkrani({ business, bitti }) {
  const arkaPlan = business?.splash_bg_color || '#0f172a'
  const metinRengi = okunakliMetinRengi(arkaPlan)
  const koyuMetin = metinRengi === '#111827'

  const sayi = Number(business?.splash_duration)
  const sure = Number.isFinite(sayi) && sayi > 0 ? Math.min(sayi, 6000) : 1500

  const isletmeAdi = String(business?.name || '').trim()
  const basHarf = isletmeAdi ? isletmeAdi.charAt(0).toLocaleUpperCase('tr') : '•'

  // Prop her renderda yenilense bile zamanlayıcı sıfırlanmasın.
  const bittiRef = useRef(bitti)
  useEffect(() => {
    bittiRef.current = bitti
  }, [bitti])

  useEffect(() => {
    const zamanlayici = setTimeout(() => {
      bittiRef.current?.()
    }, sure)

    return () => clearTimeout(zamanlayici)
  }, [sure])

  function atla() {
    bittiRef.current?.()
  }

  return (
    <div
      role="button"
      tabIndex={0}
      aria-label="Karşılama ekranını geç"
      onClick={atla}
      onKeyDown={(olay) => {
        if (olay.key === 'Enter' || olay.key === ' ' || olay.key === 'Escape') atla()
      }}
      className="fixed inset-0 z-50 flex cursor-pointer select-none items-center justify-center px-6"
      style={{ backgroundColor: arkaPlan, color: metinRengi }}
    >
      <style>{ANIMASYON_STILI}</style>

      <div
        className="flex flex-col items-center gap-4 text-center"
        style={{ animation: 'kutuAcil 320ms ease-out both' }}
      >
        {business?.logo_url ? (
          <img src={business.logo_url} alt="" className="h-24 w-24 object-contain" />
        ) : (
          <div
            className="flex h-24 w-24 items-center justify-center rounded-full text-4xl font-semibold"
            style={{
              backgroundColor: koyuMetin ? 'rgba(17, 24, 39, 0.08)' : 'rgba(255, 255, 255, 0.14)',
              color: metinRengi,
            }}
            aria-hidden="true"
          >
            {basHarf}
          </div>
        )}

        {isletmeAdi ? <h1 className="text-2xl font-semibold">{isletmeAdi}</h1> : null}

        {business?.splash_text ? (
          <p className="max-w-xs text-sm" style={{ opacity: 0.8 }}>
            {business.splash_text}
          </p>
        ) : null}
      </div>
    </div>
  )
}
