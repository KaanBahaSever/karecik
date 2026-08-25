import { useEffect, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'

import api from '../../lib/api'
import { subdomainAl } from '../../lib/subdomain'
import { metin } from '../../locales/index.js'
import Yukleniyor from '../../components/ui/Yukleniyor.jsx'
import MenuIcerik from '../../components/menu/MenuIcerik.jsx'
import KarsilamaEkrani from '../../components/menu/KarsilamaEkrani.jsx'

/**
 * QR okutulunca açılan müşteri menüsü.
 *
 * Slug üç yoldan gelebilir:
 *   1. prop        -> <MusteriMenusu slug="demo-kafe" />   (landing iframe'i)
 *   2. URL yolu    -> /m/:slug                              (yedek adres)
 *   3. subdomain   -> kahve-duragi.karecik.com              (asıl adres)
 *
 * @param {string}  slug   - Zorlanmış işletme adresi (opsiyonel)
 * @param {boolean} gomulu - iframe / dar alan içinde çalışıyor
 */
export default function MusteriMenusu({ slug: slugProp, gomulu = false }) {
  const { slug: slugParam } = useParams()
  const slug = slugProp || slugParam || subdomainAl() || ''

  // Kullanıcı dil seçmediyse boş kalır; backend işletmenin varsayılan dilini kullanır.
  const [secilenDil, setSecilenDil] = useState('')
  const [menu, setMenu] = useState(null)
  const [yukleniyor, setYukleniyor] = useState(true)
  const [hata, setHata] = useState('')
  const [karsilamaGoster, setKarsilamaGoster] = useState(false)

  // Karşılama ekranı dil değişiminde tekrar oynamasın.
  const karsilamaGosterildi = useRef(false)

  const aktifDil = secilenDil || menu?.business?.default_language || 'tr'

  /* ------------------------------------------------------------ menü yükle */
  useEffect(() => {
    let iptal = false

    async function menuyuGetir() {
      setYukleniyor(true)
      setHata('')
      try {
        const veri = slug
          ? await api.publicMenu(slug, secilenDil)
          : await api.publicMenuByHost(secilenDil)
        if (iptal) return
        setMenu(veri)
      } catch (err) {
        if (iptal) return
        setHata(err.message || 'Menü yüklenemedi.')
      } finally {
        if (!iptal) setYukleniyor(false)
      }
    }

    menuyuGetir()
    return () => {
      iptal = true
    }
  }, [slug, secilenDil])

  /* --------------------------------------------------- karşılama ekranı */
  useEffect(() => {
    if (gomulu || !menu?.business?.splash_enabled) return
    if (karsilamaGosterildi.current) return

    const anahtar = `karecik_splash_${menu.business.slug || slug}`
    try {
      if (sessionStorage.getItem(anahtar)) {
        karsilamaGosterildi.current = true
        return
      }
      sessionStorage.setItem(anahtar, '1')
    } catch {
      /* gizli sekmede sessionStorage kapalı olabilir — yine de bir kez gösteririz */
    }

    karsilamaGosterildi.current = true
    setKarsilamaGoster(true)
  }, [menu, gomulu, slug])

  /* ---------------------------------------------------- sekme başlığı */
  useEffect(() => {
    if (gomulu || !menu?.business?.name) return undefined

    const oncekiBaslik = document.title
    document.title = menu.business.name
    return () => {
      document.title = oncekiBaslik
    }
  }, [gomulu, menu])

  /* ------------------------------------------------------------- render */

  if (yukleniyor && !menu) {
    return (
      <div
        className={`flex items-center justify-center bg-white ${
          gomulu ? 'h-full' : 'min-h-screen'
        }`}
      >
        <Yukleniyor metin={metin('yukleniyor', aktifDil)} />
      </div>
    )
  }

  if (!menu) {
    return (
      <div
        className={`flex items-center justify-center bg-white px-6 ${
          gomulu ? 'h-full' : 'min-h-screen'
        }`}
      >
        <div className="w-full max-w-sm rounded-2xl border border-gray-200 p-8 text-center">
          <p className="text-4xl" aria-hidden="true">
            🔍
          </p>
          <h1 className="mt-4 text-lg font-semibold text-gray-900">
            {metin('bulunamadi', aktifDil)}
          </h1>
          <p className="mt-1.5 text-sm text-gray-600">
            {metin('bulunamadiAciklama', aktifDil)}
          </p>
          {hata ? <p className="mt-4 text-xs text-gray-400">{hata}</p> : null}
        </div>
      </div>
    )
  }

  return (
    <>
      {karsilamaGoster ? (
        <KarsilamaEkrani business={menu.business} bitti={() => setKarsilamaGoster(false)} />
      ) : null}

      <MenuIcerik
        menu={menu}
        dil={aktifDil}
        dilDegisti={(yeniDil) => setSecilenDil(yeniDil)}
        gomulu={gomulu}
      />
    </>
  )
}
