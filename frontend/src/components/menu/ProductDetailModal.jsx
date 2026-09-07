import { useEffect, useMemo, useRef, useState } from 'react'

import { formatPrice } from '../../lib/format'
import { BadgeIcon } from '../../themes/badges'
import { allergenLabel, findAllergen, t } from '../../locales/index.js'

/**
 * Bottom sheet shown when a product is tapped in the customer menu.
 *
 * It behaves like a native mobile drawer: it slides up from the bottom edge and
 * is dismissed by the overlay, by Escape, by the close button, or by dragging
 * its grab handle downwards. The drag is hand-written pointer events — no
 * gesture library.
 *
 * Option groups are presentation only: the selections feed the live total and
 * nothing else. There is no cart and nothing is ever submitted.
 *
 * @param {object|null} product  - Selected product (renders nothing when null)
 * @param {object}      business - PublicMenu.business (currency and colours)
 * @param {string}      language - Active language code
 * @param {Function}    onClose  - Closes the sheet
 */

/* The entrance deliberately does NOT use `animation-fill-mode: both`: a filled
   animation keeps applying its own `transform` after it finishes and would beat
   the inline transform the drag writes, freezing the sheet in place. With no
   delay there is nothing for `backwards` to cover anyway.

   Reduced motion keeps the fade and drops the movement, which is the part the
   preference is about; the drag itself still dismisses. */
const ANIMATION_STYLE = `
@keyframes karecikSheetUp {
  from { opacity: 0; transform: translateY(16px); }
  to   { opacity: 1; transform: translateY(0); }
}
@keyframes karecikSheetFade {
  from { opacity: 0; }
  to   { opacity: 1; }
}
@media (prefers-reduced-motion: reduce) {
  .karecik-sheet-panel {
    animation-name: karecikSheetFade !important;
  }
}
`

/** Breathing room between the last option group and the sticky total bar. */
const FOOTER_GAP = 20

/** Drag distance, in pixels, past which letting go dismisses the sheet. */
const DISMISS_DISTANCE = 90

/** Downward flick speed, in px/ms, that dismisses whatever the distance. */
const DISMISS_VELOCITY = 0.6

/** A flick still has to travel a little, so a shaky tap never dismisses. */
const FLICK_MIN_DISTANCE = 12

/** Milliseconds between the last movement and the release, past which the
    gesture counts as held rather than flicked. */
const FLICK_MAX_IDLE = 120

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

/**
 * Keeps only the option groups a customer can actually answer. The dashboard
 * already drops the rest, but a menu saved before that rule existed may still
 * carry an unnamed or empty group.
 */
function optionGroupsOf(product) {
  if (!Array.isArray(product?.options)) return []

  return product.options
    .map((group) => ({
      name: String(group?.name || '').trim(),
      type: group?.type === 'multiple' ? 'multiple' : 'single',
      required: Boolean(group?.required),
      items: (Array.isArray(group?.items) ? group.items : [])
        .map((item) => ({
          name: String(item?.name || '').trim(),
          price: Number.isFinite(Number(item?.price)) ? Number(item.price) : 0,
        }))
        .filter((item) => item.name !== ''),
    }))
    .filter((group) => group.name !== '' && group.items.length > 0)
}

/**
 * A required single-choice group answers itself with its first item; every
 * other group starts empty. Selections are item indexes inside their group.
 */
function defaultSelections(groups) {
  const initial = {}
  groups.forEach((group, index) => {
    initial[index] = group.type === 'single' && group.required ? [0] : []
  })
  return initial
}

/**
 * Tracks `prefers-reduced-motion` for the parts of the drawer CSS cannot reach.
 * The spring back to rest is a transition set from JavaScript, so the media
 * query has to be readable here as well as in the stylesheet above.
 */
