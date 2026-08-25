import { useRef, useState } from 'react'
import { ImagePlus, Loader2, Trash2 } from 'lucide-react'

import api from '../../lib/api'
import { useBildirim } from './Bildirim.jsx'

/**
 * Tek görsel yükleme alanı. Yükleme bitince sunucudan dönen URL'i bildirir.
 *
 * @param {string|null} deger     - Mevcut görsel adresi (/uploads/...)
 * @param {Function}    degisti   - (url|null) => void
 * @param {string}      etiket
 * @param {string}      ipucu
 * @param {boolean}     yuvarlak  - Logo için dairesel önizleme
 */
export default function GorselYukleyici({
  deger,
  degisti,
  etiket = 'Görsel',
  ipucu = 'JPG, PNG veya WEBP · en fazla 5 MB',
  yuvarlak = false,
}) {
  const [yukleniyor, setYukleniyor] = useState(false)
  const girdiRef = useRef(null)
  const bildirim = useBildirim()

  async function dosyaSecildi(olay) {
    const dosya = olay.target.files?.[0]
    olay.target.value = '' // aynı dosya tekrar seçilebilsin
    if (!dosya) return

    if (dosya.size > 5 * 1024 * 1024) {
      bildirim.hata('Dosya çok büyük. En fazla 5 MB yükleyebilirsiniz.')
      return
    }

    setYukleniyor(true)
    try {
      const sonuc = await api.upload(dosya)
      degisti?.(sonuc.url)
    } catch (hata) {
      bildirim.hata(hata.message)
    } finally {
      setYukleniyor(false)
    }
  }

  return (
    <div>
      <span className="etiket">{etiket}</span>

      <div className="flex items-center gap-3">
        <div
          className={`flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden border border-gray-200 bg-gray-50 ${
            yuvarlak ? 'rounded-full' : 'rounded-lg'
          }`}
        >
          {deger ? (
            <img src={deger} alt="" className="h-full w-full object-cover" />
          ) : (
            <ImagePlus className="h-5 w-5 text-gray-300" aria-hidden="true" />
          )}
        </div>

        <div className="flex flex-col gap-1.5">
          <div className="flex gap-2">
            <button
              type="button"
              className="btn-ikincil btn-kucuk"
              onClick={() => girdiRef.current?.click()}
              disabled={yukleniyor}
            >
              {yukleniyor ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin" /> Yükleniyor
                </>
              ) : (
                <>{deger ? 'Değiştir' : 'Görsel seç'}</>
              )}
            </button>

            {deger ? (
              <button
                type="button"
                className="btn-sessiz btn-kucuk text-red-600 hover:bg-red-50"
                onClick={() => degisti?.(null)}
                disabled={yukleniyor}
              >
                <Trash2 className="h-3.5 w-3.5" /> Kaldır
              </button>
            ) : null}
          </div>

          <p className="text-xs text-gray-400">{ipucu}</p>
        </div>
      </div>

      <input
        ref={girdiRef}
        type="file"
        accept="image/jpeg,image/png,image/webp,image/gif"
        className="hidden"
        onChange={dosyaSecildi}
      />
    </div>
  )
}
