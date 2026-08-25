import { useEffect, useState } from 'react'
import { ArrowRight, Info, Loader2, Percent } from 'lucide-react'

import api from '../../lib/api'
import { fiyatBicimle } from '../../lib/format'
import Modal from '../ui/Modal.jsx'
import { useBildirim } from '../ui/Bildirim.jsx'

/** Backend'deki utils/pricing.go sabitleriyle birebir aynı olmalı. */
const YUVARLAMA_SECENEKLERI = [
  { deger: 'none', etiket: 'Yuvarlama yok' },
  { deger: 'integer', etiket: 'Tam sayıya (148)' },
  { deger: 'nearest_5', etiket: "5'in katına (150)" },
  { deger: 'nearest_10', etiket: "10'un katına (150)" },
  { deger: 'ends_50', etiket: "0,50'nin katına (147,50)" },
  { deger: 'ends_95', etiket: '…,95 ile bitir (147,95)' },
  { deger: 'ends_99', etiket: '…,99 ile bitir (147,99)' },
]

const ZAM_KISAYOLLARI = [5, 10, 15, 20]
const INDIRIM_KISAYOLLARI = [-5, -10]

const EN_FAZLA_SATIR = 50

/** Kategori adını çevirilerden çıkarır. */
function kategoriAdi(kategori) {
  const ceviriler = kategori?.translations || {}
  return ceviriler.tr?.name || Object.values(ceviriler)[0]?.name || 'Adsız kategori'
}

/**
 * Toplu fiyat güncelleme modalı (zam / indirim + yuvarlama).
 *
 * @param {boolean}  acik
 * @param {Function} kapat
 * @param {Array}    kategoriler
 * @param {string}   paraBirimi
 * @param {Function} uygulandi   - güncelleme sonrası listeyi tazelemek için
 */