function usePrefersReducedMotion() {
  const [prefersReducedMotion, setPrefersReducedMotion] = useState(
    () =>
      typeof window !== 'undefined' &&
      typeof window.matchMedia === 'function' &&
      window.matchMedia('(prefers-reduced-motion: reduce)').matches,
  )

  useEffect(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return undefined

    const query = window.matchMedia('(prefers-reduced-motion: reduce)')
    function onChange(event) {
      setPrefersReducedMotion(event.matches)
    }

    setPrefersReducedMotion(query.matches)
    query.addEventListener('change', onChange)
    return () => query.removeEventListener('change', onChange)
  }, [])

  return prefersReducedMotion
}

export default function ProductDetailModal({ product, business, language = 'tr', onClose }) {
  // Derived from the product, so the array identity changes exactly when the
  // sheet is handed a different product — including the null it gets on close.
  const optionGroups = useMemo(() => optionGroupsOf(product), [product])

  /* Selections belong to ONE product and are dropped rather than carried into
     the next sheet.

     The reset runs DURING render, not from an effect. MenuContent keeps this
     component mounted and only swaps its `product` prop, so the state survives
     the change; an effect would only clear it after the browser had already
     painted one frame of the previous product's ticked options and its total on
     top of the new product. Re-rendering from inside render is what React
     documents for adjusting state when a prop changes — the guard below makes
     it run exactly once per product, and the output of this pass is thrown away
     before it is ever committed. */
  const [selections, setSelections] = useState(() => defaultSelections(optionGroups))
  const [selectionsFor, setSelectionsFor] = useState(optionGroups)

  /* Drag to dismiss. `dragOffset` is how far the panel has been pushed down and
     is never negative — the sheet already sits on the bottom edge, so an upward
     drag has nowhere to go. The gesture itself lives in a ref because a
     pointermove must not wait for a re-render to know where it started. */
  const [dragOffset, setDragOffset] = useState(0)
  const [isDragging, setIsDragging] = useState(false)
  const dragRef = useRef(null)

  // Measured height of the sticky bottom bar, reserved by the scroll region.
  const footerRef = useRef(null)
  const [footerHeight, setFooterHeight] = useState(0)

  const prefersReducedMotion = usePrefersReducedMotion()

  if (selectionsFor !== optionGroups) {
    setSelectionsFor(optionGroups)
    setSelections(defaultSelections(optionGroups))

    /* A new product opens a sheet at rest, never one left half-dragged.

       The in-flight gesture is abandoned with it. Escape can close the sheet
       while a pointer is still down, and the handle unmounts before its own
       pointerup can reach releaseDrag — leaving a finished gesture in the ref.
       A mouse always reports pointerId 1, so the next sheet would match that
       stale record against an unrelated pointerup and dismiss itself from a
       startY belonging to the previous product. Clearing it here is idempotent,
       so React re-running this render pass changes nothing. */
    dragRef.current = null
    setIsDragging(false)
    setDragOffset(0)
  }

  // Close on Escape — registered unconditionally so the hook order stays stable.
  useEffect(() => {
    if (!product) return undefined

    function onKeyDown(event) {
      if (event.key === 'Escape') onClose?.()
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [product, onClose])

  /* The bottom bar is `sticky bottom-0` inside the sheet's own scroller, so the
     content above it has to reserve exactly its height. Measuring beats a
     hardcoded number: the bar grows when a long total or a translated label
     wraps onto a second line. */
  useEffect(() => {
    const node = footerRef.current
    if (!node) return undefined

    function measure() {
      setFooterHeight(node.offsetHeight)
    }

    measure()
    if (typeof ResizeObserver !== 'function') return undefined

    const observer = new ResizeObserver(measure)
    observer.observe(node)
    return () => observer.disconnect()
  }, [product])

  if (!product) return null

  const currency = business?.currency || 'TRY'
  const onAccentText = readableTextColor(business?.primary_color)

  const allergens = Array.isArray(product.allergens) ? product.allergens.filter(Boolean) : []
  const badges = Array.isArray(product.badges) ? product.badges.filter((badge) => badge?.text) : []
  // The chip states a fact, so anything that is not a positive number — null, a
  // zero, a stray string — reads as "unknown" and the chip simply stays away.
  const caloriesNumber = Number(product.calories)
  const calories = Number.isFinite(caloriesNumber) && caloriesNumber > 0 ? caloriesNumber : null
  const isDiscounted =
    Number(product.compare_price) > 0 && Number(product.compare_price) > Number(product.price)

  /* --------------------------------------------------------- option state */

  function chooseSingle(groupIndex, itemIndex) {
    setSelections((previous) => ({ ...previous, [groupIndex]: [itemIndex] }))
  }

  function toggleMultiple(groupIndex, itemIndex) {
    setSelections((previous) => {
      const chosen = previous[groupIndex] || []
      return {
        ...previous,
        [groupIndex]: chosen.includes(itemIndex)
          ? chosen.filter((value) => value !== itemIndex)
          : [...chosen, itemIndex],
      }
    })
  }

  // The live total: the product price plus every selected surcharge.
  const basePrice = Number.isFinite(Number(product.price)) ? Number(product.price) : 0
  const extras = optionGroups.reduce((sum, group, groupIndex) => {
    const chosen = selections[groupIndex] || []
    return (
      sum +
      chosen.reduce((groupSum, itemIndex) => groupSum + (group.items[itemIndex]?.price || 0), 0)
    )
  }, 0)
  const total = basePrice + extras
  // Prices are stored with two decimals, so a half-kuruş gap is float noise.
  const totalDiffersFromBase = Math.abs(total - basePrice) >= 0.005

  /* One struck-through price in the bar, never two. What the options did to the
     price is the more useful comparison, so it wins; the discount's reference
     price only appears while the total is still the plain product price. */
  let strikePrice = null
  if (totalDiffersFromBase) strikePrice = basePrice
  else if (isDiscounted) strikePrice = Number(product.compare_price)

  /* ------------------------------------------------------- drag to dismiss */

  /* The handle captures the pointer on the way down, so a drag that wanders off
     the pill — or off the sheet entirely — keeps reporting here, and the
     capture is released on both pointerup and pointercancel. `touch-action` is
     set on the handle ALONE: on the panel it would stop the sheet's own body
     from scrolling. */

  function startDrag(event) {
    // Ignore secondary mouse buttons; touch and pen report button 0.
    if (event.button > 0) return

    dragRef.current = {
      pointerId: event.pointerId,
      startY: event.clientY,
      previousY: event.clientY,
      previousTime: event.timeStamp,
      lastY: event.clientY,
      lastTime: event.timeStamp,
    }
    event.currentTarget.setPointerCapture?.(event.pointerId)
    setIsDragging(true)
  }

  function moveDrag(event) {
    const drag = dragRef.current
    if (!drag || drag.pointerId !== event.pointerId) return

    drag.previousY = drag.lastY
    drag.previousTime = drag.lastTime
    drag.lastY = event.clientY
    drag.lastTime = event.timeStamp
    setDragOffset(Math.max(0, event.clientY - drag.startY))
  }

  /** Releases the capture and hands back the finished gesture, if there was one. */
  function releaseDrag(event) {
    const drag = dragRef.current
    if (!drag || drag.pointerId !== event.pointerId) return null

    if (event.currentTarget.hasPointerCapture?.(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId)
    }
    dragRef.current = null
    setIsDragging(false)
    return drag
  }

  function endDrag(event) {
    const drag = releaseDrag(event)
    if (!drag) return

    // pointerup carries the release position, which is fresher than the last
    // move sample when the browser coalesced the tail of the gesture.
    const distance = Math.max(0, event.clientY - drag.startY)

    /* Speed over the last pair of move samples only, so a slow drag that ends
       in a flick still throws the sheet away — but a pause before letting go is
       not a flick, however fast the pointer was before it. */
    const sampled = drag.lastTime - drag.previousTime
    const speed = sampled > 0 ? (drag.lastY - drag.previousY) / sampled : 0
    const velocity = event.timeStamp - drag.lastTime <= FLICK_MAX_IDLE ? speed : 0

    const flicked = velocity > DISMISS_VELOCITY && distance > FLICK_MIN_DISTANCE
    if (distance > DISMISS_DISTANCE || flicked) {
      onClose?.()
      return
    }
    setDragOffset(0)
  }

  function cancelDrag(event) {
    if (releaseDrag(event)) setDragOffset(0)
  }

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
        className="karecik-sheet-panel max-h-[92vh] w-full max-w-lg overflow-y-auto rounded-t-2xl"
        style={{
          backgroundColor: 'var(--menu-bg)',
          color: 'var(--menu-text)',
          fontFamily: 'var(--menu-font)',
          animation: 'karecikSheetUp 220ms ease-out',
          transform: dragOffset > 0 ? `translateY(${dragOffset}px)` : undefined,
          // The spring back to rest is the one motion here that carries no
          // information, so reduced motion snaps instead. Dismissing is
          // untouched either way.
          transition:
            isDragging || prefersReducedMotion
              ? 'none'
              : 'transform 240ms cubic-bezier(0.22, 1, 0.36, 1)',
        }}
      >
        {/* image + grab handle. The handle rides on top of the image rather
            than pushing it down, so the full-bleed treatment survives and the
            drag target is there whether or not the product has a picture. */}
        <div className="relative">
          {product.image_url ? (
            <img src={product.image_url} alt="" className="h-56 w-full rounded-t-2xl object-cover" />
          ) : null}

          <div
            onPointerDown={startDrag}
            onPointerMove={moveDrag}
            onPointerUp={endDrag}
            onPointerCancel={cancelDrag}
            className={
              product.image_url
                ? 'absolute inset-x-0 top-0 flex justify-center pb-6 pt-3'
                : 'flex justify-center pb-1 pt-3'
            }
            style={{ touchAction: 'none', cursor: isDragging ? 'grabbing' : 'grab' }}
            role="presentation"
          >
            <span
              className="h-1.5 w-12 rounded-full"
              style={{
                backgroundColor: product.image_url
                  ? 'rgba(255, 255, 255, 0.75)'
                  : 'var(--menu-border)',
              }}
              aria-hidden="true"
            />
          </div>
        </div>

        {/* The reserved space is the bar's measured height PLUS a gap, not the
            height alone. Reserving exactly the height left the last option group
            touching the bar with nothing between them, which read as an overlap.
            The fallback keeps the first paint roomy while the measurement lands. */}
        <div
          className="flex flex-col gap-4 p-5"
          style={{ paddingBottom: `${(footerHeight || 72) + FOOTER_GAP}px` }}
        >
          {/* name + calories */}
          <div>
            <h2 className="text-lg font-semibold leading-snug">{product.name}</h2>

            {calories != null || product.is_featured ? (
              <div className="mt-2 flex flex-wrap items-center gap-1.5">
                {/* The same quiet bordered chip the list card carries. */}
                {calories != null ? (
                  <span
                    className="rounded-full px-2 py-0.5 text-[10px]"
                    style={{ border: '1px solid var(--menu-border)', color: 'var(--menu-muted)' }}
                  >
                    {calories} {t('kcal', language)}
                  </span>
                ) : null}

                {product.is_featured ? (
                  <span
                    className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium"
                    style={{ backgroundColor: 'var(--menu-surface)', color: 'var(--menu-muted)' }}
                  >
                    ⭐ {t('featured', language)}
                  </span>
                ) : null}
              </div>
            ) : null}
          </div>

          {/* custom badges designed by the business */}
          {badges.length > 0 ? (
            <div className="flex flex-wrap gap-1.5">
              {badges.map((badge, index) => (
                <span
                  key={badge.id || `${badge.text}-${index}`}
                  className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium"
                  style={{
                    backgroundColor: badge.bg_color || 'var(--menu-primary)',
                    color: badge.text_color || '#ffffff',
                  }}
                >
                  <BadgeIcon id={badge.icon} className="h-3.5 w-3.5 shrink-0" />
                  {badge.text}
                </span>
              ))}
            </div>
          ) : null}

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

          {/* option groups — the selection only moves the total in the bar below */}
          {optionGroups.map((group, groupIndex) => {
            const chosen = selections[groupIndex] || []
            const isSingle = group.type === 'single'

            return (
              <div key={`${group.name}-${groupIndex}`}>
                <div className="mb-2 flex flex-wrap items-center gap-2">
                  <h3 className="text-xs font-semibold uppercase tracking-wide">{group.name}</h3>
                  {group.required ? (
                    <span
                      className="rounded-full px-2 py-0.5 text-[10px] font-medium"
                      style={{ border: '1px solid var(--menu-border)', color: 'var(--menu-muted)' }}
                    >
                      Zorunlu
                    </span>
                  ) : null}
                </div>

                <div className="flex flex-col gap-1.5">
                  {group.items.map((item, itemIndex) => {
                    const isChosen = chosen.includes(itemIndex)
                    return (
                      <label
                        key={`${item.name}-${itemIndex}`}
                        className="flex cursor-pointer items-center gap-3 px-3 py-2.5 text-sm"
                        style={{
                          backgroundColor: 'var(--menu-surface)',
                          border: `1px solid ${
                            isChosen ? 'var(--menu-primary)' : 'var(--menu-border)'
                          }`,
                          borderRadius: 'var(--menu-radius)',
                        }}
                      >
                        <input
                          type={isSingle ? 'radio' : 'checkbox'}
                          name={`karecik-option-${groupIndex}`}
                          checked={isChosen}
                          onChange={() =>
                            isSingle
                              ? chooseSingle(groupIndex, itemIndex)
                              : toggleMultiple(groupIndex, itemIndex)
                          }
                          className="h-4 w-4 shrink-0"
                          style={{ accentColor: 'var(--menu-primary)' }}
                        />
                        <span className="min-w-0 flex-1">{item.name}</span>
                        {item.price > 0 ? (
                          <span
                            className="shrink-0 text-xs font-medium"
                            style={{ color: 'var(--menu-muted)' }}
                          >
                            +{formatPrice(item.price, currency)}
                          </span>
                        ) : null}
                      </label>
                    )
                  })}
                </div>
              </div>
            )
          })}
        </div>

        {/* Sticky bottom bar: the running total, then the close button.
            It sticks inside the sheet's own scroller, and the content wrapper
            above reserves exactly its height. The negative top margin hands
            that reserved space back to the bar — without it the sheet would end
            in an empty strip as tall as the bar itself. */}
        <div
          ref={footerRef}
          className="sticky bottom-0 z-10"
          style={{
            marginTop: footerHeight ? `-${footerHeight}px` : undefined,
            backgroundColor: 'var(--menu-bg)',
            borderTop: '1px solid var(--menu-border)',
          }}
        >
          <div className="flex items-baseline justify-between gap-3 px-5 py-3">
            <span
              className="text-[11px] font-medium uppercase tracking-wide"
              style={{ color: 'var(--menu-muted)' }}
            >
              Toplam
            </span>

            <span className="flex items-baseline gap-2">
              {strikePrice != null ? (
                <span className="text-xs line-through" style={{ color: 'var(--menu-muted)' }}>
                  {formatPrice(strikePrice, currency)}
                </span>
              ) : null}
              <span className="text-xl font-bold" style={{ color: 'var(--menu-primary)' }}>
                {formatPrice(total, currency)}
              </span>
            </span>
          </div>

          {/* close — clear of the home indicator on a phone, unchanged elsewhere */}
          <div
            className="px-5"
            style={{ paddingBottom: 'calc(1.25rem + env(safe-area-inset-bottom, 0px))' }}
          >
            <button
              type="button"
              onClick={() => onClose?.()}
              className="w-full px-4 py-3 text-sm font-medium"
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
    </div>
  )
}
