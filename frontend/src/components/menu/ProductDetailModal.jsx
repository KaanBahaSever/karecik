import { useEffect } from 'react'

import { fiyatBicimle } from '../../lib/format'
import { alerjenBul, alerjenEtiketi, metin } from '../../locales/index.js'

/**
 * Müşteri menüsünde ürüne dokununca alttan açılan detay paneli.
 *
 * @param {object|null} urun     - Seçili ürün (null ise hiçbir şey render edilmez)
 * @param {object}      business - PublicMenu.business (para birimi vb.)
 * @param {string}      dil      - Aktif dil kodu
 * @param {function}    kapat    - Paneli kapatır
 */

const ANIMASYON_STILI = `
@keyframes panelYukari {
  from { opacity: 0; transform: translateY(16px); }
  to   { opacity: 1; transform: translateY(0); }
}
`

/** Vurgu rengi üzerinde okunaklı metin rengi seçer. */
function okunakliMetinRengi(hex) {
  const temiz = String(hex || '').trim().replace('#', '')
  let r
  let g
  let b

  if (temiz.length === 3) {
    r = Number.parseInt(temiz[0] + temiz[0], 16)
    g = Number.parseInt(temiz[1] + temiz[1], 16)
    b = Number.parseInt(temiz[2] + temiz[2], 16)
  } else if (temiz.length === 6) {
    r = Number.parseInt(temiz.slice(0, 2), 16)
    g = Number.parseInt(temiz.slice(2, 4), 16)
    b = Number.parseInt(temiz.slice(4, 6), 16)
  } else {
    return '#ffffff'
  }

  if ([r, g, b].some(Number.isNaN)) return '#ffffff'

  const parlaklik = (0.299 * r + 0.587 * g + 0.114 * b) / 255
  return parlaklik > 0.6 ? '#111827' : '#ffffff'
}

export default function UrunDetayModal({ urun, business, dil = 'tr', kapat }) {
  // Escape ile kapatma — ürün yokken de güvenle çalışsın diye koşulsuz kurulur.
  useEffect(() => {
    if (!urun) return undefined

    function tusaBasildi(olay) {
      if (olay.key === 'Escape') kapat?.()
    }

    window.addEventListener('keydown', tusaBasildi)
    return () => window.removeEventListener('keydown', tusaBasildi)
  }, [urun, kapat])

  if (!urun) return null

  const paraBirimi = business?.currency || 'TRY'
  const vurguMetni = okunakliMetinRengi(business?.primary_color)

  const alerjenler = Array.isArray(urun.allergens) ? urun.allergens.filter(Boolean) : []
  const indirimliMi =
    Number(urun.compare_price) > 0 && Number(urun.compare_price) > Number(urun.price)

  return (
    <div
      className="fixed inset-0 z-50 flex items-end justify-center bg-black/50"
      onClick={() => kapat?.()}
      role="presentation"
    >
      <style>{ANIMASYON_STILI}</style>

      <div
        role="dialog"
        aria-modal="true"
        aria-label={urun.name || 'Ürün detayı'}
        onClick={(olay) => olay.stopPropagation()}
        className="max-h-[92vh] w-full max-w-lg overflow-y-auto rounded-t-2xl"
        style={{
          backgroundColor: 'var(--menu-bg)',
          color: 'var(--menu-text)',
          fontFamily: 'var(--menu-font)',
          animation: 'panelYukari 220ms ease-out both',
        }}
      >
        {urun.image_url ? (
          <img src={urun.image_url} alt="" className="h-56 w-full rounded-t-2xl object-cover" />
        ) : (
          <div className="flex justify-center pt-3">
            <span
              className="h-1.5 w-12 rounded-full"
              style={{ backgroundColor: 'var(--menu-border)' }}
              aria-hidden="true"
            />
          </div>
        )}

        <div className="flex flex-col gap-4 p-5">
          {/* Ad + fiyat */}
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <h2 className="text-lg font-semibold leading-snug">{urun.name}</h2>

              {urun.is_featured ? (
                <span
                  className="mt-1.5 inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium"
                  style={{ backgroundColor: 'var(--menu-surface)', color: 'var(--menu-muted)' }}
                >
                  ⭐ {metin('oneCikan', dil)}
                </span>
              ) : null}
            </div>

            <div className="shrink-0 text-end">
              <div className="text-lg font-bold" style={{ color: 'var(--menu-primary)' }}>
                {fiyatBicimle(urun.price, paraBirimi)}
              </div>
              {indirimliMi ? (
                <div className="text-xs line-through" style={{ color: 'var(--menu-muted)' }}>
                  {fiyatBicimle(urun.compare_price, paraBirimi)}
                </div>
              ) : null}
            </div>
          </div>

          {/* Açıklama */}
          {urun.description ? (
            <p className="text-sm leading-relaxed" style={{ color: 'var(--menu-muted)' }}>
              {urun.description}
            </p>
          ) : null}

          {/* İçindekiler */}
          {urun.ingredients ? (
            <div
              className="p-3"
              style={{
                backgroundColor: 'var(--menu-surface)',
                borderRadius: 'var(--menu-radius)',
                border: '1px solid var(--menu-border)',
              }}
            >
              <h3 className="mb-1 text-xs font-semibold uppercase tracking-wide">
                {metin('icindekiler', dil)}
              </h3>
              <p className="text-sm leading-relaxed" style={{ color: 'var(--menu-muted)' }}>
                {urun.ingredients}
              </p>
            </div>
          ) : null}

          {/* Alerjenler */}
          {alerjenler.length > 0 ? (
            <div>
              <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide">
                {metin('alerjenler', dil)}
              </h3>
              <div className="flex flex-wrap gap-2">
                {alerjenler.map((kod) => {
                  const bilgi = alerjenBul(kod)
                  return (
                    <span
                      key={kod}
                      className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs"
                      style={{
                        backgroundColor: 'var(--menu-surface)',
                        color: 'var(--menu-text)',
                        border: '1px solid var(--menu-border)',
                        borderRadius: '999px',
                      }}
                    >
                      <span aria-hidden="true">{bilgi?.emoji || '•'}</span>
                      {alerjenEtiketi(kod, dil)}
                    </span>
                  )
                })}
              </div>
            </div>
          ) : null}

          {/* Kapat */}
          <button
            type="button"
            onClick={() => kapat?.()}
            className="mt-1 w-full px-4 py-3 text-sm font-medium"
            style={{
              backgroundColor: 'var(--menu-primary)',
              color: vurguMetni,
              borderRadius: 'var(--menu-radius)',
            }}
          >
            {metin('kapat', dil)}
          </button>
        </div>
      </div>
    </div>
  )
}
