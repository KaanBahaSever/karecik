import { useState } from 'react'
import { Link, Navigate } from 'react-router-dom'
import { MailCheck } from 'lucide-react'

import { api } from '../lib/api'
import { useAuth } from '../lib/auth.jsx'
import { BrandLockup } from '../components/ui/Logo.jsx'

/**
 * "Şifremi unuttum" — asks the server to e-mail a reset link.
 *
 * The success panel deliberately does NOT confirm that the address exists. The
 * endpoint answers identically either way so that nobody can use this form to
 * discover which addresses are registered, and the wording here has to hold
 * that line: it says a link was sent *if* the address is registered, which is
 * true in both cases. Changing it to "e-postanıza gönderdik" would leak the
 * very thing the server is careful not to.
 */
export default function ForgotPassword() {
  const { isAuthenticated } = useAuth()

  const [email, setEmail] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [sent, setSent] = useState(false)

  async function onSubmit(event) {
    event.preventDefault()
    if (submitting) return

    const trimmed = email.trim()
    if (!trimmed) {
      setError('E-posta adresinizi girin.')
      return
    }

    setError('')
    setSubmitting(true)
    try {
      await api.forgotPassword(trimmed)
      setSent(true)
    } catch (err) {
      // Only the failures that are true regardless of the address reach here:
      // e-mail not configured on this deployment, or the rate limiter.
      setError(err.message)
    } finally {
      setSubmitting(false)
    }
  }

  // Already signed in: there is nothing to recover.
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
          {sent ? (
            <div className="text-center">
              <MailCheck className="mx-auto h-10 w-10 text-brand-600" aria-hidden="true" />
              <h1 className="mt-4 text-2xl font-bold tracking-tight text-gray-900">
                Bağlantıyı gönderdik
              </h1>
              <p className="mt-3 text-sm leading-relaxed text-gray-600">
                <span className="font-medium text-gray-900">{email.trim()}</span> adresi kayıtlıysa,
                şifre sıfırlama bağlantısı gönderildi. Gelen kutunuzu ve spam klasörünüzü kontrol
                edin.
              </p>
              <p className="mt-3 text-sm text-gray-500">
                Bağlantı <strong>1 saat</strong> geçerlidir ve yalnızca bir kez kullanılabilir.
              </p>
              <Link to="/giris" className="btn-secondary mt-6 w-full">
                Giriş sayfasına dön
              </Link>
            </div>
          ) : (
            <>
              <h1 className="text-2xl font-bold tracking-tight text-gray-900">Şifrenizi mi unuttunuz?</h1>
              <p className="mt-1.5 text-sm text-gray-500">
                Hesabınızın e-posta adresini girin, sıfırlama bağlantısını gönderelim.
              </p>

              {error ? (
                <div className="mt-5 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
                  {error}
                </div>
              ) : null}

              <form onSubmit={onSubmit} className="mt-6 space-y-4" noValidate>
                <div>
                  <label htmlFor="forgot-email" className="label">
                    E-posta
                  </label>
                  <input
                    id="forgot-email"
                    type="email"
                    className="input"
                    placeholder="ornek@isletmeniz.com"
                    autoComplete="email"
                    value={email}
                    onChange={(event) => setEmail(event.target.value)}
                    disabled={submitting}
                  />
                </div>

                <button type="submit" className="btn-primary w-full" disabled={submitting}>
                  {submitting ? 'Gönderiliyor...' : 'Sıfırlama bağlantısı gönder'}
                </button>
              </form>

              <p className="mt-6 text-center text-sm text-gray-600">
                Şifrenizi hatırladınız mı?{' '}
                <Link to="/giris" className="font-medium text-brand-600 hover:text-brand-700">
                  Giriş yapın
                </Link>
              </p>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
