import { useEffect } from 'react'

import { formatPrice } from '../../lib/format'
import { allergenLabel, findAllergen, t } from '../../locales/index.js'

/**
 * Bottom sheet shown when a product is tapped in the customer menu.
 *
 * @param {object|null} product  - Selected product (renders nothing when null)
 * @param {object}      business - PublicMenu.business (currency and colours)
 * @param {string}      language - Active language code
 * @param {Function}    onClose  - Closes the sheet
 */

const ANIMATION_STYLE = `
@keyframes karecikSheetUp {
  from { opacity: 0; transform: translateY(16px); }
  to   { opacity: 1; transform: translateY(0); }
}
`

/** Picks a readable text colour to sit on top of the accent colour. */
function readableTextColor(hex) {
  const clean = String(hex || '').trim().replace('#', '')
  let r
  let g
  let b

  if (clean.length === 3) {
    r = Number.parseInt(clean[0] + clean[0], 16)
    g = Number.parseInt(clean[1] + clean[1], 16)
    b = Number.parseInt(clean[2] + clean[2], 16)
  } else if (clean.length === 6) {
    r = Number.parseInt(clean.slice(0, 2), 16)
    g = Number.parseInt(clean.slice(2, 4), 16)
    b = Number.parseInt(clean.slice(4, 6), 16)
  } else {
    return '#ffffff'
  }

  if ([r, g, b].some(Number.isNaN)) return '#ffffff'

  const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255
  return luminance > 0.6 ? '#111827' : '#ffffff'
}

export default function ProductDetailModal({ product, business, language = 'tr', onClose }) {
  // Close on Escape — registered unconditionally so the hook order stays stable.
  useEffect(() => {
    if (!product) return undefined

    function onKeyDown(event) {
      if (event.key === 'Escape') onClose?.()
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [product, onClose])

  if (!product) return null

  const currency = business?.currency || 'TRY'
  const onAccentText = readableTextColor(business?.primary_color)

  const allergens = Array.isArray(product.allergens) ? product.allergens.filter(Boolean) : []
  const isDiscounted =
    Number(product.compare_price) > 0 && Number(product.compare_price) > Number(product.price)

  return (
    <div
      className="fixed inset-0 z-50 flex items-end justify-center bg-black/50"
      onClick={() => onClose?.()}
      role="presentation"
    >
      <style>{ANIMATION_STYLE}</style>

      <div
        role="dialog"
        aria-modal="true"
        aria-label={product.name || 'Ürün detayı'}
        onClick={(event) => event.stopPropagation()}
        className="max-h-[92vh] w-full max-w-lg overflow-y-auto rounded-t-2xl"
        style={{
          backgroundColor: 'var(--menu-bg)',
          color: 'var(--menu-text)',
          fontFamily: 'var(--menu-font)',
          animation: 'karecikSheetUp 220ms ease-out both',
        }}
      >
        {product.image_url ? (
          <img src={product.image_url} alt="" className="h-56 w-full rounded-t-2xl object-cover" />
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
          {/* name + price */}
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <h2 className="text-lg font-semibold leading-snug">{product.name}</h2>

              {product.is_featured ? (
                <span
                  className="mt-1.5 inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium"
                  style={{ backgroundColor: 'var(--menu-surface)', color: 'var(--menu-muted)' }}
                >
                  ⭐ {t('featured', language)}
                </span>
              ) : null}
            </div>

            <div className="shrink-0 text-end">
              <div className="text-lg font-bold" style={{ color: 'var(--menu-primary)' }}>
                {formatPrice(product.price, currency)}
              </div>
              {isDiscounted ? (
                <div className="text-xs line-through" style={{ color: 'var(--menu-muted)' }}>
                  {formatPrice(product.compare_price, currency)}
                </div>
              ) : null}
            </div>
          </div>

          {/* description */}
          {product.description ? (
            <p className="text-sm leading-relaxed" style={{ color: 'var(--menu-muted)' }}>
              {product.description}
            </p>
          ) : null}

          {/* ingredients */}
          {product.ingredients ? (
            <div
              className="p-3"
              style={{
                backgroundColor: 'var(--menu-surface)',
                borderRadius: 'var(--menu-radius)',
                border: '1px solid var(--menu-border)',
              }}
            >
              <h3 className="mb-1 text-xs font-semibold uppercase tracking-wide">
                {t('ingredients', language)}
              </h3>
              <p className="text-sm leading-relaxed" style={{ color: 'var(--menu-muted)' }}>
                {product.ingredients}
              </p>
            </div>
          ) : null}

          {/* allergens */}
          {allergens.length > 0 ? (
            <div>
              <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide">
                {t('allergens', language)}
              </h3>
              <div className="flex flex-wrap gap-2">
                {allergens.map((code) => {
                  const allergen = findAllergen(code)
                  return (
                    <span
                      key={code}
                      className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs"
                      style={{
                        backgroundColor: 'var(--menu-surface)',
                        color: 'var(--menu-text)',
                        border: '1px solid var(--menu-border)',
                        borderRadius: '999px',
                      }}
                    >
                      <span aria-hidden="true">{allergen?.emoji || '•'}</span>
                      {allergenLabel(code, language)}
                    </span>
                  )
                })}
              </div>
            </div>
          ) : null}

          {/* close */}
          <button
            type="button"
            onClick={() => onClose?.()}
            className="mt-1 w-full px-4 py-3 text-sm font-medium"
            style={{
              backgroundColor: 'var(--menu-primary)',
              color: onAccentText,
              borderRadius: 'var(--menu-radius)',
            }}
          >
            {t('close', language)}
          </button>
        </div>
      </div>
    </div>
  )
}
