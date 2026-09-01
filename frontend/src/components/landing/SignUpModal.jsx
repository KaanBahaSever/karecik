import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { AlertCircle, Eye, EyeOff } from 'lucide-react'

import Modal from '../ui/Modal.jsx'
import { useToast } from '../ui/Toast.jsx'
import { useAuth } from '../../lib/auth.jsx'

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/

/**
 * Quick sign-up dialog opened from the landing page.
 *
 * @param {boolean}  open
 * @param {Function} onClose
 */
export default function SignUpModal({ open, onClose }) {
  const { register } = useAuth()
  const toast = useToast()
  const navigate = useNavigate()

  const [businessName, setBusinessName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [passwordVisible, setPasswordVisible] = useState(false)
  const [errors, setErrors] = useState({})
  const [generalError, setGeneralError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  // Clear any leftover errors every time the dialog opens.
  useEffect(() => {
    if (!open) return
    setErrors({})
    setGeneralError('')
    setSubmitting(false)
    setPasswordVisible(false)
  }, [open])

  /** Client-side validation; returns a field name -> message dictionary. */
  function validate() {
    const found = {}

    if (!businessName.trim()) {
      found.businessName = 'İşletme adını girin.'
    } else if (businessName.trim().length < 2) {
      found.businessName = 'İşletme adı en az 2 karakter olmalı.'
    }

    if (!email.trim()) {
      found.email = 'E-posta adresinizi girin.'
    } else if (!EMAIL_PATTERN.test(email.trim())) {
      found.email = 'Geçerli bir e-posta adresi girin.'
    }

    if (!password) {
      found.password = 'Bir şifre belirleyin.'
    } else if (password.length < 8) {
      found.password = 'Şifre en az 8 karakter olmalı.'
    }

    return found
  }

  async function onSubmit(event) {
    event.preventDefault()
    if (submitting) return

    setGeneralError('')
    const found = validate()
    setErrors(found)
    if (Object.keys(found).length > 0) return

    setSubmitting(true)
    try {
      await register(businessName.trim(), email.trim(), password)
      toast.success('Hoş geldiniz! Menünüzü oluşturmaya başlayabilirsiniz.')
      onClose?.()
      navigate('/panel')
    } catch (error) {
      setGeneralError(error.message)
      toast.error(error.message)
      setSubmitting(false)
    }
  }

  return (
    <Modal
      open={open}
      onClose={submitting ? () => {} : onClose}
      title="Ücretsiz hesabınızı oluşturun"
      description="Birkaç saniyede QR menünüzü hazırlamaya başlayın."
      width="max-w-md"
      footer={
        <button
          type="submit"
          form="karecik-signup-form"
          disabled={submitting}
          className="btn-primary w-full sm:w-auto"
        >
          {submitting ? 'Hesap oluşturuluyor...' : 'Hesabımı oluştur'}
        </button>
      }
    >
      <form id="karecik-signup-form" onSubmit={onSubmit} noValidate className="space-y-4">
        {generalError ? (
          <div className="flex items-start gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2.5 text-sm text-red-700">
            <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
            <p className="leading-snug">{generalError}</p>
          </div>
        ) : null}

        <div>
          <label className="label" htmlFor="signup-business-name">
            İşletme Adı
          </label>
          <input
            id="signup-business-name"
            type="text"
            className="input"
            placeholder="Örn. Kahve Durağı"
            autoComplete="organization"
            value={businessName}
            disabled={submitting}
            onChange={(event) => setBusinessName(event.target.value)}
          />
          {errors.businessName ? <p className="error-text">{errors.businessName}</p> : null}
        </div>

        <div>
          <label className="label" htmlFor="signup-email">
            E-posta
          </label>
          <input
            id="signup-email"
            type="email"
            className="input"
            placeholder="ornek@isletmem.com"
            autoComplete="email"
            value={email}
            disabled={submitting}
            onChange={(event) => setEmail(event.target.value)}
          />
          {errors.email ? <p className="error-text">{errors.email}</p> : null}
        </div>

        <div>
          <label className="label" htmlFor="signup-password">
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
              disabled={submitting}
              onChange={(event) => setPassword(event.target.value)}
            />
            <button
              type="button"
              onClick={() => setPasswordVisible((previous) => !previous)}
              className="absolute right-1 top-1/2 -translate-y-1/2 rounded-lg p-2 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
              aria-label={passwordVisible ? 'Şifreyi gizle' : 'Şifreyi göster'}
            >
              {passwordVisible ? (
                <EyeOff className="h-4 w-4" aria-hidden="true" />
              ) : (
                <Eye className="h-4 w-4" aria-hidden="true" />
              )}
            </button>
          </div>
          {errors.password ? (
            <p className="error-text">{errors.password}</p>
          ) : (
            <p className="help-text">Şifreniz en az 8 karakter olmalı.</p>
          )}
        </div>

        <p className="border-t border-gray-200 pt-4 text-sm text-gray-600">
          Zaten hesabınız var mı?{' '}
          <Link
            to="/giris"
            onClick={() => onClose?.()}
            className="font-medium text-brand-600 hover:text-brand-700"
          >
            Giriş yapın
          </Link>
        </p>
      </form>
    </Modal>
  )
}
