import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { AlertCircle, Eye, EyeOff } from 'lucide-react'

import Modal from '../ui/Modal.jsx'
import { useToast } from '../ui/Toast.jsx'
import { useAuth } from '../../lib/auth.jsx'
import { landingText, signUpErrorMessage, validateSignUp } from '../../locales/landing.js'

/**
 * Quick sign-up dialog opened from the landing page.
 *
 * It speaks the language picked in the landing header. Every string comes from
 * locales/landing.js — including the validation messages and the request
 * errors, which the server would otherwise hand over in Turkish.
 *
 * @param {boolean}  open
 * @param {Function} onClose
 * @param {string}   language - Landing language code (tr | en | de)
 */
export default function SignUpModal({ open, onClose, language = 'tr' }) {
  const { register } = useAuth()
  const toast = useToast()
  const navigate = useNavigate()

  const t = landingText(language)

  const [businessName, setBusinessName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [passwordVisible, setPasswordVisible] = useState(false)
  // Field -> dictionary KEY (see validateSignUp), and the failed request itself.
  // Both are turned into words during render, so they follow the language on
  // screen instead of freezing the one that was active when the form was sent.
  const [errors, setErrors] = useState({})
  const [requestError, setRequestError] = useState(null)
  const [submitting, setSubmitting] = useState(false)

  // Clear any leftover errors every time the dialog opens.
  useEffect(() => {
    if (!open) return
    setErrors({})
    setRequestError(null)
    setSubmitting(false)
    setPasswordVisible(false)
  }, [open])

  async function onSubmit(event) {
    event.preventDefault()
    if (submitting) return

    setRequestError(null)
    const found = validateSignUp({ businessName, email, password })
    setErrors(found)
    if (Object.keys(found).length > 0) return

    setSubmitting(true)
    try {
      await register(businessName.trim(), email.trim(), password)
      // The toast's own close button is announced in the visitor's language
      // too — the provider's default label is the panel's Turkish.
      toast.success(t.signUpWelcome, { closeLabel: t.dismissNotification })
      onClose?.()
      navigate('/panel')
    } catch (error) {
      setRequestError(error)
      toast.error(signUpErrorMessage(error, language), { closeLabel: t.dismissNotification })
      setSubmitting(false)
    }
  }

  const generalError = requestError ? signUpErrorMessage(requestError, language) : ''

  return (
    <Modal
      open={open}
      onClose={submitting ? () => {} : onClose}
      title={t.signUpTitle}
      description={t.signUpDescription}
      // The dialog's X button. components/ui/Modal.jsx labels it "Kapat" by
      // default, which is right for the Turkish panel; here it is announced
      // in the language the visitor picked on the landing page.
      closeLabel={t.close}
      width="max-w-md"
      footer={
        <button
          type="submit"
          form="karecik-signup-form"
          disabled={submitting}
          className="btn-primary w-full sm:w-auto"
        >
          {submitting ? t.signUpSubmitting : t.signUpSubmit}
        </button>
      }
    >
      <form id="karecik-signup-form" onSubmit={onSubmit} noValidate className="space-y-4">
        {generalError ? (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2.5 text-sm text-red-700"
          >
            <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
            <p className="leading-snug">{generalError}</p>
          </div>
        ) : null}

        <div>
          <label className="label" htmlFor="signup-business-name">
            {t.signUpBusinessName}
          </label>
          <input
            id="signup-business-name"
            type="text"
            className="input"
            placeholder={t.signUpBusinessNamePlaceholder}
            autoComplete="organization"
            value={businessName}
            disabled={submitting}
            aria-invalid={errors.businessName ? true : undefined}
            aria-describedby={errors.businessName ? 'signup-business-name-error' : undefined}
            onChange={(event) => setBusinessName(event.target.value)}
          />
          {errors.businessName ? (
            <p id="signup-business-name-error" className="error-text">
              {t[errors.businessName]}
            </p>
          ) : null}
        </div>

        <div>
          <label className="label" htmlFor="signup-email">
            {t.signUpEmail}
          </label>
          <input
            id="signup-email"
            type="email"
            className="input"
            placeholder={t.signUpEmailPlaceholder}
            autoComplete="email"
            value={email}
            disabled={submitting}
            aria-invalid={errors.email ? true : undefined}
            aria-describedby={errors.email ? 'signup-email-error' : undefined}
            onChange={(event) => setEmail(event.target.value)}
          />
          {errors.email ? (
            <p id="signup-email-error" className="error-text">
              {t[errors.email]}
            </p>
          ) : null}
        </div>

        <div>
          <label className="label" htmlFor="signup-password">
            {t.signUpPassword}
          </label>
          <div className="relative">
            <input
              id="signup-password"
              type={passwordVisible ? 'text' : 'password'}
              className="input pr-11"
              placeholder={t.signUpPasswordPlaceholder}
              autoComplete="new-password"
              value={password}
              disabled={submitting}
              aria-invalid={errors.password ? true : undefined}
              aria-describedby="signup-password-note"
              onChange={(event) => setPassword(event.target.value)}
            />
            <button
              type="button"
              onClick={() => setPasswordVisible((previous) => !previous)}
              className="absolute right-1 top-1/2 -translate-y-1/2 rounded-lg p-2 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
              aria-label={passwordVisible ? t.signUpHidePassword : t.signUpShowPassword}
            >
              {passwordVisible ? (
                <EyeOff className="h-4 w-4" aria-hidden="true" />
              ) : (
                <Eye className="h-4 w-4" aria-hidden="true" />
              )}
            </button>
          </div>
          {/* One id for both lines: only one of them is ever on screen, and the
              field is described by whichever it is. */}
          {errors.password ? (
            <p id="signup-password-note" className="error-text">
              {t[errors.password]}
            </p>
          ) : (
            <p id="signup-password-note" className="help-text">
              {t.signUpPasswordHelp}
            </p>
          )}
        </div>

        <p className="border-t border-gray-200 pt-4 text-sm text-gray-600">
          {t.signUpHaveAccount}{' '}
          <Link
            to="/giris"
            onClick={() => onClose?.()}
            className="font-medium text-brand-600 hover:text-brand-700"
          >
            {t.signUpSignInLink}
          </Link>
        </p>
      </form>
    </Modal>
  )
}
