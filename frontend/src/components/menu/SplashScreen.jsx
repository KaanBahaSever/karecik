import { useEffect, useRef, useState } from 'react'

import { DEFAULT_SPLASH_EXIT, SPLASH_DISPLAY_MODES, isValidSplashEasing } from '../../themes/splash'

/**
 * Splash screen shown once when the customer menu opens.
 *
 * The sequence is always the same: hold for `splash_duration` ms, play the exit
 * animation for `splash_exit_duration` ms, and only then call `onDone`. Tapping
 * the screen (or Enter / Space / Escape) starts the exit immediately instead of
 * cutting the whole thing short.
 *
 * Two components render it:
 *   1. pages/menu/CustomerMenu.jsx            — the real menu behind a QR code
 *   2. components/dashboard/LivePreview.jsx   — the dashboard "Replay" button
 * The prop signature is therefore FIXED; do not change it.
 *
 * @param {object}   business  - PublicMenu.business (logo_url, name, splash_*)
 * @param {function} onDone    - Called once the exit animation has finished
 * @param {number}   replayKey - Changing it while mounted restarts the sequence
 * @param {boolean}  contained - Position inside the nearest positioned ancestor
 *                               (the dashboard phone frame) instead of the viewport
 */

/* A very short, one-off opening animation — no library involved. */
const ANIMATION_STYLE = `
@keyframes karecikSplashIn {
  from { opacity: 0; transform: scale(0.94); }
  to   { opacity: 1; transform: scale(1); }
}
@keyframes karecikSplashOutUp {
  from { opacity: 1; transform: translateY(0); }
  to   { opacity: 0; transform: translateY(-100%); }
}
@keyframes karecikSplashOutDown {
  from { opacity: 1; transform: translateY(0); }
  to   { opacity: 0; transform: translateY(100%); }
}
@keyframes karecikSplashOutLeft {
  from { opacity: 1; transform: translateX(0); }
  to   { opacity: 0; transform: translateX(-100%); }
}
@keyframes karecikSplashOutRight {
  from { opacity: 1; transform: translateX(0); }
  to   { opacity: 0; transform: translateX(100%); }
}
/* The same four slides without the fade: a solid curtain sliding away.
   Opacity is pinned so the panel cannot inherit a fade from anywhere. */
@keyframes karecikSplashSolidUp {
  from { opacity: 1; transform: translateY(0); }
  to   { opacity: 1; transform: translateY(-100%); }
}
@keyframes karecikSplashSolidDown {
  from { opacity: 1; transform: translateY(0); }
  to   { opacity: 1; transform: translateY(100%); }
}
@keyframes karecikSplashSolidLeft {
  from { opacity: 1; transform: translateX(0); }
  to   { opacity: 1; transform: translateX(-100%); }
}
@keyframes karecikSplashSolidRight {
  from { opacity: 1; transform: translateX(0); }
  to   { opacity: 1; transform: translateX(100%); }
}
@keyframes karecikSplashOutZoomIn {
  from { opacity: 1; transform: scale(1); }
  to   { opacity: 0; transform: scale(1.25); }
}
@keyframes karecikSplashOutZoomOut {
  from { opacity: 1; transform: scale(1); }
  to   { opacity: 0; transform: scale(0.85); }
}
@keyframes karecikSplashOutFade {
  from { opacity: 1; }
  to   { opacity: 0; }
}
/* Visitors who asked for less motion get the same timing without the movement. */
@media (prefers-reduced-motion: reduce) {
  .karecik-splash,
  .karecik-splash-content {
    animation: none !important;
  }
}
`

/** Exit keyframes per splash_exit_animation id (utils.SplashExitAnimations). */
const EXIT_KEYFRAMES = {
  'slide-up': 'karecikSplashOutUp',
  'slide-down': 'karecikSplashOutDown',
  'slide-left': 'karecikSplashOutLeft',
  'slide-right': 'karecikSplashOutRight',
  'zoom-in': 'karecikSplashOutZoomIn',
  'zoom-out': 'karecikSplashOutZoomOut',
  fade: 'karecikSplashOutFade',
}

/**
 * The solid-curtain variants of the four slide exits: identical movement, but
 * the panel keeps 100% opacity all the way out. Picked when the menu sets
 * splash_slide_fade to false; every non-slide animation ignores the flag.
 */
const SOLID_SLIDE_KEYFRAMES = {
  'slide-up': 'karecikSplashSolidUp',
  'slide-down': 'karecikSplashSolidDown',
  'slide-left': 'karecikSplashSolidLeft',
  'slide-right': 'karecikSplashSolidRight',
}

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

