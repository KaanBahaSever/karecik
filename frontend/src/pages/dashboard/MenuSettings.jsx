import { useEffect, useMemo, useState } from 'react'
import {
  AlertCircle,
  ArrowDown,
  ArrowUp,
  Eye,
  EyeOff,
  Globe,
  Image as ImageIcon,
  Info,
  Instagram,
  Link2,
  MapPin,
  Palette,
  Percent,
  Phone,
  Plus,
  Save,
  Sparkles,
  Store,
  Trash2,
  Wifi,
} from 'lucide-react'

import { useAuth } from '../../lib/auth.jsx'
import { useActiveMenu } from '../../lib/menuContext.jsx'
import {
  completeLinkUrl,
  CONTACT_DISPLAY_MODES,
  contactDisplayMode,
  instagramHandle,
  instagramUrl,
  keptLinkIds,
  LINK_MESSAGES,
  linkLabelProblem,
  linkUrlProblem,
  MAX_LINKS,
  trimSpace,
} from '../../lib/contact'
import { CURRENCY_LIST, formatDate, formatPrice } from '../../lib/format'
import { cleanSlugInput, MIN_SLUG_LENGTH, slugify } from '../../lib/slugify'
import { APP_DOMAIN } from '../../lib/subdomain'
import { LANGUAGES } from '../../locales/index.js'
import { FONTS, loadAllFonts } from '../../themes/fonts'
import {
  DEFAULT_SPLASH_EXIT,
  HEADER_DISPLAY_MODES,
  isValidSplashAnimation,
  isValidSplashEasing,
  SPLASH_DISPLAY_MODES,
  SPLASH_EASINGS,
  SPLASH_EXIT_ANIMATIONS,
} from '../../themes/splash'
import { backgroundStyles, THEMES } from '../../themes/themes'
import EmptyState from '../../components/ui/EmptyState.jsx'
import Loading from '../../components/ui/Loading.jsx'
import ImageUploader from '../../components/ui/ImageUploader.jsx'
import { useToast } from '../../components/ui/Toast.jsx'
import ActiveMenuBar from '../../components/dashboard/ActiveMenuBar.jsx'
import LivePreview from '../../components/dashboard/LivePreview.jsx'

/**
 * Everything a menu owns, on one page: identity, contact details, currency and
 * languages, appearance, the splash screen and the footer notices.
 *
 * A menu is the primary entity now, so this page replaces the old business-wide
 * Settings and Design pages. It saves through the menu context — the business
 * endpoint it used to write to does not exist any more.
 */

/* Menu fields owned by this page.
   EVERY control below must appear here, otherwise its value is diffed against
   nothing and silently never sent. */
const MENU_FIELDS = [
  /* 1. identity */
  'name',
  'slogan',
  'slug',
  'description',
  'logo_url',
  'cover_url',
  'header_display',
  /* 2. contact */
  'phone',
  'address',
  'instagram',
  'wifi_ssid',
  'wifi_password',
  'links',
  'contact_display',
  /* 3. currency and languages */
  'currency',
  'languages',
  'default_language',
  /* 4. appearance */
  'theme',
  'font_family',
  'primary_color',
  'text_color',
  'background_type',
  'background_color',
  'background_image_url',
  'background_overlay_opacity',
  /* 5. splash screen */
  'splash_enabled',
  'splash_logo_url',
  'splash_headline',
  'splash_text',
  'splash_bg_color',
  'splash_duration',
  'splash_entrance',
  'splash_exit_animation',
  'splash_exit_easing',
  'splash_exit_duration',
  'splash_display',
  'splash_slide_fade',
  /* 6. footer notices */
  'show_price_date',
  'show_vat_note',
  'vat_note_text',
  'show_yerli_uretim',
  'yerli_uretim_logo_url',
  /* 7. status */
  'is_active',
]

/* Text fields that may be cleared (sent as null when empty) */
const NULLABLE_FIELDS = [
  'logo_url',
  'cover_url',
  'phone',
  'address',
  'instagram',
  'wifi_ssid',
  'wifi_password',
  'background_color',
  'background_image_url',
  'splash_logo_url',
  'yerli_uretim_logo_url',
]

/* Fields sent as numbers even though the inputs hand back strings */
const NUMERIC_FIELDS = ['splash_duration', 'splash_exit_duration']

/* Text fields stored trimmed (the columns are NOT NULL, so never null) */
const TRIMMED_FIELDS = [
  'description',
  'slogan',
  'vat_note_text',
  'splash_headline',
  'splash_text',
]

/* The sentence the customer menu prints for a blank VAT text. The settings field
   shows it only as its placeholder; see buildDraft. */
const DEFAULT_VAT_NOTE = 'Fiyatlarımıza KDV dahildir.'
/* The domain every public address sits under; lib/subdomain.js owns the value */
const MENU_SUFFIX = `.${APP_DOMAIN}`
const DEFAULT_OVERLAY_OPACITY = 0.4
const DEFAULT_PRIMARY_COLOR = '#1d4ed8'
const DEFAULT_TEXT_COLOR = '#111827'
const DEFAULT_SPLASH_BG_COLOR = '#0f172a'

/**
 * How the splash content arrives, bound to `splash_entrance`.
 * Mirrors utils.SplashEntrances in backend/internal/utils/appearance.go — and
 * therefore the CHECK constraint on the menus table.
 *
 * 'fade' is the opacity 0 -> 1 the splash has always played and is the column
 * default; 'none' draws the logo and the text with no animation at all.
 */
const SPLASH_ENTRANCES = [
  { id: 'fade', label: 'Yumuşak belirsin' },
  { id: 'none', label: 'Direkt gelsin' },
]

/**
 * The two slide styles, bound to the boolean `splash_slide_fade`.
 * Mirrors utils.SlideFadeModes in backend/internal/utils/appearance.go — only
 * the labels live here, the stored value is the boolean itself.
 */
const SLIDE_FADE_MODES = [
  { value: true, label: 'Kayarken soluklaşsın' },
  { value: false, label: 'Tam opak kaysın (perde gibi)' },
]

/**
 * One line under each option of the contact display picker. The ids and labels
 * are CONTACT_DISPLAY_MODES in lib/contact.js; only the help text lives here,
 * beside the one control that shows it.
 */
const CONTACT_DISPLAY_HELP = {
  inline: 'Ana sayfada küçük düğmeler halinde yan yana dizilir; dokunulan düğmenin bilgisi açılır.',
  list: 'Ana sayfada tüm bilgiler her zaman açık bir liste halinde görünür.',
  footer: 'Ana sayfada görünmez; yalnızca ürün ekranlarının en altında yer alır.',
  hidden: 'Telefon, Instagram, Wi-Fi ve linkler menünün hiçbir yerinde gösterilmez.',
}

/*
  The owner's custom links are checked with the link rules of lib/contact.js -
  the limits, the messages and the order they are checked in - so a list this
  page lets through is not answered with a 422 the owner cannot trace back to a
  row, and the first problem reported here is the one the server reports.
*/

/* Problems typing more cannot fix, so a row shows them at once rather than
   after its field is left: a label or an address that is too long, a label
   holding a control character. */
const IMMEDIATE_LINK_PROBLEMS = [
  LINK_MESSAGES.labelTooLong,
  LINK_MESSAGES.labelInvalid,
  LINK_MESSAGES.urlTooLong,
]

/* Menu background presets — light shades first, then the darker ones */
const PRESET_BACKGROUND_COLORS = [
  '#ffffff',
  '#f8fafc',
  '#faf6ef',
  '#fffbf6',
  '#f0fdf4',
  '#eff6ff',
  '#1f2937',
  '#0f172a',
]

/* Accent colour presets */
const PRESET_COLORS = [
  '#1d4ed8',
  '#0f766e',
  '#b45309',
  '#c2410c',
  '#be123c',
  '#7c3aed',
  '#0369a1',
  '#111827',
]

/* Menu text presets — the near-blacks and warm browns first, the two light
   tones last for menus on a dark background */
const PRESET_TEXT_COLORS = [
  '#111827',
  '#1f1a17',
  '#3b2f22',
  '#0f172a',
  '#374151',
  '#4b5563',
  '#f8fafc',
  '#ffffff',
]

/* Splash background presets */
const PRESET_SPLASH_COLORS = [
  '#0f172a',
  '#111827',
  '#1d4ed8',
  '#7c3aed',
  '#be123c',
  '#c2410c',
  '#faf6ef',
  '#ffffff',
]

/* ------------------------------------------------------------- helpers */

// The address rules live in lib/slugify.js, which mirrors the Go slugifier.
// A menu slug is a path segment inside the business subdomain, so — unlike the
// business slug — it is NOT reserved-checked: "admin" is a fine menu address
// under someone else's subdomain.

/** Falls back to 'both' for an unknown or missing customer-menu header mode. */
function headerMode(value) {
  return HEADER_DISPLAY_MODES.some((mode) => mode.id === value) ? value : 'both'
}

/** Validates the #RRGGBB format. */
function isValidColor(value) {
  return /^#[0-9a-fA-F]{6}$/.test(String(value || ''))
}

/** Falls back to a value the colour input will accept. */
function safeColor(value, fallback) {
  return isValidColor(value) ? value : fallback
}

/** Keeps the overlay value inside 0..1 and falls back to the column default. */
function clampOpacity(value) {
  const number = Number(value)
  if (!Number.isFinite(number)) return DEFAULT_OVERLAY_OPACITY
  return Math.min(1, Math.max(0, number))
}

/** 0.4 -> "%40" */
function formatOpacity(value) {
  return `%${Math.round(clampOpacity(value) * 100)}`
}

/** Short readability hint shown next to the darkening slider. */
function readabilityHint(value) {
  const opacity = clampOpacity(value)
  if (opacity < 0.25) return 'Görsel çok baskın, yazılar okunmayabilir.'
  if (opacity < 0.6) return 'Dengeli: görsel görünür, yazılar okunur.'
  return 'Yazılar çok net, görsel geri planda kalır.'
}

/**
 * 1200 -> "1,2 saniye", 450 -> "0,45 saniye".
 * One decimal is enough for the hold duration, but the exit animation is set in
 * 50 ms steps, so a second decimal is kept when it carries information.
 */
