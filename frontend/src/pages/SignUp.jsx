import { useState } from 'react'
import { Link, Navigate, useNavigate } from 'react-router-dom'
import { Eye, EyeOff } from 'lucide-react'

import { useAuth } from '../lib/auth.jsx'
import { useBildirim } from '../components/ui/Bildirim.jsx'
import Yukleniyor from '../components/ui/Yukleniyor.jsx'

/* Türkçe harflerin ASCII karşılıkları — slug üretiminde kullanılır. */
const TURKCE_ASCII = {
  ç: 'c',
  Ç: 'c',
  ğ: 'g',
  Ğ: 'g',
  ı: 'i',
  I: 'i',
  İ: 'i',
  i: 'i',
  ö: 'o',
  Ö: 'o',
  ş: 's',
  Ş: 's',
  ü: 'u',
  Ü: 'u',
}

/**
 * İşletme adından menü adresi (slug) üretir.
 * "Kahve Durağı" -> "kahve-duragi"
 */
function slugUret(metin) {
  const asciye = String(metin || '')
    .split('')
    .map((harf) => TURKCE_ASCII[harf] || harf)
    .join('')

  return asciye
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/-{2,}/g, '-')
    .replace(/^-+/, '')
    .replace(/-+$/, '')
}

/** Karecik markası: küçük QR benzeri işaret + kalın yazı. */
function KarecikLogosu() {
  return (
    <Link to="/" className="inline-flex items-center gap-2.5">
      <svg viewBox="0 0 32 32" className="h-9 w-9 text-marka-600" aria-hidden="true">
        <rect className="fill-current" width="32" height="32" rx="8" />
        <g fill="#ffffff">
          <path d="M7 7h7v7H7V7zm2 2v3h3V9H9z" />
          <path d="M18 7h7v7h-7V7zm2 2v3h3V9h-3z" />
          <path d="M7 18h7v7H7v-7zm2 2v3h3v-3H9z" />
          <rect x="18" y="18" width="3" height="3" rx="0.7" />
          <rect x="22.5" y="22.5" width="2.5" height="2.5" rx="0.6" />
          <rect x="18" y="23" width="2" height="2" rx="0.5" />
          <rect x="23" y="18" width="2" height="2" rx="0.5" />
        </g>
      </svg>
      <span className="text-xl font-bold tracking-tight text-gray-900">Karecik</span>
    </Link>
  )
}

export default function Kayit() {
  const { isAuthenticated, loading: oturumYukleniyor, register } = useAuth()
  const navigate = useNavigate()
  const bildirim = useBildirim()

  const [isletmeAdi, setIsletmeAdi] = useState('')
  const [eposta, setEposta] = useState('')
  const [sifre, setSifre] = useState('')
  const [sifreGorunsun, setSifreGorunsun] = useState(false)
  const [gonderiliyor, setGonderiliyor] = useState(false)
  const [hata, setHata] = useState('')

  const slug = slugUret(isletmeAdi)

  async function gonder(olay) {
    olay.preventDefault()
    if (gonderiliyor) return

    const temizAd = isletmeAdi.trim()
    const temizEposta = eposta.trim()

    if (!temizAd) {
      setHata('Lütfen işletmenizin adını girin.')
      return
    }
    if (!slug) {
      setHata('İşletme adı en az bir harf veya rakam içermelidir.')
      return
    }
    if (!temizEposta) {
      setHata('Lütfen e-posta adresinizi girin.')
      return
    }
    if (sifre.length < 8) {
      setHata('Şifreniz en az 8 karakter olmalıdır.')
      return
    }

    setHata('')
    setGonderiliyor(true)
    try {
      await register(temizAd, temizEposta, sifre)
      bildirim.basari('Hesabınız oluşturuldu. Karecik\u2019e hoş geldiniz!')
      navigate('/panel', { replace: true })
    } catch (err) {
      setHata(err.message)
      bildirim.hata(err.message)
      setGonderiliyor(false)
    }
  }

  // Oturum doğrulanırken formu göstermeyelim, boşuna yanıp sönmesin.
  if (oturumYukleniyor) return <Yukleniyor tamEkran metin="Oturum kontrol ediliyor..." />
  if (isAuthenticated) return <Navigate to="/panel" replace />

  return (
    <div className="min-h-screen bg-white px-4 py-12 sm:py-16">
      <div className="mx-auto w-full max-w-md">
        <div className="mb-8 text-center">
          <KarecikLogosu />
        </div>

        <div className="rounded-2xl border border-gray-200 p-6 sm:p-8">
          <h1 className="text-2xl font-bold tracking-tight text-gray-900">
            Ücretsiz hesabınızı oluşturun
          </h1>
          <p className="mt-1.5 text-sm text-gray-500">
            Kredi kartı gerekmez. Menünüz birkaç dakikada hazır.
          </p>

          {hata ? (
            <div className="mt-5 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
              {hata}
            </div>
          ) : null}

          <form onSubmit={gonder} className="mt-6 space-y-4" noValidate>
            <div>
              <label htmlFor="isletme-adi" className="etiket">
                İşletme Adı
              </label>
              <input
                id="isletme-adi"
                type="text"
                className="girdi"
                placeholder="Kahve Durağı"
                autoComplete="organization"
                value={isletmeAdi}
                onChange={(olay) => setIsletmeAdi(olay.target.value)}
                disabled={gonderiliyor}
              />
              <p className="yardim">
                Menü adresiniz:{' '}
                <span className="font-medium text-gray-700">
                  {slug || 'isletmeniz'}.karecik.com
                </span>
              </p>
            </div>

            <div>
              <label htmlFor="eposta" className="etiket">
                E-posta
              </label>
              <input
                id="eposta"
                type="email"
                className="girdi"
                placeholder="ornek@isletmeniz.com"
                autoComplete="email"
                value={eposta}
                onChange={(olay) => setEposta(olay.target.value)}
                disabled={gonderiliyor}
              />
            </div>

            <div>
              <label htmlFor="sifre" className="etiket">
                Şifre
              </label>
              <div className="relative">
                <input
                  id="sifre"
                  type={sifreGorunsun ? 'text' : 'password'}
                  className="girdi pr-11"
                  placeholder="En az 8 karakter"
                  autoComplete="new-password"
                  value={sifre}
                  onChange={(olay) => setSifre(olay.target.value)}
                  disabled={gonderiliyor}
                />
                <button
                  type="button"
                  onClick={() => setSifreGorunsun((onceki) => !onceki)}
                  className="absolute inset-y-0 right-0 flex w-11 items-center justify-center rounded-r-lg text-gray-400 hover:text-gray-600"
                  aria-label={sifreGorunsun ? 'Şifreyi gizle' : 'Şifreyi göster'}
                >
                  {sifreGorunsun ? (
                    <EyeOff className="h-4 w-4" aria-hidden="true" />
                  ) : (
                    <Eye className="h-4 w-4" aria-hidden="true" />
                  )}
                </button>
              </div>
              <p className="yardim">Şifreniz en az 8 karakter olmalıdır.</p>
            </div>

            <button type="submit" className="btn-birincil w-full" disabled={gonderiliyor}>
              {gonderiliyor ? 'Hesap oluşturuluyor...' : 'Hesabımı oluştur'}
            </button>
          </form>

          <p className="mt-6 text-center text-sm text-gray-600">
            Zaten hesabınız var mı?{' '}
            <Link to="/giris" className="font-medium text-marka-600 hover:text-marka-700">
              Giriş yapın
            </Link>
          </p>
        </div>
      </div>
    </div>
  )
}
