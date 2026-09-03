import { useEffect, useMemo, useState } from 'react'
import {
  AlertCircle,
  Eye,
  EyeOff,
  Globe,
  Image as ImageIcon,
  Info,
  Instagram,
  Languages,
  MapPin,
  Percent,
  Phone,
  Save,
  Store,
  Wifi,
} from 'lucide-react'

import { useAuth } from '../../lib/auth.jsx'
import { CURRENCY_LIST, formatDate, formatPrice } from '../../lib/format'
import { LANGUAGES } from '../../locales/index.js'
import { HEADER_DISPLAY_MODES } from '../../themes/splash'
import { backgroundStyles } from '../../themes/themes'
import Loading from '../../components/ui/Loading.jsx'
import ImageUploader from '../../components/ui/ImageUploader.jsx'
import { useToast } from '../../components/ui/Toast.jsx'

/* Business fields owned by this page */
const SETTING_FIELDS = [
  'logo_url',
  'name',
  'slug',
  'phone',
  'address',
  'instagram',
  'wifi_ssid',
  'wifi_password',
  'header_display',
  'currency',
  'languages',
  'default_language',
  'background_type',
  'background_color',
  'background_image_url',
  'background_overlay_opacity',
  'show_price_date',
  'show_vat_note',
  'vat_note_text',
  'is_active',
]

/* Text fields that may be cleared (sent as null when empty) */
const NULLABLE_FIELDS = [
  'logo_url',
  'phone',
  'address',
  'instagram',
  'wifi_ssid',
  'wifi_password',
  'background_color',
  'background_image_url',
]

const DEFAULT_VAT_NOTE = 'Fiyatlarımıza KDV dahildir.'
const MENU_SUFFIX = '.karecik.com'
const DEFAULT_OVERLAY_OPACITY = 0.4

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

/* Menu addresses reserved by the system (mirrors backend utils/slug.go) */
const RESERVED_SLUGS = [
  'www',
  'api',
  'admin',
  'app',
  'panel',
  'mail',
  'ftp',
  'blog',
  'help',
  'destek',
  'karecik',
  'static',
  'cdn',
  'assets',
  'dashboard',
  'login',
  'register',
  'demo',
]

/* ASCII equivalents of Turkish letters */
const TURKISH_LETTERS = {
  ç: 'c',
  Ç: 'c',
  ğ: 'g',
  Ğ: 'g',
  ı: 'i',
  I: 'i',
  İ: 'i',
  ö: 'o',
  Ö: 'o',
  ş: 's',
  Ş: 's',
  ü: 'u',
  Ü: 'u',
  â: 'a',
  Â: 'a',
  î: 'i',
  Î: 'i',
  û: 'u',
  Û: 'u',
}

/**
 * Cleans the menu address while the user types.
 * A trailing dash is kept so typing can continue; it is dropped on save.
 */
function cleanSlug(input) {
  return String(input ?? '')
    .replace(/[çÇğĞıIİöÖşŞüÜâÂîÎûÛ]/g, (letter) => TURKISH_LETTERS[letter] || letter)
    .replace(/&/g, '-ve-')
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, '-')
    .replace(/-{2,}/g, '-')
    .replace(/^-+/, '')
    .slice(0, 60)
}

/** Final address to store: leading and trailing dashes removed. */
function finalizeSlug(input) {
  return cleanSlug(input).replace(/-+$/, '')
}

/** Falls back to 'both' for an unknown or missing customer-menu header mode. */
function headerMode(value) {
  return HEADER_DISPLAY_MODES.some((mode) => mode.id === value) ? value : 'both'
}

