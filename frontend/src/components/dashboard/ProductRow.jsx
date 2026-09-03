import { useEffect, useState } from 'react'
import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { Eye, EyeOff, GripVertical, Image as ImageIcon, Pencil, Star, Trash2 } from 'lucide-react'

import { currencySymbol, parsePrice, priceToInput } from '../../lib/format'
import { allergenLabel, findAllergen } from '../../locales/index.js'
import { BadgeIcon, DEFAULT_BADGE } from '../../themes/badges.js'

/* The row must stay one line tall, so only this many badges get a pill; the
   rest are folded into a "+N" chip. */
const VISIBLE_BADGES = 2

/**
 * Draggable product row in the menu editor.
 * The price can be edited inline.
 *
 * @param {object}   product
 * @param {string}   language        - Editing language (key inside translations)
 * @param {string}   currency        - "TRY", "USD"...
 * @param {Function} onEdit          - Opens the product modal
 * @param {Function} onDelete        - Opens the delete confirmation
 * @param {Function} onPriceChange   - (productId, newPrice) => void
 * @param {Function} onActiveChange  - (productId, nextState) => void
 */
export default function ProductRow({
  product,
  language = 'tr',
  currency = 'TRY',
  onEdit,
  onDelete,
  onPriceChange,
  onActiveChange,
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: product.id,
  })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  }

  const [priceText, setPriceText] = useState(() => priceToInput(product.price))

  // Refresh the input when the price changes from the outside (after saving,
  // a bulk update, or a rollback following an error).
  useEffect(() => {
    setPriceText(priceToInput(product.price))
  }, [product.price])

  const name =
    product.translations?.[language]?.name || product.translations?.tr?.name || 'İsimsiz ürün'
  const description =
    product.translations?.[language]?.description || product.translations?.tr?.description || ''

  const allergens = Array.isArray(product.allergens) ? product.allergens : []
  const hidden = product.is_active === false

  const badges = Array.isArray(product.badges)
    ? product.badges.filter((badge) => badge && badge.text)
    : []
  const shownBadges = badges.slice(0, VISIBLE_BADGES)
  const hiddenBadges = badges.slice(VISIBLE_BADGES)

  const calories =
    product.calories === null || product.calories === undefined ? null : Number(product.calories)

  /** Parses the input and notifies the parent only when the value really changed. */
  function applyPrice() {
    const nextPrice = parsePrice(priceText)
    const currentPrice = Number(product.price)
    const safeCurrent = Number.isFinite(currentPrice) ? currentPrice : 0

    // Skip the request when the difference is below one kuruş.
    if (Math.abs(nextPrice - safeCurrent) < 0.005) {
      setPriceText(priceToInput(safeCurrent))
      return
    }

    setPriceText(priceToInput(nextPrice))
    onPriceChange?.(product.id, nextPrice)
  }

  function onKeyDown(event) {
    if (event.key === 'Enter') {
      event.preventDefault()
      event.currentTarget.blur() // blur -> applyPrice
    } else if (event.key === 'Escape') {
      setPriceText(priceToInput(product.price))
      event.currentTarget.blur()
    }
  }

  const VisibilityIcon = hidden ? EyeOff : Eye

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={`flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-2 py-2 ${
        hidden ? 'opacity-60' : ''
      } ${isDragging ? 'dragging relative z-10 shadow-panel' : ''}`}
    >
      {/* ------------------------------------------------------- drag handle */}
      <button
        type="button"
        className="shrink-0 cursor-grab touch-none rounded-md p-1 text-gray-300 hover:bg-gray-100 hover:text-gray-500 active:cursor-grabbing"
        aria-label="Ürünü sürükle"
        title="Sürükleyerek sıralayın"
        {...attributes}
        {...listeners}
      >
        <GripVertical className="h-4 w-4" aria-hidden="true" />
      </button>

      {/* ------------------------------------------------------------ image */}
      <div className="h-10 w-10 shrink-0 overflow-hidden rounded-md bg-gray-100">
        {product.image_url ? (
          <img src={product.image_url} alt={name} className="h-full w-full object-cover" />
        ) : (
          <div className="flex h-full w-full items-center justify-center">
            <ImageIcon className="h-4 w-4 text-gray-400" aria-hidden="true" />
          </div>
        )}
      </div>

      {/* ------------------------------------------------------------- info */}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          {product.is_featured ? (
            <Star
              className="h-3.5 w-3.5 shrink-0 fill-amber-500 text-amber-500"
              aria-label="Öne çıkan ürün"
            />
          ) : null}

          <span className="truncate text-sm font-medium text-gray-900">{name}</span>

          {shownBadges.map((badge, index) => (
            <span
              key={badge.id || `${badge.text}-${index}`}
              title={badge.text}
              className="inline-flex max-w-[104px] shrink-0 items-center gap-0.5 rounded-full px-1.5 py-0.5 text-[10px] font-medium leading-none"
              style={{
                backgroundColor: badge.bg_color || DEFAULT_BADGE.bg_color,
                color: badge.text_color || DEFAULT_BADGE.text_color,
              }}
            >
              <BadgeIcon id={badge.icon} className="h-2.5 w-2.5 shrink-0" />
              <span className="truncate">{badge.text}</span>
            </span>
          ))}

          {hiddenBadges.length > 0 ? (
            <span
              title={hiddenBadges.map((badge) => badge.text).join(', ')}
              className="shrink-0 rounded-full bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium leading-none text-gray-500"
            >
              +{hiddenBadges.length}
            </span>
          ) : null}

          {Number.isFinite(calories) ? (
            <span className="shrink-0 text-[11px] leading-none text-gray-500">{calories} kcal</span>
          ) : null}

          {allergens.length > 0 ? (
            <span className="shrink-0 text-xs leading-none" aria-hidden="true">
              {allergens
                .map((code) => findAllergen(code)?.emoji)
                .filter(Boolean)
                .join(' ')}
            </span>
          ) : null}
        </div>

        {description ? <p className="truncate text-xs text-gray-500">{description}</p> : null}

        {allergens.length > 0 ? (
          <span className="sr-only">
            {allergens.map((code) => allergenLabel(code, 'tr')).join(', ')}
          </span>
        ) : null}
      </div>

      {/* ------------------------------------------------------ quick price */}
      <div className="flex shrink-0 items-center gap-1">
        <input
          type="text"
          inputMode="decimal"
          value={priceText}
          onChange={(event) => setPriceText(event.target.value)}
          onFocus={(event) => event.target.select()}
          onBlur={applyPrice}
          onKeyDown={onKeyDown}
          className="input w-24 px-2 py-1.5 text-right text-xs"
          aria-label={`${name} fiyatı`}
          title="Fiyatı düzenlemek için tıklayın"
        />
        <span className="w-3 text-xs text-gray-500" aria-hidden="true">
          {currencySymbol(currency)}
        </span>
      </div>

      {/* ---------------------------------------------------------- actions */}
      <div className="flex shrink-0 items-center">
        <button
          type="button"
          onClick={() => onActiveChange?.(product.id, !product.is_active)}
          className="rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-700"
          aria-label={hidden ? 'Ürünü menüde göster' : 'Ürünü menüde gizle'}
          title={hidden ? 'Menüde gizli — göstermek için tıklayın' : 'Menüde görünür'}
        >
          <VisibilityIcon className="h-4 w-4" aria-hidden="true" />
        </button>

        <button
          type="button"
          onClick={onEdit}
          className="rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-700"
          aria-label="Ürünü düzenle"
          title="Düzenle"
        >
          <Pencil className="h-4 w-4" aria-hidden="true" />
        </button>

        <button
          type="button"
          onClick={onDelete}
          className="rounded-md p-1.5 text-gray-400 hover:bg-red-50 hover:text-red-600"
          aria-label="Ürünü sil"
          title="Sil"
        >
          <Trash2 className="h-4 w-4" aria-hidden="true" />
        </button>
      </div>
    </div>
  )
}