export default function SplashScreen({ business, onDone, replayKey = 0, contained = false }) {
  const background = business?.splash_bg_color || '#0f172a'
  const textColor = readableTextColor(background)
  const isDarkText = textColor === '#111827'

  const configured = Number(business?.splash_duration)
  const duration = Number.isFinite(configured) && configured > 0 ? Math.min(configured, 6000) : 1500

  const configuredExit = Number(business?.splash_exit_duration)
  const exitDuration =
    Number.isFinite(configuredExit) && configuredExit > 0
      ? Math.min(Math.max(configuredExit, 100), 2000)
      : 450

  // splash_slide_fade only ever applies to the four slide-* exits: false keeps
  // the panel fully opaque so it leaves like a curtain. A missing flag means
  // "fade", which mirrors the column default.
  const exitAnimation = business?.splash_exit_animation
  const solidSlide =
    business?.splash_slide_fade === false && Boolean(SOLID_SLIDE_KEYFRAMES[exitAnimation])

  const exitKeyframes =
    (solidSlide ? SOLID_SLIDE_KEYFRAMES[exitAnimation] : EXIT_KEYFRAMES[exitAnimation]) ||
    EXIT_KEYFRAMES.fade

  // The easing goes straight into the `animation` shorthand, where an unknown
  // keyword would invalidate the whole declaration — hence the catalogue check.
  const exitEasing = isValidSplashEasing(business?.splash_exit_easing)
    ? business.splash_exit_easing
    : DEFAULT_SPLASH_EXIT.easing

  // How the content ARRIVES, from splash_entrance (utils.SplashEntrances).
  // Only 'none' takes the entrance away; anything else — an unknown id, or a
  // payload from before the column existed — is 'fade', which is what this
  // screen has always done and what the column defaults to. The timing and the
  // scale below are the original ones and are deliberately not configurable.
  //
  // 'none' yields `undefined`, so the wrapper carries NO animation property at
  // all: not a zero-length animation and not a keyframe ending at opacity 1,
  // both of which the browser would still run and `replayKey` still restart.
  // That is exactly how MenuContent.jsx drops the header logo's fade when
  // `logo_fade_in` is false. The karecikSplashIn keyframes stay in the style
  // block either way — it is shared with the exit animation, so it is never
  // conditional the way the logo's own <style> is.
  const entranceAnimation =
    business?.splash_entrance === 'none' ? undefined : 'karecikSplashIn 320ms ease-out both'

  // 'logo' drops the text, 'text' drops the logo AND the initial-letter badge.
  const displayMode = SPLASH_DISPLAY_MODES.some((mode) => mode.id === business?.splash_display)
    ? business.splash_display
    : 'both'
  const showLogo = displayMode !== 'text'
  const showText = displayMode !== 'logo'

  const businessName = String(business?.name || '').trim()
  const configuredHeadline = String(business?.splash_headline || '').trim()
  const tagline = String(business?.splash_text || '').trim()

  // Both strings are optional and simply disappear when empty. In 'text' mode
  // there is no logo left to carry the screen, so the business name stands in.
  const headline =
    displayMode === 'text' && !configuredHeadline && !tagline ? businessName : configuredHeadline

  // The badge keeps following the business name when no headline is configured.
  const badgeSource = configuredHeadline || businessName
  const initial = badgeSource ? badgeSource.charAt(0).toLocaleUpperCase('tr') : '•'

  // The splash logo is optional; without it the regular business logo is used.
  const logoUrl = business?.splash_logo_url || business?.logo_url || ''

  // false while the screen holds, true once the exit animation is running.
  const [exiting, setExiting] = useState(false)

  // Keep the timers stable even when the prop identity changes on re-render.
  const onDoneRef = useRef(onDone)
  useEffect(() => {
    onDoneRef.current = onDone
  }, [onDone])

  // The dashboard "Replay" button bumps replayKey while the screen is mounted;
  // that rewinds the sequence back to the hold phase.
  useEffect(() => {
    setExiting(false)
  }, [replayKey])

  // 1) hold, then start the exit animation
  useEffect(() => {
    if (exiting) return undefined

    const timer = setTimeout(() => setExiting(true), duration)
    return () => clearTimeout(timer)
  }, [exiting, duration, replayKey])

  // 2) let the exit animation finish before handing the menu over
  useEffect(() => {
    if (!exiting) return undefined

    const timer = setTimeout(() => onDoneRef.current?.(), exitDuration)
    return () => clearTimeout(timer)
  }, [exiting, exitDuration])

  function skip() {
    setExiting(true)
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
      className={`karecik-splash z-50 flex cursor-pointer select-none items-center justify-center overflow-hidden px-6 ${
        contained ? 'absolute inset-0' : 'fixed inset-0'
      }`}
      style={{
        backgroundColor: background,
        color: textColor,
        animation: exiting ? `${exitKeyframes} ${exitDuration}ms ${exitEasing} both` : undefined,
      }}
    >
      <style>{ANIMATION_STYLE}</style>

      {/* Keyed on replayKey so the dashboard "Replay" button re-runs the entrance
          animation too, not just the hold/exit timers. With splash_entrance
          'none' there is nothing to re-run — the key simply remounts a wrapper
          that draws itself immediately. */}
      <div
        key={replayKey}
        className="karecik-splash-content flex flex-col items-center gap-4 text-center"
        style={{ animation: entranceAnimation }}
      >
        {/* Every child below is either rendered or `null`, so the gap-4 above
            never leaves a hole where a hidden element used to be. */}
        {showLogo && logoUrl ? (
          /* Wide logos are common — never crop them into a square. */
          <img
            src={logoUrl}
            alt=""
            className="h-auto max-h-24 w-auto max-w-[70%] object-contain"
          />
        ) : null}

        {showLogo && !logoUrl ? (
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
        ) : null}

        {showText && headline ? <h1 className="text-2xl font-semibold">{headline}</h1> : null}

        {showText && tagline ? (
          <p className="max-w-xs text-sm" style={{ opacity: 0.8 }}>
            {tagline}
          </p>
        ) : null}
      </div>
    </div>
  )
}
