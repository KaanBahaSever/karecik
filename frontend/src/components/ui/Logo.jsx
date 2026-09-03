/**
 * The Karecik brand mark — the single source of truth for the logo in the
 * interface.
 *
 * The same artwork also ships as `public/logo.svg`, which is what the browser
 * tab icon and the README point at. This component inlines the identical
 * geometry so the mark needs no network request and can be recoloured with a
 * Tailwind text colour.
 *
 * The plate and the small "finder" dots use `currentColor`, the squares stay
 * white. Setting a text colour therefore restyles the whole mark in one place:
 *
 *     <Logo className="h-9 w-9 text-brand-600" />   dashboard, forms, landing
 *     <Logo className="h-5 w-5 text-white" />       on a dark surface
 *
 * The squares are knocked out in `knockoutColor`, white by default. Inside the
 * customer menu, where the surface colour is whatever theme the business picked,
 * pass the theme background instead so the mark stays legible on all six themes:
 *
 *     <Logo knockoutColor="var(--menu-bg)" />
 *
 * The viewBox carries no width/height attributes, so the mark scales purely
 * from its className and never distorts.
 *
 * NOTE: this is *Karecik's* logo, not the tenant's. A business' own logo comes
 * from `business.logo_url` and must never fall back to this mark — a café's
 * menu may not be branded as Karecik.
 */
export default function Logo({
  className = 'h-8 w-8 text-brand-600',
  title = 'Karecik',
  knockoutColor = '#ffffff',
}) {
  return (
    <svg
      viewBox="0 0 32 32"
      className={className}
      role={title ? 'img' : undefined}
      aria-label={title || undefined}
      aria-hidden={title ? undefined : 'true'}
      focusable="false"
    >
      {title ? <title>{title}</title> : null}

      {/* rounded plate */}
      <rect width="32" height="32" rx="7" fill="currentColor" />

      {/* three QR finder squares plus the two accent blocks */}
      <g fill={knockoutColor}>
        <rect x="6" y="6" width="8" height="8" rx="1.5" />
        <rect x="18" y="6" width="8" height="8" rx="1.5" />
        <rect x="6" y="18" width="8" height="8" rx="1.5" />
        <rect x="18" y="18" width="3.5" height="3.5" rx=".8" />
        <rect x="22.5" y="22.5" width="3.5" height="3.5" rx=".8" />
      </g>

      {/* the eyes punched back out of the finder squares */}
      <g fill="currentColor">
        <rect x="8.5" y="8.5" width="3" height="3" rx=".6" />
        <rect x="20.5" y="8.5" width="3" height="3" rx=".6" />
        <rect x="8.5" y="20.5" width="3" height="3" rx=".6" />
      </g>
    </svg>
  )
}

/**
 * Mark plus the "Karecik" wordmark, used wherever the product introduces
 * itself: the login and sign-up forms, the landing header, the dashboard rail.
 *
 * @param {string} className     - Wrapper classes (layout only)
 * @param {string} markClassName - Size and colour of the mark
 * @param {string} wordClassName - Type styles of the wordmark
 */
export function BrandLockup({
  className = 'inline-flex items-center gap-2.5',
  markClassName = 'h-9 w-9 shrink-0 text-brand-600',
  wordClassName = 'text-xl font-bold tracking-tight text-gray-900',
}) {
  return (
    <span className={className}>
      <Logo className={markClassName} title="" />
      <span className={wordClassName}>Karecik</span>
    </span>
  )
}
