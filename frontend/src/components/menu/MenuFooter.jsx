import { buildContactItems, contactInFooter } from '../../lib/contact'
import { footerPriceSentence, footerVatNote } from '../../lib/format'
import { t, textDir } from '../../locales/index.js'
import Logo from '../ui/Logo.jsx'
import { ContactList } from './ContactInfo.jsx'

/**
 * Footer of the customer menu — ONE footer, identical on every public view: the
 * category grid, a product listing, search results and the tenant directory.
 *
 * It holds exactly these, top to bottom, each only when it has something to say:
 *
 *   contact list   the compact ContactList, only when the owner put the contact
 *                  details in the footer (contactInFooter: `contact_in_footer`,
 *                  or the legacy `contact_display: 'footer'`)
 *   price date     "Prices valid from 24 August 2026." in the visitor's
 *                  language, built from footer.price_date (lib/format.js)
 *   VAT note       the owner's own sentence as typed, or the localized
 *                  "prices include VAT" when the toggle is on and the text is
 *                  blank or still the stock Turkish sentence
 *   Yerli Üretim   the certification badge, when the menu enables it
 *   signature      "Powered by Karecik" with the Karecik mark — always last
 *
 * Nothing else belongs here. Loose Instagram links, Wi-Fi fields or buttons of
 * their own have no place in a footer; the owner's contact details reach it
 * only as the one compact list above, and only when asked for.
 *
 * The home view's chips or open list are MenuContent's business and follow
 * `contact_display` alone; this footer never looks at that field.
 *
 * The address is deliberately absent: the customer is standing in the venue, so
 * a street address and a map view are noise.
 *
 * @param {object} business - PublicMenu.business (or the dashboard draft over it)
 * @param {object} footer   - PublicMenu.footer { price_date, price_note, vat_note }
 * @param {string} language - Active language code
 * @param {string} scope    - "menu" (default) | "directory". The directory has no
 *                            menu, so no menu settings either: the signature alone.
 */

/**
 * The text of the "Yerli Üretim" badge. It is the name of an official Turkish
 * certification mark, so it is printed as-is in every language — marked as
 * Turkish so a screen reader pronounces it as such — and only its descriptive
 * title is translated.
 */
const YERLI_URETIM = 'Yerli Üretim'

/**
 * The "Yerli Üretim" badge.
 *
 * `Yerli Üretim Logosu` is an official certification mark administered by the
 * Ticaret Bakanlığı, so this component NEVER draws or approximates it. Either
 * the business supplies its own certified artwork - uploaded through the
 * dashboard, or seeded as an absolute URL - and it is rendered as-is, or the
 * fallback is a plain bordered text pill that claims nothing visually.
 *
 * The artwork may well be an SVG. The explicit height below is what gives an
 * SVG without width/height attributes a box at all; the width follows from its
 * viewBox, and `object-contain` keeps the mark whole inside the max width.
 *
 * @param {string} logoUrl  - business.yerli_uretim_logo_url, may be empty
 * @param {string} language - Active language code, for the descriptive title
 */
function YerliUretimBadge({ logoUrl, language }) {
  const title = t('yerliUretimTitle', language)

  if (logoUrl) {
    return (
      <img
        src={logoUrl}
        alt={YERLI_URETIM}
        title={title}
        lang="tr"
        className="h-10 w-auto max-w-[120px] object-contain"
      />
    )
  }

  return (
    <span
      lang="tr"
      title={title}
      className="inline-flex items-center rounded-full px-2.5 py-1 text-[10px] font-medium"
      style={{ border: '1px solid var(--menu-border)', color: 'var(--menu-muted)' }}
    >
      {YERLI_URETIM}
    </span>
  )
}

/** The Karecik signature: always the last thing on the page. */
function Signature({ language }) {
  return (
    <div className="mt-12 pt-8" style={{ borderTop: '1px solid var(--menu-border)' }}>
      <p
        className="inline-flex items-center gap-1.5 pb-6 text-[11px]"
        style={{ opacity: 0.7 }}
      >
        <Logo className="h-3.5 w-3.5 shrink-0" title="" knockoutColor="var(--menu-bg)" />
        {t('poweredBy', language)}
      </p>
    </div>
  )
}

export default function MenuFooter({ business, footer, language = 'tr', scope = 'menu' }) {
  // The directory lists menus; it has no prices, VAT or badge of its own.
  if (scope === 'directory') {
    return (
      <footer className="text-center" style={{ color: 'var(--menu-muted)' }}>
        <Signature language={language} />
      </footer>
    )
  }

  /* The compact contact list, when the owner put it in the footer.

     The Wi-Fi password is masked here too, with a reveal toggle: a table of
     strangers can read a phone screen.

     The switch is read here rather than trusted to arrive as empty fields. The
     server redacts the contact fields only for a menu SAVED with nothing to
     show, and the dashboard preview lays the unsaved draft over that payload —
     so a draft with the footer switched off still carries every field. */
  const contactItems = contactInFooter(business) ? buildContactItems(business) : []
  const hasContact = contactItems.length > 0

  const priceSentence = footerPriceSentence(business, footer, language)
  const vatNote = footerVatNote(business, footer, language)

  const showYerliUretim = Boolean(business?.show_yerli_uretim)
  const yerliUretimLogoUrl =
    typeof business?.yerli_uretim_logo_url === 'string' ? business.yerli_uretim_logo_url.trim() : ''

  return (
    <footer className="text-center" style={{ color: 'var(--menu-muted)' }}>
      {hasContact || priceSentence || vatNote ? (
        <div className="mt-10 pt-6" style={{ borderTop: '1px solid var(--menu-border)' }}>
          {/* Renders nothing at all without items, so no margin is left behind. */}
          <ContactList items={contactItems} language={language} compact className="mb-5" />

          {/* Legal notices, kept up to date automatically by the system */}
          {priceSentence || vatNote ? (
            <div className="space-y-1 text-[11px] leading-relaxed">
              {priceSentence ? <p>{priceSentence}</p> : null}
              {/* The owner's own words, in whatever script they typed them:
                  textDir lets a Turkish sentence keep its direction inside
                  the Arabic menu. */}
              {vatNote ? <p dir={textDir(vatNote, language)}>{vatNote}</p> : null}
            </div>
          ) : null}
        </div>
      ) : null}

      {showYerliUretim ? (
        <div className="mt-8 flex justify-center">
          <YerliUretimBadge logoUrl={yerliUretimLogoUrl} language={language} />
        </div>
      ) : null}

      <Signature language={language} />
    </footer>
  )
}
