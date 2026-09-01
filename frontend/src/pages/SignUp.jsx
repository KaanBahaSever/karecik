import { useState } from 'react'
import { Link, Navigate, useNavigate } from 'react-router-dom'
import { Eye, EyeOff } from 'lucide-react'

import { useAuth } from '../lib/auth.jsx'
import { useToast } from '../components/ui/Toast.jsx'
import Loading from '../components/ui/Loading.jsx'

/* ASCII equivalents of Turkish letters — used when building the slug. */
const TURKISH_ASCII = {
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
 * Builds the menu address (slug) from a business name.
 * "Kahve Durağı" -> "kahve-duragi"
 *
 * Mirrors Slugify() in backend/internal/utils/slug.go.
 */
function buildSlug(text) {
  const ascii = String(text || '')
    .split('')
    .map((letter) => TURKISH_ASCII[letter] || letter)
    .join('')

  return ascii
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/-{2,}/g, '-')
    .replace(/^-+/, '')
    .replace(/-+$/, '')
}

/** Karecik brand lockup: small QR-like mark plus the wordmark. */
function BrandLogo() {
  return (
    <Link to="/" className="inline-flex items-center gap-2.5">
      <svg viewBox="0 0 32 32" className="h-9 w-9 text-brand-600" aria-hidden="true">
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

export default function SignUp() {
  const { isAuthenticated, loading: sessionLoading, register } = useAuth()
  const navigate = useNavigate()
  const toast = useToast()

  const [businessName, setBusinessName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [passwordVisible, setPasswordVisible] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  const slug = buildSlug(businessName)

  async function onSubmit(event) {
    event.preventDefault()
    if (submitting) return

    const trimmedName = businessName.trim()
    const trimmedEmail = email.trim()

    if (!trimmedName) {
      setError('Lütfen işletmenizin adını girin.')
      return
    }
    if (!slug) {
      setError('İşletme adı en az bir harf veya rakam içermelidir.')
      return
    }
    if (!trimmedEmail) {
      setError('Lütfen e-posta adresinizi girin.')
      return
    }
    if (password.length < 8) {
      setError('Şifreniz en az 8 karakter olmalıdır.')
      return
    }

    setError('')
    setSubmitting(true)
    try {
      await register(trimmedName, trimmedEmail, password)
      toast.success('Hesabınız oluşturuldu. Karecik’e hoş geldiniz!')
      navigate('/panel', { replace: true })
    } catch (err) {
      setError(err.message)
      toast.error(err.message)
      setSubmitting(false)
    }
  }

  // Hide the form while the session is being verified, to avoid a flash.
  if (sessionLoading) return <Loading fullScreen text="Oturum kontrol ediliyor..." />
  if (isAuthenticated) return <Navigate to="/panel" replace />

  return (
    <div className="min-h-screen bg-white px-4 py-12 sm:py-16">
      <div className="mx-auto w-full max-w-md">
        <div className="mb-8 text-center">
          <BrandLogo />
        </div>

        <div className="rounded-2xl border border-gray-200 p-6 sm:p-8">
          <h1 className="text-2xl font-bold tracking-tight text-gray-900">
            Ücretsiz hesabınızı oluşturun
          </h1>
          <p className="mt-1.5 text-sm text-gray-500">
            Kredi kartı gerekmez. Menünüz birkaç dakikada hazır.
          </p>

          {error ? (
            <div className="mt-5 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
              {error}
            </div>
          ) : null}

          <form onSubmit={onSubmit} className="mt-6 space-y-4" noValidate>
            <div>
              <label htmlFor="signup-business-name" className="label">
                İşletme Adı
              </label>
              <input
                id="signup-business-name"
                type="text"
                className="input"
                placeholder="Kahve Durağı"
                autoComplete="organization"
                value={businessName}
                onChange={(event) => setBusinessName(event.target.value)}
                disabled={submitting}
              />
              <p className="help-text">
                Menü adresiniz:{' '}
                <span className="font-medium text-gray-700">
                  {slug || 'isletmeniz'}.karecik.com
                </span>
              </p>
            </div>

            <div>
              <label htmlFor="signup-email" className="label">
                E-posta
              </label>
              <input
                id="signup-email"
                type="email"
                className="input"
                placeholder="ornek@isletmeniz.com"
                autoComplete="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                disabled={submitting}
              />
            </div>

            <div>
              <label htmlFor="signup-password" className="label">
                Şifre
              </label>
              <div className="relative">
                <input
                  id="signup-password"
                  type={passwordVisible ? 'text' : 'password'}
                  className="input pr-11"
                  placeholder="En az 8 karakter"
                  autoComplete="new-password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  disabled={submitting}
                />
                <button
                  type="button"
                  onClick={() => setPasswordVisible((previous) => !previous)}
                  className="absolute inset-y-0 right-0 flex w-11 items-center justify-center rounded-r-lg text-gray-400 hover:text-gray-600"
                  aria-label={passwordVisible ? 'Şifreyi gizle' : 'Şifreyi göster'}
                >
                  {passwordVisible ? (
                    <EyeOff className="h-4 w-4" aria-hidden="true" />
                  ) : (
                    <Eye className="h-4 w-4" aria-hidden="true" />
                  )}
                </button>
              </div>
              <p className="help-text">Şifreniz en az 8 karakter olmalıdır.</p>
            </div>

            <button type="submit" className="btn-primary w-full" disabled={submitting}>
              {submitting ? 'Hesap oluşturuluyor...' : 'Hesabımı oluştur'}
            </button>
          </form>

          <p className="mt-6 text-center text-sm text-gray-600">
            Zaten hesabınız var mı?{' '}
            <Link to="/giris" className="font-medium text-brand-600 hover:text-brand-700">
              Giriş yapın
            </Link>
          </p>
        </div>
      </div>
    </div>
  )
}
