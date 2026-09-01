import { Instagram, MapPin, Phone, Wifi } from 'lucide-react'

import { t } from '../../locales/index.js'

/**
 * Footer of the customer menu.
 *
 * It renders in two scopes:
 *
 *   scope="home"      The landing view with the category cards. Only the
 *                     "Karecik ile hazırlandı" line is shown; contact details
 *                     and the legal notices are omitted.
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
 * @param {object} business - PublicMenu.business
 * @param {object} footer   - PublicMenu.footer { price_note, vat_note, powered_by }
 * @param {string} language - Active language code
 * @param {string} scope    - "home" | "products"
 */
export default function MenuFooter({ business, footer, language = 'tr', scope = 'products' }) {
  const signature = footer?.powered_by?.trim() || t('poweredBy', language)

  // Home view: nothing but the signature line.
  if (scope === 'home') {
    return (
      <footer className="mt-10 pt-6 text-center" style={{ color: 'var(--menu-muted)' }}>
        <p className="pb-6 text-[11px]" style={{ opacity: 0.7 }}>
          {signature}
        </p>
      </footer>
    )
  }

  const address = business?.address?.trim()
  const phone = business?.phone?.trim()
  const wifi = business?.wifi_password?.trim()
  const instagram = business?.instagram?.trim().replace(/^@/, '')

  const hasContact = Boolean(address || phone || wifi || instagram)

  const priceNote = footer?.price_note?.trim()
  const vatNote = footer?.vat_note?.trim()

  return (
    <footer
      className="mt-10 pt-6 text-center"
      style={{ borderTop: '1px solid var(--menu-border)', color: 'var(--menu-muted)' }}
    >
      {hasContact ? (
        <div className="mb-5 flex flex-col items-center gap-2 text-xs">
          {address ? (
            <p className="flex items-start justify-center gap-1.5">
              <MapPin className="mt-px h-3.5 w-3.5 shrink-0" aria-hidden="true" />
              <span className="max-w-xs">{address}</span>
            </p>
          ) : null}

          {phone ? (
            <p className="flex items-center justify-center gap-1.5">
              <Phone className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
              <span>{phone}</span>
            </p>
          ) : null}

          {wifi ? (
            <p className="flex items-center justify-center gap-1.5">
              <Wifi className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
              <span>
                {t('wifi', language)}:{' '}
                <span style={{ color: 'var(--menu-text)' }} className="font-medium">
                  {wifi}
                </span>
              </span>
            </p>
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
        <div className="mb-4 space-y-1 text-[11px] leading-relaxed">
          {priceNote ? <p>{priceNote}</p> : null}
          {vatNote ? <p>{vatNote}</p> : null}
        </div>
      ) : null}

      <p className="pb-6 text-[11px]" style={{ opacity: 0.7 }}>
        {signature}
      </p>
    </footer>
  )
}
