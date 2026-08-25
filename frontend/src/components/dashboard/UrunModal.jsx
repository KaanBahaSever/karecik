import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'

import api from '../../lib/api'
import { fiyatCozumle, fiyatGirdiye, paraSimgesi } from '../../lib/format'
import { ALERJENLER, dilBul } from '../../locales/index.js'
import Modal from '../ui/Modal.jsx'
import GorselYukleyici from '../ui/GorselYukleyici.jsx'
import { useBildirim } from '../ui/Bildirim.jsx'

const BOS_CEVIRI = { name: '', description: '', ingredients: '' }

/** "145,00" / "145.00" gibi bir girdinin sayıya çevrilebilir olduğunu doğrular. */
function fiyatMetniGecerliMi(metin) {
  const temiz = String(metin ?? '').trim()
  if (!temiz) return false
  if (!/^[0-9.,\s]+$/.test(temiz)) return false
  return /[0-9]/.test(temiz)
}

/** Kategori seçicide gösterilecek ad. */
function kategoriAdi(kategori, varsayilanDil) {
  const ceviriler = kategori?.translations || {}
  return (
    ceviriler[varsayilanDil]?.name ||
    ceviriler.tr?.name ||
    Object.values(ceviriler)[0]?.name ||
    'Adsız kategori'
  )
}

/**
 * Ürün ekleme / düzenleme modalı.
 *
 * @param {boolean}     acik
 * @param {Function}    kapat
 * @param {object|null} urun               - null ise yeni kayıt
 * @param {Array}       kategoriler
 * @param {string}      seciliKategoriId   - yeni üründe ön seçili kategori
 * @param {string[]}    diller
 * @param {string}      varsayilanDil
 * @param {string}      paraBirimi         - 'TRY', 'EUR' ...
 * @param {Function}    kaydedildi         - (urun) => void
 */
