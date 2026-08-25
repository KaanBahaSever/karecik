import { useState } from 'react'
import { Link, Navigate } from 'react-router-dom'

import Header from '../components/landing/Header.jsx'
import IPhoneCerceve from '../components/landing/IPhoneCerceve.jsx'
import KayitModal from '../components/landing/KayitModal.jsx'
import { useAuth } from '../lib/auth.jsx'
import { diliKaydet, kayitliDiliOku, landingMetni } from '../locales/landing.js'

/**
 * Karecik açılış sayfası.
 *
 * KURAL: Bu sayfada ve components/landing altında hiçbir animasyon yoktur.
 * Animasyon kütüphanesi yok; transition-*, animate-*, duration-*, hover:scale-*
 * sınıfları kullanılmaz. Yalnızca anlık renk değişimi yapan hover sınıfları var.
 */
export default function Landing() {
  const { isAuthenticated } = useAuth()
  const [kayitAcik, setKayitAcik] = useState(false)
  const [dil, setDil] = useState(kayitliDiliOku)

  const m = landingMetni(dil)

  function dilDegisti(yeniDil) {
    setDil(yeniDil)
    diliKaydet(yeniDil)
  }

  // Oturum açıksa doğrudan panele gönder.
  if (isAuthenticated) return <Navigate to="/panel" replace />

  return (
    <div className="flex min-h-screen flex-col bg-white">
      <Header kayitAc={() => setKayitAcik(true)} dil={dil} dilDegisti={dilDegisti} />

      <main className="flex-1">
        {/*
          Sağ sütun genişliği içeriğine göre (auto) belirlenir; artan alan sola
          ve ortadaki boşluğa gider. Böylece telefon sağ kenara yaslanır,
          iki sütun arasındaki nefes payı büyür.
        */}
        <section className="mx-auto grid max-w-icerik grid-cols-1 items-center gap-14 px-4 py-16 sm:px-6 lg:grid-cols-[minmax(0,1fr)_auto] lg:gap-24 lg:py-24 lg:pr-2 xl:gap-28">
          {/* sol sütun: anlatım */}
          <div className="max-w-xl">
            <h1 className="text-4xl font-bold tracking-tight text-gray-900 sm:text-5xl lg:text-6xl">
              {m.baslik}
            </h1>

            <p className="mt-6 text-lg text-gray-600">{m.aciklama}</p>

            <div className="mt-5 flex flex-wrap items-center gap-3 text-sm text-gray-500">
              <span>{m.rozetUcretsiz}</span>
              <span className="text-gray-300">|</span>
              <span>{m.rozetKurulum}</span>
              <span className="text-gray-300">|</span>
              <span>{m.rozetMobil}</span>
            </div>

            <div className="mt-9 flex flex-wrap items-center gap-3">
              <button
                type="button"
                onClick={() => setKayitAcik(true)}
                className="btn-birincil px-6 py-3 text-base"
              >
                {m.ucretsizBasla}
              </button>
              <Link to="/giris" className="btn-ikincil px-6 py-3 text-base">
                {m.girisYap}
              </Link>
            </div>
          </div>

          {/* sağ sütun: platformdan oluşturulmuş örnek kafe menüsü */}
          <div className="flex justify-center lg:justify-end">
            <IPhoneCerceve>
              <iframe src="/demo" title={m.demoBaslik} className="h-full w-full border-0" />
            </IPhoneCerceve>
          </div>
        </section>
      </main>

      <footer className="border-t border-gray-200">
        <div className="mx-auto flex max-w-icerik flex-col items-center justify-between gap-3 px-4 py-6 sm:flex-row sm:px-6">
          <p className="text-sm text-gray-500">{m.telif}</p>
          <div className="flex flex-wrap items-center justify-center gap-3 text-xs text-gray-400">
            <span>{m.altPlatform}</span>
            <span className="text-gray-300">|</span>
            <span>{m.altKurulum}</span>
            <span className="text-gray-300">|</span>
            <span>{m.altKart}</span>
          </div>
        </div>
      </footer>

      <KayitModal acik={kayitAcik} kapat={() => setKayitAcik(false)} />
    </div>
  )
}