export default function TopluFiyatModal({ acik, kapat, kategoriler, paraBirimi, uygulandi }) {
  const bildirim = useBildirim()
  const kategoriListesi = Array.isArray(kategoriler) ? kategoriler : []

  const [yuzde, setYuzde] = useState('')
  const [yuvarlama, setYuvarlama] = useState('none')
  const [tumUrunler, setTumUrunler] = useState(true)
  const [seciliKategoriler, setSeciliKategoriler] = useState([])
  const [onizleme, setOnizleme] = useState(null)
  const [islemde, setIslemde] = useState(false)
  const [islemTuru, setIslemTuru] = useState(null) // 'onizleme' | 'uygula'
  const [hata, setHata] = useState('')

  useEffect(() => {
    if (!acik) return
    setYuzde('')
    setYuvarlama('none')
    setTumUrunler(true)
    setSeciliKategoriler([])
    setOnizleme(null)
    setIslemde(false)
    setIslemTuru(null)
    setHata('')
  }, [acik])

  const yuzdeSayi = Number(String(yuzde).replace(',', '.'))
  const yuzdeGecerli = String(yuzde).trim() !== '' && Number.isFinite(yuzdeSayi)

  function kategoriDegistir(id) {
    setOnizleme(null)
    setSeciliKategoriler((oncekiler) =>
      oncekiler.includes(id) ? oncekiler.filter((k) => k !== id) : [...oncekiler, id],
    )
  }

  function istekGovdesi(uygula) {
    return {
      percentage: yuzdeSayi,
      rounding: yuvarlama,
      category_ids: tumUrunler ? [] : seciliKategoriler,
      apply: uygula,
    }
  }

  function dogrula() {
    if (!yuzdeGecerli) {
      setHata('Bir yüzde değeri girin.')
      return false
    }
    if (yuzdeSayi < -90 || yuzdeSayi > 1000) {
      setHata('Yüzde değeri -90 ile 1000 arasında olmalıdır.')
      return false
    }
    setHata('')
    return true
  }

  async function onizle() {
    if (!dogrula()) return

    setIslemde(true)
    setIslemTuru('onizleme')
    try {
      const sonuc = await api.bulkPrice(istekGovdesi(false))
      setOnizleme(sonuc)
      if (!sonuc?.affected) {
        bildirim.bilgi('Bu ayarlarla değişecek bir fiyat bulunamadı.')
      }
    } catch (err) {
      bildirim.hata(err.message)
    } finally {
      setIslemde(false)
      setIslemTuru(null)
    }
  }

  async function uygula() {
    if (!dogrula()) return

    setIslemde(true)
    setIslemTuru('uygula')
    try {
      const sonuc = await api.bulkPrice(istekGovdesi(true))
      bildirim.basari(`${sonuc?.affected ?? 0} ürünün fiyatı güncellendi.`)
      uygulandi?.()
      kapat?.()
    } catch (err) {
      bildirim.hata(err.message)
    } finally {
      setIslemde(false)
      setIslemTuru(null)
    }
  }

  const satirlar = Array.isArray(onizleme?.preview) ? onizleme.preview : []
  const gosterilenler = satirlar.slice(0, EN_FAZLA_SATIR)
  const kalan = satirlar.length - gosterilenler.length

  return (
    <Modal
      acik={acik}
      kapat={islemde ? () => {} : kapat}
      baslik="Toplu Fiyat Güncelleme"
      aciklama="Tüm menüye ya da seçtiğiniz kategorilere tek seferde zam veya indirim uygulayın."
      genislik="max-w-2xl"
      altBilgi={
        <>
          <button type="button" className="btn-ikincil" onClick={kapat} disabled={islemde}>
            İptal
          </button>
          <button type="button" className="btn-ikincil" onClick={onizle} disabled={islemde}>
            {islemTuru === 'onizleme' ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
            Önizle
          </button>
          <button type="button" className="btn-birincil" onClick={uygula} disabled={islemde}>
            {islemTuru === 'uygula' ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" /> Uygulanıyor
              </>
            ) : (
              'Uygula'
            )}
          </button>
        </>
      }
    >
      <div className="space-y-5">
        {/* ------------------------------------------------------ canlı özet */}
        {yuzdeGecerli && yuzdeSayi !== 0 ? (
          <div className="rounded-lg border border-marka-200 bg-marka-50 px-4 py-3 text-sm font-medium text-marka-700">
            {yuzdeSayi > 0
              ? `Tüm fiyatlara %${Math.abs(yuzdeSayi)} zam uygulanacak.`
              : `Tüm fiyatlara %${Math.abs(yuzdeSayi)} indirim uygulanacak.`}
          </div>
        ) : null}

        {/* ------------------------------------------------------------ yüzde */}
        <div>
          <label className="etiket" htmlFor="toplu-yuzde">
            Değişim yüzdesi <span className="text-red-500">*</span>
          </label>
          <div className="relative">
            <input
              id="toplu-yuzde"
              type="number"
              inputMode="decimal"
              min={-90}
              max={1000}
              step="0.1"
              className="girdi pr-9"
              value={yuzde}
              onChange={(olay) => {
                setYuzde(olay.target.value)
                setOnizleme(null)
                setHata('')
              }}
              placeholder="Örn. 10"
            />
            <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-gray-400">
              <Percent className="h-4 w-4" aria-hidden="true" />
            </span>
          </div>
          {hata ? (
            <p className="hata-metni">{hata}</p>
          ) : (
            <p className="yardim">-90 ile 1000 arasında bir değer girin. Eksi değer indirimdir.</p>
          )}

          <div className="mt-3 flex flex-wrap items-center gap-2">
            <span className="text-xs font-medium text-gray-500">Zam:</span>
            {ZAM_KISAYOLLARI.map((deger) => (
              <button
                key={`zam-${deger}`}
                type="button"
                disabled={islemde}
                onClick={() => {
                  setYuzde(String(deger))
                  setOnizleme(null)
                  setHata('')
                }}
                className={`rounded-lg border px-3 py-1.5 text-xs font-medium ${
                  yuzdeGecerli && yuzdeSayi === deger
                    ? 'border-marka-600 bg-marka-50 text-marka-700'
                    : 'border-gray-300 text-gray-600 hover:bg-gray-50'
                }`}
              >
                +{deger}
              </button>
            ))}

            <span className="ml-2 text-xs font-medium text-gray-500">İndirim:</span>
            {INDIRIM_KISAYOLLARI.map((deger) => (
              <button
                key={`indirim-${deger}`}
                type="button"
                disabled={islemde}
                onClick={() => {
                  setYuzde(String(deger))
                  setOnizleme(null)
                  setHata('')
                }}
                className={`rounded-lg border px-3 py-1.5 text-xs font-medium ${
                  yuzdeGecerli && yuzdeSayi === deger
                    ? 'border-marka-600 bg-marka-50 text-marka-700'
                    : 'border-gray-300 text-gray-600 hover:bg-gray-50'
                }`}
              >
                {deger}
              </button>
            ))}
          </div>
        </div>

        {/* -------------------------------------------------------- yuvarlama */}
        <div>
          <label className="etiket" htmlFor="toplu-yuvarlama">
            Fiyat yuvarlama
          </label>
          <select
            id="toplu-yuvarlama"
            className="girdi"
            value={yuvarlama}
            onChange={(olay) => {
              setYuvarlama(olay.target.value)
              setOnizleme(null)
            }}
          >
            {YUVARLAMA_SECENEKLERI.map((secenek) => (
              <option key={secenek.deger} value={secenek.deger}>
                {secenek.etiket}
              </option>
            ))}
          </select>
          <p className="yardim">Zam/indirim hesaplandıktan sonra fiyatlar bu kurala göre yuvarlanır.</p>
        </div>

        {/* -------------------------------------------------------- kapsam */}
        <div>
          <span className="etiket">Kapsam</span>
          <div className="space-y-2">
            <label className="flex cursor-pointer items-center gap-3 rounded-lg border border-gray-200 px-3 py-2.5">
              <input
                type="radio"
                name="toplu-kapsam"
                className="h-4 w-4 border-gray-300 text-marka-600 focus:ring-marka-500"
                checked={tumUrunler}
                onChange={() => {
                  setTumUrunler(true)
                  setOnizleme(null)
                }}
              />
              <span className="text-sm font-medium text-gray-700">Tüm ürünler</span>
            </label>

            <label className="flex cursor-pointer items-center gap-3 rounded-lg border border-gray-200 px-3 py-2.5">
              <input
                type="radio"
                name="toplu-kapsam"
                className="h-4 w-4 border-gray-300 text-marka-600 focus:ring-marka-500"
                checked={!tumUrunler}
                onChange={() => {
                  setTumUrunler(false)
                  setOnizleme(null)
                }}
              />
              <span className="text-sm font-medium text-gray-700">Belirli kategoriler</span>
            </label>
          </div>

          {!tumUrunler ? (
            <div className="mt-2 max-h-48 space-y-1 overflow-y-auto rounded-lg border border-gray-200 p-2">
              {kategoriListesi.length === 0 ? (
                <p className="px-1 py-2 text-sm text-gray-500">Henüz kategori yok.</p>
              ) : (
                kategoriListesi.map((kategori) => (
                  <label
                    key={kategori.id}
                    className="flex cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 hover:bg-gray-50"
                  >
                    <input
                      type="checkbox"
                      className="h-4 w-4 shrink-0 rounded border-gray-300 text-marka-600 focus:ring-marka-500"
                      checked={seciliKategoriler.includes(kategori.id)}
                      onChange={() => kategoriDegistir(kategori.id)}
                    />
                    <span className="min-w-0 flex-1 truncate text-sm text-gray-700">
                      {kategori.icon ? `${kategori.icon} ` : ''}
                      {kategoriAdi(kategori)}
                    </span>
                    {typeof kategori.product_count === 'number' ? (
                      <span className="shrink-0 text-xs text-gray-400">
                        {kategori.product_count} ürün
                      </span>
                    ) : null}
                  </label>
                ))
              )}
              <p className="px-1 pt-1 text-xs text-gray-500">
                Hiçbiri seçilmezse güncelleme tüm ürünlere uygulanır.
              </p>
            </div>
          ) : null}
        </div>

        {/* ---------------------------------------------------------- bilgi */}
        <div className="flex items-start gap-3 rounded-lg border border-blue-200 bg-blue-50 px-4 py-3">
          <Info className="mt-0.5 h-4 w-4 shrink-0 text-blue-600" aria-hidden="true" />
          <p className="text-sm leading-snug text-blue-800">
            Fiyat güncellemesinden sonra menünüzdeki “Fiyatlarımız … tarihinden itibaren
            geçerlidir” ibaresi otomatik olarak bugüne güncellenir.
          </p>
        </div>

        {/* ------------------------------------------------------- önizleme */}
        {onizleme ? (
          <div>
            <div className="mb-2 flex items-center justify-between gap-3">
              <p className="text-sm font-medium text-gray-700">
                {onizleme.affected ?? 0} üründe fiyat değişecek.
              </p>
              <span className="text-xs text-gray-400">{satirlar.length} ürün tarandı</span>
            </div>

            {satirlar.length === 0 ? (
              <p className="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 text-sm text-gray-500">
                Bu kapsamda güncellenecek ürün bulunamadı.
              </p>
            ) : (
              <div className="overflow-hidden rounded-lg border border-gray-200">
                <div className="max-h-72 overflow-y-auto">
                  <table className="w-full text-sm">
                    <thead className="sticky top-0 bg-gray-50 text-xs uppercase text-gray-500">
                      <tr>
                        <th className="px-3 py-2 text-left font-medium">Ürün</th>
                        <th className="px-3 py-2 text-right font-medium">Eski Fiyat</th>
                        <th className="px-3 py-2 text-right font-medium">Yeni Fiyat</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100">
                      {gosterilenler.map((satir) => (
                        <tr key={satir.id}>
                          <td className="max-w-[220px] truncate px-3 py-2 text-gray-700">
                            {satir.name}
                          </td>
                          <td className="whitespace-nowrap px-3 py-2 text-right text-gray-500">
                            {fiyatBicimle(satir.old_price, paraBirimi)}
                          </td>
                          <td className="whitespace-nowrap px-3 py-2 text-right font-medium text-marka-600">
                            <span className="inline-flex items-center justify-end gap-1">
                              <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
                              {fiyatBicimle(satir.new_price, paraBirimi)}
                            </span>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>

                {kalan > 0 ? (
                  <p className="border-t border-gray-200 bg-gray-50 px-3 py-2 text-xs text-gray-500">
                    ve {kalan} ürün daha
                  </p>
                ) : null}
              </div>
            )}
          </div>
        ) : null}
      </div>
    </Modal>
  )
}