function formatDuration(milliseconds) {
  const seconds = Number(milliseconds || 0) / 1000
  const text = seconds.toFixed(2).replace(/0$/, '')
  return `${text.replace('.', ',')} saniye`
}

/** The slide styles only apply to the four slide-* animations. */
function isSlideAnimation(animation) {
  return String(animation || '').startsWith('slide-')
}

let linkIdCounter = 0

/**
 * A client id for a new link row, shaped so the server keeps it.
 *
 * Not crypto.randomUUID: that exists only in secure contexts, so a dashboard
 * served over plain HTTP would have no such function. getRandomValues has no
 * such limit, and where even that is missing the time and the counter still
 * keep ids apart within the page.
 */
function newLinkId() {
  linkIdCounter += 1

  let random = ''
  try {
    const values = new Uint32Array(2)
    globalThis.crypto.getRandomValues(values)
    random = Array.from(values, (value) => value.toString(36)).join('')
  } catch {
    random = Math.random().toString(36).slice(2, 12)
  }

  return `link-${Date.now().toString(36)}-${linkIdCounter.toString(36)}-${random}`.slice(0, 64)
}

/** True for `{...}` records; false for null, arrays, strings, numbers and the like. */
function isPlainObject(value) {
  return Object.prototype.toString.call(value) === '[object Object]'
}

/** A client id for a link row that no id in `taken` already is. */
function unusedLinkId(taken) {
  let id = newLinkId()
  while (taken.has(id)) id = newLinkId()
  return id
}

/**
 * The editor rows for `menu.links`: always an array of { id, label, url } with
 * string values, in the stored order.
 *
 * A row keeps the id the server keeps (keptLinkIds); a missing, invalid or
 * repeated id is replaced with a client id that no row of the list has. The
 * editor finds rows by id and React keys them by it, so two rows sharing one
 * would move and delete together, and React would mix them up.
 */
function buildLinkRows(links) {
  if (!Array.isArray(links)) return []

  const rows = links.filter(isPlainObject)
  const kept = keptLinkIds(rows)
  const taken = new Set(kept.filter(Boolean))

  return rows.map((link, index) => {
    let id = kept[index]
    if (!id) {
      id = unusedLinkId(taken)
      taken.add(id)
    }

    return {
      id,
      label: typeof link.label === 'string' ? link.label : '',
      url: typeof link.url === 'string' ? link.url : '',
    }
  })
}

/**
 * The rows a save sends: label and URL trimmed exactly as the server trims them
 * (Go's strings.TrimSpace, see trimSpace), and every row the owner left
 * completely empty dropped — an untouched "Link ekle" row is not a link.
 */
function savableLinks(rows) {
  return (Array.isArray(rows) ? rows : [])
    .map((row) => ({ id: row.id, label: trimSpace(row.label), url: trimSpace(row.url) }))
    .filter((row) => row.label !== '' || row.url !== '')
}

/**
 * Whether two link lists would save the same thing: the same label and URL
 * pairs, in the same order. Ids are not compared — they only tell rows apart —
 * and neither are empty rows, so adding a row and leaving it blank is not an
 * unsaved change.
 */
function linksEqual(a, b) {
  const first = savableLinks(a)
  const second = savableLinks(b)
  return (
    first.length === second.length &&
    first.every((row, index) => row.label === second[index].label && row.url === second[index].url)
  )
}

/**
 * What the Instagram field shows for a stored `instagram`: the user name when
 * instagramHandle reads one in it - the rule the customer menu links with - and
 * otherwise the stored text, trimmed, as it is.
 *
 * So a stored value that holds no user name ("@", "@ x", a sentence) stays in
 * the field, with the warning under it, where the owner can see it and clear
 * it. buildDraft puts this into both the draft and `stored`, so the
 * unsaved-changes check compares the field with the stored value read the same
 * way: an untouched value is not a change, and clearing a value that holds no
 * user name is one, which saves null.
 */
function instagramFieldValue(value) {
  const text = typeof value === 'string' ? value.trim() : ''
  return instagramHandle(text) || text
}

/** Builds an editable draft from a menu object. */
function buildDraft(menu) {
  const languages =
    Array.isArray(menu.languages) && menu.languages.length > 0 ? [...menu.languages] : ['tr']

  return {
    /* identity */
    name: menu.name || '',
    // NOT NULL with a '' default on the server, so '' is a real value here too:
    // it is how the slogan is removed, and the header then prints nothing.
    slogan: menu.slogan || '',
    slug: menu.slug || '',
    description: menu.description || '',
    logo_url: menu.logo_url || null,
    cover_url: menu.cover_url || null,
    header_display: headerMode(menu.header_display),
    /* contact */
    phone: menu.phone || '',
    address: menu.address || '',
    instagram: instagramFieldValue(menu.instagram),
    wifi_ssid: menu.wifi_ssid || '',
    wifi_password: menu.wifi_password || '',
    // Always an array of string-valued rows with ids the editor can key on;
    // a payload from before the column existed simply has no links.
    links: buildLinkRows(menu.links),
    // Unknown or missing means 'inline', the column default.
    contact_display: contactDisplayMode(menu.contact_display),
    /* currency and languages */
    currency: menu.currency || 'TRY',
    languages,
    default_language: languages.includes(menu.default_language)
      ? menu.default_language
      : languages[0],
    /* appearance */
    theme: menu.theme || 'modern-light',
    font_family: menu.font_family || 'inter',
    primary_color: menu.primary_color || DEFAULT_PRIMARY_COLOR,
    text_color: menu.text_color || DEFAULT_TEXT_COLOR,
    background_type: menu.background_type === 'image' ? 'image' : 'color',
    background_color: menu.background_color || '',
    background_image_url: menu.background_image_url || null,
    background_overlay_opacity: clampOpacity(menu.background_overlay_opacity),
    /* splash screen */
    splash_enabled: Boolean(menu.splash_enabled),
    splash_logo_url: menu.splash_logo_url || null,
    splash_headline: menu.splash_headline || '',
    splash_text: menu.splash_text || '',
    splash_bg_color: menu.splash_bg_color || DEFAULT_SPLASH_BG_COLOR,
    splash_duration: Number(menu.splash_duration) || 1200,
    // Unknown or missing means 'fade': the behaviour every menu had before the
    // column existed, and the column default.
    splash_entrance: SPLASH_ENTRANCES.some((entrance) => entrance.id === menu.splash_entrance)
      ? menu.splash_entrance
      : 'fade',
    splash_exit_animation: isValidSplashAnimation(menu.splash_exit_animation)
      ? menu.splash_exit_animation
      : DEFAULT_SPLASH_EXIT.animation,
    splash_exit_easing: isValidSplashEasing(menu.splash_exit_easing)
      ? menu.splash_exit_easing
      : DEFAULT_SPLASH_EXIT.easing,
    splash_exit_duration: Number(menu.splash_exit_duration) || DEFAULT_SPLASH_EXIT.duration,
    splash_display: SPLASH_DISPLAY_MODES.some((mode) => mode.id === menu.splash_display)
      ? menu.splash_display
      : 'both',
    splash_slide_fade: menu.splash_slide_fade !== false,
    /* footer notices */
    show_price_date: Boolean(menu.show_price_date),
    show_vat_note: Boolean(menu.show_vat_note),
    // Kept blank when it is blank. The menu prints DEFAULT_VAT_NOTE for a blank
    // text, and the field shows that sentence as its placeholder only: filled
    // into the field, it would come back after a blank text is saved, and the
    // next keystroke would be appended to a sentence the owner has just deleted.
    vat_note_text: typeof menu.vat_note_text === 'string' ? menu.vat_note_text : '',
    // The column defaults to true, so a payload without the field means "on".
    show_yerli_uretim: menu.show_yerli_uretim !== false,
    yerli_uretim_logo_url: menu.yerli_uretim_logo_url || null,
    /* status */
    is_active: menu.is_active !== false,
  }
}

/**
 * Compares two field values (array order is irrelevant).
 *
 * The array branch sorts and then compares by identity. That is right for
 * `languages`, a set of codes, and wrong for `links`: an ordered array of
 * objects whose rows become new objects on every edit, so the list would read
 * as changed even after the owner typed the original values back. Links go
 * through linksEqual instead; see fieldEqual.
 */
function isEqual(a, b) {
  if (Array.isArray(a) || Array.isArray(b)) {
    const first = (Array.isArray(a) ? a : []).slice().sort()
    const second = (Array.isArray(b) ? b : []).slice().sort()
    return first.length === second.length && first.every((value, i) => value === second[i])
  }
  if (typeof a === 'boolean' || typeof b === 'boolean') return Boolean(a) === Boolean(b)
  return String(a ?? '').trim() === String(b ?? '').trim()
}

/** Compares one field of the draft with the stored menu. */
function fieldEqual(field, a, b) {
  return field === 'links' ? linksEqual(a, b) : isEqual(a, b)
}

/* ---------------------------------------------------------- small pieces */

/** On/off switch. */
function Switch({ checked, onChange, label, description, disabled = false }) {
  return (
    <div className="flex items-start justify-between gap-4">
      <div className="min-w-0">
        <p className="text-sm font-medium text-gray-900">{label}</p>
        {description ? <p className="help-text">{description}</p> : null}
      </div>

      <button
        type="button"
        role="switch"
        aria-checked={checked}
        aria-label={label}
        disabled={disabled}
        onClick={() => onChange(!checked)}
        className={`relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-brand-100 disabled:cursor-not-allowed ${
          checked ? 'bg-brand-600' : 'bg-gray-300'
        }`}
      >
        <span
          className={`inline-block h-5 w-5 rounded-full bg-white shadow transition-transform ${
            checked ? 'translate-x-[22px]' : 'translate-x-0.5'
          }`}
        />
      </button>
    </div>
  )
}

/** Card section heading. */
function SectionHeading({ icon: Icon, title, description }) {
  return (
    <div className="mb-4">
      <div className="flex items-center gap-2">
        <Icon className="h-4 w-4 text-brand-600" aria-hidden="true" />
        <h2 className="text-base font-semibold text-gray-900">{title}</h2>
      </div>
      {description ? <p className="help-text">{description}</p> : null}
    </div>
  )
}

