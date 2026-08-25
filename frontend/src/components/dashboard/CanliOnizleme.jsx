import { useEffect, useState } from 'react'
import { AlertCircle, Eye } from 'lucide-react'

import api from '../../lib/api'
import { paraSimgesi } from '../../lib/format'
import { fontYukle } from '../../themes/fonts'
import { dilBul } from '../../locales/index.js'
import Yukleniyor from '../ui/Yukleniyor.jsx'
import MenuIcerik from '../menu/MenuIcerik.jsx'

/**
 * Panelde yapılan değişikliklerin müşteri menüsünde nasıl göründüğünü
 * anında gösteren telefon önizlemesi.
 *
 * Menü içeriği sunucudan (api.previewMenu) gelir; görünüm ayarları ise
 * prop olarak gelen — henüz kaydedilmemiş olabilecek — business nesnesinden
 * alınır. Böylece tema/font/renk seçimi kaydedilmeden önce de görünür.
 *
 * @param {object} business  - Taslak (kaydedilmemiş) işletme ayarları
 * @param {number} yenile    - Değeri değiştikçe menü yeniden çekilir
 * @param {string} className
 */
export default function CanliOnizleme({ business, yenile = 0, className = '' }) {
  const [dil, setDil] = useState(business?.default_language || 'tr')
  const [menu, setMenu] = useState(null)
  const [yukleniyor, setYukleniyor] = useState(true)
  const [hata, setHata] = useState('')

  const diller = Array.isArray(business?.languages) && business.languages.length
    ? business.languages
    : ['tr']

  // Seçili dil, işletmenin dil listesinden çıkarıldıysa ilk dile dön.
  useEffect(() => {
    if (!diller.includes(dil)) setDil(diller[0])
  }, [diller.join(','), dil]) // eslint-disable-line react-hooks/exhaustive-deps

  // Seçilen yazı tipi anında yüklensin.
  useEffect(() => {
    if (business?.font_family) fontYukle(business.font_family)
  }, [business?.font_family])

  // Menü içeriğini çek.
  useEffect(() => {
    let iptal = false

    async function menuyuGetir() {
      setYukleniyor(true)
      setHata('')
      try {
        const veri = await api.previewMenu(dil)
        if (iptal) return
        setMenu(veri)
      } catch (err) {
        if (iptal) return
        // Bildirim kullanmıyoruz: önizleme hatası gürültü yapmasın.
        setHata(err.message || 'Önizleme yüklenemedi.')
      } finally {
        if (!iptal) setYukleniyor(false)
      }
    }

    menuyuGetir()
    return () => {
      iptal = true
    }
  }, [yenile, dil])

  // Sunucudan gelen menünün business alanını taslak ayarlarla birleştir.
  const onizlemeMenu = menu
    ? {
        ...menu,
        business: {
          ...menu.business,
          ...(business || {}),
          currency_symbol: paraSimgesi(business?.currency),
        },
      }
    : null

  return (
    <div className={className}>
      {/* Başlık satırı */}
      <div className="mb-3 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2 text-sm font-medium text-gray-700">
          <Eye className="h-4 w-4 text-marka-600" aria-hidden="true" />
          <span>Canlı Önizleme</span>
        </div>

        {diller.length > 1 ? (
          <select
            value={dil}
            onChange={(olay) => setDil(olay.target.value)}
            aria-label="Önizleme dili"
            className="rounded-lg border border-gray-300 bg-white px-2 py-1 text-xs text-gray-700 outline-none focus:border-marka-600 focus:ring-2 focus:ring-marka-100"
          >
            {diller.map((kod) => {
              const bilgi = dilBul(kod)
              return (
                <option key={kod} value={kod}>
                  {bilgi.kisa} {bilgi.label}
                </option>
              )
            })}
          </select>
        ) : null}
      </div>

      {/* Telefon çerçevesi */}
      <div className="bg-gray-900 rounded-[2.25rem] p-2.5 shadow-panel">
        <div className="relative h-[640px] overflow-hidden rounded-[1.75rem] bg-white">
          {/* çentik */}
          <div className="pointer-events-none absolute left-1/2 top-2 z-10 h-1.5 w-20 -translate-x-1/2 rounded-full bg-gray-900/15" />

          <div className="kaydirma-gizli h-full overflow-y-auto overflow-x-hidden">
            {yukleniyor ? (
              <Yukleniyor metin="Önizleme hazırlanıyor..." />
            ) : hata ? (
              <div className="flex h-full items-center justify-center p-6">
                <div className="flex items-start gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2.5 text-left">
                  <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-red-600" aria-hidden="true" />
                  <div>
                    <p className="text-xs font-medium text-red-800">Önizleme yüklenemedi</p>
                    <p className="mt-0.5 text-[11px] leading-snug text-red-700">{hata}</p>
                  </div>
                </div>
              </div>
            ) : onizlemeMenu ? (
              <MenuIcerik menu={onizlemeMenu} dil={dil} dilDegisti={(yeni) => setDil(yeni)} gomulu />
            ) : null}
          </div>
        </div>
      </div>

      <p className="mt-3 text-center text-xs text-gray-500">
        Değişiklikler burada anında görünür, kaydedene kadar müşterilere yansımaz.
      </p>
    </div>
  )
}
