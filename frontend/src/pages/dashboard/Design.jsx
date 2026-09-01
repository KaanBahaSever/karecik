import { useEffect, useState } from 'react'
import { Info, Palette, Save } from 'lucide-react'

import { useAuth } from '../../lib/auth.jsx'
import { THEMES } from '../../themes/themes'
import { FONTS, loadAllFonts } from '../../themes/fonts'
import Loading from '../../components/ui/Loading.jsx'
import { useToast } from '../../components/ui/Toast.jsx'
import LivePreview from '../../components/dashboard/LivePreview.jsx'

/* Fields owned (and saved) by this page */
const DESIGN_FIELDS = [
  'theme',
  'font_family',
  'primary_color',
  'splash_enabled',
  'splash_duration',
  'splash_bg_color',
  'splash_text',
]

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

/** Validates the #RRGGBB format. */
function isValidColor(value) {
  return /^#[0-9a-fA-F]{6}$/.test(String(value || ''))
}

/** Falls back to a value the color input will accept. */
function safeColor(value, fallback) {
  return isValidColor(value) ? value : fallback
}

/** 1200 -> "1,2 saniye" */
function formatDuration(milliseconds) {
  const seconds = Number(milliseconds || 0) / 1000
  return `${seconds.toFixed(1).replace('.', ',')} saniye`
}

/** White text on dark backgrounds, dark text on light ones. */
function textColorOn(hex) {
  if (!isValidColor(hex)) return '#ffffff'
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  const luminance = (r * 299 + g * 587 + b * 114) / 1000
  return luminance > 150 ? '#111827' : '#ffffff'
}