/** Validates the #RRGGBB format. */
function isValidColor(value) {
  return /^#[0-9a-fA-F]{6}$/.test(String(value || ''))
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

/** Builds an editable draft from a business object. */
function buildDraft(business) {
  const languages =
    Array.isArray(business.languages) && business.languages.length > 0
      ? [...business.languages]
      : ['tr']

  return {
    logo_url: business.logo_url || null,
    name: business.name || '',
    slug: business.slug || '',
    phone: business.phone || '',
    address: business.address || '',
    instagram: String(business.instagram || '').replace(/^@+/, ''),
    wifi_ssid: business.wifi_ssid || '',
    wifi_password: business.wifi_password || '',
    header_display: headerMode(business.header_display),
    currency: business.currency || 'TRY',
    languages,
    default_language: languages.includes(business.default_language)
      ? business.default_language
      : languages[0],
    background_type: business.background_type === 'image' ? 'image' : 'color',
    background_color: business.background_color || '',
    background_image_url: business.background_image_url || null,
    background_overlay_opacity: clampOpacity(business.background_overlay_opacity),
    show_price_date: Boolean(business.show_price_date),
    show_vat_note: Boolean(business.show_vat_note),
    vat_note_text: business.vat_note_text || DEFAULT_VAT_NOTE,
    is_active: Boolean(business.is_active),
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
function Switch({ checked, onChange, label, description }) {
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
        onClick={() => onChange(!checked)}
        className={`relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-brand-100 ${
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

/* -------------------------------------------------------------------- page */

export default function Settings() {
  const { business, saveBusiness } = useAuth()
  const toast = useToast()

  const [draft, setDraft] = useState(null)
  const [saving, setSaving] = useState(false)
  const [slugError, setSlugError] = useState('')
  const [passwordVisible, setPasswordVisible] = useState(false)

  // The stored state on the server (used for comparison).
  const stored = useMemo(() => (business ? buildDraft(business) : null), [business])

  // Refresh the draft when the business changes (first load or after saving).
  useEffect(() => {
    setDraft(stored ? { ...stored } : null)
    setSlugError('')
  }, [stored])

  if (!business || !stored || !draft) {
    return <Loading text="Ayarlar yükleniyor..." />
  }

  /* --------------------------------------------------------------- helpers */

  const update = (field, value) => {
    setDraft((previous) => ({ ...previous, [field]: value }))
  }

  const changedFields = SETTING_FIELDS.filter((field) => !isEqual(draft[field], stored[field]))
  const hasChanges = changedFields.length > 0

  const finalSlug = finalizeSlug(draft.slug)
  const displaySlug = finalSlug || 'menu-adresiniz'
  const priceDate = formatDate(business.price_updated_at) || formatDate(new Date())

  const usesImageBackground = draft.background_type === 'image'
  const backgroundColorInvalid =
    Boolean(draft.background_color) && !isValidColor(draft.background_color)

  // The very same helper the customer menu uses, so the tile below is truthful.
  const { containerStyle, overlayStyle } = backgroundStyles(business.theme, {
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

  /** Collects only the changed fields and saves them. */
  const save = async () => {
    if (!hasChanges || saving) return

    setSlugError('')

    const name = draft.name.trim()
    if (name.length < 2 || name.length > 100) {
      toast.error('İşletme adı 2 ile 100 karakter arasında olmalıdır.')
      return
    }
    if (finalSlug.length < 2) {
      setSlugError(
        'Menü adresi en az 2 karakter olmalı ve yalnızca harf, rakam ve tire içerebilir.',
      )
      return
    }
    if (RESERVED_SLUGS.includes(finalSlug)) {
      setSlugError('Bu menü adresi sistem tarafından ayrılmıştır. Lütfen başka bir adres deneyin.')
      return
    }
    if (draft.languages.length === 0) {
      toast.error('En az bir menü dili seçmelisiniz.')
      return
    }
    if (draft.vat_note_text.trim().length > 200) {
      toast.error('KDV ibaresi en fazla 200 karakter olabilir.')
      return
    }
    if (backgroundColorInvalid) {
      toast.error('Arka plan rengi #RRGGBB biçiminde olmalıdır (örn. #ffffff).')
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
      if (field === 'vat_note_text') {
        body.vat_note_text = draft.vat_note_text.trim()
        return
      }
      if (field === 'background_overlay_opacity') {
        body.background_overlay_opacity = clampOpacity(draft.background_overlay_opacity)
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
      await saveBusiness(body)
      toast.success('Ayarlar kaydedildi.')
    } catch (error) {
      if (error.status === 409) {
        setSlugError(error.message)
      } else {
        toast.error(error.message)
      }
    } finally {
      setSaving(false)
    }
  }

  /* ---------------------------------------------------------------- render */

  return (
    <div className="mx-auto w-full max-w-3xl pb-10">
      {/* Heading. Deliberately a normal block: the old sticky bar broke out of
          the centred column with negative margins and drifted out of alignment
          with the cards. Saving lives in the sticky bar at the bottom instead. */}
      <div className="mb-6">
        <h1 className="text-2xl font-semibold text-gray-900">Ayarlar</h1>
        <p className="mt-1 text-sm text-gray-500">
          İşletme bilgilerinizi ve menü ayarlarınızı yönetin.
        </p>
      </div>

      <div className="space-y-6">
        {/* ------------------------------------------- 1. Business details */}
        <section className="card p-5">
          <SectionHeading icon={Store} title="İşletme Bilgileri" />

          <div className="space-y-5">
            <ImageUploader
              value={draft.logo_url}
              onChange={(url) => update('logo_url', url)}
              label="Logo"
              hint="Kare veya yuvarlak logo önerilir · en fazla 5 MB"
              round
            />

            <div>
              <label className="label" htmlFor="business-name">
                İşletme Adı
              </label>
              <input
                id="business-name"
                type="text"
                className="input"
                value={draft.name}
                maxLength={100}
                placeholder="Kahve Durağı"
                onChange={(event) => update('name', event.target.value)}
              />
              <p className="help-text">2 ile 100 karakter arasında olmalıdır.</p>
            </div>

            <div>
              <label className="label" htmlFor="menu-slug">
                Menü Adresi (subdomain)
              </label>
              <div className="flex">
                <input
                  id="menu-slug"
                  type="text"
                  className="input rounded-r-none"
                  value={draft.slug}
                  placeholder="kahve-duragi"
                  autoComplete="off"
                  spellCheck={false}
                  onChange={(event) => {
                    setSlugError('')
                    update('slug', cleanSlug(event.target.value))
                  }}
                />
                <span className="inline-flex shrink-0 items-center rounded-r-lg border border-l-0 border-gray-300 bg-gray-100 px-3 text-sm text-gray-500">
                  {MENU_SUFFIX}
                </span>
              </div>

              {slugError ? <p className="error-text">{slugError}</p> : null}

              <p className="help-text">
                Menünüz şu adresten açılacak:{' '}
                <b className="text-gray-700">
                  {displaySlug}
                  {MENU_SUFFIX}
                </b>
              </p>
              <p className="help-text">
                Adresi değiştirirseniz eski QR kodlarınız çalışmaya devam ETMEZ. Değişiklikten sonra
                QR kodunuzu yeniden indirin.
              </p>
            </div>

            <div>
              <label className="label" htmlFor="business-phone">
                <span className="inline-flex items-center gap-1.5">
                  <Phone className="h-3.5 w-3.5 text-gray-400" aria-hidden="true" />
                  Telefon
                </span>
              </label>
              <input
                id="business-phone"
                type="tel"
                className="input"
                value={draft.phone}
                placeholder="+90 555 000 00 00"
                onChange={(event) => update('phone', event.target.value)}
              />
            </div>

            <div>
              <label className="label" htmlFor="business-address">
                <span className="inline-flex items-center gap-1.5">
                  <MapPin className="h-3.5 w-3.5 text-gray-400" aria-hidden="true" />
                  Adres
                </span>
              </label>
              <textarea
                id="business-address"
                rows={3}
                className="input resize-y"
                value={draft.address}
                placeholder="Caferağa Mah. Moda Cad. No: 12, Kadıköy / İstanbul"
                onChange={(event) => update('address', event.target.value)}
              />
            </div>

            <div>
              <label className="label" htmlFor="business-instagram">
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
                  id="business-instagram"
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

            {/* Which of the two identity fields above — the logo or the name —
                the customer menu prints at the top of the page. */}
            <div className="border-t border-gray-100 pt-5">
              <span className="label">Menü Başlığı</span>

              <div
                className="inline-flex flex-wrap rounded-lg border border-gray-200 bg-gray-50 p-1"
                role="group"
                aria-label="Menü başlığı görünümü"
              >
                {HEADER_DISPLAY_MODES.map((mode) => {
                  const isSelected = draft.header_display === mode.id
                  return (
                    <button
                      key={mode.id}
                      type="button"
                      onClick={() => update('header_display', mode.id)}
                      aria-pressed={isSelected}
                      className={`rounded-md px-4 py-1.5 text-sm ${
                        isSelected
                          ? 'bg-white font-medium text-gray-900 shadow-card'
                          : 'text-gray-500 hover:text-gray-700'
                      }`}
                    >
                      {mode.label}
                    </button>
                  )
                })}
              </div>

              <p className="help-text">
                Müşteri menüsünün en üstünde ne görüneceğini seçin. Logo yoksa işletme adı
                gösterilir.
              </p>
            </div>
          </div>
        </section>

        {/* -------------------------------------------------------- 2. Wi-Fi */}
        <section className="card p-5">
          <SectionHeading
            icon={Wifi}
            title="Wi-Fi"
            description="Menüde müşterilerinize gösterilir; ağ adını ve şifreyi tek dokunuşla kopyalayabilirler."
          />

          <div className="grid gap-5 sm:grid-cols-2">
            <div>
              <label className="label" htmlFor="wifi-ssid">
                Wi-Fi ağ adı
              </label>
              <input
                id="wifi-ssid"
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
              <label className="label" htmlFor="business-wifi">
                Wi-Fi şifresi
              </label>
              <div className="relative">
                <input
                  id="business-wifi"
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
        </section>

        {/* --------------------------------------------------- 3. Background */}
        <section className="card p-5">
          <SectionHeading
            icon={ImageIcon}
            title="Menü Arka Planı"
            description="Menü sayfasının zemini. Düz bir renk ya da işletmenize ait bir görsel kullanabilirsiniz."
          />

          {/* Segmented control */}
          <div
            className="inline-flex rounded-lg border border-gray-200 bg-gray-50 p-1"
            role="group"
            aria-label="Arka plan türü"
          >
            {[
              { id: 'color', label: 'Renk' },
              { id: 'image', label: 'Görsel' },
            ].map((option) => {
              const isSelected = draft.background_type === option.id
              return (
                <button
                  key={option.id}
                  type="button"
                  onClick={() => update('background_type', option.id)}
                  aria-pressed={isSelected}
                  className={`rounded-md px-4 py-1.5 text-sm ${
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
                <p className="help-text">{readabilityHint(draft.background_overlay_opacity)}</p>
              </div>
            </div>
          ) : (
            <div className="mt-5">
              <span className="label">Arka plan rengi</span>
              <div className="flex flex-wrap items-start gap-3">
                <input
                  type="color"
                  value={isValidColor(draft.background_color) ? draft.background_color : '#ffffff'}
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
                  const isSelected = String(draft.background_color || '').toLowerCase() === color
                  return (
                    <button
                      key={color}
                      type="button"
                      onClick={() => update('background_color', color)}
                      title={color}
                      aria-label={`Arka planı ${color} yap`}
                      className={`h-9 w-9 rounded-lg border ${
                        isSelected ? 'border-transparent ring-2 ring-brand-600' : 'border-gray-200'
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
        </section>

        {/* ---------------------------------------------------- 4. Currency */}
        <section className="card p-5">
          <SectionHeading
            icon={Globe}
            title="Para Birimi"
            description="Menüdeki tüm fiyatlar bu para birimiyle gösterilir."
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
        </section>

        {/* ----------------------------------------------- 5. Menu languages */}
        <section className="card p-5">
          <SectionHeading icon={Languages} title="Menü Dilleri" />

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
        </section>

        {/* ------------------------------------- 6. Footer / legal notices */}
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
                  Bu tarih, toplu fiyat güncellemesi yaptığınızda otomatik olarak yenilenir.
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
          </div>
        </section>

        {/* ------------------------------------------------- 7. Menu status */}
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
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600" aria-hidden="true" />
              <p className="text-sm text-amber-800">
                Menünüz kapalı. Müşteriler QR kodu okuttuğunda menüyü göremez.
              </p>
            </div>
          ) : null}
        </section>
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
