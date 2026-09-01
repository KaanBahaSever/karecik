// Landing page and top bar copy.
//
// The language picker in the header reads from this dictionary and stores the
// choice in localStorage. The dashboard and the customer menu use their own
// dictionaries (see locales/index.js).

export const LANDING_LANGUAGES = [
  { code: 'tr', short: 'TR', name: 'Türkçe' },
  { code: 'en', short: 'EN', name: 'English' },
  { code: 'de', short: 'DE', name: 'Deutsch' },
]

export const DEFAULT_LANDING_LANGUAGE = 'tr'

const STRINGS = {
  tr: {
    brandHome: 'Karecik ana sayfa',
    selectLanguage: 'Dil seçin',
    signIn: 'Giriş Yap',
    startFree: 'Hemen Ücretsiz Başla',

    heading: 'İşletmeniz için QR menü servisi',
    description: 'QR menü hizmetimizle satışlarınızı kolay ve pratik bir şekilde dijitalleştirin.',

    badgeFree: 'Tamamen ücretsiz',
    badgeSetup: 'Hızlı kurulum',
    badgeMobile: 'Mobil uyum',

    demoTitle: 'Örnek kafe menüsü',

    copyright: '© 2026 Karecik',
    footerPlatform: 'QR menü platformu',
    footerSetup: 'Kurulum ücreti yok',
    footerCard: 'Kredi kartı gerekmez',
  },

  en: {
    brandHome: 'Karecik home page',
    selectLanguage: 'Select language',
    signIn: 'Sign In',
    startFree: 'Start Free Now',

    heading: 'A QR menu service for your business',
    description: 'Digitise your sales easily and practically with our QR menu service.',

    badgeFree: 'Completely free',
    badgeSetup: 'Quick setup',
    badgeMobile: 'Mobile friendly',

    demoTitle: 'Sample cafe menu',

    copyright: '© 2026 Karecik',
    footerPlatform: 'QR menu platform',
    footerSetup: 'No setup fee',
    footerCard: 'No credit card required',
  },

  de: {
    brandHome: 'Karecik Startseite',
    selectLanguage: 'Sprache wählen',
    signIn: 'Anmelden',
    startFree: 'Jetzt kostenlos starten',

    heading: 'QR-Menü-Service für Ihr Unternehmen',
    description: 'Digitalisieren Sie Ihren Verkauf einfach und praktisch mit unserem QR-Menü-Service.',

    badgeFree: 'Völlig kostenlos',
    badgeSetup: 'Schnelle Einrichtung',
    badgeMobile: 'Mobil optimiert',

    demoTitle: 'Beispiel-Cafémenü',

    copyright: '© 2026 Karecik',
    footerPlatform: 'QR-Menü-Plattform',
    footerSetup: 'Keine Einrichtungsgebühr',
    footerCard: 'Keine Kreditkarte nötig',
  },
}

/** Returns the string dictionary for a language, falling back to Turkish. */
export function landingText(language) {
  return STRINGS[language] || STRINGS[DEFAULT_LANDING_LANGUAGE]
}

const STORAGE_KEY = 'karecik_landing_language'

/** Reads the visitor's previous language choice. */
export function readSavedLanguage() {
  try {
    const value = localStorage.getItem(STORAGE_KEY)
    return LANDING_LANGUAGES.some((language) => language.code === value)
      ? value
      : DEFAULT_LANDING_LANGUAGE
  } catch {
    return DEFAULT_LANDING_LANGUAGE
  }
}

/** Stores the language choice (silently ignored in private windows). */
export function saveLanguage(language) {
  try {
    localStorage.setItem(STORAGE_KEY, language)
  } catch {
    /* localStorage may be disabled */
  }
}
