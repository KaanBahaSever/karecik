import { useState } from 'react'
import { KeyRound, ShieldCheck } from 'lucide-react'

import { useAuth } from '../../lib/auth.jsx'
import { useToast } from '../../components/ui/Toast.jsx'

/**
 * Account settings — everything that belongs to the LOGIN rather than to a menu.
 *
 * It lives on its own page because every other settings surface in the panel is
 * scoped to the active menu, and a password is not: a business with no menus at
 * all must still be able to change it, which a menu-scoped page cannot promise.
 */

/** Matches the server's rule; the form refuses early rather than round-tripping. */
const MIN_PASSWORD_LENGTH = 8

export default function Account() {
  const { user, changePassword } = useAuth()
  const toast = useToast()

  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  async function onSubmit(event) {
    event.preventDefault()
    if (saving) return

    /* The confirmation field is checked HERE and nowhere else: the server never
       sees it, because "you typed your new password twice" is a question about
       this form, not about the account. */
    if (newPassword !== confirmPassword) {
      setError('Yeni şifre ve tekrarı aynı değil.')
      return
    }
    if (newPassword.length < MIN_PASSWORD_LENGTH) {
      setError(`Yeni şifreniz en az ${MIN_PASSWORD_LENGTH} karakter olmalıdır.`)
      return
    }
    if (newPassword === currentPassword) {
      setError('Yeni şifreniz mevcut şifrenizden farklı olmalıdır.')
      return
    }

    setError('')
    setSaving(true)
    try {
      const result = await changePassword(currentPassword, newPassword)
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')

      /* The count comes from the server because only it knows how many other
         browsers were signed in. Saying so matters: the user asked to change a
         password, and being told their other devices were logged out explains
         a consequence they would otherwise discover by surprise. */
      const revoked = Number(result?.revoked_sessions) || 0
      toast.success(
        revoked > 0
          ? `Şifreniz güncellendi. Diğer ${revoked} oturum kapatıldı.`
          : 'Şifreniz güncellendi.',
      )
    } catch (err) {
      // A wrong current password comes back 401 with its own Turkish message.
      setError(err.message)
      toast.error(err.message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="mx-auto w-full max-w-3xl pb-10">
      <div className="mb-6">
        <h1 className="text-2xl font-semibold text-gray-900">Hesap</h1>
        <p className="mt-1 text-sm text-gray-500">
          Giriş bilgilerinizi yönetin. Menü ayarları için “Görünüm ve Ayarlar” sayfasını
          kullanın.
        </p>
      </div>

      <div className="space-y-6">
        <section className="card p-5">
          <div className="mb-4 flex items-center gap-2">
            <ShieldCheck className="h-4 w-4 text-brand-600" aria-hidden="true" />
            <h2 className="text-base font-semibold text-gray-900">Giriş Bilgileri</h2>
          </div>
          <dl className="text-sm">
            <dt className="text-gray-500">E-posta</dt>
            <dd className="mt-0.5 font-medium text-gray-900">{user?.email || '—'}</dd>
          </dl>
          <p className="help-text mt-3">
            E-posta adresi şu an panelden değiştirilemiyor.
          </p>
        </section>

        <section className="card p-5">
          <div className="mb-4 flex items-center gap-2">
            <KeyRound className="h-4 w-4 text-brand-600" aria-hidden="true" />
            <h2 className="text-base font-semibold text-gray-900">Şifre Değiştir</h2>
          </div>

          {error ? (
            <div className="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
              {error}
            </div>
          ) : null}

          <form onSubmit={onSubmit} className="space-y-4" noValidate>
            <div>
              <label className="label" htmlFor="current-password">
                Mevcut şifreniz
              </label>
              <input
                id="current-password"
                type="password"
                className="input"
                autoComplete="current-password"
                value={currentPassword}
                onChange={(event) => setCurrentPassword(event.target.value)}
                disabled={saving}
              />
            </div>

            <div>
              <label className="label" htmlFor="new-password">
                Yeni şifre
              </label>
              <input
                id="new-password"
                type="password"
                className="input"
                autoComplete="new-password"
                value={newPassword}
                onChange={(event) => setNewPassword(event.target.value)}
                disabled={saving}
              />
              <p className="help-text">En az {MIN_PASSWORD_LENGTH} karakter.</p>
            </div>

            <div>
              <label className="label" htmlFor="confirm-password">
                Yeni şifre (tekrar)
              </label>
              <input
                id="confirm-password"
                type="password"
                className="input"
                autoComplete="new-password"
                value={confirmPassword}
                onChange={(event) => setConfirmPassword(event.target.value)}
                disabled={saving}
              />
            </div>

            <div className="flex items-start gap-2 rounded-lg border border-blue-200 bg-blue-50 px-3 py-2.5">
              <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-blue-600" aria-hidden="true" />
              <p className="text-xs leading-relaxed text-blue-800">
                Şifrenizi değiştirdiğinizde diğer cihazlardaki oturumlarınız kapatılır. Bu
                cihazdaki oturumunuz açık kalır.
              </p>
            </div>

            <button
              type="submit"
              className="btn-primary"
              disabled={saving || !currentPassword || !newPassword || !confirmPassword}
            >
              {saving ? 'Güncelleniyor...' : 'Şifreyi güncelle'}
            </button>
          </form>
        </section>
      </div>
    </div>
  )
}
