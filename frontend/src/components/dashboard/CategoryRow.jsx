import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { ChevronDown, ChevronRight, GripVertical, Pencil, Plus, Trash2 } from 'lucide-react'

/**
 * Draggable category card (accordion) in the menu editor.
 *
 * @param {object}   category     - Category object
 * @param {Array}    products     - Products of this category (for the count badge)
 * @param {boolean}  open         - Whether the accordion is expanded
 * @param {Function} onToggle     - Expand/collapse callback
 * @param {string}   language     - Editing language (key inside translations)
 * @param {Function} onEdit       - Opens the category modal
 * @param {Function} onDelete     - Opens the delete confirmation
 * @param {Function} onAddProduct - Opens the product modal for this category
 * @param {node}     children     - Content shown while expanded (product list + add button)
 */
export default function CategoryRow({
  category,
  products = [],
  open = false,
  onToggle,
  language = 'tr',
  onEdit,
  onDelete,
  onAddProduct,
  children,
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: category.id,
  })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  }

  const name =
    category.translations?.[language]?.name ||
    category.translations?.tr?.name ||
    'İsimsiz kategori'

  // The icon field may hold either a code such as "coffee" or an emoji.
  // Only emoji are rendered in the header.
  const iconEmoji =
    typeof category.icon === 'string' &&
    category.icon.trim() !== '' &&
    (category.icon.codePointAt(0) || 0) > 127
      ? category.icon
      : null

  const ToggleIcon = open ? ChevronDown : ChevronRight

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={`card overflow-hidden ${isDragging ? 'dragging relative z-10 shadow-panel' : ''}`}
    >
      {/* ---------------------------------------------------------- header */}
      <div className="flex items-center gap-1 px-2 py-2 sm:gap-2 sm:px-3">
        <button
          type="button"
          className="shrink-0 cursor-grab touch-none rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 active:cursor-grabbing"
          aria-label="Kategoriyi sürükle"
          title="Sürükleyerek sıralayın"
          {...attributes}
          {...listeners}
        >
          <GripVertical className="h-4 w-4" aria-hidden="true" />
        </button>

        <button
          type="button"
          onClick={onToggle}
          aria-expanded={open}
          className="flex min-w-0 flex-1 items-center gap-2 rounded-lg px-1 py-1.5 text-left hover:bg-gray-50"
        >
          {iconEmoji ? (
            <span className="shrink-0 text-base leading-none" aria-hidden="true">
              {iconEmoji}
            </span>
          ) : null}
          <span className="truncate text-sm font-semibold text-gray-900">{name}</span>
        </button>

        <span className="badge shrink-0 bg-brand-50 text-brand-700">{products.length} ürün</span>

        {category.is_active === false ? (
          <span className="badge shrink-0 bg-gray-100 text-gray-600">Gizli</span>
        ) : null}

        <div className="flex shrink-0 items-center">
          <button
            type="button"
            onClick={onAddProduct}
            className="rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-brand-600"
            aria-label="Bu kategoriye ürün ekle"
            title="Ürün ekle"
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
          </button>

          <button
            type="button"
            onClick={onEdit}
            className="rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-700"
            aria-label="Kategoriyi düzenle"
            title="Düzenle"
          >
            <Pencil className="h-4 w-4" aria-hidden="true" />
          </button>

          <button
            type="button"
            onClick={onDelete}
            className="rounded-md p-1.5 text-gray-400 hover:bg-red-50 hover:text-red-600"
            aria-label="Kategoriyi sil"
            title="Sil"
          >
            <Trash2 className="h-4 w-4" aria-hidden="true" />
          </button>

          <button
            type="button"
            onClick={onToggle}
            className="rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-700"
            aria-label={open ? 'Kategoriyi kapat' : 'Kategoriyi aç'}
            aria-expanded={open}
          >
            <ToggleIcon className="h-4 w-4" aria-hidden="true" />
          </button>
        </div>
      </div>

      {/* ---------------------------------------------------- product list */}
      {open ? <div>{children}</div> : null}
    </div>
  )
}
