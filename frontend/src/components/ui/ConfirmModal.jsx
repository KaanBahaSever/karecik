import { AlertTriangle } from 'lucide-react'
import Modal from './Modal.jsx'

/**
 * Silme gibi geri alınamaz işlemler için onay penceresi.
 *
 * @param {boolean}  acik
 * @param {Function} kapat
 * @param {Function} onayla     - Onaylandığında çağrılır
 * @param {string}   baslik
 * @param {string}   mesaj
 * @param {string}   onayMetni  - Onay butonu yazısı (varsayılan "Sil")
 * @param {boolean}  islemde    - İstek sürerken butonları kilitler
 */
export default function OnayModal({
  acik,
  kapat,
  onayla,
  baslik = 'Emin misiniz?',
  mesaj = 'Bu işlem geri alınamaz.',
  onayMetni = 'Sil',
  islemde = false,
}) {
  return (
    <Modal
      acik={acik}
      kapat={kapat}
      baslik={baslik}
      genislik="max-w-md"
      altBilgi={
        <>
          <button type="button" className="btn-ikincil" onClick={kapat} disabled={islemde}>
            Vazgeç
          </button>
          <button
            type="button"
            className="btn bg-red-600 text-white hover:bg-red-700"
            onClick={onayla}
            disabled={islemde}
          >
            {islemde ? 'İşleniyor...' : onayMetni}
          </button>
        </>
      }
    >
      <div className="flex gap-3">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-red-50">
          <AlertTriangle className="h-5 w-5 text-red-600" aria-hidden="true" />
        </div>
        <p className="pt-2 text-sm leading-relaxed text-gray-600">{mesaj}</p>
      </div>
    </Modal>
  )
}
