// Fonts available for the customer menu.
// The ids mirror backend/internal/utils/appearance.go exactly.
//
// Fonts are not loaded on page start but on demand, when one is selected.

export const FONTS = [
  { id: 'inter', label: 'Inter', stack: "'Inter', system-ui, sans-serif", google: 'Inter:wght@400;500;600;700' },
  { id: 'poppins', label: 'Poppins', stack: "'Poppins', system-ui, sans-serif", google: 'Poppins:wght@400;500;600;700' },
  { id: 'montserrat', label: 'Montserrat', stack: "'Montserrat', system-ui, sans-serif", google: 'Montserrat:wght@400;500;600;700' },
  { id: 'nunito', label: 'Nunito', stack: "'Nunito', system-ui, sans-serif", google: 'Nunito:wght@400;600;700' },
  { id: 'playfair', label: 'Playfair Display', stack: "'Playfair Display', Georgia, serif", google: 'Playfair+Display:wght@400;600;700' },
  { id: 'lora', label: 'Lora', stack: "'Lora', Georgia, serif", google: 'Lora:wght@400;500;600' },
  { id: 'dm-serif', label: 'DM Serif Display', stack: "'DM Serif Display', Georgia, serif", google: 'DM+Serif+Display' },
  { id: 'space-grotesk', label: 'Space Grotesk', stack: "'Space Grotesk', system-ui, sans-serif", google: 'Space+Grotesk:wght@400;500;700' },
]

export const DEFAULT_FONT = FONTS[0]

/** Finds a font by id, falling back to Inter. */
export function findFont(id) {
  return FONTS.find((font) => font.id === id) || DEFAULT_FONT
}

/** CSS font-family value for a font id. */
export function fontStack(id) {
  return findFont(id).stack
}

const loaded = new Set(['inter']) // Inter is already linked from index.html

/**
 * Loads the selected font from Google Fonts exactly once, so that changing the
 * font in the dashboard updates the live preview immediately.
 */
export function loadFont(id) {
  const font = findFont(id)
  if (!font.google || loaded.has(font.id)) return

  const link = document.createElement('link')
  link.rel = 'stylesheet'
  link.href = `https://fonts.googleapis.com/css2?family=${font.google}&display=swap`
  document.head.appendChild(link)
  loaded.add(font.id)
}

/** Preloads every menu font (used by the design page previews). */
export function loadAllFonts() {
  FONTS.forEach((font) => loadFont(font.id))
}
