import { ChevronRight, UtensilsCrossed } from 'lucide-react'
import { Link } from 'react-router-dom'

import { getSubdomain } from '../../lib/subdomain'
import { isRtl } from '../../locales/index.js'
import { themeVariables } from '../../themes/themes'
import MenuFooter from './MenuFooter.jsx'

/**
 * The tenant landing page: {business-slug}.karecik.com with no menu in the path.
 *
 * It renders when the backend identified the business but NOT a single menu —
 * the payload arrives with `menu_resolved: false`. There is consequently no
 * menu here, and every setting lives on a menu: no theme, no logo, no accent
 * colour, no Wi-Fi, no languages. So the page is drawn with the neutral
 * defaults of themeVariables(), which keeps the same --menu-* contract the rest
 * of the customer menu reads through var() and stops this screen from looking
 * like it came from a different product.
 *
 * Two states share the layout, because they are the same page with a different
 * number of menus on it:
 *   two or more menus -> one tappable card each
 *   none              -> the "no active menus" placeholder
 * (Exactly one menu never reaches here: the backend resolves it and CustomerMenu
 * sends the visitor on to that menu's own address.)
 */

/** Neutral --menu-* values: no theme, no accent colour, no font override. */
const NEUTRAL_THEME = themeVariables(undefined, undefined, undefined)

/** Clamp to two lines without needing the Tailwind line-clamp plugin. */
const TWO_LINES = {
  display: '-webkit-box',
  WebkitLineClamp: 2,
  WebkitBoxOrient: 'vertical',
  overflow: 'hidden',
}

// Turkish is the product language; every other language falls back to English,
// exactly as the inline strings in MenuContent do.
const COPY = {
  tr: {
    choose: 'Bir menü seçin',
    empty: 'Yayında menü yok',
    emptyDetail: 'Bu işletmenin şu anda yayında bir menüsü bulunmuyor.',
  },
  en: {
    choose: 'Choose a menu',
    empty: 'No active menus',
    emptyDetail: 'This business has no published menu right now.',
  },
}

function copyFor(language) {
  return COPY[language] || COPY.en
}

/**
 * Where a card points.
 *
 * On a tenant subdomain the host already names the business, so the menu is a
 * single path segment away; on the path fallback both segments are needed.
 * Both forms are routes of this same app, so <Link> keeps it a client-side
 * navigation instead of a full page load.
 */
function menuPath(businessSlug, menuSlug) {
  return getSubdomain() ? `/${menuSlug}` : `/m/${businessSlug}/${menuSlug}`
}

/**
 * @param {object}  business - PublicMenu.business — tenant identity only here
 * @param {Array}   menus    - PublicMenu.menus, [{ slug, name, description }]
 * @param {string}  language - Active language code
 * @param {boolean} embedded - Rendered inside an iframe / narrow container
 */
export default function MenuDirectory({ business, menus, language = 'tr', embedded = false }) {
  const tenant = business || {}
  const entries = Array.isArray(menus) ? menus.filter((entry) => entry?.slug) : []
  const businessSlug = tenant.business_slug || ''
  const text = copyFor(language)

  // `business_name` is the tenant; `name` is a menu name and is absent on an
  // unresolved payload. The slug is the last resort — it is at least the thing
  // the visitor typed.
  const heading = tenant.business_name || tenant.name || businessSlug

  return (
    <div
      className={embedded ? 'min-h-full w-full' : 'min-h-screen w-full'}
      style={NEUTRAL_THEME}
      dir={isRtl(language) ? 'rtl' : 'ltr'}
    >
      <div className="mx-auto max-w-lg px-4 py-5">
        <header className="pt-2">
          <h1
            className="text-xl font-semibold leading-tight"
            style={{ color: 'var(--menu-text)' }}
          >
            {heading}
          </h1>
          {entries.length > 0 ? (
            <p className="mt-1 text-sm" style={{ color: 'var(--menu-muted)' }}>
              {text.choose}
            </p>
          ) : null}
        </header>

        {entries.length === 0 ? (
          /* --------------------------------------------- no active menus */
          <div className="mt-12 flex flex-col items-center px-2 text-center">
            <div
              className="flex h-14 w-14 items-center justify-center rounded-full"
              style={{
                backgroundColor: 'var(--menu-surface)',
                border: '1px solid var(--menu-border)',
              }}
            >
              <UtensilsCrossed
                className="h-6 w-6"
                style={{ color: 'var(--menu-muted)' }}
                aria-hidden="true"
              />
            </div>

            <h2 className="mt-4 text-base font-semibold" style={{ color: 'var(--menu-text)' }}>
              {text.empty}
            </h2>
            <p className="mt-1.5 text-sm" style={{ color: 'var(--menu-muted)' }}>
              {text.emptyDetail}
            </p>
          </div>
        ) : (
          /* ------------------------------------------------- menu cards */
          <nav className="mt-5 flex flex-col gap-2.5" aria-label={text.choose}>
            {entries.map((entry) => (
              <Link
                key={entry.slug}
                to={menuPath(businessSlug, entry.slug)}
                className="flex items-center gap-3 px-4 py-3.5"
                style={{
                  backgroundColor: 'var(--menu-surface)',
                  border: '1px solid var(--menu-border)',
                  borderRadius: 'var(--menu-radius)',
                  boxShadow: 'var(--menu-shadow)',
                }}
              >
                <div className="min-w-0 flex-1">
                  <p
                    className="truncate text-sm font-medium"
                    style={{ color: 'var(--menu-text)' }}
                  >
                    {entry.name || entry.slug}
                  </p>
                  {entry.description ? (
                    <p
                      className="mt-0.5 text-xs"
                      style={{ color: 'var(--menu-muted)', ...TWO_LINES }}
                    >
                      {entry.description}
                    </p>
                  ) : null}
                </div>

                <ChevronRight
                  className="h-4 w-4 shrink-0"
                  style={{
                    color: 'var(--menu-muted)',
                    transform: isRtl(language) ? 'scaleX(-1)' : 'none',
                  }}
                  aria-hidden="true"
                />
              </Link>
            ))}
          </nav>
        )}

        {/* No menu is resolved, so there is no footer payload — MenuFooter's
            home scope is just the "Karecik ile hazırlandı" signature. */}
        <MenuFooter business={tenant} footer={null} language={language} scope="home" />
      </div>
    </div>
  )
}
