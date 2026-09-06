import { useEffect, useMemo, useState } from 'react'
import {
  AlertCircle,
  Eye,
  EyeOff,
  Globe,
  Image as ImageIcon,
  Info,
  Instagram,
  MapPin,
  Palette,
  Percent,
  Phone,
  Save,
  Sparkles,
  Store,
  Wifi,
} from 'lucide-react'

import { useAuth } from '../../lib/auth.jsx'
import { useActiveMenu } from '../../lib/menuContext.jsx'
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
  'slug',
  'description',
  'logo_url',
  'cover_url',
  'header_display',
  'logo_fade_in',
  /* 2. contact */
  'phone',
  'address',
  'instagram',
  'wifi_ssid',
  'wifi_password',
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
const TRIMMED_FIELDS = ['description', 'vat_note_text', 'splash_headline', 'splash_text']

const DEFAULT_VAT_NOTE = 'Fiyatlarımıza KDV dahildir.'
/* The domain every public address sits under; lib/subdomain.js owns the value */
const MENU_SUFFIX = `.${APP_DOMAIN}`
const DEFAULT_OVERLAY_OPACITY = 0.4
const DEFAULT_PRIMARY_COLOR = '#1d4ed8'
const DEFAULT_TEXT_COLOR = '#111827'
const DEFAULT_SPLASH_BG_COLOR = '#0f172a'

/**
 * The two slide styles, bound to the boolean `splash_slide_fade`.
 * Mirrors utils.SlideFadeModes in backend/internal/utils/appearance.go — only
 * the labels live here, the stored value is the boolean itself.
 */
const SLIDE_FADE_MODES = [
  { value: true, label: 'Kayarken soluklaşsın' },
  { value: false, label: 'Tam opak kaysın (perde gibi)' },
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

/** Builds an editable draft from a menu object. */
function buildDraft(menu) {
  const languages =
    Array.isArray(menu.languages) && menu.languages.length > 0 ? [...menu.languages] : ['tr']

  return {
    /* identity */
    name: menu.name || '',
    slug: menu.slug || '',
    description: menu.description || '',
    logo_url: menu.logo_url || null,
    cover_url: menu.cover_url || null,
    header_display: headerMode(menu.header_display),
    logo_fade_in: Boolean(menu.logo_fade_in),
    /* contact */
    phone: menu.phone || '',
    address: menu.address || '',
    instagram: String(menu.instagram || '').replace(/^@+/, ''),
    wifi_ssid: menu.wifi_ssid || '',
    wifi_password: menu.wifi_password || '',
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
    vat_note_text: menu.vat_note_text || DEFAULT_VAT_NOTE,
    // The column defaults to true, so a payload without the field means "on".
    show_yerli_uretim: menu.show_yerli_uretim !== false,
    yerli_uretim_logo_url: menu.yerli_uretim_logo_url || null,
    /* status */
    is_active: menu.is_active !== false,
  }
}

/** Compares two field values (array order is irrelevant). */
function isEqual(a, b) {
  if (Array.isArray(a) || Array.isArray(b)) {
    const first = (Array.isArray(a) ? a : []).slice().sort()
    const second = (Array.isArray(b) ? b : []).slice().sort()
    return first.length === second.length && first.every((value, i) => value === second[i])
  }
  if (typeof a === 'boolean' || typeof b === 'boolean') return Boolean(a) === Boolean(b)
  return String(a ?? '').trim() === String(b ?? '').trim()
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

  // The stored state on the server (used for comparison).
  const stored = useMemo(() => (activeMenu ? buildDraft(activeMenu) : null), [activeMenu])

  // Refresh the draft when the active menu changes (switch, first load or save).
  useEffect(() => {
    setDraft(stored ? { ...stored } : null)
    setSlugError('')
  }, [stored])

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

  const changedFields = MENU_FIELDS.filter((field) => !isEqual(draft[field], stored[field]))
  const hasChanges = changedFields.length > 0

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

                {/* How that logo ENTERS. Off means no animation at all, not a
                    zero-duration one, so a menu that never asked for motion
                    keeps rendering the logo at full opacity right away. */}
                <div className="mt-5">
                  <Switch
                    checked={Boolean(draft.logo_fade_in)}
                    onChange={(value) => update('logo_fade_in', value)}
                    label="Logo Giriş Animasyonu (Fade-In)"
                    description="Menü açıldığında logo yumuşak bir geçişle belirir."
                  />
                </div>
              </div>
            </div>
          </section>

          {/* ------------------------------------------------ 2. Contact */}
          <section className="card p-5">
            <SectionHeading
              icon={Phone}
              title="İletişim"
              description="Menünün altında müşterilerinize gösterilir."
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
                <p className="help-text">Menünün altında Instagram bağlantısı olarak görünür.</p>
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
                    Bu tarih, bu menüde toplu fiyat güncellemesi yaptığınızda otomatik olarak
                    yenilenir.
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
                  <p className="help-text">{draft.vat_note_text.length} / 200 karakter</p>
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
