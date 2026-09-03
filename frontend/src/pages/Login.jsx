import { useState } from 'react'
import { Link, Navigate, useNavigate } from 'react-router-dom'
import { Eye, EyeOff } from 'lucide-react'

import { useAuth } from '../lib/auth.jsx'
import { useToast } from '../components/ui/Toast.jsx'
import Loading from '../components/ui/Loading.jsx'
import { BrandLockup } from '../components/ui/Logo.jsx'

/* Seeded account, handy for signing in quickly during development */
const DEMO_EMAIL = 'demo@karecik.com'
const DEMO_PASSWORD = 'demo1234'

/** Karecik brand lockup, linking back to the landing page. */
function BrandLogo() {
  return (
    <Link to="/" aria-label="Karecik">
      <BrandLockup />
    </Link>
  )
}

export default function Login() {
  const { isAuthenticated, loading: sessionLoading, login } = useAuth()
  const navigate = useNavigate()
  const toast = useToast()

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [passwordVisible, setPasswordVisible] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  async function onSubmit(event) {
    event.preventDefault()
    if (submitting) return

    const trimmedEmail = email.trim()
    if (!trimmedEmail || !password) {
      setError('Lütfen e-posta ve şifrenizi girin.')
      return
    }

    setError('')
    setSubmitting(true)
    try {
      await login(trimmedEmail, password)
      toast.success('Hoş geldiniz!')
      navigate('/panel', { replace: true })
    } catch (err) {
      setError(err.message)
      toast.error(err.message)
      setSubmitting(false)
    }
  }

  /** Fills the form with the demo account credentials. */
  function fillDemo() {
    setEmail(DEMO_EMAIL)
    setPassword(DEMO_PASSWORD)
    setError('')
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
          <h1 className="text-2xl font-bold tracking-tight text-gray-900">Tekrar hoş geldiniz</h1>
          <p className="mt-1.5 text-sm text-gray-500">Menünüzü yönetmek için giriş yapın.</p>

          {error ? (
            <div className="mt-5 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
              {error}
            </div>
          ) : null}

          <form onSubmit={onSubmit} className="mt-6 space-y-4" noValidate>
            <div>
              <label htmlFor="login-email" className="label">
                E-posta
              </label>
              <input
                id="login-email"
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
              <label htmlFor="login-password" className="label">
                Şifre
              </label>
              <div className="relative">
                <input
                  id="login-password"
                  type={passwordVisible ? 'text' : 'password'}
                  className="input pr-11"
                  placeholder="••••••••"
                  autoComplete="current-password"
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
            </div>

            <button type="submit" className="btn-primary w-full" disabled={submitting}>
              {submitting ? 'Giriş yapılıyor...' : 'Giriş yap'}
            </button>
          </form>

          <p className="mt-6 text-center text-sm text-gray-600">
            Hesabınız yok mu?{' '}
            <Link to="/kayit" className="font-medium text-brand-600 hover:text-brand-700">
              Ücretsiz oluşturun
            </Link>
          </p>
        </div>

        <div className="mt-4 rounded-xl border border-gray-200 bg-gray-50 p-3 text-xs text-gray-500">
          <div className="flex items-center justify-between gap-3">
            <span>
              Demo hesap: <span className="font-medium text-gray-700">{DEMO_EMAIL}</span> /{' '}
              <span className="font-medium text-gray-700">{DEMO_PASSWORD}</span>
            </span>
            <button
              type="button"
              onClick={fillDemo}
              className="btn-secondary btn-sm shrink-0"
              disabled={submitting}
            >
              Doldur
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
