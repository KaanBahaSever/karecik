import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { AlertCircle, Eye, LayoutList, Play } from 'lucide-react'

import api from '../../lib/api'
import { currencySymbol } from '../../lib/format'
import { loadFont } from '../../themes/fonts'
import { backgroundStyles, findTheme } from '../../themes/themes'
import { findLanguage } from '../../locales/index.js'
import Loading from '../ui/Loading.jsx'
import MenuContent from '../menu/MenuContent.jsx'
import SplashScreen from '../menu/SplashScreen.jsx'

/* The preview is a real mobile viewport, not an approximation: 390 x 844 CSS px
   is the iPhone 14 / 15 logical screen. The frame adds the bezel on every side,
   so the device as a whole is FRAME_WIDTH x FRAME_HEIGHT and is scaled down as
   one unit whenever the dashboard column is narrower than that. */
const VIEWPORT_WIDTH = 390
const VIEWPORT_HEIGHT = 844
const BEZEL = 12
const FRAME_WIDTH = VIEWPORT_WIDTH + BEZEL * 2
const FRAME_HEIGHT = VIEWPORT_HEIGHT + BEZEL * 2

/* Vertical space the dashboard takes around the frame: the 64px topbar, the
   sticky 24px offset, the preview toolbar and the caption underneath, plus a
   little breathing room. Subtracted from the viewport height so the phone is
   never taller than the scrollport it lives in. */
const CHROME_ALLOWANCE = 190

/* Never shrink past this, even on a very short window — below it the preview
   stops being readable and a scrollbar is the better trade. */
const MIN_FRAME_HEIGHT = 460

/* Every splash setting whose change should replay the entrance. They are joined
   into one string and watched as a single value, so any change to any of them
   replays without a twelve-entry dependency array.

   The header logo used to be in here too. It has no entrance of its own any
   more — that motion belongs to the splash, which is the moment the menu opens
   — so there is nothing left to replay for it. */
const REPLAY_FIELDS = [
  'splash_enabled',
  'splash_logo_url',
  'splash_headline',
  'splash_text',
  'splash_bg_color',
  'splash_duration',
  'splash_entrance',
  'splash_exit_animation',
  'splash_exit_duration',
  'splash_exit_easing',
  'splash_display',
  'splash_slide_fade',
]

/* Long enough that dragging the duration slider replays the splash once, when
   the pointer settles, instead of on every pixel of the drag. */
const SPLASH_REPLAY_DELAY = 250

/**
 * Phone-shaped preview that shows how dashboard changes look in the customer menu.
 *
 * The menu content comes from the server (api.previewMenu), while the appearance
 * settings come from the `business` prop — which may still be unsaved. That is
 * what lets a theme, font or colour change show up before it is persisted.
 *
 * With no menu slug there is no menu to preview — a business may own none — so
 * the request is skipped entirely and the frame shows a placeholder instead.
 *
 * @param {object}  business          - Draft (possibly unsaved) menu settings
 * @param {number}  refresh           - Bump this value to refetch the menu
 * @param {string}  menuSlug          - Preview this menu; empty means "no menu"
 * @param {string}  className
 * @param {boolean} showSplashControl - Adds the "replay splash screen" button
 */
