import { useEffect, useState } from 'react'
import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { Eye, EyeOff, GripVertical, Image as ImageIcon, Pencil, Star, Trash2 } from 'lucide-react'

import { fiyatCozumle, fiyatGirdiye, paraSimgesi } from '../../lib/format'
import { alerjenBul, alerjenEtiketi } from '../../locales/index.js'

/**
 * Menü editöründeki sürüklenebilir ürün satırı.
 * Fiyat, satır içinde hızlıca düzenlenebilir.
 *
 * @param {object}   urun
 * @param {string}   dil             - Düzenleme dili (translations anahtarı)
 * @param {string}   paraBirimi      - "TRY", "USD"...
 * @param {Function} duzenle         - Ürün düzenleme modalini açar
 * @param {Function} sil             - Silme onayını açar
 * @param {Function} fiyatDegisti    - (urunId, yeniFiyat) => void
 * @param {Function} aktiflikDegisti - (urunId, yeniDurum) => void
 */
export default function UrunSatiri({
  urun,
  dil = 'tr',
  paraBirimi = 'TRY',
  duzenle,
  sil,
  fiyatDegisti,
  aktiflikDegisti,
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: urun.id,
  })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  }

  const [fiyatMetni, setFiyatMetni] = useState(() => fiyatGirdiye(urun.price))

  // Fiyat dışarıdan değiştiyse (kaydetme, toplu güncelleme, hata sonrası geri alma)
  // girdi kutusunu tazele.
  useEffect(() => {
    setFiyatMetni(fiyatGirdiye(urun.price))
  }, [urun.price])

  const ad = urun.translations?.[dil]?.name || urun.translations?.tr?.name || 'İsimsiz ürün'
  const aciklama =
    urun.translations?.[dil]?.description || urun.translations?.tr?.description || ''

  const alerjenler = Array.isArray(urun.allergens) ? urun.allergens : []
  const pasif = urun.is_active === false

  /** Girdideki metni sayıya çevirir; değer gerçekten değiştiyse üst bileşene bildirir. */
  function fiyatiUygula() {
    const yeniFiyat = fiyatCozumle(fiyatMetni)
    const eskiFiyat = Number(urun.price)
    const guvenliEski = Number.isFinite(eskiFiyat) ? eskiFiyat : 0

    // Kuruş farkı yoksa istek atma.
    if (Math.abs(yeniFiyat - guvenliEski) < 0.005) {
      setFiyatMetni(fiyatGirdiye(guvenliEski))
      return
    }

    setFiyatMetni(fiyatGirdiye(yeniFiyat))
    fiyatDegisti?.(urun.id, yeniFiyat)
  }

  function tusaBasildi(olay) {
    if (olay.key === 'Enter') {
      olay.preventDefault()
      olay.currentTarget.blur() // blur -> fiyatiUygula
    } else if (olay.key === 'Escape') {
      setFiyatMetni(fiyatGirdiye(urun.price))
      olay.currentTarget.blur()
    }
  }

  const GozIkonu = pasif ? EyeOff : Eye

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={`flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-2 py-2 ${
        pasif ? 'opacity-60' : ''
      } ${isDragging ? 'surukleniyor relative z-10 shadow-panel' : ''}`}
    >
      {/* ------------------------------------------------------------ tutamaç */}
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

      {/* ------------------------------------------------------------- görsel */}
      <div className="h-10 w-10 shrink-0 overflow-hidden rounded-md bg-gray-100">
        {urun.image_url ? (
          <img src={urun.image_url} alt={ad} className="h-full w-full object-cover" />
        ) : (
          <div className="flex h-full w-full items-center justify-center">
            <ImageIcon className="h-4 w-4 text-gray-400" aria-hidden="true" />
          </div>
        )}
      </div>

      {/* --------------------------------------------------------------- bilgi */}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          {urun.is_featured ? (
            <Star
              className="h-3.5 w-3.5 shrink-0 fill-amber-500 text-amber-500"
              aria-label="Öne çıkan ürün"
            />
          ) : null}

          <span className="truncate text-sm font-medium text-gray-900">{ad}</span>

          {alerjenler.length > 0 ? (
            <span className="shrink-0 text-xs leading-none" aria-hidden="true">
              {alerjenler
                .map((kod) => alerjenBul(kod)?.emoji)
                .filter(Boolean)
                .join(' ')}
            </span>
          ) : null}
        </div>

        {aciklama ? <p className="truncate text-xs text-gray-500">{aciklama}</p> : null}

        {alerjenler.length > 0 ? (
          <span className="sr-only">
            {alerjenler.map((kod) => alerjenEtiketi(kod, 'tr')).join(', ')}
          </span>
        ) : null}
      </div>

      {/* ------------------------------------------------------ hızlı fiyat */}
      <div className="flex shrink-0 items-center gap-1">
        <input
          type="text"
          inputMode="decimal"
          value={fiyatMetni}
          onChange={(olay) => setFiyatMetni(olay.target.value)}
          onFocus={(olay) => olay.target.select()}
          onBlur={fiyatiUygula}
          onKeyDown={tusaBasildi}
          className="girdi w-24 px-2 py-1.5 text-right text-xs"
          aria-label={`${ad} fiyatı`}
          title="Fiyatı düzenlemek için tıklayın"
        />
        <span className="w-3 text-xs text-gray-500" aria-hidden="true">
          {paraSimgesi(paraBirimi)}
        </span>
      </div>

      {/* ------------------------------------------------------------ işlemler */}
      <div className="flex shrink-0 items-center">
        <button
          type="button"
          onClick={() => aktiflikDegisti?.(urun.id, !urun.is_active)}
          className="rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-700"
          aria-label={pasif ? 'Ürünü menüde göster' : 'Ürünü menüde gizle'}
          title={pasif ? 'Menüde gizli — göstermek için tıklayın' : 'Menüde görünür'}
        >
          <GozIkonu className="h-4 w-4" aria-hidden="true" />
        </button>

        <button
          type="button"
          onClick={duzenle}
          className="rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-700"
          aria-label="Ürünü düzenle"
          title="Düzenle"
        >
          <Pencil className="h-4 w-4" aria-hidden="true" />
        </button>

        <button
          type="button"
          onClick={sil}
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
