import { useEffect, useMemo, useRef, useState } from 'react'
import { ArrowLeft, Check, Copy, Eye, EyeOff, Search, Star, Wifi } from 'lucide-react'

import { formatPrice } from '../../lib/format'
import { getSubdomain } from '../../lib/subdomain'
import { backgroundStyles, themeVariables } from '../../themes/themes'
import { BadgeIcon } from '../../themes/badges'
import { fontStack, loadFont } from '../../themes/fonts'
import { findAllergen, findLanguage, isRtl, t } from '../../locales/index.js'
import ProductDetailModal from './ProductDetailModal.jsx'
import MenuFooter from './MenuFooter.jsx'

/**
 * The main screen of the customer menu.
 *
 * Two components render it:
 *   1. pages/menu/CustomerMenu.jsx            — the real menu behind a QR code
 *   2. components/dashboard/LivePreview.jsx   — the dashboard live preview
 * The prop signature is therefore FIXED; do not change it.
 *
 * Colours come from theme-derived CSS custom properties rather than Tailwind
 * classes, so all six themes work with a single component tree.
 *
 * The customer is standing in the venue, so the address and the map view are
 * deliberately absent — the things worth showing here are the Wi-Fi details and
 * the menu itself.
 *
 * @param {object}   menu             - { business, categories, footer, menus }
 * @param {string}   language         - Active language code
 * @param {Function} onLanguageChange - Called when the language changes
 * @param {boolean}  embedded         - Compact rendering for narrow containers
 * @param {boolean}  showMenuSwitcher - Draw the in-menu switcher pill row when the
 *                                      tenant owns more than one menu. A visitor who
 *                                      opened a dedicated menu address asked for THAT
 *                                      menu, so CustomerMenu turns it off there; the
 *                                      dashboard preview keeps the default.
 */

/** Picks a readable text colour to sit on top of the accent colour. */
function readableTextColor(hex) {
  const clean = String(hex || '').trim().replace('#', '')
  let r
  let g
  let b

  if (clean.length === 3) {
    r = Number.parseInt(clean[0] + clean[0], 16)
    g = Number.parseInt(clean[1] + clean[1], 16)
    b = Number.parseInt(clean[2] + clean[2], 16)
  } else if (clean.length === 6) {
    r = Number.parseInt(clean.slice(0, 2), 16)
    g = Number.parseInt(clean.slice(2, 4), 16)
    b = Number.parseInt(clean.slice(4, 6), 16)
  } else {
    return '#ffffff'
  }

  if ([r, g, b].some(Number.isNaN)) return '#ffffff'

  const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255
  return luminance > 0.6 ? '#111827' : '#ffffff'
}

/** Turkish-aware lowercasing, used for search. */
function lower(value) {
  return String(value || '').toLocaleLowerCase('tr')
}

/**
 * What the header's TOP LINE shows, from `business.header_display`.
 *
 *   'logo'  logo alone
 *   'name'  the business name alone
 *   'both'  logo + business name
 *
 * It governs the top line only: the menu name underneath is a separate line
 * that renders in all three modes.
 *
 * Anything unknown — including a payload from before the column existed —
 * is 'both', which is the behaviour this component always had.
 */
function headerMode(value) {
  return value === 'logo' || value === 'name' ? value : 'both'
}

/**
 * Top padding of the content column.
 *
 * The dashboard preview draws a Dynamic Island over the first 34 px of its
 * viewport and sets `--menu-safe-top` so the header clears it. On a real phone
 * the variable is unset and the display cutout inset applies instead. Both are
 * ADDED to the normal 20 px (`py-5`) padding rather than replacing it, so a
 * device with neither still gets the ordinary spacing.
 */
const SAFE_TOP_PADDING = 'calc(1.25rem + var(--menu-safe-top, env(safe-area-inset-top, 0px)))'

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

