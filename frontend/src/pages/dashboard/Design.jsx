import { useEffect, useState } from 'react'
import { Palette, Save } from 'lucide-react'

import { useAuth } from '../../lib/auth.jsx'
import { THEMES } from '../../themes/themes'
import { FONTS, loadAllFonts } from '../../themes/fonts'
import {
  DEFAULT_SPLASH_EXIT,
  isValidSplashAnimation,
  isValidSplashEasing,
  SPLASH_DISPLAY_MODES,
  SPLASH_EASINGS,
  SPLASH_EXIT_ANIMATIONS,
} from '../../themes/splash'
import Loading from '../../components/ui/Loading.jsx'
import ImageUploader from '../../components/ui/ImageUploader.jsx'
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
  'splash_logo_url',
  'splash_headline',
  'splash_exit_animation',
  'splash_exit_easing',
  'splash_exit_duration',
  'splash_display',
]

/* Fields sent as numbers even though the inputs hand back strings */
const NUMERIC_FIELDS = ['splash_duration', 'splash_exit_duration']

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

  // The selects are controlled, so a value the catalogue does not know would
  // leave them blank — every one of them falls back to the column default.
  const splashExitAnimation = isValidSplashAnimation(draft.splash_exit_animation)
    ? draft.splash_exit_animation
    : DEFAULT_SPLASH_EXIT.animation
  const splashExitEasing = isValidSplashEasing(draft.splash_exit_easing)
    ? draft.splash_exit_easing
    : DEFAULT_SPLASH_EXIT.easing
  const splashExitDuration = Number(draft.splash_exit_duration) || DEFAULT_SPLASH_EXIT.duration
  const splashDisplay = SPLASH_DISPLAY_MODES.some((mode) => mode.id === draft.splash_display)
    ? draft.splash_display
    : 'both'

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
      if (NUMERIC_FIELDS.includes(field)) {
        body[field] = Number(draft[field])
        return
      }
      if (field === 'splash_logo_url') {
        // An empty logo clears the column instead of storing "".
        body.splash_logo_url = draft.splash_logo_url || null
        return
      }
      if (field === 'splash_headline') {
        body.splash_headline = String(draft.splash_headline || '').trim()
        return
      }
      body[field] = draft[field]
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

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_clamp(340px,30vw,460px)]">
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
                  Müşterilere oturum başına yalnızca bir kez görünür.
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
              {/* ------------------------------------------- logo and text */}
              <div className="space-y-5">
                {/* What the splash screen shows */}
                <div>
                  <span className="label">Görünüm</span>
                  <div
                    className="inline-flex flex-wrap rounded-lg border border-gray-200 bg-gray-50 p-1"
                    role="group"
                    aria-label="Karşılama ekranı görünümü"
                  >
                    {SPLASH_DISPLAY_MODES.map((mode) => {
                      const isSelected = splashDisplay === mode.id
                      return (
                        <button
                          key={mode.id}
                          type="button"
                          onClick={() => update('splash_display', mode.id)}
                          disabled={!splashEnabled}
                          aria-pressed={isSelected}
                          className={`rounded-md px-3 py-1.5 text-sm disabled:cursor-not-allowed ${
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
                    Sadece yazı seçilirse logo gösterilmez, sadece logo seçilirse başlık ve alt
                    başlık gizlenir.
                  </p>
                </div>

                <div>
                  <ImageUploader
                    value={draft.splash_logo_url || null}
                    onChange={(url) => update('splash_logo_url', url)}
                    label="Karşılama logosu"
                    hint="Yatay logolar desteklenir · SVG, PNG veya JPG"
                  />
                  <p className="help-text">Boş bırakırsanız işletme logonuz kullanılır.</p>
                </div>

                {/* Headline */}
                <div>
                  <label htmlFor="splash-headline" className="label">
                    Karşılama başlığı (opsiyonel)
                  </label>
                  <input
                    id="splash-headline"
                    type="text"
                    className="input"
                    value={draft.splash_headline || ''}
                    onChange={(event) => update('splash_headline', event.target.value.slice(0, 60))}
                    placeholder={draft.name || 'Kahve Durağı'}
                    maxLength={60}
                    disabled={!splashEnabled}
                  />
                  <p className="help-text">
                    Boş bırakırsanız gösterilmez ·{' '}
                    {(draft.splash_headline || '').length} / 60 karakter
                  </p>
                </div>

                {/* Tagline */}
                <div>
                  <label htmlFor="splash-text" className="label">
                    Alt başlık (opsiyonel)
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

              {/* --------------------------------------- colour and timing */}
              <div className="space-y-5">
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
                        onChange={(event) => update('splash_bg_color', event.target.value.trim())}
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
                      const isSelected = String(draft.splash_bg_color || '').toLowerCase() === color
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
                    value={splashExitAnimation}
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

                {/* Exit easing */}
                <div>
                  <label htmlFor="splash-exit-easing" className="label">
                    Animasyon eğrisi
                  </label>
                  <select
                    id="splash-exit-easing"
                    className="input"
                    value={splashExitEasing}
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
                      {formatDuration(splashExitDuration)}
                    </span>
                  </div>
                  <input
                    id="splash-exit-duration"
                    type="range"
                    min={100}
                    max={2000}
                    step={50}
                    value={splashExitDuration}
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
              <b className="font-medium text-gray-700">Karşılama ekranını oynat</b> düğmesine basarak
              nasıl göründüğünü deneyebilirsiniz.
            </p>
          </section>
        </div>

        {/* ----------------------------------------------------- right column */}
        <div className="min-w-0">
          <div className="xl:sticky xl:top-6">
            <LivePreview business={draft} refresh={0} showSplashControl />
          </div>
        </div>
      </div>
    </div>
  )
}
