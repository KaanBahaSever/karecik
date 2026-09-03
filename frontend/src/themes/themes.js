// Customer menu themes.
// The ids mirror backend/internal/utils/appearance.go exactly.
//
// NOTE: labels and descriptions stay Turkish — they are shown in the dashboard.

export const THEMES = [
  {
    id: 'modern-light',
    label: 'Modern Açık',
    description: 'Beyaz zemin, mavi vurgu',
    dark: false,
    colors: {
      primary: '#1d4ed8',
      background: '#ffffff',
      surface: '#f6f7f8',
      text: '#111827',
      muted: '#6b7280',
      border: '#e5e7eb',
    },
    style: { radius: '0.875rem', cardShadow: '0 1px 3px rgb(0 0 0 / 0.07)' },
  },
  {
    id: 'modern-dark',
    label: 'Modern Koyu',
    description: 'Koyu zemin, gece kullanımı',
    dark: true,
    colors: {
      primary: '#60a5fa',
      background: '#0f172a',
      surface: '#1e293b',
      text: '#f1f5f9',
      muted: '#94a3b8',
      border: '#334155',
    },
    style: { radius: '0.875rem', cardShadow: '0 1px 3px rgb(0 0 0 / 0.4)' },
  },
  {
    id: 'zarif',
    label: 'Zarif',
    description: 'Krem zemin, serif başlıklar',
    dark: false,
    colors: {
      primary: '#8a6a3c',
      background: '#faf6ef',
      surface: '#f2ead9',
      text: '#3b2f22',
      muted: '#8a7a66',
      border: '#e3d7c1',
    },
    style: { radius: '0.25rem', cardShadow: 'none' },
  },
  {
    id: 'sicak',
    label: 'Sıcak',
    description: 'Kiremit tonları, samimi',
    dark: false,
    colors: {
      primary: '#c2410c',
      background: '#fffbf6',
      surface: '#ffedd5',
      text: '#431407',
      muted: '#9a6b52',
      border: '#fed7aa',
    },
    style: { radius: '1.25rem', cardShadow: '0 2px 8px rgb(194 65 12 / 0.08)' },
  },
  {
    id: 'minimal',
    label: 'Minimal',
    description: 'Siyah-beyaz, sade tipografi',
    dark: false,
    colors: {
      primary: '#111111',
      background: '#ffffff',
      surface: '#fafafa',
      text: '#111111',
      muted: '#737373',
      border: '#e5e5e5',
    },
    style: { radius: '0rem', cardShadow: 'none' },
  },
  {
    id: 'canli',
    label: 'Canlı',
    description: 'Mor vurgu, yüksek kontrast',
    dark: false,
    colors: {
      primary: '#7c3aed',
      background: '#ffffff',
      surface: '#f5f3ff',
      text: '#1e1b4b',
      muted: '#6d6a8a',
      border: '#ddd6fe',
    },
    style: { radius: '1rem', cardShadow: '0 2px 10px rgb(124 58 237 / 0.10)' },
  },
]

export const DEFAULT_THEME = THEMES[0]

/** Finds a theme by id, falling back to the default one. */
export function findTheme(id) {
  return THEMES.find((theme) => theme.id === id) || DEFAULT_THEME
}

/**
 * Turns a theme into CSS custom properties. The result is applied as the
 * `style` of the menu container, and every component inside reads the values
 * through var(--menu-*).
 *
 * @param {object|string} theme       - Theme object or its id
 * @param {string}        accentColor - The business' own primary_color; when set
 *                                      it overrides the theme's primary tone
 * @param {string}        fontStack   - CSS font-family value
 */
export function themeVariables(theme, accentColor, fontStack) {
  const resolved = typeof theme === 'string' ? findTheme(theme) : theme || DEFAULT_THEME
  const primary = accentColor || resolved.colors.primary

  return {
    '--menu-primary': primary,
    '--menu-bg': resolved.colors.background,
    '--menu-surface': resolved.colors.surface,
    '--menu-text': resolved.colors.text,
    '--menu-muted': resolved.colors.muted,
    '--menu-border': resolved.colors.border,
    '--menu-radius': resolved.style.radius,
    '--menu-shadow': resolved.style.cardShadow,
    '--menu-font': fontStack || "'Inter', system-ui, sans-serif",
    backgroundColor: resolved.colors.background,
    color: resolved.colors.text,
    fontFamily: fontStack || "'Inter', system-ui, sans-serif",
  }
}

/* ------------------------------------------------------------- background */

const HEX_COLOR = /^#[0-9a-fA-F]{6}$/

/** Turns "#1d4ed8" into "rgb(29 78 216 / 0.4)". */
function hexToRgba(hex, alpha) {
  const red = parseInt(hex.slice(1, 3), 16)
  const green = parseInt(hex.slice(3, 5), 16)
  const blue = parseInt(hex.slice(5, 7), 16)
  return `rgb(${red} ${green} ${blue} / ${alpha})`
}

/**
 * Background layer styles for the customer menu.
 *
 * The business may either tint the theme background with its own colour or put
 * a photo behind the whole menu. A photo needs a scrim on top of it, otherwise
 * the card text stops being readable — hence the second style object.
 *
 * @param {object|string} theme    - Theme object or its id
 * @param {object}        business - Carries background_type, background_color,
 *                                   background_image_url and
 *                                   background_overlay_opacity
 * @returns {{containerStyle: object, overlayStyle: object|null}}
 */
export function backgroundStyles(theme, business) {
  const resolved = typeof theme === 'string' ? findTheme(theme) : theme || DEFAULT_THEME
  const source = business || {}
  const imageUrl = (source.background_image_url || '').trim()

  if (source.background_type === 'image' && imageUrl) {
    const raw = Number(source.background_overlay_opacity)
    const opacity = Number.isFinite(raw) ? Math.min(1, Math.max(0, raw)) : 0.4

    return {
      containerStyle: {
        backgroundImage: `url("${imageUrl}")`,
        backgroundSize: 'cover',
        backgroundPosition: 'center',
        // `fixed` janks on iOS Safari while the menu scrolls.
        backgroundAttachment: 'scroll',
      },
      overlayStyle: {
        position: 'absolute',
        inset: 0,
        backgroundColor: hexToRgba(resolved.colors.background, opacity),
        backdropFilter: 'blur(2px)',
        pointerEvents: 'none',
      },
    }
  }

  const color = (source.background_color || '').trim()

  return {
    containerStyle: {
      backgroundColor: HEX_COLOR.test(color) ? color : resolved.colors.background,
    },
    overlayStyle: null,
  }
}
