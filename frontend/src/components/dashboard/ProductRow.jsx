import { useEffect, useState } from 'react'
import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { Eye, EyeOff, GripVertical, Image as ImageIcon, Pencil, Star, Trash2 } from 'lucide-react'

import { currencySymbol, parsePrice, priceToInput } from '../../lib/format'
import { allergenLabel, findAllergen } from '../../locales/index.js'

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
