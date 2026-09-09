import { useState } from 'react'
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router-dom'
import { Eye, EyeOff } from 'lucide-react'

import { api } from '../lib/api'
import { useAuth } from '../lib/auth.jsx'
import { useToast } from '../components/ui/Toast.jsx'
import { BrandLockup } from '../components/ui/Logo.jsx'

/** Matches minPasswordLength in the backend; the server rejects shorter ones. */
const MIN_PASSWORD_LENGTH = 8

/**
 * Sets a new password from an e-mailed link.
 *
 * The token arrives as ?token=... and is never displayed or put in the page
 * title — it is a bearer credential for the account, and a screenshot or a
 * shoulder-glance should not carry it. It is also not validated up front:
 * asking the server "is this token real?" before the new password exists would
 * add an endpoint that tells an attacker which guesses are live, so the single
 * submit is both the check and the change.
 */
export default function ResetPassword() {
  const { isAuthenticated } = useAuth()
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const toast = useToast()

  const token = (searchParams.get('token') || '').trim()

  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [visible, setVisible] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  async function onSubmit(event) {
    event.preventDefault()
    if (submitting) return

    if (password.length < MIN_PASSWORD_LENGTH) {
      setError(`Yeni şifreniz en az ${MIN_PASSWORD_LENGTH} karakter olmalıdır.`)
      return
    }
    if (password !== confirm) {
      setError('Yeni şifre ve tekrarı aynı değil.')
      return
    }

    setError('')
    setSubmitting(true)
    try {
      await api.resetPassword(token, password)
      toast.success('Şifreniz güncellendi. Yeni şifrenizle giriş yapabilirsiniz.')
      navigate('/giris', { replace: true })
    } catch (err) {
      setError(err.message)
      setSubmitting(false)
    }
  }

  if (isAuthenticated) return <Navigate to="/panel" replace />

  return (
    <div className="min-h-screen bg-white px-4 py-12 sm:py-16">
      <div className="mx-auto w-full max-w-md">
        <div className="mb-8 text-center">
          <Link to="/" aria-label="Karecik">
            <BrandLockup />
          </Link>
        </div>

        <div className="rounded-2xl border border-gray-200 p-6 sm:p-8">
          {!token ? (
            /* Someone opened /sifre-sifirla directly, or a mail client broke
               the link across lines. Saying so beats a form that can only fail. */
            <div className="text-center">
              <h1 className="text-2xl font-bold tracking-tight text-gray-900">
                Bağlantı eksik
              </h1>
              <p className="mt-3 text-sm leading-relaxed text-gray-600">
                Bu sayfa yalnızca e-postayla gönderilen sıfırlama bağlantısıyla açılabilir.
                Bağlantıyı e-postanızdan tam olarak kopyaladığınızdan emin olun.
              </p>
              <Link to="/sifremi-unuttum" className="btn-primary mt-6 w-full">
                Yeni bağlantı iste
              </Link>
            </div>
          ) : (
            <>
              <h1 className="text-2xl font-bold tracking-tight text-gray-900">Yeni şifre belirleyin</h1>
              <p className="mt-1.5 text-sm text-gray-500">
                Şifreniz değiştiğinde tüm cihazlardaki oturumlarınız kapatılır.
              </p>

              {error ? (
                <div className="mt-5 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
                  {error}
                </div>
              ) : null}

              <form onSubmit={onSubmit} className="mt-6 space-y-4" noValidate>
                <div>
                  <label htmlFor="reset-password" className="label">
                    Yeni şifre
                  </label>
                  <div className="relative">
                    <input
                      id="reset-password"
                      type={visible ? 'text' : 'password'}
                      className="input pr-11"
                      placeholder="••••••••"
                      autoComplete="new-password"
                      value={password}
                      onChange={(event) => setPassword(event.target.value)}
                      disabled={submitting}
                    />
                    <button
                      type="button"
                      onClick={() => setVisible((previous) => !previous)}
                      className="absolute inset-y-0 right-0 flex w-11 items-center justify-center rounded-r-lg text-gray-400 hover:text-gray-600"
                      aria-label={visible ? 'Şifreyi gizle' : 'Şifreyi göster'}
                    >
                      {visible ? (
                        <EyeOff className="h-4 w-4" aria-hidden="true" />
                      ) : (
                        <Eye className="h-4 w-4" aria-hidden="true" />
                      )}
                    </button>
                  </div>
                  <p className="mt-1.5 text-xs text-gray-500">
                    En az {MIN_PASSWORD_LENGTH} karakter.
                  </p>
                </div>

                <div>
                  <label htmlFor="reset-confirm" className="label">
                    Yeni şifre (tekrar)
                  </label>
                  <input
                    id="reset-confirm"
                    type={visible ? 'text' : 'password'}
                    className="input"
                    placeholder="••••••••"
                    autoComplete="new-password"
                    value={confirm}
                    onChange={(event) => setConfirm(event.target.value)}
                    disabled={submitting}
                  />
                </div>

                <button type="submit" className="btn-primary w-full" disabled={submitting}>
                  {submitting ? 'Kaydediliyor...' : 'Şifreyi güncelle'}
                </button>
              </form>

              <p className="mt-6 text-center text-sm text-gray-600">
                <Link to="/giris" className="font-medium text-brand-600 hover:text-brand-700">
                  Giriş sayfasına dön
                </Link>
              </p>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
