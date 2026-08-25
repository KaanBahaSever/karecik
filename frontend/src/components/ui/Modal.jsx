import { useEffect } from 'react'
import { X } from 'lucide-react'

/**
 * Ortak modal penceresi.
 *
 * @param {boolean}  acik      - Görünürlük
 * @param {Function} kapat     - Kapatma çağrısı
 * @param {string}   baslik
 * @param {string}   aciklama  - Başlığın altındaki küçük açıklama
 * @param {string}   genislik  - Tailwind max-w sınıfı (varsayılan max-w-lg)
 * @param {node}     altBilgi  - Alt çubuğa yerleşecek butonlar
 * @param {node}     children  - Gövde
 */
export default function Modal({
  acik,
  kapat,
  baslik,
  aciklama,
  genislik = 'max-w-lg',
  altBilgi,
  children,
}) {
  useEffect(() => {
    if (!acik) return undefined

    const escDinleyici = (olay) => {
      if (olay.key === 'Escape') kapat?.()
    }
    document.addEventListener('keydown', escDinleyici)

    const oncekiOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'

    return () => {
      document.removeEventListener('keydown', escDinleyici)
      document.body.style.overflow = oncekiOverflow
    }
  }, [acik, kapat])

  if (!acik) return null

  return (
    <div
      className="fixed inset-0 z-50 flex items-end justify-center bg-black/40 p-0 sm:items-center sm:p-4"
      role="dialog"
      aria-modal="true"
      aria-label={baslik}
      onMouseDown={(olay) => {
        if (olay.target === olay.currentTarget) kapat?.()
      }}
    >
      <div
        className={`flex max-h-[92vh] w-full ${genislik} flex-col rounded-t-2xl bg-white shadow-panel sm:rounded-2xl`}
      >
        <div className="flex items-start justify-between gap-4 border-b border-gray-200 px-5 py-4">
          <div className="min-w-0">
            <h2 className="truncate text-base font-semibold text-gray-900">{baslik}</h2>
            {aciklama ? <p className="mt-0.5 text-sm text-gray-500">{aciklama}</p> : null}
          </div>
          <button
            type="button"
            onClick={kapat}
            className="-mr-1 -mt-1 shrink-0 rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
            aria-label="Kapat"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-4">{children}</div>

        {altBilgi ? (
          <div className="flex items-center justify-end gap-2 border-t border-gray-200 px-5 py-3">
            {altBilgi}
          </div>
        ) : null}
      </div>
    </div>
  )
}