export default function Design() {
  const { business, saveBusiness } = useAuth()
  const toast = useToast()

  const [draft, setDraft] = useState(null)
  const [saving, setSaving] = useState(false)

  // Refresh the draft whenever the stored business changes (first load or save).
  useEffect(() => {
    setDraft(business ? { ...business } : null)
  }, [business])

  // Render the font cards in their own typefaces.
  useEffect(() => {
    loadAllFonts()
  }, [])

  if (!business || !draft) return <Loading text="Tasarım ayarları yükleniyor..." />

  /* --------------------------------------------------------------- helpers */

  const update = (field, value) => {
    setDraft((previous) => ({ ...previous, [field]: value }))
  }

  const changedFields = DESIGN_FIELDS.filter((field) => draft[field] !== business[field])
  const hasChanges = changedFields.length > 0

  const splashEnabled = Boolean(draft.splash_enabled)
  const primaryColor = safeColor(draft.primary_color, '#1d4ed8')
  const splashBackground = safeColor(draft.splash_bg_color, '#0f172a')
  const primaryColorInvalid = !isValidColor(draft.primary_color)
  const splashBackgroundInvalid = !isValidColor(draft.splash_bg_color)

  const revert = () => {
    setDraft({ ...business })
  }

  const save = async () => {
    if (!hasChanges || saving) return

    if (primaryColorInvalid) {
      toast.error('Ana renk #RRGGBB biçiminde olmalı (örn. #1d4ed8).')
      return
    }
    if (splashBackgroundInvalid) {
      toast.error('Karşılama ekranı arka plan rengi #RRGGBB biçiminde olmalı.')
      return
    }

    // Send only the fields that actually changed.
    const body = {}
    changedFields.forEach((field) => {
      body[field] = field === 'splash_duration' ? Number(draft[field]) : draft[field]
    })

    setSaving(true)
    try {
      await saveBusiness(body)
      toast.success('Tasarım kaydedildi.')
    } catch (err) {
      toast.error(err.message)
    } finally {
      setSaving(false)
    }
  }

  /* ---------------------------------------------------------------- render */

  return (
    <div className="pb-10">
      {/* Top bar */}
      <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900">Tasarım</h1>
          <p className="mt-1 text-sm text-gray-500">Menünüzün görünümünü özelleştirin.</p>
        </div>

        <div className="flex shrink-0 items-center gap-2">
          <button
            type="button"
            className="btn-secondary"
            onClick={revert}
            disabled={!hasChanges || saving}
          >
            Değişiklikleri geri al
          </button>
          <button
            type="button"
            className="btn-primary"
            onClick={save}
            disabled={!hasChanges || saving}
          >
            <Save className="h-4 w-4" aria-hidden="true" />
            {saving ? 'Kaydediliyor...' : 'Kaydet'}
          </button>
        </div>
      </div>

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_380px]">
        {/* ------------------------------------------------------ left column */}
        <div className="min-w-0 space-y-6">
          {/* 1. Theme */}
          <section className="card p-5">
            <div className="mb-4 flex items-center gap-2">
              <Palette className="h-4 w-4 text-brand-600" aria-hidden="true" />
              <h2 className="text-base font-semibold text-gray-900">Tasarım Tarzı</h2>
            </div>

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
                    <p className="mt-0.5 text-xs leading-snug text-gray-500">{theme.description}</p>
                  </button>
                )
              })}
            </div>
          </section>

          {/* 2. Font */}
          <section className="card p-5">
            <div className="mb-1 flex items-center gap-2">
              <span className="text-base leading-none" aria-hidden="true">
                🅰️
              </span>
              <h2 className="text-base font-semibold text-gray-900">Yazı Tipi</h2>
            </div>
            <p className="help-text mb-4">
              Menüdeki tüm başlık ve metinler bu yazı tipiyle görünür.
            </p>

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
          </section>

          {/* 3. Accent colour */}
          <section className="card p-5">
            <h2 className="text-base font-semibold text-gray-900">Ana Renk</h2>
            <p className="help-text mb-4">
              Butonlar, fiyatlar ve seçili kategori bu renkle vurgulanır.
            </p>

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
                  placeholder="#1d4ed8"
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

            <div className="mt-4 flex flex-wrap gap-2">
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
          </section>

          {/* 4. Splash screen */}
          <section className="card p-5">
            <div className="flex items-start justify-between gap-4">
              <div>
                <h2 className="text-base font-semibold text-gray-900">Karşılama Ekranı</h2>
                <p className="help-text max-w-xl">
                  Menü açıldığında logonuzla birlikte kısa bir karşılama ekranı gösterilir.
                  Yalnızca müşteri menüsünde görünür, panelde çalışmaz.
                </p>
              </div>

              <button
                type="button"
                role="switch"
                aria-checked={splashEnabled}
                aria-label="Karşılama ekranını aç veya kapat"
                onClick={() => update('splash_enabled', !splashEnabled)}
                className={`relative mt-1 inline-flex h-6 w-11 shrink-0 items-center rounded-full ${
                  splashEnabled ? 'bg-brand-600' : 'bg-gray-300'
                }`}
              >
                <span
                  className={`inline-block h-4 w-4 rounded-full bg-white shadow ${
                    splashEnabled ? 'translate-x-6' : 'translate-x-1'
                  }`}
                />
              </button>
            </div>

            <div className={`mt-5 grid gap-5 lg:grid-cols-2 ${splashEnabled ? '' : 'opacity-60'}`}>
              <div className="space-y-5">
                {/* Duration */}
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

                {/* Background colour */}
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
                        onChange={(event) =>
                          update('splash_bg_color', event.target.value.trim())
                        }
                        placeholder="#0f172a"
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

                {/* Text */}
                <div>
                  <label htmlFor="splash-text" className="label">
                    Karşılama metni
                  </label>
                  <input
                    id="splash-text"
                    type="text"
                    className="input"
                    value={draft.splash_text || ''}
                    onChange={(event) => update('splash_text', event.target.value.slice(0, 200))}
                    placeholder="Hoş geldiniz"
                    maxLength={200}
                    disabled={!splashEnabled}
                  />
                  <p className="help-text">{(draft.splash_text || '').length} / 200 karakter</p>
                </div>
              </div>

              {/* Static preview box */}
              <div>
                <span className="label">Önizleme</span>
                <div
                  className="flex h-56 flex-col items-center justify-center gap-3 rounded-xl border border-gray-200 px-6 text-center"
                  style={{ backgroundColor: splashBackground }}
                >
                  {draft.logo_url ? (
                    <img
                      src={draft.logo_url}
                      alt=""
                      className="h-16 w-16 rounded-full object-cover"
                    />
                  ) : (
                    <div
                      className="flex h-16 w-16 items-center justify-center rounded-full text-lg font-semibold"
                      style={{
                        backgroundColor: primaryColor,
                        color: textColorOn(primaryColor),
                      }}
                    >
                      {(draft.name || 'K').trim().charAt(0).toUpperCase()}
                    </div>
                  )}

                  <p
                    className="text-base font-medium"
                    style={{ color: textColorOn(splashBackground) }}
                  >
                    {draft.splash_text || 'Hoş geldiniz'}
                  </p>
                </div>

                <div className="mt-3 flex items-start gap-2 rounded-lg border border-blue-200 bg-blue-50 px-3 py-2">
                  <Info className="mt-0.5 h-4 w-4 shrink-0 text-blue-600" aria-hidden="true" />
                  <p className="text-xs leading-snug text-blue-800">
                    Karşılama ekranı yalnızca müşteriler menüyü ilk açtığında görünür; sağdaki
                    canlı önizlemede gösterilmez.
                  </p>
                </div>
              </div>
            </div>
          </section>
        </div>

        {/* ----------------------------------------------------- right column */}
        <div className="min-w-0">
          <div className="xl:sticky xl:top-6">
            <LivePreview business={draft} refresh={0} />
          </div>
        </div>
      </div>
    </div>
  )
}