export default function LivePreview({
  business,
  refresh = 0,
  menuSlug = '',
  className = '',
  showSplashControl = false,
}) {
  const [language, setLanguage] = useState(business?.default_language || 'tr')
  const [menu, setMenu] = useState(null)
  const [loading, setLoading] = useState(Boolean(menuSlug))
  const [error, setError] = useState('')

  // Splash replay: `splashKey` is handed to SplashScreen as `replayKey`, so
  // pressing the button again while the screen is up restarts the sequence.
  const [splashOpen, setSplashOpen] = useState(false)
  const [splashKey, setSplashKey] = useState(0)

  // The device renders at its true pixel size and is scaled to fit the column.
  const wrapperRef = useRef(null)
  const [scale, setScale] = useState(1)

  const languages =
    Array.isArray(business?.languages) && business.languages.length ? business.languages : ['tr']

  // If the selected language is removed from the business, fall back to the first.
  useEffect(() => {
    if (!languages.includes(language)) setLanguage(languages[0])
  }, [languages.join(','), language]) // eslint-disable-line react-hooks/exhaustive-deps

  // Load the selected font right away.
  useEffect(() => {
    if (business?.font_family) loadFont(business.font_family)
  }, [business?.font_family])

  /* Measure the space available and scale the frame down to fit it. This runs in
     a layout effect so the very first paint already has the right size.

     Scaling by width alone is not enough: the preview column sits in a `sticky`
     wrapper, so on a short screen (a 1366x768 laptop) a full-height frame is
     taller than the scrollport and its bottom can never be scrolled into view.
     The height budget below subtracts the dashboard chrome above and around the
     frame, so the whole phone always fits on screen. */
  useLayoutEffect(() => {
    const node = wrapperRef.current
    if (!node) return undefined

    function measure(width) {
      const available = Number.isFinite(width) ? width : node.clientWidth
      if (!available) return

      const viewportHeight = window.innerHeight || FRAME_HEIGHT + CHROME_ALLOWANCE
      const heightBudget = Math.max(MIN_FRAME_HEIGHT, viewportHeight - CHROME_ALLOWANCE)

      setScale(Math.min(1, available / FRAME_WIDTH, heightBudget / FRAME_HEIGHT))
    }

    measure()

    window.addEventListener('resize', measure)

    if (typeof ResizeObserver === 'undefined') {
      return () => window.removeEventListener('resize', measure)
    }

    const observer = new ResizeObserver((entries) => {
      measure(entries[0]?.contentRect?.width)
    })
    observer.observe(node)

    return () => {
      window.removeEventListener('resize', measure)
      observer.disconnect()
    }
  }, [])

  // Fetch the menu content.
  useEffect(() => {
    let cancelled = false

    // Nothing to preview: there is no default menu to fall back to, so the
    // request is not made at all and the frame renders its placeholder.
    if (!menuSlug) {
      setMenu(null)
      setError('')
      setLoading(false)
      return undefined
    }

    async function loadMenu() {
      setLoading(true)
      setError('')
      try {
        const data = await api.previewMenu(language, { menu: menuSlug })
        if (cancelled) return
        setMenu(data)
      } catch (err) {
        if (cancelled) return
        // No toast here: a preview failure should not be noisy.
        setError(err.message || 'Önizleme yüklenemedi.')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    loadMenu()
    return () => {
      cancelled = true
    }
  }, [refresh, language, menuSlug])

  /* Merge the draft settings over the menu payload returned by the server.

     This is a WHOLE-OBJECT spread on purpose — there is no field allowlist —
     so every column the dashboard draft carries reaches the customer
     components the moment `buildDraft` knows about it, with no change here.
     That is what makes text_color, show_yerli_uretim and yerli_uretim_logo_url
     preview live: MenuContent hands text_color to themeVariables and
     MenuFooter reads the other two, both straight off this object.

     The one thing to watch is that a key merely PRESENT on the draft wins,
     even when its value is undefined — so a draft field must be built with a
     real default (null for a clearable URL) rather than left undefined, or it
     would blank out the server's value in the preview alone. */
  const previewBusiness = {
    ...(menu?.business || {}),
    ...(business || {}),
    currency_symbol: currencySymbol(business?.currency),
  }

  const previewMenu = menu ? { ...menu, business: previewBusiness } : null

  /* The screen used to be a hardcoded `bg-white`, which is what bled as a pale
     rim around the menu: every theme but a pure-white one paints a different
     colour, and the viewport's own white showed through wherever the menu's
     square content was clipped by the 2.5rem corner radius — and below the fold
     on a short menu. Painting the screen in the previewed menu's own background
     removes the mismatch instead of masking it. With no menu to preview the
     placeholder is styled for light, so white stays right there. */
  const screenBackground = previewMenu
    ? backgroundStyles(previewBusiness.theme, previewBusiness).containerStyle?.backgroundColor ||
      findTheme(previewBusiness.theme).colors.background
    : '#ffffff'

  /* Bumping the key first means a second press replays the sequence instead of
     doing nothing while the screen is still on. */
  function playSplash() {
    setSplashKey((previous) => previous + 1)
    setSplashOpen(true)
  }

  /* Auto-replay: editing any splash setting should show its result without the
     user reaching for the button.

     The first settings the preview ever sees are the baseline, never a replay —
     mounting is not a change. That baseline is the first NON-EMPTY reading and
     not simply the first render: the menu editor mounts this component before
     the menu context has resolved a menu, so on that page the settings arrive a
     tick late and would otherwise read as an edit.

     Every later run schedules the replay instead of playing it at once, and the
     cleanup cancels a pending one — so dragging the duration slider replays
     once, when it comes to rest, and never after the preview is unmounted. */
  const replaySignature = REPLAY_FIELDS.map((field) => String(business?.[field] ?? '')).join('|')
  const replaySettled =
    Boolean(business) && REPLAY_FIELDS.some((field) => business[field] !== undefined)
  const splashBaselineRef = useRef(false)

  useEffect(() => {
    if (!replaySettled) return undefined

    if (!splashBaselineRef.current) {
      splashBaselineRef.current = true
      return undefined
    }

    // Switching the splash off is a change too, but replaying a screen the
    // customer will never see would only confuse. The logo fade-in still shows
    // itself in that case: MenuContent adds and removes the animation property
    // on the header logo as the toggle flips, so it plays on the spot.
    if (business?.splash_enabled === false) return undefined

    const timer = setTimeout(() => {
      setSplashKey((previous) => previous + 1)
      setSplashOpen(true)
    }, SPLASH_REPLAY_DELAY)

    return () => clearTimeout(timer)
  }, [replaySignature]) // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className={className}>
      {/* header row */}
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2 text-sm font-medium text-gray-700">
          <Eye className="h-4 w-4 text-brand-600" aria-hidden="true" />
          <span>Canlı Önizleme</span>
        </div>

        <div className="flex items-center gap-2">
          {showSplashControl && menuSlug ? (
            <button
              type="button"
              onClick={playSplash}
              title="Karşılama ekranını oynat"
              aria-label="Karşılama ekranını oynat"
              className="inline-flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-2 py-1 text-xs font-medium text-gray-700 outline-none hover:bg-gray-50 focus:border-brand-600 focus:ring-2 focus:ring-brand-100"
            >
              <Play className="h-3.5 w-3.5 text-brand-600" aria-hidden="true" />
              Karşılama ekranını oynat
            </button>
          ) : null}

          {languages.length > 1 ? (
            <select
              value={language}
              onChange={(event) => setLanguage(event.target.value)}
              aria-label="Önizleme dili"
              className="rounded-lg border border-gray-300 bg-white px-2 py-1 text-xs text-gray-700 outline-none focus:border-brand-600 focus:ring-2 focus:ring-brand-100"
            >
              {languages.map((code) => {
                const info = findLanguage(code)
                return (
                  <option key={code} value={code}>
                    {info.short} {info.label}
                  </option>
                )
              })}
            </select>
          ) : null}
        </div>
      </div>

      {/* Scaling wrapper. The frame keeps its real size and only the transform
          shrinks it, so the menu inside is genuinely laid out at 390 px. The
          wrapper reserves the SCALED height — without it whatever follows on
          the page would slide underneath the frame. */}
      <div ref={wrapperRef} className="flex justify-center" style={{ height: FRAME_HEIGHT * scale }}>
        <div
          className="relative shrink-0 rounded-[3.25rem] bg-gray-900 shadow-panel ring-1 ring-black/10"
          style={{
            width: FRAME_WIDTH,
            height: FRAME_HEIGHT,
            padding: BEZEL,
            transform: `scale(${scale})`,
            transformOrigin: 'top center',
          }}
        >
          {/* Side buttons, drawn on the bezel itself so they never widen the frame. */}
          <span
            className="pointer-events-none absolute left-0 top-[120px] h-8 w-[3px] rounded-r-full bg-gray-700"
            aria-hidden="true"
          />
          <span
            className="pointer-events-none absolute left-0 top-[168px] h-12 w-[3px] rounded-r-full bg-gray-700"
            aria-hidden="true"
          />
          <span
            className="pointer-events-none absolute right-0 top-[150px] h-16 w-[3px] rounded-l-full bg-gray-700"
            aria-hidden="true"
          />

          {/* The viewport: exactly 390 x 844 CSS px. */}
          <div
            className="relative overflow-hidden rounded-[2.5rem]"
            style={{
              width: VIEWPORT_WIDTH,
              height: VIEWPORT_HEIGHT,
              backgroundColor: screenBackground,
            }}
          >
            {/* Dynamic Island. It sits above the menu (`z-20`) and the viewport's
                own `overflow-hidden rounded-[2.5rem]` clips it to the screen, so
                it never paints out onto the bezel. The slight light ring keeps it
                readable on the dark themes too. */}
            <div
              className="pointer-events-none absolute left-1/2 top-2 z-20 h-[26px] w-[110px] -translate-x-1/2 rounded-full bg-gray-950 ring-1 ring-white/10"
              aria-hidden="true"
            />

            {/* The island covers y = 8..34 px of the viewport, which is exactly
                where MenuContent's own padding puts the logo and the language
                switcher — the switcher was not even clickable there. This custom
                property is the safe area MenuContent adds to its top padding.
                On a real phone it is unset, so `env(safe-area-inset-top)` applies
                instead, which is the right value there. */}
            <div
              className="no-scrollbar h-full overflow-y-auto overflow-x-hidden"
              style={{ '--menu-safe-top': '54px' }}
            >
              {!menuSlug ? (
                /* No menu to preview. The placeholder stays inside the phone so
                   the dashboard keeps its shape while the account has no menu. */
                <div className="flex h-full flex-col items-center justify-center px-8 text-center">
                  <span
                    className="mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-gray-100 text-gray-400"
                    aria-hidden="true"
                  >
                    <LayoutList className="h-6 w-6" />
                  </span>
                  <p className="text-sm font-semibold text-gray-900">Henüz menü eklenmedi</p>
                  <p className="mt-1 text-xs text-gray-500">
                    Bir menü oluşturduğunuzda burada görünecek
                  </p>
                </div>
              ) : loading ? (
                <Loading text="Önizleme hazırlanıyor..." />
              ) : error ? (
                <div className="flex h-full items-center justify-center p-6">
                  <div className="flex items-start gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2.5 text-left">
                    <AlertCircle
                      className="mt-0.5 h-4 w-4 shrink-0 text-red-600"
                      aria-hidden="true"
                    />
                    <div>
                      <p className="text-xs font-medium text-red-800">Önizleme yüklenemedi</p>
                      <p className="mt-0.5 text-[11px] leading-snug text-red-700">{error}</p>
                    </div>
                  </div>
                </div>
              ) : previewMenu ? (
                <MenuContent
                  menu={previewMenu}
                  language={language}
                  onLanguageChange={(next) => setLanguage(next)}
                  embedded
                />
              ) : null}
            </div>

            {/* `contained` keeps the splash inside the phone instead of covering
                the whole dashboard window. */}
            {splashOpen ? (
              <SplashScreen
                contained
                business={previewBusiness}
                replayKey={splashKey}
                onDone={() => setSplashOpen(false)}
              />
            ) : null}
          </div>
        </div>
      </div>

      <p className="mt-3 text-center text-xs text-gray-500">
        Değişiklikler burada anında görünür, kaydedene kadar müşterilere yansımaz.
      </p>
    </div>
  )
}
