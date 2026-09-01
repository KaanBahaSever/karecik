import { useEffect } from 'react'
import { X } from 'lucide-react'

/**
 * Shared modal dialog.
 *
 * @param {boolean}  open        - Visibility
 * @param {Function} onClose     - Called when the dialog should close
 * @param {string}   title
 * @param {string}   description - Small caption below the title
 * @param {string}   width       - Tailwind max-w class (defaults to max-w-lg)
 * @param {node}     footer      - Buttons placed in the footer bar
 * @param {node}     children    - Dialog body
 */
export default function Modal({
  open,
  onClose,
  title,
  description,
  width = 'max-w-lg',
  footer,
  children,
}) {
  useEffect(() => {
    if (!open) return undefined

    const onKeyDown = (event) => {
      if (event.key === 'Escape') onClose?.()
    }
    document.addEventListener('keydown', onKeyDown)

    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'

    return () => {
      document.removeEventListener('keydown', onKeyDown)
      document.body.style.overflow = previousOverflow
    }
  }, [open, onClose])

  if (!open) return null

  return (
    <div
      className="fixed inset-0 z-50 flex items-end justify-center bg-black/40 p-0 sm:items-center sm:p-4"
      role="dialog"
      aria-modal="true"
      aria-label={title}
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose?.()
      }}
    >
      <div
        className={`flex max-h-[92vh] w-full ${width} flex-col rounded-t-2xl bg-white shadow-panel sm:rounded-2xl`}
      >
        <div className="flex items-start justify-between gap-4 border-b border-gray-200 px-5 py-4">
          <div className="min-w-0">
            <h2 className="truncate text-base font-semibold text-gray-900">{title}</h2>
            {description ? <p className="mt-0.5 text-sm text-gray-500">{description}</p> : null}
          </div>
          <button
            type="button"
            onClick={onClose}
            className="-mr-1 -mt-1 shrink-0 rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
            aria-label="Kapat"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-4">{children}</div>

        {footer ? (
          <div className="flex items-center justify-end gap-2 border-t border-gray-200 px-5 py-3">
            {footer}
          </div>
        ) : null}
      </div>
    </div>
  )
}
