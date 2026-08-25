import { useState } from 'react'
import { Link, Navigate, useNavigate } from 'react-router-dom'
import { Eye, EyeOff } from 'lucide-react'

import { useAuth } from '../lib/auth.jsx'
import { useBildirim } from '../components/ui/Bildirim.jsx'
import Yukleniyor from '../components/ui/Yukleniyor.jsx'

/* Geliştirme sırasında hızlı giriş için hazır hesap bilgileri */
const DEMO_EPOSTA = 'demo@karecik.com'
const DEMO_SIFRE = 'demo1234'

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

export default function Giris() {
  const { isAuthenticated, loading: oturumYukleniyor, login } = useAuth()
  const navigate = useNavigate()
  const bildirim = useBildirim()

  const [eposta, setEposta] = useState('')
  const [sifre, setSifre] = useState('')
  const [sifreGorunsun, setSifreGorunsun] = useState(false)
  const [gonderiliyor, setGonderiliyor] = useState(false)
  const [hata, setHata] = useState('')

  async function gonder(olay) {
    olay.preventDefault()
    if (gonderiliyor) return

    const temizEposta = eposta.trim()
    if (!temizEposta || !sifre) {
      setHata('Lütfen e-posta ve şifrenizi girin.')
      return
    }

    setHata('')
    setGonderiliyor(true)
    try {
      await login(temizEposta, sifre)
      bildirim.basari('Hoş geldiniz!')
      navigate('/panel', { replace: true })
    } catch (err) {
      setHata(err.message)
      bildirim.hata(err.message)
      setGonderiliyor(false)
    }
  }

  /** Demo hesabın bilgilerini forma yazar. */
  function demoyuDoldur() {
    setEposta(DEMO_EPOSTA)
    setSifre(DEMO_SIFRE)
    setHata('')
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
          <h1 className="text-2xl font-bold tracking-tight text-gray-900">Tekrar hoş geldiniz</h1>
          <p className="mt-1.5 text-sm text-gray-500">Menünüzü yönetmek için giriş yapın.</p>

          {hata ? (
            <div className="mt-5 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
              {hata}
            </div>
          ) : null}

          <form onSubmit={gonder} className="mt-6 space-y-4" noValidate>
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
                  placeholder="••••••••"
                  autoComplete="current-password"
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
            </div>

            <button type="submit" className="btn-birincil w-full" disabled={gonderiliyor}>
              {gonderiliyor ? 'Giriş yapılıyor...' : 'Giriş yap'}
            </button>
          </form>

          <p className="mt-6 text-center text-sm text-gray-600">
            Hesabınız yok mu?{' '}
            <Link to="/kayit" className="font-medium text-marka-600 hover:text-marka-700">
              Ücretsiz oluşturun
            </Link>
          </p>
        </div>

        <div className="mt-4 rounded-xl border border-gray-200 bg-gray-50 p-3 text-xs text-gray-500">
          <div className="flex items-center justify-between gap-3">
            <span>
              Demo hesap: <span className="font-medium text-gray-700">{DEMO_EPOSTA}</span> /{' '}
              <span className="font-medium text-gray-700">{DEMO_SIFRE}</span>
            </span>
            <button
              type="button"
              onClick={demoyuDoldur}
              className="btn-ikincil btn-kucuk shrink-0"
              disabled={gonderiliyor}
            >
              Doldur
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
