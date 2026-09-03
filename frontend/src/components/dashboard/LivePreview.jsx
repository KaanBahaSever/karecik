import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { AlertCircle, Eye, Play } from 'lucide-react'

import api from '../../lib/api'
import { currencySymbol } from '../../lib/format'
import { loadFont } from '../../themes/fonts'
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

/* Contact details a branch may override. The server has already resolved these
   against the previewed branch, so the business-level draft must NOT be spread
   over them — otherwise a branch with its own Wi-Fi or phone number would be
   previewed with the head office' values and the preview would stop matching
   what the customer sees at that branch. */
const BRANCH_SCOPED_FIELDS = ['phone', 'address', 'wifi_ssid', 'wifi_password']

/**
 * Phone-shaped preview that shows how dashboard changes look in the customer menu.
 *
 * The menu content comes from the server (api.previewMenu), while the appearance
 * settings come from the `business` prop — which may still be unsaved. That is
 * what lets a theme, font or colour change show up before it is persisted.
 *
 * @param {object}  business          - Draft (possibly unsaved) business settings
 * @param {number}  refresh           - Bump this value to refetch the menu
 * @param {string}  branchSlug        - Preview this branch (defaults to the business')
 * @param {string}  menuSlug          - Preview this menu (defaults to the default menu)
 * @param {string}  className
 * @param {boolean} showSplashControl - Adds the "replay splash screen" button
 */
export default function LivePreview({
  business,
  refresh = 0,
  branchSlug = '',
  menuSlug = '',
  className = '',
  showSplashControl = false,
}) {
  const [language, setLanguage] = useState(business?.default_language || 'tr')
  const [menu, setMenu] = useState(null)
  const [loading, setLoading] = useState(true)
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

    async function loadMenu() {
      setLoading(true)
      setError('')
      try {
        // Empty slugs are dropped by the query builder, so the server keeps
        // falling back to the business' default branch and menu.
        const data = await api.previewMenu(language, { branch: branchSlug, menu: menuSlug })
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
  }, [refresh, language, branchSlug, menuSlug])

  // Merge the draft settings over the business object returned by the server.
  const previewBusiness = {
    ...(menu?.business || {}),
    ...(business || {}),
    currency_symbol: currencySymbol(business?.currency),
  }

  // ...except the contact details, which the server resolved for the branch
  // being previewed. `branch_slug` is only set when a branch actually resolved.
  if (menu?.business?.branch_slug) {
    BRANCH_SCOPED_FIELDS.forEach((field) => {
      previewBusiness[field] = menu.business[field]
    })
  }

  const previewMenu = menu ? { ...menu, business: previewBusiness } : null

  /* Bumping the key first means a second press replays the sequence instead of
     doing nothing while the screen is still on. */
  function playSplash() {
    setSplashKey((previous) => previous + 1)
    setSplashOpen(true)
  }

  return (
    <div className={className}>
      {/* header row */}
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2 text-sm font-medium text-gray-700">
          <Eye className="h-4 w-4 text-brand-600" aria-hidden="true" />
          <span>Canlı Önizleme</span>
        </div>

        <div className="flex items-center gap-2">
          {showSplashControl ? (
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
            className="relative overflow-hidden rounded-[2.5rem] bg-white"
            style={{ width: VIEWPORT_WIDTH, height: VIEWPORT_HEIGHT }}
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
              {loading ? (
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
