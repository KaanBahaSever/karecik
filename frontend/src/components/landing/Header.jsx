import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { Check, ChevronDown, Globe } from 'lucide-react'

import { LANDING_DILLERI, landingMetni } from '../../locales/landing.js'

/**
 * Açılış sayfasının üst çubuğu.
 *
 * Not: Landing page kuralı gereği burada hiçbir animasyon/geçiş yoktur.
 * Yalnızca anlık renk değişimi yapan hover sınıfları kullanılır; açılır
 * pencere de geçiş efekti olmadan doğrudan görünür/gizlenir.
 *
 * @param {Function} kayitAc    - Kayıt modalını açan çağrı
 * @param {string}   dil        - Seçili dil kodu (tr | en | de)
 * @param {Function} dilDegisti - Dil değiştiğinde çağrılır
 */
export default function Header({ kayitAc, dil = 'tr', dilDegisti }) {
  const m = landingMetni(dil)

  return (
    <header className="sticky top-0 z-40 border-b border-gray-200 bg-white">
      {/*
        Yerleşim iki kademeli:

        telefon (sm altı) : İKİ SATIR. Üstte logo, altta dil seçici + butonlar.
                            Tek satırda butonlar ~300px yer kapladığı için
                            "Karecik" yazısına yer kalmıyor ve kırpılıyordu.
                            Alt satıra alınca marka adı tam görünüyor ve
                            logo boyutu (36px / 22px) küçültülmek zorunda kalmıyor.
        sm ve üzeri       : TEK SATIR, logo 56px / 34px (eskisinin ~1,55 katı).
      */}
      <div className="mx-auto flex max-w-icerik flex-wrap items-center justify-between gap-x-2 gap-y-2.5 px-4 py-3 sm:h-24 sm:flex-nowrap sm:gap-4 sm:px-6 sm:py-0">
        <Link to="/" className="flex shrink-0 items-center gap-2.5 sm:gap-3" aria-label={m.markaAnaSayfa}>
          <KarecikIsareti />
          <span className="text-[1.375rem] font-bold leading-none tracking-tight text-gray-900 sm:text-[2.125rem]">
            Karecik
          </span>
        </Link>

        <nav className="flex w-full flex-wrap items-center justify-end gap-1.5 sm:w-auto sm:flex-nowrap sm:gap-2">
          <DilSecici dil={dil} dilDegisti={dilDegisti} etiket={m.dilSec} />

          <Link
            to="/giris"
            className="btn-sessiz whitespace-nowrap px-2.5 py-2 text-xs sm:px-4 sm:py-2.5 sm:text-sm"
          >
            {m.girisYap}
          </Link>
          <button
            type="button"
            onClick={kayitAc}
            className="btn-birincil whitespace-nowrap px-2.5 py-2 text-xs sm:px-4 sm:py-2.5 sm:text-sm"
          >
            {m.ucretsizBasla}
          </button>
        </nav>
      </div>
    </header>
  )
}

/**
 * Minimalist dil seçici.
 *
 * Bayrak emojisi kullanmaz: Windows bu emojileri çizemediği için yerlerine
 * ülke kodu basıyor ve İngilizce "GB" olarak görünüyor. Bunun yerine her
 * dilin kısa kodunu (TR / EN / DE) gösteriyoruz.
 */
function DilSecici({ dil, dilDegisti, etiket }) {
  const [acik, setAcik] = useState(false)
  const kapsayiciRef = useRef(null)

  const seciliDil = LANDING_DILLERI.find((d) => d.kod === dil) || LANDING_DILLERI[0]

  // Dışarı tıklayınca ve Escape ile kapat
  useEffect(() => {
    if (!acik) return undefined

    function disariTiklandi(olay) {
      if (kapsayiciRef.current && !kapsayiciRef.current.contains(olay.target)) {
        setAcik(false)
      }
    }
    function tusaBasildi(olay) {
      if (olay.key === 'Escape') setAcik(false)
    }

    document.addEventListener('mousedown', disariTiklandi)
    document.addEventListener('keydown', tusaBasildi)
    return () => {
      document.removeEventListener('mousedown', disariTiklandi)
      document.removeEventListener('keydown', tusaBasildi)
    }
  }, [acik])

  function sec(kod) {
    dilDegisti?.(kod)
    setAcik(false)
  }

  return (
    <div className="relative" ref={kapsayiciRef}>
      <button
        type="button"
        onClick={() => setAcik((onceki) => !onceki)}
        aria-label={etiket}
        aria-haspopup="listbox"
        aria-expanded={acik}
        className="flex items-center gap-1 rounded-lg px-2 py-2 text-xs font-medium text-gray-600 hover:bg-gray-100 hover:text-gray-900 sm:gap-1.5 sm:px-2.5 sm:text-sm"
      >
        <Globe className="h-4 w-4 shrink-0" aria-hidden="true" />
        <span>{seciliDil.kisa}</span>
        <ChevronDown className="h-3.5 w-3.5 shrink-0 text-gray-400" aria-hidden="true" />
      </button>

      {acik ? (
        <ul
          role="listbox"
          aria-label={etiket}
          className="absolute right-0 top-full z-50 mt-1 w-40 overflow-hidden rounded-lg border border-gray-200 bg-white py-1 shadow-panel"
        >
          {LANDING_DILLERI.map((secenek) => {
            const secili = secenek.kod === dil
            return (
              <li key={secenek.kod}>
                <button
                  type="button"
                  role="option"
                  aria-selected={secili}
                  onClick={() => sec(secenek.kod)}
                  className={`flex w-full items-center justify-between gap-2 px-3 py-2 text-left text-sm hover:bg-gray-50 ${
                    secili ? 'font-medium text-marka-700' : 'text-gray-700'
                  }`}
                >
                  <span className="flex items-center gap-2">
                    <span className="w-6 shrink-0 text-xs font-semibold text-gray-400">
                      {secenek.kisa}
                    </span>
                    <span>{secenek.ad}</span>
                  </span>
                  {secili ? (
                    <Check className="h-4 w-4 shrink-0 text-marka-600" aria-hidden="true" />
                  ) : null}
                </button>
              </li>
            )
          })}
        </ul>
      ) : null}
    </div>
  )
}

/**
 * Marka işareti: QR karesini andıran dört bloklu sade simge.
 * Şekil değişmedi; yalnızca rengi kurumsal maviye güncellendi.
 */
function KarecikIsareti() {
  return (
    <svg
      viewBox="0 0 24 24"
      className="h-9 w-9 shrink-0 sm:h-14 sm:w-14"
      fill="none"
      aria-hidden="true"
      focusable="false"
    >
      <rect x="1" y="1" width="22" height="22" rx="5.5" fill="#1d4ed8" />
      <rect x="5.5" y="5.5" width="5.5" height="5.5" rx="1.4" fill="#ffffff" />
      <rect x="13" y="5.5" width="5.5" height="5.5" rx="1.4" fill="#ffffff" fillOpacity="0.55" />
      <rect x="5.5" y="13" width="5.5" height="5.5" rx="1.4" fill="#ffffff" fillOpacity="0.55" />
      <rect x="13" y="13" width="5.5" height="5.5" rx="1.4" fill="#ffffff" />
    </svg>
  )
}
