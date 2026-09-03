// Splash screen and header catalogues.
// The ids and their order mirror backend/internal/utils/appearance.go
// (utils.SplashExitAnimations, utils.SplashEasings, utils.SplashDisplayModes,
// utils.HeaderDisplayModes) exactly — and therefore the CHECK constraints on
// the businesses table.
//
// NOTE: labels stay Turkish — they are shown in the dashboard pickers.

/* ------------------------------------------------------------ animations */

/** How the splash screen leaves the screen. */
export const SPLASH_EXIT_ANIMATIONS = [
  { id: 'fade', label: 'Yumuşak geçiş' },
  { id: 'slide-up', label: 'Yukarı kayar' },
  { id: 'slide-down', label: 'Aşağı kayar' },
  { id: 'slide-left', label: 'Sola kayar' },
  { id: 'slide-right', label: 'Sağa kayar' },
  { id: 'zoom-in', label: 'Yakınlaşarak kaybolur' },
  { id: 'zoom-out', label: 'Uzaklaşarak kaybolur' },
]

/** The CSS timing function of the exit animation. */
export const SPLASH_EASINGS = [
  { id: 'ease-in', label: 'Yavaş başla' },
  { id: 'ease-out', label: 'Yavaş bitir' },
  { id: 'ease-in-out', label: 'Yavaş başla ve bitir' },
  { id: 'ease', label: 'Varsayılan' },
  { id: 'linear', label: 'Sabit hız' },
]

/* --------------------------------------------------------- display modes */

/** What the splash screen shows. */
export const SPLASH_DISPLAY_MODES = [
  { id: 'both', label: 'Logo ve yazı' },
  { id: 'logo', label: 'Sadece logo' },
  { id: 'text', label: 'Sadece yazı' },
]

/** What the customer menu header shows. */
export const HEADER_DISPLAY_MODES = [
  { id: 'both', label: 'Logo ve işletme adı' },
  { id: 'logo', label: 'Sadece logo' },
  { id: 'name', label: 'Sadece işletme adı' },
]

/* -------------------------------------------------------------- defaults */

/** Mirrors the businesses table defaults for the splash exit transition. */
export const DEFAULT_SPLASH_EXIT = {
  animation: 'fade',
  easing: 'ease-in',
  duration: 450,
}

/* ------------------------------------------------------------ validation */

/** Reports whether a splash exit animation id exists in the catalogue. */
export function isValidSplashAnimation(id) {
  return SPLASH_EXIT_ANIMATIONS.some((animation) => animation.id === id)
}

/** Reports whether a splash easing id exists in the catalogue. */
export function isValidSplashEasing(id) {
  return SPLASH_EASINGS.some((easing) => easing.id === id)
}