/** Clamp to two lines without needing the Tailwind line-clamp plugin. */
const TWO_LINES = {
  display: '-webkit-box',
  WebkitLineClamp: 2,
  WebkitBoxOrient: 'vertical',
  overflow: 'hidden',
}

/** How long the "Kopyalandı" confirmation stays on the button. */
const COPY_FEEDBACK_MS = 1600

export default function MenuContent({
  menu,
  language = 'tr',
  onLanguageChange,
  embedded = false,
  showMenuSwitcher = true,
}) {
  const business = menu?.business || {}
  const categories = useMemo(() => menu?.categories || [], [menu])
  const menus = useMemo(() => (Array.isArray(menu?.menus) ? menu.menus : []), [menu])

  const [selectedCategoryId, setSelectedCategoryId] = useState(null)
  const [search, setSearch] = useState('')
  const [selectedProduct, setSelectedProduct] = useState(null)
  const [wifiRevealed, setWifiRevealed] = useState(false)
  const [copyState, setCopyState] = useState(null) // null | { field, ok }
  const copyTimer = useRef(null)

  // Load the selected font
  useEffect(() => {
    if (business.font_family) loadFont(business.font_family)
  }, [business.font_family])

  // Drop a stale selection when the menu changes (e.g. after a language switch)
  useEffect(() => {
    if (selectedCategoryId && !categories.some((category) => category.id === selectedCategoryId)) {
      setSelectedCategoryId(null)
    }
  }, [categories, selectedCategoryId])

  useEffect(() => () => clearTimeout(copyTimer.current), [])

  // The theme variables carry the theme's own background colour, so the
  // business' background must be spread AFTER them to win.
  const { containerStyle, overlayStyle } = backgroundStyles(business.theme, business)
  const style = {
    /* text_color is the fourth argument: it overrides --menu-text (and the
       container's inherited `color`) for the whole tree. Category headings,
       product titles and body text already read var(--menu-text), so this one
       override is enough — no inline colours anywhere else. */
    ...themeVariables(
      business.theme,
      business.primary_color,
      fontStack(business.font_family),
      business.text_color,
    ),
    ...containerStyle,
  }

  const onAccentText = readableTextColor(business.primary_color)
  const languages = Array.isArray(business.languages) ? business.languages : []

  /* Header display. 'logo' without a logo falls back to the name — a header with
     nothing in it is worse than the wrong mode — and 'name' never draws the
     initial-letter badge, so the name really is on its own there. */
  const headerDisplay = headerMode(business.header_display)
  const showLogo = headerDisplay !== 'name' && Boolean(business.logo_url)
  const showInitial = headerDisplay === 'both' && !business.logo_url
  const showName = headerDisplay !== 'logo' || !showLogo
  /* The logo and the initial-letter badge are the two things that can sit at
     the top of the stack; the text lines below key their top margin off it. */
  const showBrandMark = showLogo || showInitial

  /* The two names, in the order the customer needs them. `business_name` is the
     TENANT ("Melly Coffee") and `name` is the MENU ("Suadiye"), so the tenant is
     the heading and the menu is the quiet line underneath it. A payload with no
     tenant name — a preview of an older draft, say — puts the menu name on the
     top line rather than leaving the header empty. */
  const businessName = String(business.business_name || '').trim()
  const menuName = String(business.name || '').trim()
  const topLineName = businessName || menuName

  /* A tenant whose only menu is named after itself must not print the same words
     twice. Compared case-insensitively because "Melly Coffee" and "melly coffee"
     are one name to the customer reading them. */
  const showMenuName = Boolean(menuName) && lower(menuName) !== lower(topLineName)

  /* The owner's one line about the place. It arrives as a plain string — never
     null and never absent — so a trimmed emptiness test is the whole check: ''
     is how a slogan is removed and it renders nothing. `header_display` does
     not govern it; the slogan shows in all three modes. */
  const slogan = String(business.slogan || '').trim()

  /* Vertical rhythm of the stacked header. Every line opens a gap against what
     is ACTUALLY above it: `mt-3` clears the logo or the badge, the text lines
     sit closer together, and whichever element comes first carries no top
     margin at all. That is what keeps a header with no logo, no slogan and one
     language down to two name lines with nothing dangling. */
  const nameMargin = showBrandMark ? 'mt-3' : ''
  const menuNameMargin = showName ? 'mt-1' : showBrandMark ? 'mt-3' : ''
  const sloganMargin = showName || showMenuName ? 'mt-1.5' : showBrandMark ? 'mt-3' : ''

  const searchTerm = search.trim()
  const searching = searchTerm.length > 0

  const productCountLabel = (count) => (language === 'tr' ? `${count} ürün` : `${count} items`)

  /* ------------------------------------------------------------------ wifi */

  const wifiSsid = String(business.wifi_ssid || '').trim()
  const wifiPassword = String(business.wifi_password || '').trim()

  // The dot count is capped so a long password does not leak its length.
  const maskedWifiPassword = '•'.repeat(Math.min(wifiPassword.length, 12))

  const revealWifiButton = (
    <button
      type="button"
      onClick={() => setWifiRevealed((visible) => !visible)}
      aria-label={t('wifiPassword', language)}
      aria-pressed={wifiRevealed}
      className="shrink-0"
      style={{ color: 'var(--menu-muted)' }}
    >
      {wifiRevealed ? (
        <EyeOff className="h-3.5 w-3.5" aria-hidden="true" />
      ) : (
        <Eye className="h-3.5 w-3.5" aria-hidden="true" />
      )}
    </button>
  )

  async function copyValue(field, value) {
    const ok = await copyToClipboard(value)
    setCopyState({ field, ok })
    clearTimeout(copyTimer.current)
    copyTimer.current = setTimeout(() => setCopyState(null), COPY_FEEDBACK_MS)
  }

  /** Label of a copy button: idle, confirmed or failed. */
  function copyLabel(field) {
    if (copyState?.field !== field) return t('copy', language)
    return copyState.ok ? t('copied', language) : t('copyFailed', language)
  }

  /* ----------------------------------------------------------------- search */

  const searchResults = useMemo(() => {
    if (!searching) return []
    const needle = lower(searchTerm)

    return categories.flatMap((category) =>
      (category.products || [])
        .filter((product) =>
          [product.name, product.description, product.ingredients].some((field) =>
            lower(field).includes(needle),
          ),
        )
        .map((product) => ({ product, categoryName: category.name })),
    )
  }, [searching, searchTerm, categories])

  const selectedCategory = categories.find((category) => category.id === selectedCategoryId) || null
  const isHome = !searching && !selectedCategory

  /* --------------------------------------------------------- menu switching */

  // The prop signature is fixed, so switching menus is a navigation rather than
  // a callback. Both targets are routes of this same app, so only the path
  // changes: on a tenant subdomain the host already names the business and the
  // menu is one segment away, while the path fallback has to carry both slugs —
  // a menu slug is unique only inside its own business.
  function switchMenu(slug) {
    if (!slug || slug === business.menu_slug) return

    // The dashboard live preview and the landing iframe render this component
    // too; navigating there would tear the surrounding page down.
    if (embedded) return

    if (getSubdomain()) {
      window.location.assign(`/${slug}`)
      return
    }

    // Without the tenant segment `/m/{slug}` resolves the MENU slug as a
    // business slug and 404s, so an unidentified tenant stays where it is.
    const tenant = business.business_slug
    if (!tenant) return
    window.location.assign(`/m/${tenant}/${slug}`)
  }

  /* ------------------------------------------------------------- fragments */

  function ProductRow({ product, categoryName }) {
    const allergens = Array.isArray(product.allergens) ? product.allergens : []
    const badges = Array.isArray(product.badges) ? product.badges.filter((b) => b?.text) : []
    /* The chip states a fact, so anything that is not a positive number — null,
       a zero, a stray string — reads as "unknown" and the chip stays away. The
       detail sheet applies exactly the same rule, so a product can never carry
       a calorie chip on the card and none in the sheet. */
    const caloriesNumber = Number(product.calories)
    const calories =
      Number.isFinite(caloriesNumber) && caloriesNumber > 0 ? caloriesNumber : null
    // Only that the product HAS options is shown here; the groups themselves
    // belong to the detail sheet, which is where a choice can be made.
    const hasOptions = Array.isArray(product.options) && product.options.length > 0
    const hasMeta =
      badges.length > 0 ||
      calories != null ||
      hasOptions ||
      allergens.length > 0 ||
      product.is_featured ||
      product.is_active === false

    return (
      <button
        type="button"
        onClick={() => setSelectedProduct(product)}
        className="flex w-full items-start gap-3 p-3 text-left"
        style={{
          backgroundColor: 'var(--menu-surface)',
          borderRadius: 'var(--menu-radius)',
          border: '1px solid var(--menu-border)',
          boxShadow: 'var(--menu-shadow)',
          opacity: product.is_active === false ? 0.5 : 1,
        }}
      >
        {product.image_url ? (
          <img
            src={product.image_url}
            alt=""
            className="h-[72px] w-[72px] shrink-0 object-cover"
            style={{ borderRadius: 'calc(var(--menu-radius) * 0.7)' }}
          />
        ) : null}

        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="font-medium leading-snug" style={{ color: 'var(--menu-text)' }}>
                {product.name}
              </p>
              {categoryName ? (
                <p className="mt-0.5 text-[11px]" style={{ color: 'var(--menu-muted)' }}>
                  {categoryName}
                </p>
              ) : null}
            </div>

            <div className="shrink-0 text-right">
              {product.compare_price ? (
                <p className="text-[11px] line-through" style={{ color: 'var(--menu-muted)' }}>
                  {formatPrice(product.compare_price, business.currency)}
                </p>
              ) : null}
              <p className="font-semibold" style={{ color: 'var(--menu-primary)' }}>
                {formatPrice(product.price, business.currency)}
              </p>
            </div>
          </div>

          {product.description ? (
            <p
              className="mt-1 text-xs leading-relaxed"
              style={{ ...TWO_LINES, color: 'var(--menu-muted)' }}
            >
              {product.description}
            </p>
          ) : null}

          {/* One wrapping meta row: pills stay small so the card keeps its height */}
          {hasMeta && (
            <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
              {product.is_featured ? (
                <span
                  className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium"
                  style={{
                    backgroundColor: 'var(--menu-primary)',
                    color: onAccentText,
                  }}
                >
                  <Star className="h-2.5 w-2.5" aria-hidden="true" />
                  {t('featured', language)}
                </span>
              ) : null}

              {product.is_active === false ? (
                <span
                  className="rounded-full px-2 py-0.5 text-[10px] font-medium"
                  style={{ border: '1px solid var(--menu-border)', color: 'var(--menu-muted)' }}
                >
                  Gizli
                </span>
              ) : null}

              {badges.map((badge, index) => (
                <span
                  key={badge.id || `${badge.text}-${index}`}
                  className="inline-flex max-w-[9rem] items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium"
                  style={{
                    backgroundColor: badge.bg_color || 'var(--menu-primary)',
                    color: badge.text_color || '#ffffff',
                  }}
                >
                  <BadgeIcon id={badge.icon} className="h-2.5 w-2.5 shrink-0" />
                  <span className="truncate">{badge.text}</span>
                </span>
              ))}

              {calories != null ? (
                <span
                  className="rounded-full px-2 py-0.5 text-[10px]"
                  style={{ border: '1px solid var(--menu-border)', color: 'var(--menu-muted)' }}
                >
                  {calories} {t('kcal', language)}
                </span>
              ) : null}

              {/* Quiet like the calorie chip on purpose: it is a hint that the
                  detail sheet has choices, not a badge competing with them.
                  It joins the existing wrapping row, so the card keeps its
                  height. */}
              {hasOptions ? (
                <span
                  className="rounded-full px-2 py-0.5 text-[10px]"
                  style={{ border: '1px solid var(--menu-border)', color: 'var(--menu-muted)' }}
                >
                  + Seçenekler
                </span>
              ) : null}

              {allergens.map((code) => {
                const allergen = findAllergen(code)
                if (!allergen) return null
                return (
                  <span
                    key={code}
                    title={language === 'tr' ? allergen.tr : allergen.en}
                    className="text-xs"
                  >
                    {allergen.emoji}
                  </span>
                )
              })}
            </div>
          )}
        </div>
      </button>
    )
  }

  function CategoryStrip() {
    return (
      <div className="no-scrollbar -mx-4 mb-4 flex gap-2 overflow-x-auto px-4 pb-1">
        {categories.map((category) => {
          const isSelected = category.id === selectedCategoryId
          return (
            <button
              key={category.id}
              type="button"
              onClick={() => setSelectedCategoryId(category.id)}
              className="shrink-0 whitespace-nowrap rounded-full px-3.5 py-1.5 text-sm"
              style={
                isSelected
                  ? { backgroundColor: 'var(--menu-primary)', color: onAccentText }
                  : {
                      backgroundColor: 'var(--menu-surface)',
                      color: 'var(--menu-text)',
                      border: '1px solid var(--menu-border)',
                    }
              }
            >
              {category.icon ? `${category.icon} ` : ''}
              {category.name}
            </button>
          )
        })}
      </div>
    )
  }

  function EmptyLine({ text }) {
    return (
      <p className="py-10 text-center text-sm" style={{ color: 'var(--menu-muted)' }}>
        {text}
      </p>
    )
  }

  /* -------------------------------------------------------------- render */

  return (
    <div
      className={`relative ${embedded ? 'min-h-full w-full' : 'min-h-screen w-full'}`}
      style={style}
      dir={isRtl(language) ? 'rtl' : 'ltr'}
    >
      {/* Background photo scrim; the content wrapper below sits on top of it. */}
      {overlayStyle ? (
        <div className="absolute inset-0 z-0" style={overlayStyle} aria-hidden="true" />
      ) : null}

      <div
        className="relative z-10 mx-auto max-w-lg px-4 py-5"
        style={{ paddingTop: SAFE_TOP_PADDING }}
      >
        {/* --------------------------------- header: one centred, stacked block */}
        {/* This used to be a single flex ROW — logo beside the business name,
            language switcher pinned right — which squeezed a wide horizontal
            logo into a 160-pixel cap to leave the text its share of the line.
            It is a centred COLUMN now: the language switcher on a row of its
            own, then the logo across the FULL content width, then the business
            name, the menu name and the slogan.

            Every absent element renders `null` rather than an empty wrapper,
            and each line's top margin is computed above from what is really
            above it, so no combination leaves a stray gap behind. */}
        <header className="flex flex-col text-center">
          {/* Lifted out of the branding block so it can no longer take width
              away from the logo. A single language draws no row at all — not an
              empty one — so nothing hangs over the logo. */}
          {languages.length > 1 ? (
            /* A segmented pill rather than loose buttons: one bordered track in
               the theme's surface colour, with the active language filled in the
               accent. It reads as a single control instead of two competing
               ones, and it inherits every theme through the --menu-* variables.

               `role="group"` plus `aria-pressed` is the honest markup for a set
               of toggles — this switches the page's language rather than
               navigating, so these are buttons, not links or a listbox. */
            <div className="mb-3 flex justify-end">
              <div
                role="group"
                aria-label={t('language', language)}
                className="inline-flex items-center gap-0.5 rounded-full p-0.5"
                style={{
                  backgroundColor: 'var(--menu-surface)',
                  border: '1px solid var(--menu-border)',
                }}
              >
                {languages.map((code) => {
                  const info = findLanguage(code)
                  const isSelected = code === language
                  return (
                    <button
                      key={code}
                      type="button"
                      onClick={() => onLanguageChange?.(code)}
                      aria-label={info.label}
                      aria-pressed={isSelected}
                      title={info.label}
                      className="rounded-full px-2.5 py-1 text-[11px] font-semibold uppercase leading-none tracking-wide"
                      style={
                        isSelected
                          ? { backgroundColor: 'var(--menu-primary)', color: onAccentText }
                          : { color: 'var(--menu-muted)', backgroundColor: 'transparent' }
                      }
                    >
                      {/* Short code instead of a flag emoji: Windows cannot draw
                          flags and rendered English as "GB". */}
                      {info.short}
                    </button>
                  )
                })}
              </div>
            </div>
          ) : null}

          {showLogo ? (
            /* The full content width and NO `max-w` at all — that is the whole
               point of the stack: a wide horizontal logo finally gets the room
               it needs, while `max-h-24` bounds a tall one and `object-contain`
               keeps every aspect ratio intact. Never cropped, never squared off.

               The logo has NO entrance animation here on purpose. An entrance
               belongs to the splash screen, which is the moment the menu opens;
               replaying it in the header meant the logo faded in again on every
               language switch and every re-render. The splash owns that motion
               through `splash_entrance`. */
            <img
              src={business.logo_url}
              alt=""
              className="mx-auto h-auto w-full max-h-24 object-contain"
              style={{ borderRadius: 'calc(var(--menu-radius) * 0.6)' }}
            />
          ) : showInitial ? (
            <div
              className="mx-auto flex h-14 w-14 items-center justify-center text-lg font-semibold"
              style={{
                backgroundColor: 'var(--menu-primary)',
                color: onAccentText,
                borderRadius: 'calc(var(--menu-radius) * 0.6)',
              }}
              aria-hidden="true"
            >
              {String(topLineName || '•').charAt(0).toLocaleUpperCase('tr')}
            </div>
          ) : null}

          {/* The venue's own name — the words on the sign outside. No address
              here on purpose: the customer is already inside. It has a centred
              row to itself now, so a long name WRAPS inside `max-w-sm` instead
              of being truncated. */}
          {showName ? (
            <h1
              className={`mx-auto max-w-sm text-xl font-semibold leading-tight ${nameMargin}`.trim()}
              style={{ color: 'var(--menu-text)' }}
            >
              {topLineName}
            </h1>
          ) : null}

          {/* Which of the venue's menus this is — smaller and muted, because it
              answers a question the customer only asks second. */}
          {showMenuName ? (
            <p
              className={`mx-auto max-w-sm text-sm ${menuNameMargin}`.trim()}
              style={{ color: 'var(--menu-muted)' }}
            >
              {menuName}
            </p>
          ) : null}

          {/* The owner's own line. Optional in the truest sense: an empty
              slogan renders nothing here, not an empty paragraph. */}
          {slogan ? (
            <p
              className={`mx-auto max-w-sm text-xs italic ${sloganMargin}`.trim()}
              style={{ color: 'var(--menu-muted)' }}
            >
              {slogan}
            </p>
          ) : null}
        </header>

        {/* ------------------------------------------------------ wi-fi card */}
        {/* Only on the home view — deeper screens are about the products. */}
        {isHome && (wifiSsid || wifiPassword) ? (
          <div
            className="mt-4 flex items-center gap-3 px-3.5 py-3"
            style={{
              backgroundColor: 'var(--menu-surface)',
              border: '1px solid var(--menu-border)',
              borderRadius: 'var(--menu-radius)',
              boxShadow: 'var(--menu-shadow)',
            }}
          >
            <Wifi
              className="h-4 w-4 shrink-0"
              style={{ color: 'var(--menu-primary)' }}
              aria-hidden="true"
            />

            <div className="min-w-0 flex-1">
              <p className="text-[11px] leading-none" style={{ color: 'var(--menu-muted)' }}>
                {wifiSsid ? t('wifiName', language) : t('wifiPassword', language)}
              </p>
              {/* With an SSID the headline is the network name and the password
                  sits on its own masked line below. Without one the password IS
                  the headline — and must still be masked, never printed in the
                  clear where anyone glancing at the table can read it. */}
              {wifiSsid ? (
                <p
                  className="mt-1 truncate text-sm font-medium leading-none"
                  style={{ color: 'var(--menu-text)' }}
                >
                  {wifiSsid}
                </p>
              ) : (
                <div className="mt-1 flex items-center gap-1.5">
                  <span
                    className="truncate text-sm font-medium leading-none"
                    style={{ color: 'var(--menu-text)' }}
                  >
                    {wifiRevealed ? wifiPassword : maskedWifiPassword}
                  </span>
                  {revealWifiButton}
                </div>
              )}

              {wifiSsid && wifiPassword ? (
                <div className="mt-1.5 flex items-center gap-1.5">
                  <span
                    className="truncate text-xs tracking-wide"
                    style={{ color: 'var(--menu-muted)' }}
                  >
                    {wifiRevealed ? wifiPassword : maskedWifiPassword}
                  </span>
                  {revealWifiButton}
                </div>
              ) : null}
            </div>

            {wifiPassword ? (
              <button
                type="button"
                onClick={() => copyValue('wifi', wifiPassword)}
                className="inline-flex shrink-0 items-center gap-1.5 rounded-full px-2.5 py-1.5 text-[11px] font-medium"
                style={{ backgroundColor: 'var(--menu-primary)', color: onAccentText }}
              >
                {copyState?.field === 'wifi' && copyState.ok ? (
                  <Check className="h-3 w-3" aria-hidden="true" />
                ) : (
                  <Copy className="h-3 w-3" aria-hidden="true" />
                )}
                {copyLabel('wifi')}
              </button>
            ) : null}
          </div>
        ) : null}

        {/* --------------------------------------------------------- search */}
        {categories.length > 0 ? (
          <div className="relative mt-4">
            <Search
              className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2"
              style={{ color: 'var(--menu-muted)' }}
              aria-hidden="true"
            />
            <input
              type="search"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={t('search', language)}
              className="w-full py-2.5 pl-9 pr-3 text-sm outline-none"
              style={{
                backgroundColor: 'var(--menu-surface)',
                color: 'var(--menu-text)',
                border: '1px solid var(--menu-border)',
                borderRadius: 'var(--menu-radius)',
              }}
            />
          </div>
        ) : null}

        {/* ------------------------------------------------- menu switcher */}
        {/* Only where the visitor did not already name a menu. A dedicated menu
            address is a request for THAT menu, so offering the tenant's other
            menus there is the picker the customer just walked past; the
            dashboard preview, which has no address, keeps it. */}
        {showMenuSwitcher && menus.length > 1 ? (
          <nav
            className="no-scrollbar -mx-4 mt-4 flex gap-2 overflow-x-auto px-4 pb-1"
            aria-label={t('menuLabel', language)}
          >
            {menus.map((entry) => {
              const isSelected = entry.slug === business.menu_slug
              return (
                <button
                  key={entry.slug}
                  type="button"
                  onClick={() => switchMenu(entry.slug)}
                  className="shrink-0 whitespace-nowrap rounded-full px-3 py-1 text-xs font-medium"
                  style={
                    isSelected
                      ? { backgroundColor: 'var(--menu-primary)', color: onAccentText }
                      : {
                          backgroundColor: 'var(--menu-surface)',
                          color: 'var(--menu-muted)',
                          border: '1px solid var(--menu-border)',
                        }
                  }
                >
                  {entry.name}
                </button>
              )
            })}
          </nav>
        ) : null}

        <div className="mt-4">
          {/* --------------------------------------------- 1) search results */}
          {searching ? (
            searchResults.length === 0 ? (
              <EmptyLine text={t('noResults', language)} />
            ) : (
              <div className="flex flex-col gap-2.5">
                {searchResults.map(({ product, categoryName }) => (
                  <ProductRow key={product.id} product={product} categoryName={categoryName} />
                ))}
              </div>
            )
          ) : selectedCategory ? (
            /* ------------------------------------------- 2) product listing */
            <>
              <div className="mb-3 flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => setSelectedCategoryId(null)}
                  aria-label={language === 'tr' ? 'Kategorilere dön' : 'Back to categories'}
                  className="flex h-8 w-8 shrink-0 items-center justify-center"
                  style={{
                    backgroundColor: 'var(--menu-surface)',
                    border: '1px solid var(--menu-border)',
                    borderRadius: 'calc(var(--menu-radius) * 0.7)',
                    color: 'var(--menu-text)',
                  }}
                >
                  <ArrowLeft
                    className="h-4 w-4"
                    style={{ transform: isRtl(language) ? 'scaleX(-1)' : 'none' }}
                  />
                </button>

                <h2
                  className="truncate text-base font-semibold"
                  style={{ color: 'var(--menu-text)' }}
                >
                  {selectedCategory.icon ? `${selectedCategory.icon} ` : ''}
                  {selectedCategory.name}
                </h2>
              </div>

              <CategoryStrip />

              {selectedCategory.description ? (
                <p className="mb-3 text-xs" style={{ color: 'var(--menu-muted)' }}>
                  {selectedCategory.description}
                </p>
              ) : null}

              {(selectedCategory.products || []).length === 0 ? (
                <EmptyLine text={t('emptyCategory', language)} />
              ) : (
                <div className="flex flex-col gap-2.5">
                  {selectedCategory.products.map((product) => (
                    <ProductRow key={product.id} product={product} />
                  ))}
                </div>
              )}
            </>
          ) : categories.length === 0 ? (
            /* ------------------------------------------------ 3) empty menu */
            <EmptyLine text={t('emptyMenu', language)} />
          ) : (
            /* --------------------------------------------- 4) category grid */
            <div className="grid grid-cols-2 gap-3">
              {categories.map((category) => (
                <button
                  key={category.id}
                  type="button"
                  onClick={() => setSelectedCategoryId(category.id)}
                  className="flex flex-col overflow-hidden text-left"
                  style={{
                    backgroundColor: 'var(--menu-surface)',
                    border: '1px solid var(--menu-border)',
                    borderRadius: 'var(--menu-radius)',
                    boxShadow: 'var(--menu-shadow)',
                    opacity: category.is_active === false ? 0.5 : 1,
                  }}
                >
                  {category.image_url ? (
                    <img
                      src={category.image_url}
                      alt=""
                      className={embedded ? 'h-20 w-full object-cover' : 'h-24 w-full object-cover'}
                    />
                  ) : (
                    <div
                      className={`flex w-full items-center justify-center text-4xl ${
                        embedded ? 'h-20' : 'h-24'
                      }`}
                    >
                      {category.icon || '🍽️'}
                    </div>
                  )}

                  <div className="px-3 py-2.5">
                    <p
                      className="truncate text-sm font-medium"
                      style={{ color: 'var(--menu-text)' }}
                    >
                      {category.name}
                    </p>
                    <p className="mt-0.5 text-[11px]" style={{ color: 'var(--menu-muted)' }}>
                      {productCountLabel((category.products || []).length)}
                    </p>
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>

        {/*
          On the home view the footer is only the "Karecik ile hazırlandı"
          signature. The price date, the VAT notice and the contact details live
          on the screens that list products.
        */}
        <MenuFooter
          business={business}
          footer={menu?.footer}
          language={language}
          scope={isHome ? 'home' : 'products'}
        />
      </div>

      <ProductDetailModal
        product={selectedProduct}
        business={business}
        language={language}
        onClose={() => setSelectedProduct(null)}
      />
    </div>
  )
}
