import { useEffect, useRef } from 'react'

/**
 * Splash screen shown once when the customer menu opens.
 *
 * @param {object}   business - PublicMenu.business (logo_url, name, splash_*)
 * @param {function} onDone   - Called when the timer ends or the screen is tapped
 */

/* A very short, one-off opening animation — no library involved. */
const ANIMATION_STYLE = `
@keyframes karecikSplashIn {
  from { opacity: 0; transform: scale(0.94); }
  to   { opacity: 1; transform: scale(1); }
}
`

/** "#0f172a" or "#fff" -> { r, g, b }; null when unparseable. */
function parseHexColor(hex) {
  const clean = String(hex || '').trim().replace('#', '')

  if (clean.length === 3) {
    const r = Number.parseInt(clean[0] + clean[0], 16)
    const g = Number.parseInt(clean[1] + clean[1], 16)
    const b = Number.parseInt(clean[2] + clean[2], 16)
    if ([r, g, b].some(Number.isNaN)) return null
    return { r, g, b }
  }

  if (clean.length === 6) {
    const r = Number.parseInt(clean.slice(0, 2), 16)
    const g = Number.parseInt(clean.slice(2, 4), 16)
    const b = Number.parseInt(clean.slice(4, 6), 16)
    if ([r, g, b].some(Number.isNaN)) return null
    return { r, g, b }
  }

  return null
}

/**
 * Picks a readable text colour for the given background.
 * The logo's own colours are unknown, so the text follows the backdrop.
 */
function readableTextColor(background) {
  const rgb = parseHexColor(background)
  if (!rgb) return '#ffffff'

  const luminance = (0.299 * rgb.r + 0.587 * rgb.g + 0.114 * rgb.b) / 255
  return luminance > 0.6 ? '#111827' : '#ffffff'
}

export default function SplashScreen({ business, onDone }) {
  const background = business?.splash_bg_color || '#0f172a'
  const textColor = readableTextColor(background)
  const isDarkText = textColor === '#111827'

  const configured = Number(business?.splash_duration)
  const duration = Number.isFinite(configured) && configured > 0 ? Math.min(configured, 6000) : 1500

  const businessName = String(business?.name || '').trim()
  const initial = businessName ? businessName.charAt(0).toLocaleUpperCase('tr') : '•'

  // Keep the timer stable even when the prop identity changes on re-render.
  const onDoneRef = useRef(onDone)
  useEffect(() => {
    onDoneRef.current = onDone
  }, [onDone])

  useEffect(() => {
    const timer = setTimeout(() => {
      onDoneRef.current?.()
    }, duration)

    return () => clearTimeout(timer)
  }, [duration])

  function skip() {
    onDoneRef.current?.()
  }

  return (
    <div
      role="button"
      tabIndex={0}
      aria-label="Karşılama ekranını geç"
      onClick={skip}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ' || event.key === 'Escape') skip()
      }}
      className="fixed inset-0 z-50 flex cursor-pointer select-none items-center justify-center px-6"
      style={{ backgroundColor: background, color: textColor }}
    >
      <style>{ANIMATION_STYLE}</style>

      <div
        className="flex flex-col items-center gap-4 text-center"
        style={{ animation: 'karecikSplashIn 320ms ease-out both' }}
      >
        {business?.logo_url ? (
          <img src={business.logo_url} alt="" className="h-24 w-24 object-contain" />
        ) : (
          <div
            className="flex h-24 w-24 items-center justify-center rounded-full text-4xl font-semibold"
            style={{
              backgroundColor: isDarkText ? 'rgba(17, 24, 39, 0.08)' : 'rgba(255, 255, 255, 0.14)',
              color: textColor,
            }}
            aria-hidden="true"
          >
            {initial}
          </div>
        )}

        {businessName ? <h1 className="text-2xl font-semibold">{businessName}</h1> : null}

        {business?.splash_text ? (
          <p className="max-w-xs text-sm" style={{ opacity: 0.8 }}>
            {business.splash_text}
          </p>
        ) : null}
      </div>
    </div>
  )
}
