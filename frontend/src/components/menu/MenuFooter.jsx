import { useEffect, useRef, useState } from 'react'
import { Check, Copy, Instagram, KeyRound, Phone, Wifi } from 'lucide-react'

import { t } from '../../locales/index.js'
import Logo from '../ui/Logo.jsx'

/**
 * Footer of the customer menu.
 *
 * It renders in two scopes:
 *
 *   scope="home"      The landing view with the category cards. Only the
 *                     "Karecik ile hazırlandı" signature is shown; contact
 *                     details and the legal notices are omitted.
 *
 *   scope="products"  Any screen listing or searching products. Contact details
 *                     and the automatically generated legal notices appear here:
 *                       "Fiyatlarımız 24.08.2026 tarihinden itibaren geçerlidir."
 *                       "Fiyatlarımıza KDV dahildir."
 *
 * Both notices are produced by the backend (repository/menu.go -> buildFooter)
 * and merely displayed here. When the business turns them off they arrive empty
 * and the corresponding line is not rendered.
 *
 * The address is deliberately absent: the customer is standing in the venue, so
 * a street address and a map view are noise. Wi-Fi is what they actually want.
 *
 * On both scopes the signature is the true page footer — last element, generous
 * top spacing, hairline rule above it.
 *
 * @param {object} business - PublicMenu.business
 * @param {object} footer   - PublicMenu.footer { price_note, vat_note, powered_by }
 * @param {string} language - Active language code
 * @param {string} scope    - "home" | "products"
 */

/** How long the "Kopyalandı" confirmation stays on the button. */
const COPY_FEEDBACK_MS = 1600

/**
 * Copies one string to the clipboard.
 *
 * navigator.clipboard is unavailable on plain-HTTP hosts and on older in-app
 * browsers, which is exactly where a QR menu tends to be opened, so the hidden
 * textarea + execCommand path stays as a fallback.
 */
async function copyToClipboard(value) {
  const text = String(value || '')
  if (!text) return false

  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
      return true
    }
  } catch {
    /* falls through to the textarea fallback below */
  }

  try {
    const area = document.createElement('textarea')
    area.value = text
    area.setAttribute('readonly', '')
    area.style.position = 'fixed'
    area.style.top = '-1000px'
    area.style.opacity = '0'
    document.body.appendChild(area)
    area.select()
    const copied = document.execCommand('copy')
    document.body.removeChild(area)
    return copied
  } catch {
    return false
  }
}

/** Small inline "Kopyala" affordance next to a Wi-Fi value. */
function CopyButton({ value, language, label }) {
  const [state, setState] = useState('idle') // idle | copied | failed
  const timer = useRef(null)

  useEffect(() => () => clearTimeout(timer.current), [])

  async function handleCopy() {
    const ok = await copyToClipboard(value)
    setState(ok ? 'copied' : 'failed')
    clearTimeout(timer.current)
    timer.current = setTimeout(() => setState('idle'), COPY_FEEDBACK_MS)
  }

  const text =
    state === 'copied'
      ? t('copied', language)
      : state === 'failed'
        ? t('copyFailed', language)
        : t('copy', language)

  return (
    <button
      type="button"
      onClick={handleCopy}
      aria-label={`${label}: ${text}`}
      className="inline-flex shrink-0 items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium"
      style={{ border: '1px solid var(--menu-border)', color: 'var(--menu-muted)' }}
    >
      {state === 'copied' ? (
        <Check className="h-3 w-3" aria-hidden="true" />
      ) : (
        <Copy className="h-3 w-3" aria-hidden="true" />
      )}
      {text}
    </button>
  )
}

/** The Karecik signature: always the last thing on the page. */
function Signature({ text }) {
  return (
    <div className="mt-12 pt-8" style={{ borderTop: '1px solid var(--menu-border)' }}>
      <p
        className="inline-flex items-center gap-1.5 pb-6 text-[11px]"
        style={{ opacity: 0.7 }}
      >
        <Logo className="h-3.5 w-3.5 shrink-0" title="" knockoutColor="var(--menu-bg)" />
        {text}
      </p>
    </div>
  )
}

export default function MenuFooter({ business, footer, language = 'tr', scope = 'products' }) {
  const signature = footer?.powered_by?.trim() || t('poweredBy', language)

  // Home view: nothing but the signature line.
  if (scope === 'home') {
    return (
      <footer className="text-center" style={{ color: 'var(--menu-muted)' }}>
        <Signature text={signature} />
      </footer>
    )
  }

  const phone = business?.phone?.trim()
  const wifiSsid = business?.wifi_ssid?.trim()
  const wifiPassword = business?.wifi_password?.trim()
  const instagram = business?.instagram?.trim().replace(/^@/, '')

  const hasContact = Boolean(phone || wifiSsid || wifiPassword || instagram)

  const priceNote = footer?.price_note?.trim()
  const vatNote = footer?.vat_note?.trim()

  return (
    <footer className="text-center" style={{ color: 'var(--menu-muted)' }}>
      {hasContact || priceNote || vatNote ? (
        <div
          className="mt-10 pt-6"
          style={{ borderTop: '1px solid var(--menu-border)' }}
        >
          {hasContact ? (
            <div className="mb-5 flex flex-col items-center gap-2 text-xs">
              {phone ? (
                <p className="flex items-center justify-center gap-1.5">
                  <Phone className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                  <span>{phone}</span>
                </p>
              ) : null}

              {wifiSsid ? (
                <div className="flex flex-wrap items-center justify-center gap-1.5">
                  <Wifi className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                  <span>
                    {t('wifiName', language)}:{' '}
                    <span style={{ color: 'var(--menu-text)' }} className="font-medium">
                      {wifiSsid}
                    </span>
                  </span>
                  <CopyButton
                    value={wifiSsid}
                    language={language}
                    label={t('wifiName', language)}
                  />
                </div>
              ) : null}

              {wifiPassword ? (
                <div className="flex flex-wrap items-center justify-center gap-1.5">
                  <KeyRound className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                  <span>
                    {t('wifiPassword', language)}:{' '}
                    <span style={{ color: 'var(--menu-text)' }} className="font-medium">
                      {wifiPassword}
                    </span>
                  </span>
                  <CopyButton
                    value={wifiPassword}
                    language={language}
                    label={t('wifiPassword', language)}
                  />
                </div>
              ) : null}

              {instagram ? (
                <p className="flex items-center justify-center gap-1.5">
                  <Instagram className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                  <span>@{instagram}</span>
                </p>
              ) : null}
            </div>
          ) : null}

          {/* Legal notices, kept up to date automatically by the system */}
          {priceNote || vatNote ? (
            <div className="space-y-1 text-[11px] leading-relaxed">
              {priceNote ? <p>{priceNote}</p> : null}
              {vatNote ? <p>{vatNote}</p> : null}
            </div>
          ) : null}
        </div>
      ) : null}

      <Signature text={signature} />
    </footer>
  )
}
