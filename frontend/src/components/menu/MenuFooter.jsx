import { buildContactItems, contactDisplayMode, trimSpace } from '../../lib/contact'
import { t } from '../../locales/index.js'
import Logo from '../ui/Logo.jsx'
import { ContactList } from './ContactInfo.jsx'

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
 *                     and the legal notices appear here:
 *                       "Fiyatlarımız 24.08.2026 tarihinden itibaren geçerlidir."
 *                       the menu's VAT sentence, while its toggle is on
 *                     plus, when the menu enables it, the "Yerli Üretim" badge
 *                     just above the signature.
 *
 * The contact details are the compact ContactList from ContactInfo.jsx, shown in
 * the 'inline', 'list' and 'footer' modes of `contact_display` and absent in
 * 'hidden'. What the home view shows for the first two modes is MenuContent's
 * business, not this footer's.
 *
 * The price date is produced by the backend (repository/menu.go -> BuildFooter)
 * and merely displayed here; it arrives empty when the menu turns it off. The
 * VAT sentence is read from the menu's own fields instead - see vatNoteText.
 *
 * The address is deliberately absent: the customer is standing in the venue, so
 * a street address and a map view are noise. Wi-Fi is what they actually want.
 *
 * On both scopes the signature is the true page footer - last element, generous
 * top spacing, hairline rule above it.
 *
 * @param {object} business - PublicMenu.business
 * @param {object} footer   - PublicMenu.footer { price_note, vat_note, powered_by }
 * @param {string} language - Active language code
 * @param {string} scope    - "home" | "products"
 */

/**
 * The VAT sentence of a menu whose toggle is on but whose own text is blank.
 *
 * It is the server's defaultVatNote (repository/menu.go), which BuildFooter puts
 * into footer.vat_note in that case, and the sentence the settings page shows as
 * that field's default.
 */
const DEFAULT_VAT_NOTE = 'Fiyatlarımıza KDV dahildir.'

/**
 * The one VAT sentence the page prints, or '' for none.
 *
 * The menu's own toggle governs it, and there is no other VAT sentence anywhere
 * on the page:
 *
 *   show_vat_note false  nothing
 *   show_vat_note true   vat_note_text, trimmed as the server trims it, or
 *                        DEFAULT_VAT_NOTE when that is blank
 *
 * This reads the business fields rather than footer.vat_note. On the customer
 * menu the two say the same thing - BuildFooter derives footer.vat_note from
 * exactly these two columns - but the dashboard live preview lays the unsaved
 * draft over the last SAVED payload, and only the business fields carry an edit
 * that is not saved yet. footer.vat_note is the fallback for a payload whose
 * business lacks the fields.
 */
function vatNoteText(business, footer) {
  const enabled = business?.show_vat_note
  if (enabled === false) return ''

  if (enabled === true) {
    const text =
      typeof business.vat_note_text === 'string' ? business.vat_note_text : footer?.vat_note
    return trimSpace(text) || DEFAULT_VAT_NOTE
  }

  return trimSpace(footer?.vat_note)
}

/**
 * The "Yerli Üretim" badge.
 *
 * `Yerli Üretim Logosu` is an official certification mark administered by the
 * Ticaret Bakanlığı, so this component NEVER draws or approximates it. Either
 * the business supplies its own certified artwork - uploaded through the
 * dashboard, or seeded as an absolute URL - and it is rendered as-is, or the
 * fallback is a plain bordered text pill that claims nothing visually.
 *
 * @param {string} logoUrl - business.yerli_uretim_logo_url, may be empty
 */
function YerliUretimBadge({ logoUrl }) {
  if (logoUrl) {
    return (
      <img
        src={logoUrl}
        alt="Yerli Üretim"
        className="h-10 w-auto max-w-[120px] object-contain"
      />
    )
  }

  return (
    <span
      className="inline-flex items-center rounded-full px-2.5 py-1 text-[10px] font-medium"
      style={{ border: '1px solid var(--menu-border)', color: 'var(--menu-muted)' }}
    >
      Yerli Üretim
    </span>
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

  /* Contact details, in every display mode but 'hidden'. The product screens
     never show the home view's chips or list, so this is where a customer who is
     browsing products finds them - and in 'footer' mode it is the only place.

     The Wi-Fi password is masked here too: these are exactly the screens a
     customer holds up at the table.

     'hidden' is checked here rather than trusted to arrive empty. The server
     redacts only the mode it has SAVED, and the dashboard preview lays the
     unsaved draft over that payload - so a draft switched to 'hidden' still
     carries every contact field. */
  const contactItems =
    contactDisplayMode(business?.contact_display) === 'hidden' ? [] : buildContactItems(business)
  const hasContact = contactItems.length > 0

  const priceNote = footer?.price_note?.trim()
  const vatNote = vatNoteText(business, footer)

  const showYerliUretim = Boolean(business?.show_yerli_uretim)
  const yerliUretimLogoUrl = business?.yerli_uretim_logo_url?.trim() || ''

  return (
    <footer className="text-center" style={{ color: 'var(--menu-muted)' }}>
      {hasContact || priceNote || vatNote ? (
        <div
          className="mt-10 pt-6"
          style={{ borderTop: '1px solid var(--menu-border)' }}
        >
          {/* Renders nothing at all without items, so no margin is left behind. */}
          <ContactList items={contactItems} language={language} compact className="mb-5" />

          {/* Legal notices, kept up to date automatically by the system */}
          {priceNote || vatNote ? (
            <div className="space-y-1 text-[11px] leading-relaxed">
              {priceNote ? <p>{priceNote}</p> : null}
              {vatNote ? <p>{vatNote}</p> : null}
            </div>
          ) : null}
        </div>
      ) : null}

      {/* The badge alone, with no VAT sentence of its own: the menu's VAT
          toggle governs the only one, vatNote above. */}
      {showYerliUretim ? (
        <div className="mt-8 flex justify-center">
          <YerliUretimBadge logoUrl={yerliUretimLogoUrl} />
        </div>
      ) : null}

      <Signature text={signature} />
    </footer>
  )
}