export default function UrunModal({
  acik,
  kapat,
  urun,
  kategoriler,
  seciliKategoriId,
  diller,
  varsayilanDil,
  paraBirimi,
  kaydedildi,
}) {
  const bildirim = useBildirim()

  const kategoriListesi = Array.isArray(kategoriler) ? kategoriler : []
  const dilListesi = Array.isArray(diller) && diller.length > 0 ? diller : ['tr']
  const anaDil = dilListesi.includes(varsayilanDil) ? varsayilanDil : dilListesi[0]
  const dilAnahtari = dilListesi.join(',')
  const simge = paraSimgesi(paraBirimi)

  const [kategoriId, setKategoriId] = useState('')
  const [aktifDil, setAktifDil] = useState(anaDil)
  const [ceviriler, setCeviriler] = useState({})
  const [fiyat, setFiyat] = useState('')
  const [karsilastirmaFiyati, setKarsilastirmaFiyati] = useState('')
  const [gorselUrl, setGorselUrl] = useState(null)
  const [alerjenler, setAlerjenler] = useState([])
  const [menudeGoster, setMenudeGoster] = useState(true)
  const [oneCikar, setOneCikar] = useState(false)
  const [hatalar, setHatalar] = useState({})
  const [kaydediliyor, setKaydediliyor] = useState(false)

  /* Modal açıldığında formu doldur. */
  useEffect(() => {
    if (!acik) return

    const baslangic = {}
    dilListesi.forEach((kod) => {
      baslangic[kod] = {
        name: urun?.translations?.[kod]?.name || '',
        description: urun?.translations?.[kod]?.description || '',
        ingredients: urun?.translations?.[kod]?.ingredients || '',
      }
    })

    setCeviriler(baslangic)
    setKategoriId(urun?.category_id || seciliKategoriId || kategoriListesi[0]?.id || '')
    setFiyat(urun ? fiyatGirdiye(urun.price) : '')
    setKarsilastirmaFiyati(
      urun?.compare_price !== null && urun?.compare_price !== undefined
        ? fiyatGirdiye(urun.compare_price)
        : '',
    )
    setGorselUrl(urun?.image_url || null)
    setAlerjenler(Array.isArray(urun?.allergens) ? [...urun.allergens] : [])
    setMenudeGoster(urun?.is_active !== false)
    setOneCikar(Boolean(urun?.is_featured))
    setAktifDil(anaDil)
    setHatalar({})
    setKaydediliyor(false)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [acik, urun, seciliKategoriId, anaDil, dilAnahtari])

  function alanDegistir(dilKodu, alan, deger) {
    setCeviriler((oncekiler) => ({
      ...oncekiler,
      [dilKodu]: { ...(oncekiler[dilKodu] || BOS_CEVIRI), [alan]: deger },
    }))
    if (alan === 'name' && dilKodu === anaDil && deger.trim()) {
      setHatalar((oncekiler) => ({ ...oncekiler, ad: '' }))
    }
  }

  function alerjenDegistir(kod) {
    setAlerjenler((oncekiler) =>
      oncekiler.includes(kod) ? oncekiler.filter((a) => a !== kod) : [...oncekiler, kod],
    )
  }

  async function kaydet() {
    const yeniHatalar = {}

    if (!kategoriId) yeniHatalar.kategori = 'Ürünün ekleneceği kategoriyi seçin.'

    const anaAd = (ceviriler[anaDil]?.name || '').trim()
    if (!anaAd) yeniHatalar.ad = 'Ürün adı zorunludur.'

    if (!fiyatMetniGecerliMi(fiyat)) {
      yeniHatalar.fiyat = 'Geçerli bir fiyat girin. Örn. 145,00'
    } else if (fiyatCozumle(fiyat) < 0) {
      yeniHatalar.fiyat = 'Fiyat sıfırdan küçük olamaz.'
    }

    const karsilastirmaDolu = String(karsilastirmaFiyati).trim() !== ''
    if (karsilastirmaDolu) {
      if (!fiyatMetniGecerliMi(karsilastirmaFiyati)) {
        yeniHatalar.karsilastirma = 'Geçerli bir fiyat girin. Örn. 180,00'
      } else if (fiyatCozumle(karsilastirmaFiyati) < 0) {
        yeniHatalar.karsilastirma = 'Fiyat sıfırdan küçük olamaz.'
      }
    }

    if (Object.keys(yeniHatalar).length > 0) {
      setHatalar(yeniHatalar)
      if (yeniHatalar.ad) setAktifDil(anaDil)
      return
    }

    // Adı boş olan dilleri gönderme.
    const translations = {}
    dilListesi.forEach((kod) => {
      const ceviri = ceviriler[kod]
      if (!ceviri) return
      const ad = (ceviri.name || '').trim()
      if (!ad) return
      translations[kod] = {
        name: ad,
        description: (ceviri.description || '').trim(),
        ingredients: (ceviri.ingredients || '').trim(),
      }
    })

    const payload = {
      category_id: kategoriId,
      translations,
      price: fiyatCozumle(fiyat),
      compare_price: karsilastirmaDolu ? fiyatCozumle(karsilastirmaFiyati) : null,
      image_url: gorselUrl || null,
      allergens: alerjenler,
      is_active: menudeGoster,
      is_featured: oneCikar,
    }

    setKaydediliyor(true)
    try {
      const sonuc = urun
        ? await api.updateProduct(urun.id, payload)
        : await api.createProduct(payload)

      kaydedildi?.(sonuc)
      bildirim.basari('Ürün kaydedildi.')
      kapat?.()
    } catch (hata) {
      bildirim.hata(hata.message)
    } finally {
      setKaydediliyor(false)
    }
  }

  const aktifCeviri = ceviriler[aktifDil] || BOS_CEVIRI
  const cokDilli = dilListesi.length > 1

  return (
    <Modal
      acik={acik}
      kapat={kaydediliyor ? () => {} : kapat}
      baslik={urun ? 'Ürünü Düzenle' : 'Yeni Ürün'}
      aciklama="Ürün bilgileri, fiyatı ve alerjen uyarıları."
      genislik="max-w-2xl"
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
        {/* --------------------------------------------------------- kategori */}
        <div>
          <label className="etiket" htmlFor="urun-kategori">
            Kategori <span className="text-red-500">*</span>
          </label>
          <select
            id="urun-kategori"
            className="girdi"
            value={kategoriId}
            onChange={(olay) => {
              setKategoriId(olay.target.value)
              setHatalar((oncekiler) => ({ ...oncekiler, kategori: '' }))
            }}
          >
            <option value="">Kategori seçin</option>
            {kategoriListesi.map((kategori) => (
              <option key={kategori.id} value={kategori.id}>
                {kategori.icon ? `${kategori.icon} ` : ''}
                {kategoriAdi(kategori, varsayilanDil)}
              </option>
            ))}
          </select>
          {hatalar.kategori ? <p className="hata-metni">{hatalar.kategori}</p> : null}
        </div>

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

        {/* ------------------------------------------------ çok dilli alanlar */}
        <div className="space-y-4">
          <div>
            <label className="etiket" htmlFor={`urun-ad-${aktifDil}`}>
              Ürün adı
              {aktifDil === anaDil ? <span className="text-red-500"> *</span> : null}
            </label>
            <input
              id={`urun-ad-${aktifDil}`}
              type="text"
              className="girdi"
              value={aktifCeviri.name}
              onChange={(olay) => alanDegistir(aktifDil, 'name', olay.target.value)}
              placeholder={aktifDil === 'tr' ? 'Örn. Filtre Kahve' : 'Örn. Filter Coffee'}
              maxLength={120}
              autoComplete="off"
            />
            {aktifDil === anaDil && hatalar.ad ? (
              <p className="hata-metni">{hatalar.ad}</p>
            ) : aktifDil !== anaDil ? (
              <p className="yardim">
                {dilBul(aktifDil).label} çevirisi boş bırakılırsa bu dil gönderilmez.
              </p>
            ) : null}
          </div>

          <div>
            <label className="etiket" htmlFor={`urun-aciklama-${aktifDil}`}>
              Açıklama (opsiyonel)
            </label>
            <textarea
              id={`urun-aciklama-${aktifDil}`}
              className="girdi resize-none"
              rows={2}
              value={aktifCeviri.description}
              onChange={(olay) => alanDegistir(aktifDil, 'description', olay.target.value)}
              placeholder="Ürünü kısaca tanıtın"
              maxLength={400}
            />
          </div>

          <div>
            <label className="etiket" htmlFor={`urun-icindekiler-${aktifDil}`}>
              İçindekiler (opsiyonel)
            </label>
            <textarea
              id={`urun-icindekiler-${aktifDil}`}
              className="girdi resize-none"
              rows={2}
              value={aktifCeviri.ingredients}
              onChange={(olay) => alanDegistir(aktifDil, 'ingredients', olay.target.value)}
              placeholder="Örn. espresso, süt, kakao"
              maxLength={400}
            />
          </div>
        </div>

        {/* ----------------------------------------------------------- fiyat */}
        <div className="grid gap-4 sm:grid-cols-2">
          <div>
            <label className="etiket" htmlFor="urun-fiyat">
              Fiyat <span className="text-red-500">*</span>
            </label>
            <div className="relative">
              <input
                id="urun-fiyat"
                type="text"
                inputMode="decimal"
                className="girdi pr-9"
                value={fiyat}
                onChange={(olay) => {
                  setFiyat(olay.target.value)
                  setHatalar((oncekiler) => ({ ...oncekiler, fiyat: '' }))
                }}
                onBlur={() => {
                  if (fiyatMetniGecerliMi(fiyat)) setFiyat(fiyatGirdiye(fiyatCozumle(fiyat)))
                }}
                placeholder="145,00"
                autoComplete="off"
              />
              <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-sm text-gray-500">
                {simge}
              </span>
            </div>
            {hatalar.fiyat ? <p className="hata-metni">{hatalar.fiyat}</p> : null}
          </div>

          <div>
            <label className="etiket" htmlFor="urun-karsilastirma-fiyati">
              Karşılaştırma fiyatı (opsiyonel)
            </label>
            <div className="relative">
              <input
                id="urun-karsilastirma-fiyati"
                type="text"
                inputMode="decimal"
                className="girdi pr-9"
                value={karsilastirmaFiyati}
                onChange={(olay) => {
                  setKarsilastirmaFiyati(olay.target.value)
                  setHatalar((oncekiler) => ({ ...oncekiler, karsilastirma: '' }))
                }}
                onBlur={() => {
                  if (fiyatMetniGecerliMi(karsilastirmaFiyati)) {
                    setKarsilastirmaFiyati(fiyatGirdiye(fiyatCozumle(karsilastirmaFiyati)))
                  }
                }}
                placeholder="180,00"
                autoComplete="off"
              />
              <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-sm text-gray-500">
                {simge}
              </span>
            </div>
            {hatalar.karsilastirma ? (
              <p className="hata-metni">{hatalar.karsilastirma}</p>
            ) : (
              <p className="yardim">Üstü çizili eski fiyat. Boş bırakabilirsiniz.</p>
            )}
          </div>
        </div>

        {/* ---------------------------------------------------------- görsel */}
        <GorselYukleyici
          deger={gorselUrl}
          degisti={setGorselUrl}
          etiket="Ürün görseli (opsiyonel)"
        />

        {/* -------------------------------------------------------- alerjenler */}
        <div>
          <span className="etiket">Alerjen / uyarı etiketleri (opsiyonel)</span>
          <div className="flex flex-wrap gap-2">
            {ALERJENLER.map((alerjen) => {
              const secili = alerjenler.includes(alerjen.code)
              return (
                <button
                  key={alerjen.code}
                  type="button"
                  onClick={() => alerjenDegistir(alerjen.code)}
                  aria-pressed={secili}
                  className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-xs font-medium ${
                    secili
                      ? 'border-marka-600 bg-marka-50 text-marka-700'
                      : 'border-gray-300 text-gray-600 hover:bg-gray-50'
                  }`}
                >
                  <span aria-hidden="true">{alerjen.emoji}</span>
                  {alerjen.tr}
                </button>
              )
            })}
          </div>
          <p className="yardim">Seçtikleriniz ürün kartında küçük rozetler olarak görünür.</p>
        </div>

        {/* -------------------------------------------------------- anahtarlar */}
        <div className="grid gap-3 sm:grid-cols-2">
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
                Kapatırsanız ürün müşteri menüsünde görünmez.
              </span>
            </span>
          </label>

          <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-gray-200 p-3">
            <input
              type="checkbox"
              className="mt-0.5 h-4 w-4 shrink-0 rounded border-gray-300 text-marka-600 focus:ring-marka-500"
              checked={oneCikar}
              onChange={(olay) => setOneCikar(olay.target.checked)}
            />
            <span className="min-w-0">
              <span className="block text-sm font-medium text-gray-700">Öne çıkar</span>
              <span className="block text-xs text-gray-500">
                Ürün kartında “Öne çıkan” rozeti gösterilir.
              </span>
            </span>
          </label>
        </div>
      </div>
    </Modal>
  )
}
