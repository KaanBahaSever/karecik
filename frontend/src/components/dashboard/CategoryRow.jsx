import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { ChevronDown, ChevronRight, GripVertical, Pencil, Plus, Trash2 } from 'lucide-react'

/**
 * Menü editöründeki sürüklenebilir kategori kartı (akordeon).
 *
 * @param {object}   kategori  - Category nesnesi
 * @param {Array}    urunler   - Bu kategorideki ürünler (rozet sayısı için)
 * @param {boolean}  acik      - Akordeon açık mı
 * @param {Function} acKapat   - Aç/kapat çağrısı
 * @param {string}   dil       - Düzenleme dili (translations anahtarı)
 * @param {Function} duzenle   - Kategori düzenleme modalini açar
 * @param {Function} sil       - Silme onayını açar
 * @param {Function} urunEkle  - Bu kategoriye ürün ekleme modalini açar
 * @param {node}     cocuklar  - Açıkken gösterilen içerik (ürün listesi + "Ürün Ekle")
 */
export default function KategoriSatiri({
  kategori,
  urunler = [],
  acik = false,
  acKapat,
  dil = 'tr',
  duzenle,
  sil,
  urunEkle,
  cocuklar,
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: kategori.id,
  })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  }

  const ad =
    kategori.translations?.[dil]?.name ||
    kategori.translations?.tr?.name ||
    'İsimsiz kategori'

  // icon alanı "coffee" gibi bir kod ya da doğrudan emoji olabilir.
  // Yalnızca emoji ise başlıkta gösteriyoruz.
  const ikonEmoji =
    typeof kategori.icon === 'string' &&
    kategori.icon.trim() !== '' &&
    (kategori.icon.codePointAt(0) || 0) > 127
      ? kategori.icon
      : null

  const AcKapatIkonu = acik ? ChevronDown : ChevronRight

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={`kart overflow-hidden ${
        isDragging ? 'surukleniyor relative z-10 shadow-panel' : ''
      }`}
    >
      {/* ---------------------------------------------------------- başlık */}
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
          onClick={acKapat}
          aria-expanded={acik}
          className="flex min-w-0 flex-1 items-center gap-2 rounded-lg px-1 py-1.5 text-left hover:bg-gray-50"
        >
          {ikonEmoji ? (
            <span className="shrink-0 text-base leading-none" aria-hidden="true">
              {ikonEmoji}
            </span>
          ) : null}
          <span className="truncate text-sm font-semibold text-gray-900">{ad}</span>
        </button>

        <span className="rozet shrink-0 bg-marka-50 text-marka-700">{urunler.length} ürün</span>

        {kategori.is_active === false ? (
          <span className="rozet shrink-0 bg-gray-100 text-gray-600">Gizli</span>
        ) : null}

        <div className="flex shrink-0 items-center">
          <button
            type="button"
            onClick={urunEkle}
            className="rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-marka-600"
            aria-label="Bu kategoriye ürün ekle"
            title="Ürün ekle"
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
          </button>

          <button
            type="button"
            onClick={duzenle}
            className="rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-700"
            aria-label="Kategoriyi düzenle"
            title="Düzenle"
          >
            <Pencil className="h-4 w-4" aria-hidden="true" />
          </button>

          <button
            type="button"
            onClick={sil}
            className="rounded-md p-1.5 text-gray-400 hover:bg-red-50 hover:text-red-600"
            aria-label="Kategoriyi sil"
            title="Sil"
          >
            <Trash2 className="h-4 w-4" aria-hidden="true" />
          </button>

          <button
            type="button"
            onClick={acKapat}
            className="rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-700"
            aria-label={acik ? 'Kategoriyi kapat' : 'Kategoriyi aç'}
            aria-expanded={acik}
          >
            <AcKapatIkonu className="h-4 w-4" aria-hidden="true" />
          </button>
        </div>
      </div>

      {/* ------------------------------------------------------- ürün listesi */}
      {acik ? <div>{cocuklar}</div> : null}
    </div>
  )
}
