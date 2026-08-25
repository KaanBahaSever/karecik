import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'

import api from '../../lib/api'
import { dilBul } from '../../locales/index.js'
import Modal from '../ui/Modal.jsx'
import GorselYukleyici from '../ui/GorselYukleyici.jsx'
import { useBildirim } from '../ui/Bildirim.jsx'

/** İkon ızgarasında sunulan emoji seçenekleri (backend TEXT bekliyor). */
const IKON_SECENEKLERI = [
  '☕', '🥤', '🍽️', '🍰', '🍕', '🍔', '🥗', '🍺', '🍷',
  '🧃', '🥐', '🍳', '🍜', '🌮', '🍦', '🫖', '🧊', '⭐',
]

/**
 * Kategori ekleme / düzenleme modalı.
 *
 * @param {boolean}       acik
 * @param {Function}      kapat
 * @param {object|null}   kategori       - null ise yeni kayıt
 * @param {string[]}      diller         - işletmenin menü dilleri, ör. ['tr','en']
 * @param {string}        varsayilanDil  - ad alanının zorunlu olduğu dil
 * @param {Function}      kaydedildi     - (kategori) => void
 */
export default function KategoriModal({
  acik,
  kapat,
  kategori,
  diller,
  varsayilanDil,
  kaydedildi,
}) {
  const bildirim = useBildirim()

  const dilListesi = Array.isArray(diller) && diller.length > 0 ? diller : ['tr']
  const anaDil = dilListesi.includes(varsayilanDil) ? varsayilanDil : dilListesi[0]
  const dilAnahtari = dilListesi.join(',')

  const [aktifDil, setAktifDil] = useState(anaDil)
  const [ceviriler, setCeviriler] = useState({})
  const [ikon, setIkon] = useState('')
  const [gorselUrl, setGorselUrl] = useState(null)
  const [menudeGoster, setMenudeGoster] = useState(true)
  const [adHatasi, setAdHatasi] = useState('')
  const [kaydediliyor, setKaydediliyor] = useState(false)

  /* Modal her açıldığında formu gelen kategoriye göre doldur. */
  useEffect(() => {
    if (!acik) return

    const baslangic = {}
    dilListesi.forEach((kod) => {
      baslangic[kod] = {
        name: kategori?.translations?.[kod]?.name || '',
        description: kategori?.translations?.[kod]?.description || '',
      }
    })

    setCeviriler(baslangic)
    setIkon(kategori?.icon || '')
    setGorselUrl(kategori?.image_url || null)
    setMenudeGoster(kategori?.is_active !== false)
    setAktifDil(anaDil)
    setAdHatasi('')
    setKaydediliyor(false)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [acik, kategori, anaDil, dilAnahtari])

  function alanDegistir(dilKodu, alan, deger) {
    setCeviriler((oncekiler) => ({
      ...oncekiler,
      [dilKodu]: { ...(oncekiler[dilKodu] || { name: '', description: '' }), [alan]: deger },
    }))
    if (alan === 'name' && dilKodu === anaDil && deger.trim()) setAdHatasi('')
  }

  async function kaydet() {
    const anaAd = (ceviriler[anaDil]?.name || '').trim()
    if (!anaAd) {
      setAktifDil(anaDil)
      setAdHatasi('Kategori adı zorunludur.')
      return
    }

    // Adı boş olan dilleri hiç gönderme.
    const translations = {}
    dilListesi.forEach((kod) => {
      const ceviri = ceviriler[kod]
      if (!ceviri) return
      const ad = (ceviri.name || '').trim()
      if (!ad) return
      translations[kod] = { name: ad, description: (ceviri.description || '').trim() }
    })

    const payload = {
      translations,
      icon: ikon || null,
      image_url: gorselUrl || null,
      is_active: menudeGoster,
    }

    setKaydediliyor(true)
    try {
      const sonuc = kategori
        ? await api.updateCategory(kategori.id, payload)
        : await api.createCategory(payload)

      kaydedildi?.(sonuc)
      bildirim.basari('Kategori kaydedildi.')
      kapat?.()
    } catch (hata) {
      bildirim.hata(hata.message)
    } finally {
      setKaydediliyor(false)
    }
  }

  const aktifCeviri = ceviriler[aktifDil] || { name: '', description: '' }
  const cokDilli = dilListesi.length > 1

  return (
    <Modal
      acik={acik}
      kapat={kaydediliyor ? () => {} : kapat}
      baslik={kategori ? 'Kategoriyi Düzenle' : 'Yeni Kategori'}
      aciklama="Kategori adı, görseli ve menüde görünürlüğü."
      genislik="max-w-lg"
      altBilgi={
        <>
          <button type="button" className="btn-ikincil" onClick={kapat} disabled={kaydediliyor}>
            İptal
          </button>
          <button type="button" className="btn-birincil" onClick={kaydet} disabled={kaydediliyor}>
            {kaydediliyor ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" /> Kaydediliyor
              </>
            ) : (
              'Kaydet'
            )}
          </button>
        </>
      }
    >
      <div className="space-y-5">
        {/* ---------------------------------------------------- dil sekmeleri */}
        {cokDilli ? (
          <div className="flex flex-wrap gap-1 rounded-lg bg-gray-100 p-1">
            {dilListesi.map((kod) => {
              const dil = dilBul(kod)
              const secili = kod === aktifDil
              return (
                <button
                  key={kod}
                  type="button"
                  onClick={() => setAktifDil(kod)}
                  className={`flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium ${
                    secili
                      ? 'bg-white text-gray-900 shadow-kart'
                      : 'text-gray-500 hover:text-gray-700'
                  }`}
                >
                  <span aria-hidden="true">{dil.kisa}</span>
                  {dil.label}
                  {kod === anaDil ? <span className="text-xs text-marka-600">•</span> : null}
                </button>
              )
            })}
          </div>
        ) : null}

        {/* ------------------------------------------------------ ad/açıklama */}
        <div>
          <label className="etiket" htmlFor={`kategori-ad-${aktifDil}`}>
            Kategori adı
            {aktifDil === anaDil ? <span className="text-red-500"> *</span> : null}
          </label>
          <input
            id={`kategori-ad-${aktifDil}`}
            type="text"
            className="girdi"
            value={aktifCeviri.name}
            onChange={(olay) => alanDegistir(aktifDil, 'name', olay.target.value)}
            placeholder={aktifDil === 'tr' ? 'Örn. Sıcak İçecekler' : 'Örn. Hot Drinks'}
            maxLength={80}
            autoComplete="off"
          />
          {aktifDil === anaDil && adHatasi ? (
            <p className="hata-metni">{adHatasi}</p>
          ) : (
            <p className="yardim">
              {aktifDil === anaDil
                ? 'Menüde bu ad görünür.'
                : `${dilBul(aktifDil).label} çevirisi boş bırakılırsa bu dil gönderilmez.`}
            </p>
          )}
        </div>

        <div>
          <label className="etiket" htmlFor={`kategori-aciklama-${aktifDil}`}>
            Açıklama (opsiyonel)
          </label>
          <textarea
            id={`kategori-aciklama-${aktifDil}`}
            className="girdi resize-none"
            rows={2}
            value={aktifCeviri.description}
            onChange={(olay) => alanDegistir(aktifDil, 'description', olay.target.value)}
            placeholder="Kategori hakkında kısa bir not"
            maxLength={240}
          />
        </div>

        {/* ------------------------------------------------------------ ikon */}
        <div>
          <span className="etiket">Kategori ikonu (opsiyonel)</span>
          <div className="grid grid-cols-9 gap-1.5">
            {IKON_SECENEKLERI.map((emoji) => {
              const secili = ikon === emoji
              return (
                <button
                  key={emoji}
                  type="button"
                  onClick={() => setIkon(secili ? '' : emoji)}
                  aria-pressed={secili}
                  className={`flex h-9 items-center justify-center rounded-lg border text-lg ${
                    secili
                      ? 'border-marka-600 bg-marka-50'
                      : 'border-gray-200 bg-white hover:bg-gray-50'
                  }`}
                >
                  <span aria-hidden="true">{emoji}</span>
                </button>
              )
            })}
          </div>
          <p className="yardim">
            {ikon ? 'Seçimi kaldırmak için ikona tekrar tıklayın.' : 'İsteğe bağlı.'}
          </p>
        </div>

        {/* ---------------------------------------------------------- görsel */}
        <GorselYukleyici
          deger={gorselUrl}
          degisti={setGorselUrl}
          etiket="Kategori görseli (opsiyonel)"
        />

        {/* --------------------------------------------------------- anahtar */}
        <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-gray-200 p-3">
          <input
            type="checkbox"
            className="mt-0.5 h-4 w-4 shrink-0 rounded border-gray-300 text-marka-600 focus:ring-marka-500"
            checked={menudeGoster}
            onChange={(olay) => setMenudeGoster(olay.target.checked)}
          />
          <span className="min-w-0">
            <span className="block text-sm font-medium text-gray-700">Menüde göster</span>
            <span className="block text-xs text-gray-500">
              Kapatırsanız kategori ve ürünleri müşteri menüsünde görünmez.
            </span>
          </span>
        </label>
      </div>
    </Modal>
  )
}
