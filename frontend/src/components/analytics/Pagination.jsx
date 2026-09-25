import { ChevronLeft, ChevronRight } from 'lucide-react'

import { formatCount, pageInfo } from '../../lib/analyticsFormat.js'

/**
 * The line under a paged table: which rows are on screen, and the two buttons
 * that move a page back or forward. Shared by the visit log and the change
 * history, which page the same way (limit / offset, `total` from the server).
 *
 * @param {number}   total    - rows the server matched
 * @param {number}   limit    - rows per page
 * @param {number}   offset   - index of the first row on this page
 * @param {Function} onChange - (nextOffset) => void
 * @param {boolean}  disabled - while a page is loading
 * @param {string}   noun     - what a row is, for the count ("kayıt")
 */
export default function Pagination({ total, limit, offset, onChange, disabled = false, noun = 'kayıt' }) {
  const info = pageInfo(total, limit, offset)

  // Nothing to page through and nowhere to go back to: the table's own empty
  // state says everything.
  if (info.total === 0 && !info.hasPrevious) return null

  return (
    <nav
      className="flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 px-4 py-3"
      aria-label="Sayfalar"
    >
      <p className="text-xs text-gray-500 tabular-nums">
        {info.start > 0
          ? `${formatCount(info.start)}–${formatCount(info.end)} / ${formatCount(info.total)} ${noun}`
          : `Bu sayfada ${noun} yok · toplam ${formatCount(info.total)}`}
      </p>

      <div className="flex items-center gap-2">
        <button
          type="button"
          className="btn-secondary btn-sm"
          onClick={() => onChange(info.previousOffset)}
          disabled={disabled || !info.hasPrevious}
        >
          <ChevronLeft className="h-3.5 w-3.5" aria-hidden="true" />
          Önceki
        </button>
        <span className="text-xs text-gray-500 tabular-nums">
          {info.page} / {info.pages}
        </span>
        <button
          type="button"
          className="btn-secondary btn-sm"
          onClick={() => onChange(info.nextOffset)}
          disabled={disabled || !info.hasNext}
        >
          Sonraki
          <ChevronRight className="h-3.5 w-3.5" aria-hidden="true" />
        </button>
      </div>
    </nav>
  )
}