/** Segmented control used by the header, background and splash pickers. */
function Segmented({ label, options, value, onChange, disabled = false, hint }) {
  return (
    <div>
      <span className="label">{label}</span>

      <div
        className="inline-flex flex-wrap rounded-lg border border-gray-200 bg-gray-50 p-1"
        role="group"
        aria-label={label}
      >
        {options.map((option) => {
          const isSelected = option.value === value
          return (
            <button
              key={String(option.value)}
              type="button"
              onClick={() => onChange(option.value)}
              disabled={disabled}
              aria-pressed={isSelected}
              className={`rounded-md px-3 py-1.5 text-sm disabled:cursor-not-allowed ${
                isSelected
                  ? 'bg-white font-medium text-gray-900 shadow-card'
                  : 'text-gray-500 hover:text-gray-700'
              }`}
            >
              {option.label}
            </button>
          )
        })}
      </div>

      {hint ? <p className="help-text">{hint}</p> : null}
    </div>
  )
}

/* -------------------------------------------------------------------- page */

export default function MenuSettings() {
  const { activeMenu, loading, error, saveActiveMenu } = useActiveMenu()
  const { business } = useAuth()
  const toast = useToast()

  const [draft, setDraft] = useState(null)
  const [saving, setSaving] = useState(false)
  const [slugError, setSlugError] = useState('')
  const [passwordVisible, setPasswordVisible] = useState(false)
  // The link fields the owner has already left once, as `${id}:label` and
  // `${id}:url`. On a new row a "required" or format error shows only after
  // that — or after a save attempt, which sets linkErrorsVisible — so a row does
  // not turn red while it is still being typed into. A stored row, and the
  // problems in IMMEDIATE_LINK_PROBLEMS, show theirs at once.
  const [touchedLinkFields, setTouchedLinkFields] = useState({})
  const [linkErrorsVisible, setLinkErrorsVisible] = useState(false)
  // The row whose label input takes focus after the next render ("Link ekle").
  const [focusLinkId, setFocusLinkId] = useState(null)
  // Bumped by every successful save; part of storedKey below.
  const [savedVersion, setSavedVersion] = useState(0)

  // The stored state on the server (used for comparison).
  //
  // Keyed on the menu's own values rather than on the activeMenu object. The
  // menu list hands out a new object whenever it refetches one menu — the menu
  // editor does that after every price change, to keep the price date current —
  // and keying on the object would rebuild `stored`, fire the reset below and
  // wipe whatever the owner had typed here in the meantime.
  //
  // For the same reason the values that change without this page saving are
  // left out: price_updated_at and updated_at, which a price change moves, and
  // category_count, which setMenuCategoryCount sets from the menu editor - so a
  // count set there can never reset a draft here either. Every other value goes
  // into the key exactly as the server sent it, not as buildDraft rewrites it,
  // so every stored change resets the form - a save, or a switch to another
  // menu - even one that buildDraft maps onto the draft the form already holds.
  // savedVersion covers what the values cannot: a save whose answer carries
  // exactly the values the menu already had. The key would not change, and the
  // form would keep the owner's edits and its unsaved-changes bar although the
  // server has answered with what it stores.
  const storedKey = activeMenu
    ? `${savedVersion}:` +
      JSON.stringify({
        ...activeMenu,
        price_updated_at: null,
        updated_at: null,
        category_count: null,
      })
    : ''
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const stored = useMemo(() => (activeMenu ? buildDraft(activeMenu) : null), [storedKey])

  // Refresh the draft when the active menu changes (switch, first load or save).
  useEffect(() => {
    setDraft(stored ? { ...stored } : null)
    setSlugError('')
    setTouchedLinkFields({})
    setLinkErrorsVisible(false)
  }, [stored])

  // "Link ekle" asks for its new row's label input, which exists only once that
  // render is committed. Focusing it also scrolls it into view.
  useEffect(() => {
    if (!focusLinkId) return
    document.getElementById(`link-label-${focusLinkId}`)?.focus()
    setFocusLinkId(null)
  }, [focusLinkId])

  // Render the font cards in their own typefaces.
  useEffect(() => {
    loadAllFonts()
  }, [])

  if (loading && !activeMenu) {
    return <Loading text="Menü ayarları yükleniyor..." />
  }

  // No menu, no settings: every control on this page writes onto one menu, so
  // the form is not rendered at all. The bar above carries the retry and the
  // "new menu" button in both cases.
  if (!activeMenu || !stored || !draft) {
    return (
      <div className="pb-10">
        <ActiveMenuBar />
        <EmptyState
          icon={Palette}
          title={error ? 'Menü ayarları yüklenemedi' : 'Önce bir menü oluşturun'}
          description={
            error || 'Görünüm, Wi-Fi ve karşılama ekranı ayarları seçili menüye aittir.'
          }
        />
      </div>
    )
  }

  /* --------------------------------------------------------------- helpers */

  const update = (field, value) => {
    setDraft((previous) => ({ ...previous, [field]: value }))
  }

  const changedFields = MENU_FIELDS.filter(
    (field) => !fieldEqual(field, draft[field], stored[field]),
  )
  const hasChanges = changedFields.length > 0

  // The ids of the link rows that came from the server; see the links editor.
  const storedLinkIds = new Set(stored.links.map((link) => link.id))

  // The public address is `{business-slug}.karecik.com/{menu-slug}`: the
  // subdomain names the business, this slug names the menu inside it.
  const businessSlug = business?.slug || 'isletmeniz'
  const finalSlug = slugify(draft.slug, '')
  const displaySlug = finalSlug || 'menu-adresi'
  const priceDate = formatDate(activeMenu.price_updated_at) || formatDate(new Date())

  const usesImageBackground = draft.background_type === 'image'
  const backgroundColorInvalid =
    Boolean(draft.background_color) && !isValidColor(draft.background_color)

  const primaryColor = safeColor(draft.primary_color, DEFAULT_PRIMARY_COLOR)
  const primaryColorInvalid = !isValidColor(draft.primary_color)

  const textColor = safeColor(draft.text_color, DEFAULT_TEXT_COLOR)
  const textColorInvalid = !isValidColor(draft.text_color)

  const splashEnabled = Boolean(draft.splash_enabled)
  const splashBackground = safeColor(draft.splash_bg_color, DEFAULT_SPLASH_BG_COLOR)
  const splashBackgroundInvalid = !isValidColor(draft.splash_bg_color)
  const showSlideStyle = isSlideAnimation(draft.splash_exit_animation)

  // The very same helper the customer menu uses, so the tile below is truthful.
  const { containerStyle, overlayStyle } = backgroundStyles(draft.theme, {
    background_type: draft.background_type,
    background_color: draft.background_color,
    background_image_url: draft.background_image_url,
    background_overlay_opacity: draft.background_overlay_opacity,
  })

  /** Toggles a language when its chip is clicked. */
  const toggleLanguage = (code) => {
    const isSelected = draft.languages.includes(code)

    if (isSelected && draft.languages.length <= 1) {
      toast.error('En az bir menü dili seçili kalmalıdır.')
      return
    }

    const selected = isSelected
      ? draft.languages.filter((language) => language !== code)
      : [...draft.languages, code]

    // Keep them ordered the same way as LANGUAGES.
    const nextLanguages = LANGUAGES.filter((language) => selected.includes(language.code)).map(
      (language) => language.code,
    )

    setDraft((previous) => ({
      ...previous,
      languages: nextLanguages,
      default_language: nextLanguages.includes(previous.default_language)
        ? previous.default_language
        : nextLanguages[0],
    }))
  }

  /* Link rows. Every change goes through the previous draft and finds its row
     by id, so two quick clicks in a row never act on a stale list. */

  /** Changes one field of one link row. */
  const updateLink = (id, field, value) => {
    setDraft((previous) => ({
      ...previous,
      links: previous.links.map((link) => (link.id === id ? { ...link, [field]: value } : link)),
    }))
  }

  /** Appends an empty row — never past MAX_LINKS — and focuses its label input. */
  const addLink = () => {
    const id = unusedLinkId(new Set(draft.links.map((link) => link.id)))
    setDraft((previous) =>
      previous.links.length >= MAX_LINKS
        ? previous
        : { ...previous, links: [...previous.links, { id, label: '', url: '' }] },
    )
    setFocusLinkId(id)
  }

  /** Moves a row one place up (-1) or down (+1). */
  const moveLink = (id, offset) => {
    setDraft((previous) => {
      const index = previous.links.findIndex((link) => link.id === id)
      const target = index + offset
      if (index === -1 || target < 0 || target >= previous.links.length) return previous

      const links = previous.links.slice()
      const [moved] = links.splice(index, 1)
      links.splice(target, 0, moved)
      return { ...previous, links }
    })
  }

  const removeLink = (id) => {
    setDraft((previous) => ({
      ...previous,
      links: previous.links.filter((link) => link.id !== id),
    }))
  }

  /** Records that the owner has left a link field once; see touchedLinkFields. */
  const touchLinkField = (id, field) => {
    const key = `${id}:${field}`
    setTouchedLinkFields((previous) => (previous[key] ? previous : { ...previous, [key]: true }))
  }

  /** Collects only the changed fields and saves them onto the active menu. */
  const save = async () => {
    if (!hasChanges || saving) return

    setSlugError('')

    const name = draft.name.trim()
    if (name.length < 2 || name.length > 60) {
      toast.error('Menü adı 2 ile 60 karakter arasında olmalıdır.')
      return
    }
    if (finalSlug.length < MIN_SLUG_LENGTH) {
      setSlugError(
        'Menü adresi en az 2 karakter olmalı ve yalnızca harf, rakam ve tire içerebilir.',
      )
      return
    }
    if (draft.description.trim().length > 200) {
      toast.error('Menü açıklaması en fazla 200 karakter olabilir.')
      return
    }
    // The input's maxLength already makes this unreachable from the page; the
    // check is here so the limit is stated once on each side of the wire and a
    // pasted-in value can never come back as a 422 the user cannot explain.
    if (draft.slogan.trim().length > 120) {
      toast.error('Slogan en fazla 120 karakter olabilir.')
      return
    }

    /* Links, in the server's order: the count, then each row's label and URL.
       The toast names the row by the number printed on it, and the input at
       fault takes focus, which scrolls the row into view — the save bar sits
       at the bottom of the page and the links editor may be far above it.

       Only when the links themselves changed, because only then are they sent.
       A stored link that breaks the rules — the menu endpoints return stored
       links as they are — must not stop the owner saving an unrelated setting;
       its row shows the problem either way. */
    const links = savableLinks(draft.links)
    if (changedFields.includes('links')) {
      if (links.length > MAX_LINKS) {
        setLinkErrorsVisible(true)
        toast.error(LINK_MESSAGES.tooMany)
        return
      }
      for (const link of links) {
        const labelProblem = linkLabelProblem(link.label)
        const problem = labelProblem || linkUrlProblem(link.url)
        if (!problem) continue

        const rowNumber = draft.links.findIndex((row) => row.id === link.id) + 1
        setLinkErrorsVisible(true)
        toast.error(`${rowNumber}. link: ${problem}`)
        document.getElementById(`link-${labelProblem ? 'label' : 'url'}-${link.id}`)?.focus()
        return
      }
    }
    if (draft.languages.length === 0) {
      toast.error('En az bir menü dili seçmelisiniz.')
      return
    }
    if (primaryColorInvalid) {
      toast.error('Ana renk #RRGGBB biçiminde olmalı (örn. #1d4ed8).')
      return
    }
    if (textColorInvalid) {
      toast.error('Metin rengi #RRGGBB biçiminde olmalı (örn. #111827).')
      return
    }
    if (backgroundColorInvalid) {
      toast.error('Arka plan rengi #RRGGBB biçiminde olmalıdır (örn. #ffffff).')
      return
    }
    if (splashBackgroundInvalid) {
      toast.error('Karşılama ekranı arka plan rengi #RRGGBB biçiminde olmalı.')
      return
    }
    if (draft.vat_note_text.trim().length > 200) {
      toast.error('KDV ibaresi en fazla 200 karakter olabilir.')
      return
    }

    const body = {}
    changedFields.forEach((field) => {
      if (field === 'slug') {
        body.slug = finalSlug
        return
      }
      if (field === 'name') {
        body.name = name
        return
      }
      if (field === 'links') {
        // Trimmed, with the empty rows dropped: exactly { id, label, url } per row.
        body.links = links.map(({ id, label, url }) => ({ id, label, url }))
        return
      }
      if (field === 'background_overlay_opacity') {
        body.background_overlay_opacity = clampOpacity(draft.background_overlay_opacity)
        return
      }
      if (NUMERIC_FIELDS.includes(field)) {
        body[field] = Number(draft[field])
        return
      }
      if (TRIMMED_FIELDS.includes(field)) {
        body[field] = String(draft[field] ?? '').trim()
        return
      }
      if (NULLABLE_FIELDS.includes(field)) {
        const value = String(draft[field] ?? '').trim()
        body[field] = value === '' ? null : value
        return
      }
      body[field] = draft[field]
    })

    // If only a trailing dash was trimmed there may be nothing left to send.
    if (body.slug === stored.slug) delete body.slug
    if (Object.keys(body).length === 0) {
      setDraft((previous) => ({ ...previous, slug: finalSlug }))
      return
    }

    setSaving(true)
    try {
      const updated = await saveActiveMenu(body)
      setSavedVersion((version) => version + 1)

      // The address was already taken inside this business: the server appended
      // a number, so the user is told which address was really stored.
      if (body.slug && updated?.slug && updated.slug !== body.slug) {
        toast.success(`Menü ayarları kaydedildi. Adres: ${updated.slug}`)
      } else {
        toast.success('Menü ayarları kaydedildi.')
      }
    } catch (err) {
      // A taken or reserved address comes back as 409 and belongs on the field
      // itself, not in a toast that disappears before it can be acted on.
      if (err.status === 409) {
        setSlugError(err.message)
      } else {
        toast.error(err.message)
      }
    } finally {
      setSaving(false)
    }
  }

  /* ---------------------------------------------------------------- render */

  return (
    <div className="pb-10">
      <ActiveMenuBar />

      <div className="mb-6">
        <h1 className="text-2xl font-semibold text-gray-900">Görünüm ve Ayarlar</h1>
        <p className="mt-1 text-sm text-gray-500">
          Bu sayfadaki her ayar yalnızca <b className="font-medium text-gray-700">
            {activeMenu.name}
          </b>{' '}
          menüsü için geçerlidir. Diğer menüleriniz etkilenmez.
        </p>
      </div>

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_clamp(340px,30vw,460px)]">
        {/* ------------------------------------------------------ left column */}
        <div className="min-w-0 space-y-6">
          {/* ------------------------------------------- 1. Menu identity */}
          <section className="card p-5">
            <SectionHeading
              icon={Store}
              title="Menü Kimliği"
              description="Menünün adı, adresi ve müşteri menüsünün en üstünde görünen görselleri."
            />

            <div className="space-y-5">
              <ImageUploader
                value={draft.logo_url}
                onChange={(url) => update('logo_url', url)}
                label="Logo"
                hint="Kare veya yuvarlak logo önerilir · en fazla 5 MB"
                round
              />

              <div>
                <label className="label" htmlFor="menu-name">
                  Menü adı
                </label>
                <input
                  id="menu-name"
                  type="text"
                  className="input"
                  value={draft.name}
                  maxLength={60}
                  placeholder="Kahvaltı Menüsü"
                  onChange={(event) => update('name', event.target.value)}
                />
                <p className="help-text">2 ile 60 karakter arasında olmalıdır.</p>
              </div>

              {/* The one line the owner writes about the place. Optional, and
                  empty is a real value: an empty slogan simply prints nothing
                  in the customer menu's header. */}
              <div>
                <label className="label" htmlFor="menu-slogan">
                  Slogan (opsiyonel)
                </label>
                <input
                  id="menu-slogan"
                  type="text"
                  className="input"
                  value={draft.slogan}
                  maxLength={120}
                  placeholder="Kahvenin en iyi hali"
                  onChange={(event) => update('slogan', event.target.value)}
                />
                <p className="help-text">
                  Menü başlığında işletme adının altında görünür. Boş bırakırsanız gösterilmez.
                </p>
                <p className="help-text">{draft.slogan.length} / 120 karakter</p>
              </div>

              <div>
                <label className="label" htmlFor="menu-slug">
                  Menü adresi
                </label>
                {/* The subdomain belongs to the business and is fixed here; only
                    the path segment after it names this menu. */}
                <div className="flex">
                  <span className="inline-flex shrink-0 items-center rounded-l-lg border border-r-0 border-gray-300 bg-gray-100 px-3 font-mono text-xs text-gray-500">
                    {businessSlug}
                    {MENU_SUFFIX}/
                  </span>
                  <input
                    id="menu-slug"
                    type="text"
                    className="input rounded-l-none"
                    value={draft.slug}
                    placeholder="kahvalti"
                    autoComplete="off"
                    spellCheck={false}
                    onChange={(event) => {
                      setSlugError('')
                      update('slug', cleanSlugInput(event.target.value))
                    }}
                  />
                </div>

                {slugError ? <p className="error-text">{slugError}</p> : null}

                <p className="help-text">
                  Bu menü şu adresten açılacak:{' '}
                  <b className="text-gray-700">
                    {businessSlug}
                    {MENU_SUFFIX}/{displaySlug}
                  </b>
                </p>
                <p className="help-text">
                  Bu adres bu işletmede kullanılıyorsa sonuna otomatik olarak bir numara eklenir.
                  Adresi değiştirirseniz bu menüyü gösteren eski QR kodlarınız çalışmaya devam
                  ETMEZ. Değişiklikten sonra QR kodunuzu yeniden indirin.
                </p>
              </div>

              <div>
                <label className="label" htmlFor="menu-description">
                  Açıklama (opsiyonel)
                </label>
                <textarea
                  id="menu-description"
                  rows={2}
                  className="input resize-y"
                  value={draft.description}
                  maxLength={200}
                  placeholder="Menü hakkında kısa bir not"
                  onChange={(event) => update('description', event.target.value)}
                />
                <p className="help-text">{draft.description.length} / 200 karakter</p>
              </div>

              <ImageUploader
                value={draft.cover_url}
                onChange={(url) => update('cover_url', url)}
                label="Kapak görseli (opsiyonel)"
                hint="Menünün üst bölümünde kullanılır · en fazla 5 MB"
              />

              {/* Which of the two identity fields above — the logo or the name —
                  the customer menu prints at the top of the page. */}
              <div className="border-t border-gray-100 pt-5">
                <Segmented
                  label="Menü Başlığı"
                  options={HEADER_DISPLAY_MODES.map((mode) => ({
                    value: mode.id,
                    label: mode.label,
                  }))}
                  value={draft.header_display}
                  onChange={(value) => update('header_display', value)}
                  hint="Müşteri menüsünün en üst satırında ne görüneceğini seçin. Logo yoksa işletme adı gösterilir; menü adı her modda hemen altında yer alır."
                />

                {/* The header logo has no entrance of its own any more. An
                    entrance belongs to the splash screen — the one moment the
                    menu actually opens — and it is configured there through
                    "Giriş animasyonu". In the header it replayed on every
                    language switch and every re-render, which is not an
                    entrance at all. */}
              </div>
            </div>
          </section>

          {/* ------------------------------------------------ 2. Contact */}
          <section className="card p-5">
            <SectionHeading
              icon={Phone}
              title="İletişim"
              description="Telefon, Instagram, Wi-Fi ve linkleriniz müşteri menüsünde, bu bölümün sonunda seçtiğiniz iletişim görünümüyle gösterilir. Boş bıraktıklarınız menüde hiç yer almaz."
            />

            <div className="space-y-5">
              <div>
                <label className="label" htmlFor="menu-phone">
                  <span className="inline-flex items-center gap-1.5">
                    <Phone className="h-3.5 w-3.5 text-gray-400" aria-hidden="true" />
                    Telefon
                  </span>
                </label>
                <input
                  id="menu-phone"
                  type="tel"
                  className="input"
                  value={draft.phone}
                  placeholder="+90 555 000 00 00"
                  onChange={(event) => update('phone', event.target.value)}
                />
              </div>

              <div>
                <label className="label" htmlFor="menu-address">
                  <span className="inline-flex items-center gap-1.5">
                    <MapPin className="h-3.5 w-3.5 text-gray-400" aria-hidden="true" />
                    Adres
                  </span>
                </label>
                <textarea
                  id="menu-address"
                  rows={3}
                  className="input resize-y"
                  value={draft.address}
                  placeholder="Caferağa Mah. Moda Cad. No: 12, Kadıköy / İstanbul"
                  onChange={(event) => update('address', event.target.value)}
                />
              </div>

              <div>
                <label className="label" htmlFor="menu-instagram">
                  <span className="inline-flex items-center gap-1.5">
                    <Instagram className="h-3.5 w-3.5 text-gray-400" aria-hidden="true" />
                    Instagram kullanıcı adı
                  </span>
                </label>
                <div className="flex">
                  <span className="inline-flex shrink-0 items-center rounded-l-lg border border-r-0 border-gray-300 bg-gray-100 px-3 text-sm text-gray-500">
                    @
                  </span>
                  <input
                    id="menu-instagram"
                    type="text"
                    className="input rounded-l-none"
                    value={draft.instagram}
                    placeholder="kahveduragi"
                    autoComplete="off"
                    spellCheck={false}
                    onChange={(event) =>
                      update('instagram', event.target.value.replace(/[@\s]/g, ''))
                    }
                  />
                </div>
                <p className="help-text">
                  Seçtiğiniz iletişim görünümüne göre menüde Instagram bağlantısı olarak yer alır.
                  Boş bırakırsanız gösterilmez.
                </p>
                {/* The customer menu links only a user name it can read
                    (instagramHandle in lib/contact.js: a name, @name or the
                    profile address). Anything else still appears there, as
                    plain text with no link and no "Instagram'da aç" button, so
                    this line tells the owner why. It does not block saving. */}
                {draft.instagram.trim() && !instagramUrl(draft.instagram) ? (
                  <p className="mt-1 text-xs text-amber-700">
                    Kullanıcı adınızı ya da profil adresinizi yazın; kullanıcı adı yalnızca harf,
                    rakam, nokta ve alt çizgi içerebilir. Bu haliyle menüde bağlantısız, düz metin
                    olarak görünür.
                  </p>
                ) : null}
              </div>

              <div className="grid gap-5 border-t border-gray-100 pt-5 sm:grid-cols-2">
                <div>
                  <label className="label" htmlFor="menu-wifi-ssid">
                    <span className="inline-flex items-center gap-1.5">
                      <Wifi className="h-3.5 w-3.5 text-gray-400" aria-hidden="true" />
                      Wi-Fi ağ adı
                    </span>
                  </label>
                  <input
                    id="menu-wifi-ssid"
                    type="text"
                    className="input"
                    value={draft.wifi_ssid}
                    maxLength={64}
                    placeholder="Kahve Durağı Misafir"
                    autoComplete="off"
                    spellCheck={false}
                    onChange={(event) => update('wifi_ssid', event.target.value)}
                  />
                  <p className="help-text">En fazla 64 karakter.</p>
                </div>

                <div>
                  <label className="label" htmlFor="menu-wifi-password">
                    Wi-Fi şifresi
                  </label>
                  <div className="relative">
                    <input
                      id="menu-wifi-password"
                      type={passwordVisible ? 'text' : 'password'}
                      className="input pr-11"
                      value={draft.wifi_password}
                      placeholder="kahve2026"
                      autoComplete="off"
                      onChange={(event) => update('wifi_password', event.target.value)}
                    />
                    <button
                      type="button"
                      onClick={() => setPasswordVisible((previous) => !previous)}
                      className="absolute inset-y-0 right-0 flex w-11 items-center justify-center text-gray-400 hover:text-gray-600"
                      aria-label={passwordVisible ? 'Şifreyi gizle' : 'Şifreyi göster'}
                    >
                      {passwordVisible ? (
                        <EyeOff className="h-4 w-4" aria-hidden="true" />
                      ) : (
                        <Eye className="h-4 w-4" aria-hidden="true" />
                      )}
                    </button>
                  </div>
                  <p className="help-text">Boş bırakırsanız menüde gösterilmez.</p>
                </div>
              </div>

              {/* ------------------------------------------------ custom links */}
              <div className="border-t border-gray-100 pt-5">
                <div className="flex items-center justify-between gap-3">
                  <span className="label mb-0 inline-flex items-center gap-1.5">
                    <Link2 className="h-3.5 w-3.5 text-gray-400" aria-hidden="true" />
                    Linkler
                  </span>
                  <span className="text-xs text-gray-500">
                    {draft.links.length} / {MAX_LINKS}
                  </span>
                </div>
                <p className="help-text">
                  Web siteniz, WhatsApp hattınız ya da Google Haritalar değerlendirme bağlantınız gibi
                  adresleri ekleyin. Menüde Wi-Fi, Instagram ve telefondan sonra, buradaki sırayla yer
                  alır.
                </p>

                {draft.links.length > 0 ? (
                  <ol className="mt-3 space-y-3">
                    {draft.links.map((link, index) => {
                      const rowNumber = index + 1
                      const label = trimSpace(link.label)
                      const url = trimSpace(link.url)

                      // A row with both fields empty is dropped on save, so there
                      // is nothing about it to be wrong.
                      const blank = label === '' && url === ''
                      const labelProblem = blank ? '' : linkLabelProblem(label)
                      const urlProblem = blank ? '' : linkUrlProblem(url)
                      // A stored row shows its problems from the start: nobody is
                      // typing into it, and a save of other settings does not stop
                      // on it, so this is where the owner finds out.
                      const isStoredRow = storedLinkIds.has(link.id)
                      const showLabelProblem =
                        Boolean(labelProblem) &&
                        (linkErrorsVisible ||
                          isStoredRow ||
                          Boolean(touchedLinkFields[`${link.id}:label`]) ||
                          IMMEDIATE_LINK_PROBLEMS.includes(labelProblem))
                      const showUrlProblem =
                        Boolean(urlProblem) &&
                        (linkErrorsVisible ||
                          isStoredRow ||
                          Boolean(touchedLinkFields[`${link.id}:url`]) ||
                          IMMEDIATE_LINK_PROBLEMS.includes(urlProblem))

                      return (
                        <li key={link.id} className="rounded-lg border border-gray-200 p-3">
                          <div className="mb-2 flex items-center justify-between gap-2">
                            <span className="text-xs font-medium text-gray-500">
                              {rowNumber}. link
                            </span>

                            <div className="flex items-center gap-1">
                              <button
                                type="button"
                                className="btn-ghost btn-sm px-2"
                                onClick={() => moveLink(link.id, -1)}
                                disabled={index === 0}
                                aria-label={`${rowNumber}. linki yukarı taşı`}
                                title="Yukarı taşı"
                              >
                                <ArrowUp className="h-4 w-4" aria-hidden="true" />
                              </button>
                              <button
                                type="button"
                                className="btn-ghost btn-sm px-2"
                                onClick={() => moveLink(link.id, 1)}
                                disabled={index === draft.links.length - 1}
                                aria-label={`${rowNumber}. linki aşağı taşı`}
                                title="Aşağı taşı"
                              >
                                <ArrowDown className="h-4 w-4" aria-hidden="true" />
                              </button>
                              <button
                                type="button"
                                className="btn-ghost btn-sm px-2 text-red-600 hover:bg-red-50"
                                onClick={() => removeLink(link.id)}
                                aria-label={`${rowNumber}. linki sil`}
                                title="Sil"
                              >
                                <Trash2 className="h-4 w-4" aria-hidden="true" />
                              </button>
                            </div>
                          </div>

                          <div className="grid gap-3 sm:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
                            <div>
                              <label className="label" htmlFor={`link-label-${link.id}`}>
                                Link adı
                              </label>
                              <input
                                id={`link-label-${link.id}`}
                                type="text"
                                className={`input ${
                                  showLabelProblem
                                    ? 'border-red-400 focus:border-red-500 focus:ring-red-100'
                                    : ''
                                }`}
                                value={link.label}
                                placeholder="Web sitemiz"
                                autoComplete="off"
                                aria-invalid={showLabelProblem}
                                aria-describedby={
                                  showLabelProblem ? `link-label-error-${link.id}` : undefined
                                }
                                onChange={(event) => updateLink(link.id, 'label', event.target.value)}
                                onBlur={() => touchLinkField(link.id, 'label')}
                              />
                              {showLabelProblem ? (
                                <p id={`link-label-error-${link.id}`} className="error-text">
                                  {labelProblem}
                                </p>
                              ) : null}
                            </div>

                            <div>
                              <label className="label" htmlFor={`link-url-${link.id}`}>
                                Link adresi
                              </label>
                              <input
                                id={`link-url-${link.id}`}
                                type="text"
                                inputMode="url"
                                className={`input ${
                                  showUrlProblem
                                    ? 'border-red-400 focus:border-red-500 focus:ring-red-100'
                                    : ''
                                }`}
                                value={link.url}
                                placeholder="https://www.ornek.com"
                                autoComplete="off"
                                autoCapitalize="none"
                                spellCheck={false}
                                aria-invalid={showUrlProblem}
                                aria-describedby={
                                  showUrlProblem ? `link-url-error-${link.id}` : undefined
                                }
                                onChange={(event) => updateLink(link.id, 'url', event.target.value)}
                                onBlur={(event) => {
                                  // "ornek.com" becomes "https://ornek.com"; see completeLinkUrl.
                                  const completed = completeLinkUrl(event.target.value)
                                  if (completed !== link.url) updateLink(link.id, 'url', completed)
                                  touchLinkField(link.id, 'url')
                                }}
                              />
                              {showUrlProblem ? (
                                <p id={`link-url-error-${link.id}`} className="error-text">
                                  {urlProblem}
                                </p>
                              ) : null}
                            </div>
                          </div>
                        </li>
                      )
                    })}
                  </ol>
                ) : null}

                <div className="mt-3 flex flex-wrap items-center gap-3">
                  <button
                    type="button"
                    className="btn-secondary btn-sm"
                    onClick={addLink}
                    disabled={draft.links.length >= MAX_LINKS}
                  >
                    <Plus className="h-4 w-4" aria-hidden="true" />
                    Link ekle
                  </button>

                  {/* At the limit this explains the disabled button. Past it —
                      only reachable with a stored list longer than the limit,
                      which no save accepts — it is the error the save would
                      stop on. */}
                  {draft.links.length >= MAX_LINKS ? (
                    <span
                      className={`text-xs ${
                        savableLinks(draft.links).length > MAX_LINKS
                          ? 'text-red-600'
                          : 'text-gray-500'
                      }`}
                    >
                      {LINK_MESSAGES.tooMany}
                    </span>
                  ) : null}
                </div>
              </div>

              {/* ------------------------------------------ where they appear */}
              {/* The option-card pattern of the theme and currency pickers: each
                  mode needs a line of explanation, which a segmented control
                  has no room for. */}
              <div className="border-t border-gray-100 pt-5">
                <span className="label" id="contact-display-label">
                  İletişim görünümü
                </span>

                <div
                  className="grid gap-3 sm:grid-cols-2"
                  role="group"
                  aria-labelledby="contact-display-label"
                >
                  {CONTACT_DISPLAY_MODES.map((mode) => {
                    const isSelected = draft.contact_display === mode.id
                    return (
                      <button
                        key={mode.id}
                        type="button"
                        onClick={() => update('contact_display', mode.id)}
                        aria-pressed={isSelected}
                        className={`rounded-xl border p-3 text-left hover:border-brand-400 ${
                          isSelected ? 'border-brand-600 ring-2 ring-brand-600' : 'border-gray-200'
                        }`}
                      >
                        <span className="block text-sm font-medium text-gray-900">
                          {mode.label}
                        </span>
                        <span className="mt-0.5 block text-xs leading-snug text-gray-500">
                          {CONTACT_DISPLAY_HELP[mode.id]}
                        </span>
                      </button>
                    )
                  })}
                </div>
              </div>
            </div>
          </section>

          {/* -------------------------------- 3. Currency and languages */}
          <section className="card p-5">
            <SectionHeading
              icon={Globe}
              title="Para Birimi ve Diller"
              description="Bu menüdeki tüm fiyatlar seçtiğiniz para birimiyle gösterilir."
            />

            <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
              {CURRENCY_LIST.map((currency) => {
                const isSelected = draft.currency === currency.code
                return (
                  <button
                    key={currency.code}
                    type="button"
                    onClick={() => update('currency', currency.code)}
                    aria-pressed={isSelected}
                    className={`rounded-xl border p-3 text-center hover:border-brand-400 ${
                      isSelected ? 'border-brand-600 ring-2 ring-brand-600' : 'border-gray-200'
                    }`}
                  >
                    <span className="block text-2xl leading-none text-gray-900" aria-hidden="true">
                      {currency.symbol}
                    </span>
                    <span className="mt-2 block text-sm font-medium text-gray-900">
                      {currency.code}
                    </span>
                    <span className="mt-0.5 block text-xs leading-snug text-gray-500">
                      {currency.label}
                    </span>
                  </button>
                )
              })}
            </div>

            <p className="mt-4 rounded-lg bg-gray-50 px-3 py-2.5 text-sm text-gray-600">
              Fiyatlar şöyle görünecek:{' '}
              <b className="text-gray-900">{formatPrice(145, draft.currency)}</b>
            </p>

            <div className="mt-5 border-t border-gray-100 pt-5">
              <span className="label">Menü dilleri</span>

              <div className="flex flex-wrap gap-2">
                {LANGUAGES.map((language) => {
                  const isSelected = draft.languages.includes(language.code)
                  return (
                    <button
                      key={language.code}
                      type="button"
                      onClick={() => toggleLanguage(language.code)}
                      aria-pressed={isSelected}
                      className={`inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-sm ${
                        isSelected
                          ? 'border-brand-600 bg-brand-50 font-medium text-brand-700'
                          : 'border-gray-300 bg-white text-gray-600 hover:bg-gray-50'
                      }`}
                    >
                      <span aria-hidden="true">{language.short}</span>
                      {language.label}
                    </button>
                  )
                })}
              </div>

              <div className="mt-5 max-w-xs">
                <label className="label" htmlFor="default-language">
                  Varsayılan dil
                </label>
                <select
                  id="default-language"
                  className="input"
                  value={draft.default_language}
                  onChange={(event) => update('default_language', event.target.value)}
                >
                  {LANGUAGES.filter((language) => draft.languages.includes(language.code)).map(
                    (language) => (
                      <option key={language.code} value={language.code}>
                        {language.short} {language.label}
                      </option>
                    ),
                  )}
                </select>
              </div>

              <p className="help-text mt-3">
                Birden fazla dil seçerseniz ürün eklerken her dil için ayrı ad ve açıklama
                girebilirsiniz. Müşteriler menüde dil değiştirebilir.
              </p>
            </div>
          </section>

          {/* --------------------------------------------- 4. Appearance */}
          <section className="card p-5">
            <SectionHeading
              icon={Palette}
              title="Görünüm"
              description="Bu menünün tasarım tarzı, yazı tipi, ana rengi ve arka planı."
            />

            {/* Theme */}
            <div>
              <span className="label">Tasarım tarzı</span>
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
                {THEMES.map((theme) => {
                  const isSelected = draft.theme === theme.id
                  return (
                    <button
                      key={theme.id}
                      type="button"
                      onClick={() => update('theme', theme.id)}
                      aria-pressed={isSelected}
                      className={`rounded-xl border p-2.5 text-left hover:border-brand-400 ${
                        isSelected ? 'border-brand-600 ring-2 ring-brand-600' : 'border-gray-200'
                      }`}
                    >
                      {/* colour strip */}
                      <div
                        className="flex h-14 gap-1 overflow-hidden rounded-lg border border-gray-200 p-1"
                        style={{ backgroundColor: theme.colors.background }}
                      >
                        <div
                          className="flex-1 rounded"
                          style={{ backgroundColor: theme.colors.background }}
                        />
                        <div
                          className="flex-1 rounded"
                          style={{ backgroundColor: theme.colors.surface }}
                        />
                        <div
                          className="flex-1 rounded"
                          style={{ backgroundColor: theme.colors.primary }}
                        />
                      </div>

                      <p className="mt-2 text-sm font-medium text-gray-900">{theme.label}</p>
                      <p className="mt-0.5 text-xs leading-snug text-gray-500">
                        {theme.description}
                      </p>
                    </button>
                  )
                })}
              </div>
            </div>

            {/* Font */}
            <div className="mt-5 border-t border-gray-100 pt-5">
              <span className="label">Yazı tipi</span>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                {FONTS.map((font) => {
                  const isSelected = draft.font_family === font.id
                  return (
                    <button
                      key={font.id}
                      type="button"
                      onClick={() => update('font_family', font.id)}
                      aria-pressed={isSelected}
                      className={`rounded-xl border p-3 text-left hover:border-brand-400 ${
                        isSelected ? 'border-brand-600 ring-2 ring-brand-600' : 'border-gray-200'
                      }`}
                    >
                      <p className="text-xs font-medium uppercase tracking-wide text-gray-500">
                        {font.label}
                      </p>
                      <p
                        className="mt-1.5 truncate text-lg text-gray-900"
                        style={{ fontFamily: font.stack }}
                      >
                        Türk Kahvesi 145,00 ₺
                      </p>
                    </button>
                  )
                })}
              </div>
              <p className="help-text">Menüdeki tüm başlık ve metinler bu yazı tipiyle görünür.</p>
            </div>

            {/* Accent colour */}
            <div className="mt-5 border-t border-gray-100 pt-5">
              <span className="label">Ana renk</span>

              <div className="flex flex-wrap items-start gap-3">
                <input
                  type="color"
                  value={primaryColor}
                  onChange={(event) => update('primary_color', event.target.value)}
                  aria-label="Ana renk seçici"
                  className="h-11 w-14 shrink-0 cursor-pointer rounded-lg border border-gray-300 bg-white p-1"
                />

                <div className="w-40">
                  <input
                    type="text"
                    value={draft.primary_color || ''}
                    onChange={(event) => update('primary_color', event.target.value.trim())}
                    placeholder={DEFAULT_PRIMARY_COLOR}
                    maxLength={7}
                    aria-label="Ana renk kodu"
                    className={`input font-mono uppercase ${
                      primaryColorInvalid
                        ? 'border-red-400 focus:border-red-500 focus:ring-red-100'
                        : ''
                    }`}
                  />
                  {primaryColorInvalid ? (
                    <p className="error-text">#RRGGBB biçiminde yazın.</p>
                  ) : null}
                </div>
              </div>

              <div className="mt-3 flex flex-wrap gap-2">
                {PRESET_COLORS.map((color) => {
                  const isSelected = String(draft.primary_color || '').toLowerCase() === color
                  return (
                    <button
                      key={color}
                      type="button"
                      onClick={() => update('primary_color', color)}
                      title={color}
                      aria-label={`Ana rengi ${color} yap`}
                      className={`h-9 w-9 rounded-lg border ${
                        isSelected ? 'border-transparent ring-2 ring-brand-600' : 'border-gray-200'
                      }`}
                      style={{ backgroundColor: color }}
                    />
                  )
                })}
              </div>

              <p className="help-text">
                Butonlar, fiyatlar ve seçili kategori bu renkle vurgulanır.
              </p>
            </div>

            {/* Text colour. Same control as the accent colour above: picker,
                hex field with its inline error, and the preset row. */}
            <div className="mt-5 border-t border-gray-100 pt-5">
              <span className="label">Metin Rengi</span>

              <div className="flex flex-wrap items-start gap-3">
                <input
                  type="color"
                  value={textColor}
                  onChange={(event) => update('text_color', event.target.value)}
                  aria-label="Metin rengi seçici"
                  className="h-11 w-14 shrink-0 cursor-pointer rounded-lg border border-gray-300 bg-white p-1"
                />

                <div className="w-40">
                  <input
                    type="text"
                    value={draft.text_color || ''}
                    onChange={(event) => update('text_color', event.target.value.trim())}
                    placeholder={DEFAULT_TEXT_COLOR}
                    maxLength={7}
                    aria-label="Metin rengi kodu"
                    className={`input font-mono uppercase ${
                      textColorInvalid
                        ? 'border-red-400 focus:border-red-500 focus:ring-red-100'
                        : ''
                    }`}
                  />
                  {textColorInvalid ? (
                    <p className="error-text">#RRGGBB biçiminde yazın.</p>
                  ) : null}
                </div>
              </div>

              <div className="mt-3 flex flex-wrap gap-2">
                {PRESET_TEXT_COLORS.map((color) => {
                  const isSelected = String(draft.text_color || '').toLowerCase() === color
                  return (
                    <button
                      key={color}
                      type="button"
                      onClick={() => update('text_color', color)}
                      title={color}
                      aria-label={`Metin rengini ${color} yap`}
                      className={`h-9 w-9 rounded-lg border ${
                        isSelected ? 'border-transparent ring-2 ring-brand-600' : 'border-gray-200'
                      }`}
                      style={{ backgroundColor: color }}
                    />
                  )
                })}
              </div>

              <p className="help-text">
                Kategori başlıkları, ürün adları ve açıklamalar bu renkle yazılır.
              </p>
            </div>

            {/* Background */}
            <div className="mt-5 border-t border-gray-100 pt-5">
              <div className="mb-3 flex items-center gap-2">
                <ImageIcon className="h-4 w-4 text-gray-400" aria-hidden="true" />
                <span className="text-sm font-medium text-gray-900">Menü arka planı</span>
              </div>

              <Segmented
                label="Arka plan türü"
                options={[
                  { value: 'color', label: 'Renk' },
                  { value: 'image', label: 'Görsel' },
                ]}
                value={draft.background_type}
                onChange={(value) => update('background_type', value)}
              />

              {usesImageBackground ? (
                <div className="mt-5 space-y-5">
                  <ImageUploader
                    value={draft.background_image_url}
                    onChange={(url) => update('background_image_url', url)}
                    label="Arka plan görseli"
                    hint="Geniş, sakin görseller en iyi sonucu verir · en fazla 5 MB"
                  />

                  <div>
                    <div className="flex items-center justify-between gap-3">
                      <label className="label mb-0" htmlFor="background-overlay">
                        Karartma
                      </label>
                      <span className="text-sm font-medium text-gray-900">
                        {formatOpacity(draft.background_overlay_opacity)}
                      </span>
                    </div>
                    <input
                      id="background-overlay"
                      type="range"
                      min={0}
                      max={1}
                      step={0.05}
                      value={clampOpacity(draft.background_overlay_opacity)}
                      onChange={(event) =>
                        update('background_overlay_opacity', Number(event.target.value))
                      }
                      className="mt-2 w-full accent-brand-600"
                    />
                    <p className="help-text">
                      {readabilityHint(draft.background_overlay_opacity)}
                    </p>
                  </div>
                </div>
              ) : (
                <div className="mt-5">
                  <span className="label">Arka plan rengi</span>
                  <div className="flex flex-wrap items-start gap-3">
                    <input
                      type="color"
                      value={
                        isValidColor(draft.background_color) ? draft.background_color : '#ffffff'
                      }
                      onChange={(event) => update('background_color', event.target.value)}
                      aria-label="Arka plan rengi seçici"
                      className="h-11 w-14 shrink-0 cursor-pointer rounded-lg border border-gray-300 bg-white p-1"
                    />

                    <div className="w-40">
                      <input
                        type="text"
                        value={draft.background_color}
                        onChange={(event) => update('background_color', event.target.value.trim())}
                        placeholder="#ffffff"
                        maxLength={7}
                        aria-label="Arka plan renk kodu"
                        className={`input font-mono uppercase ${
                          backgroundColorInvalid
                            ? 'border-red-400 focus:border-red-500 focus:ring-red-100'
                            : ''
                        }`}
                      />
                      {backgroundColorInvalid ? (
                        <p className="error-text">#RRGGBB biçiminde yazın.</p>
                      ) : null}
                    </div>

                    {draft.background_color ? (
                      <button
                        type="button"
                        className="btn-ghost btn-sm"
                        onClick={() => update('background_color', '')}
                      >
                        Tema rengini kullan
                      </button>
                    ) : null}
                  </div>

                  <div className="mt-3 flex flex-wrap gap-2">
                    {PRESET_BACKGROUND_COLORS.map((color) => {
                      const isSelected =
                        String(draft.background_color || '').toLowerCase() === color
                      return (
                        <button
                          key={color}
                          type="button"
                          onClick={() => update('background_color', color)}
                          title={color}
                          aria-label={`Arka planı ${color} yap`}
                          className={`h-9 w-9 rounded-lg border ${
                            isSelected
                              ? 'border-transparent ring-2 ring-brand-600'
                              : 'border-gray-200'
                          }`}
                          style={{ backgroundColor: color }}
                        />
                      )
                    })}
                  </div>

                  <p className="help-text">
                    Boş bırakırsanız seçtiğiniz tasarım tarzının kendi zemin rengi kullanılır.
                  </p>
                </div>
              )}

              {/* Readability preview — the same layers the customer menu draws. */}
              <div className="mt-5">
                <span className="label">Okunabilirlik önizlemesi</span>
                <div
                  className="relative h-32 overflow-hidden rounded-xl border border-gray-200"
                  style={containerStyle}
                >
                  {overlayStyle ? (
                    <div className="absolute inset-0 z-0" style={overlayStyle} aria-hidden="true" />
                  ) : null}

                  <div className="relative z-10 flex h-full flex-col items-center justify-center gap-1 px-4 text-center">
                    <p className="text-base font-semibold text-white drop-shadow">Türk Kahvesi</p>
                    <p className="text-sm text-white/90 drop-shadow">
                      {formatPrice(145, draft.currency)}
                    </p>
                  </div>
                </div>
                <p className="help-text">
                  Menü metinleri bu zeminin üzerinde görünür. Yazılar okunmuyorsa rengi açın ya da
                  karartmayı artırın.
                </p>
              </div>
            </div>
          </section>

          {/* ----------------------------------------- 5. Splash screen */}
          <section className="card p-5">
            <SectionHeading
              icon={Sparkles}
              title="Karşılama Ekranı"
              description="Menü açıldığında logonuzla birlikte kısa bir karşılama ekranı gösterilir. Müşterilere oturum başına yalnızca bir kez görünür."
            />

            <Switch
              checked={splashEnabled}
              onChange={(value) => update('splash_enabled', value)}
              label="Karşılama ekranı açık"
            />

            <div className={`mt-5 grid gap-5 lg:grid-cols-2 ${splashEnabled ? '' : 'opacity-60'}`}>
              {/* ------------------------------------------- logo and text */}
              <div className="space-y-5">
                <Segmented
                  label="Görünüm"
                  options={SPLASH_DISPLAY_MODES.map((mode) => ({
                    value: mode.id,
                    label: mode.label,
                  }))}
                  value={draft.splash_display}
                  onChange={(value) => update('splash_display', value)}
                  disabled={!splashEnabled}
                  hint="Sadece yazı seçilirse logo gösterilmez, sadece logo seçilirse başlık ve alt başlık gizlenir."
                />

                <div>
                  <ImageUploader
                    value={draft.splash_logo_url}
                    onChange={(url) => update('splash_logo_url', url)}
                    label="Karşılama logosu"
                    hint="Yatay logolar desteklenir · SVG, PNG veya JPG"
                  />
                  <p className="help-text">Boş bırakırsanız menü logonuz kullanılır.</p>
                </div>

                <div>
                  <label htmlFor="splash-headline" className="label">
                    Karşılama başlığı (opsiyonel)
                  </label>
                  <input
                    id="splash-headline"
                    type="text"
                    className="input"
                    value={draft.splash_headline}
                    onChange={(event) => update('splash_headline', event.target.value.slice(0, 60))}
                    placeholder={draft.name || 'Kahve Durağı'}
                    maxLength={60}
                    disabled={!splashEnabled}
                  />
                  <p className="help-text">
                    Boş bırakırsanız gösterilmez · {draft.splash_headline.length} / 60 karakter
                  </p>
                </div>

                <div>
                  <label htmlFor="splash-text" className="label">
                    Alt başlık (opsiyonel)
                  </label>
                  <input
                    id="splash-text"
                    type="text"
                    className="input"
                    value={draft.splash_text}
                    onChange={(event) => update('splash_text', event.target.value.slice(0, 200))}
                    placeholder="Hoş geldiniz"
                    maxLength={200}
                    disabled={!splashEnabled}
                  />
                  <p className="help-text">{draft.splash_text.length} / 200 karakter</p>
                </div>
              </div>

              {/* --------------------------------------- colour and timing */}
              <div className="space-y-5">
                <div>
                  <span className="label">Arka plan rengi</span>
                  <div className="flex flex-wrap items-start gap-3">
                    <input
                      type="color"
                      value={splashBackground}
                      onChange={(event) => update('splash_bg_color', event.target.value)}
                      disabled={!splashEnabled}
                      aria-label="Karşılama arka plan rengi seçici"
                      className="h-11 w-14 shrink-0 cursor-pointer rounded-lg border border-gray-300 bg-white p-1 disabled:cursor-not-allowed"
                    />
                    <div className="w-40">
                      <input
                        type="text"
                        value={draft.splash_bg_color || ''}
                        onChange={(event) => update('splash_bg_color', event.target.value.trim())}
                        placeholder={DEFAULT_SPLASH_BG_COLOR}
                        maxLength={7}
                        disabled={!splashEnabled}
                        aria-label="Karşılama arka plan renk kodu"
                        className={`input font-mono uppercase ${
                          splashBackgroundInvalid
                            ? 'border-red-400 focus:border-red-500 focus:ring-red-100'
                            : ''
                        }`}
                      />
                      {splashBackgroundInvalid ? (
                        <p className="error-text">#RRGGBB biçiminde yazın.</p>
                      ) : null}
                    </div>
                  </div>

                  <div className="mt-3 flex flex-wrap gap-2">
                    {PRESET_SPLASH_COLORS.map((color) => {
                      const isSelected =
                        String(draft.splash_bg_color || '').toLowerCase() === color
                      return (
                        <button
                          key={color}
                          type="button"
                          onClick={() => update('splash_bg_color', color)}
                          disabled={!splashEnabled}
                          title={color}
                          aria-label={`Arka planı ${color} yap`}
                          className={`h-9 w-9 rounded-lg border disabled:cursor-not-allowed ${
                            isSelected
                              ? 'border-transparent ring-2 ring-brand-600'
                              : 'border-gray-200'
                          }`}
                          style={{ backgroundColor: color }}
                        />
                      )
                    })}
                  </div>
                </div>

                {/* Hold duration */}
                <div>
                  <div className="flex items-center justify-between gap-3">
                    <label htmlFor="splash-duration" className="label mb-0">
                      Gösterim süresi
                    </label>
                    <span className="text-sm font-medium text-gray-900">
                      {formatDuration(draft.splash_duration)}
                    </span>
                  </div>
                  <input
                    id="splash-duration"
                    type="range"
                    min={300}
                    max={5000}
                    step={100}
                    value={Number(draft.splash_duration) || 1200}
                    onChange={(event) => update('splash_duration', Number(event.target.value))}
                    disabled={!splashEnabled}
                    className="mt-2 w-full accent-brand-600 disabled:cursor-not-allowed"
                  />
                  <div className="mt-1 flex justify-between text-[11px] text-gray-400">
                    <span>0,3 sn</span>
                    <span>5,0 sn</span>
                  </div>
                </div>

                {/* Entrance animation. It plays before the hold and therefore
                    before everything below it, so it comes first in the form
                    too — the controls read in the order the customer sees. */}
                <Segmented
                  label="Giriş animasyonu"
                  options={SPLASH_ENTRANCES.map((entrance) => ({
                    value: entrance.id,
                    label: entrance.label,
                  }))}
                  value={draft.splash_entrance}
                  onChange={(value) => update('splash_entrance', value)}
                  disabled={!splashEnabled}
                  hint="Logo ve yazı yumuşak bir geçişle mi belirsin, yoksa doğrudan mı görünsün?"
                />

                {/* Exit animation */}
                <div>
                  <label htmlFor="splash-exit-animation" className="label">
                    Çıkış animasyonu
                  </label>
                  <select
                    id="splash-exit-animation"
                    className="input"
                    value={draft.splash_exit_animation}
                    onChange={(event) => update('splash_exit_animation', event.target.value)}
                    disabled={!splashEnabled}
                  >
                    {SPLASH_EXIT_ANIMATIONS.map((animation) => (
                      <option key={animation.id} value={animation.id}>
                        {animation.label}
                      </option>
                    ))}
                  </select>
                  <p className="help-text">Karşılama ekranı menüye bu şekilde geçer.</p>
                </div>

                {/* Slide style — only the four slide-* animations can fade or
                    stay solid while they move. */}
                {showSlideStyle ? (
                  <Segmented
                    label="Kayma biçimi"
                    options={SLIDE_FADE_MODES}
                    value={Boolean(draft.splash_slide_fade)}
                    onChange={(value) => update('splash_slide_fade', value)}
                    disabled={!splashEnabled}
                    hint="Perde etkisi için ikinci seçeneği kullanın: karşılama ekranı saydamlaşmadan, tam opak kayar."
                  />
                ) : null}

                {/* Exit easing */}
                <div>
                  <label htmlFor="splash-exit-easing" className="label">
                    Animasyon eğrisi
                  </label>
                  <select
                    id="splash-exit-easing"
                    className="input"
                    value={draft.splash_exit_easing}
                    onChange={(event) => update('splash_exit_easing', event.target.value)}
                    disabled={!splashEnabled}
                  >
                    {SPLASH_EASINGS.map((easing) => (
                      <option key={easing.id} value={easing.id}>
                        {easing.label}
                      </option>
                    ))}
                  </select>
                  <p className="help-text">Çıkış animasyonunun hızlanma ve yavaşlama şekli.</p>
                </div>

                {/* Exit duration */}
                <div>
                  <div className="flex items-center justify-between gap-3">
                    <label htmlFor="splash-exit-duration" className="label mb-0">
                      Çıkış süresi
                    </label>
                    <span className="text-sm font-medium text-gray-900">
                      {formatDuration(draft.splash_exit_duration)}
                    </span>
                  </div>
                  <input
                    id="splash-exit-duration"
                    type="range"
                    min={100}
                    max={2000}
                    step={50}
                    value={Number(draft.splash_exit_duration) || DEFAULT_SPLASH_EXIT.duration}
                    onChange={(event) => update('splash_exit_duration', Number(event.target.value))}
                    disabled={!splashEnabled}
                    className="mt-2 w-full accent-brand-600 disabled:cursor-not-allowed"
                  />
                  <div className="mt-1 flex justify-between text-[11px] text-gray-400">
                    <span>0,1 sn</span>
                    <span>2,0 sn</span>
                  </div>
                </div>
              </div>
            </div>

            <p className="help-text mt-5">
              Sağdaki önizlemede{' '}
              <b className="font-medium text-gray-700">Karşılama ekranını oynat</b> düğmesine
              basarak nasıl göründüğünü deneyebilirsiniz.
            </p>
          </section>

          {/* --------------------------------- 6. Footer / legal notices */}
          <section className="card p-5">
            <SectionHeading icon={Percent} title="Menü Altı Bilgiler (Yasal İbareler)" />

            <div className="space-y-5">
              <div>
                <Switch
                  checked={draft.show_price_date}
                  onChange={(value) => update('show_price_date', value)}
                  label="Fiyat geçerlilik tarihini göster"
                />

                <p className="mt-3 rounded-lg bg-gray-50 px-3 py-2.5 text-sm text-gray-600">
                  Fiyatlarımız <b className="text-gray-900">{priceDate}</b> tarihinden itibaren
                  geçerlidir.
                </p>

                <p className="help-text flex items-start gap-1.5">
                  <Info className="mt-0.5 h-3.5 w-3.5 shrink-0 text-gray-400" aria-hidden="true" />
                  <span>
                    Bu tarih, bu menüde bir fiyat değiştiğinde (tek ürün düzenlemesi ya da
                    toplu güncelleme) otomatik olarak yenilenir.
                  </span>
                </p>
              </div>

              <div className="border-t border-gray-100 pt-5">
                <Switch
                  checked={draft.show_vat_note}
                  onChange={(value) => update('show_vat_note', value)}
                  label="KDV ibaresini göster"
                />

                <div className="mt-3">
                  <label className="label" htmlFor="vat-note">
                    KDV ibaresi metni
                  </label>
                  <input
                    id="vat-note"
                    type="text"
                    className="input"
                    value={draft.vat_note_text}
                    maxLength={200}
                    placeholder={DEFAULT_VAT_NOTE}
                    disabled={!draft.show_vat_note}
                    onChange={(event) => update('vat_note_text', event.target.value)}
                  />
                  <p className="help-text">
                    Boş bırakırsanız menüde “{DEFAULT_VAT_NOTE}” yazar ·{' '}
                    {draft.vat_note_text.length} / 200 karakter
                  </p>
                </div>
              </div>

              <div className="border-t border-gray-100 pt-5">
                <Switch
                  checked={draft.show_yerli_uretim}
                  onChange={(value) => update('show_yerli_uretim', value)}
                  label="Yerli Üretim rozetini göster"
                  description="Menünün en altında, Karecik imzasının hemen üzerinde görünür."
                />

                {draft.show_yerli_uretim ? (
                  <div className="mt-4">
                    <ImageUploader
                      value={draft.yerli_uretim_logo_url}
                      onChange={(url) => update('yerli_uretim_logo_url', url)}
                      label="Yerli Üretim logosu (opsiyonel)"
                      hint="Belgeli logonuzu yükleyin. Boş bırakırsanız sade bir metin rozeti gösterilir."
                    />
                  </div>
                ) : null}
              </div>
            </div>
          </section>

          {/* ------------------------------------------- 7. Menu status */}
          <section className="card p-5">
            <SectionHeading icon={Eye} title="Menü Durumu" />

            <Switch
              checked={draft.is_active}
              onChange={(value) => update('is_active', value)}
              label="Menü yayında"
              description="Kapattığınızda menüyü panelden düzenlemeye devam edebilirsiniz, müşteriler göremez."
            />

            {!draft.is_active ? (
              <div className="mt-4 flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2.5">
                <AlertCircle
                  className="mt-0.5 h-4 w-4 shrink-0 text-amber-600"
                  aria-hidden="true"
                />
                <p className="text-sm text-amber-800">
                  Bu menü kapalı. Müşteriler QR kodu okuttuğunda menüyü göremez.
                </p>
              </div>
            ) : null}
          </section>
        </div>

        {/* ----------------------------------------------------- right column */}
        <div className="min-w-0">
          <div className="xl:sticky xl:top-6">
            {/* The SAVED slug is previewed, not the draft one: the server can
                only resolve a slug it already stores, and a fetch per keystroke
                while the address is being typed would be pointless traffic. */}
            <LivePreview business={draft} menuSlug={activeMenu.slug} showSplashControl />
          </div>
        </div>
      </div>

      {/* Sticky action bar. It only exists while there is something to save, so
          the page does not shift around while the form is untouched. The
          negative bottom margin cancels the wrapper's pb-10. */}
      {hasChanges ? (
        <div className="sticky bottom-0 z-10 -mb-10 mt-6 border-t border-gray-200 bg-white/95 px-4 py-3 backdrop-blur">
          <div className="flex items-center justify-between gap-3">
            <p className="min-w-0 text-sm text-gray-500">Kaydedilmemiş değişiklikleriniz var.</p>

            <button type="button" className="btn-primary shrink-0" onClick={save} disabled={saving}>
              <Save className="h-4 w-4" aria-hidden="true" />
              {saving ? 'Kaydediliyor...' : 'Kaydet'}
            </button>
          </div>
        </div>
      ) : null}
    </div>
  )
}
