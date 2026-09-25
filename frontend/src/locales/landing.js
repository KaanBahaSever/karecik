// Landing page, top bar and sign-up dialog copy.
//
// The language picker in the header reads from this dictionary and stores the
// choice in localStorage. The dashboard and the customer menu use their own
// dictionaries (see locales/index.js).
//
// EVERY visible or announced string of pages/Landing.jsx and components/landing
// lives here — labels, placeholders, aria-labels, alt texts, validation and
// request errors alike. A string left in the JSX stays Turkish whatever the
// visitor picked, which is exactly the half-translated page this file exists to
// prevent. tests/landingLocales.test.mjs keeps the three dictionaries in step
// and fails on a key the components use but a language lacks.
//
// "Karecik" is the product's name and is never translated.

export const LANDING_LANGUAGES = [
  { code: 'tr', short: 'TR', name: 'Türkçe' },
  { code: 'en', short: 'EN', name: 'English' },
  { code: 'de', short: 'DE', name: 'Deutsch' },
]

export const DEFAULT_LANDING_LANGUAGE = 'tr'

export const LANDING_STRINGS = {
  tr: {
    // The browser tab while the landing page is open. Matches index.html,
    // which is what the tab shows before the app has booted.
    documentTitle: 'Karecik — İşletmeniz için QR menü',

    brandHome: 'Karecik ana sayfa',
    selectLanguage: 'Dil seçin',
    signIn: 'Giriş Yap',
    startFree: 'Hemen Ücretsiz Başla',
    // Short form for the phone navbar, where the full label does not fit.
    startFreeShort: 'Ücretsiz Başla',

    heading: 'İşletmeniz için QR menü servisi',
    description: 'QR menü hizmetimizle satışlarınızı kolay ve pratik bir şekilde dijitalleştirin.',

    badgeFree: 'Tamamen ücretsiz',
    badgeSetup: 'Hızlı kurulum',
    badgeMobile: 'Mobil uyum',

    demoTitle: 'Örnek kafe menüsü',
    // Shown inside the phone when no sample venue is deployed.
    demoPlaceholderTitle: 'Menünüz burada görünür',
    demoPlaceholderText: 'Kategoriler, ürünler, fiyatlar ve kendi logonuz — hepsi telefonda.',

    copyright: '© 2026 Karecik',
    footerPlatform: 'QR menü platformu',
    footerSetup: 'Kurulum ücreti yok',
    footerCard: 'Kredi kartı gerekmez',

    // Sign-up dialog
    close: 'Kapat',
    dismissNotification: 'Bildirimi kapat',
    signUpTitle: 'Ücretsiz hesabınızı oluşturun',
    signUpDescription: 'Birkaç saniyede QR menünüzü hazırlamaya başlayın.',
    signUpBusinessName: 'İşletme Adı',
    signUpBusinessNamePlaceholder: 'Örn. Kahve Durağı',
    signUpEmail: 'E-posta',
    signUpEmailPlaceholder: 'ornek@isletmem.com',
    signUpPassword: 'Şifre',
    signUpPasswordPlaceholder: 'En az 8 karakter',
    signUpPasswordHelp: 'Şifreniz en az 8 karakter olmalı.',
    signUpShowPassword: 'Şifreyi göster',
    signUpHidePassword: 'Şifreyi gizle',
    signUpSubmit: 'Hesabımı oluştur',
    signUpSubmitting: 'Hesap oluşturuluyor...',
    signUpHaveAccount: 'Zaten hesabınız var mı?',
    signUpSignInLink: 'Giriş yapın',
    signUpWelcome: 'Hoş geldiniz! Menünüzü oluşturmaya başlayabilirsiniz.',

    // Sign-up validation (checked in the browser before anything is sent)
    signUpErrorBusinessNameRequired: 'İşletme adını girin.',
    signUpErrorBusinessNameShort: 'İşletme adı en az 2 karakter olmalı.',
    signUpErrorBusinessNameLong: 'İşletme adı en fazla 100 karakter olabilir.',
    signUpErrorEmailRequired: 'E-posta adresinizi girin.',
    signUpErrorEmailInvalid: 'Geçerli bir e-posta adresi girin.',
    signUpErrorPasswordRequired: 'Bir şifre belirleyin.',
    signUpErrorPasswordShort: 'Şifre en az 8 karakter olmalı.',
    signUpErrorPasswordLong: 'Şifre çok uzun: en fazla 72 bayt olabilir, Türkçe karakterler 2 bayt sayılır.',

    // Sign-up request failures (see signUpErrorMessage below)
    signUpErrorEmailTaken: 'Bu e-posta adresi zaten kayıtlı. Giriş yapmayı deneyin.',
    signUpErrorNetwork: 'Sunucuya ulaşılamadı. İnternet bağlantınızı kontrol edip tekrar deneyin.',
    signUpErrorRateLimited: 'Çok fazla deneme yapıldı. Biraz bekleyip tekrar deneyin.',
    signUpErrorServer: 'Sunucuda beklenmeyen bir hata oluştu. Lütfen tekrar deneyin.',
    signUpErrorGeneric: 'Hesabınız oluşturulamadı. Bilgilerinizi kontrol edip tekrar deneyin.',
  },

  en: {
    documentTitle: 'Karecik — QR menu for your business',

    brandHome: 'Karecik home page',
    selectLanguage: 'Select language',
    signIn: 'Sign In',
    startFree: 'Start Free Now',
    startFreeShort: 'Start Free',

    heading: 'A QR menu service for your business',
    description: 'Digitise your sales easily and practically with our QR menu service.',

    badgeFree: 'Completely free',
    badgeSetup: 'Quick setup',
    badgeMobile: 'Mobile friendly',

    demoTitle: 'Sample cafe menu',
    demoPlaceholderTitle: 'Your menu appears here',
    demoPlaceholderText: 'Categories, products, prices and your own logo — all on the phone.',

    copyright: '© 2026 Karecik',
    footerPlatform: 'QR menu platform',
    footerSetup: 'No setup fee',
    footerCard: 'No credit card required',

    close: 'Close',
    dismissNotification: 'Dismiss notification',
    signUpTitle: 'Create your free account',
    signUpDescription: 'Start building your QR menu in seconds.',
    signUpBusinessName: 'Business name',
    signUpBusinessNamePlaceholder: 'e.g. Corner Coffee',
    signUpEmail: 'Email',
    signUpEmailPlaceholder: 'you@yourbusiness.com',
    signUpPassword: 'Password',
    signUpPasswordPlaceholder: 'At least 8 characters',
    signUpPasswordHelp: 'Your password must be at least 8 characters long.',
    signUpShowPassword: 'Show password',
    signUpHidePassword: 'Hide password',
    signUpSubmit: 'Create my account',
    signUpSubmitting: 'Creating your account...',
    signUpHaveAccount: 'Already have an account?',
    signUpSignInLink: 'Sign in',
    signUpWelcome: 'Welcome! You can start building your menu now.',

    signUpErrorBusinessNameRequired: 'Enter your business name.',
    signUpErrorBusinessNameShort: 'The business name must be at least 2 characters long.',
    signUpErrorBusinessNameLong: 'The business name can be at most 100 characters long.',
    signUpErrorEmailRequired: 'Enter your email address.',
    signUpErrorEmailInvalid: 'Enter a valid email address.',
    signUpErrorPasswordRequired: 'Choose a password.',
    signUpErrorPasswordShort: 'The password must be at least 8 characters long.',
    signUpErrorPasswordLong: 'The password is too long: 72 bytes at most, and letters such as ü or ş count as 2.',

    signUpErrorEmailTaken: 'This email address is already registered. Try signing in.',
    signUpErrorNetwork: 'Could not reach the server. Check your internet connection and try again.',
    signUpErrorRateLimited: 'Too many attempts. Please wait a moment and try again.',
    signUpErrorServer: 'Something went wrong on our side. Please try again.',
    signUpErrorGeneric: 'Your account could not be created. Please check your details and try again.',
  },

  de: {
    documentTitle: 'Karecik — QR-Menü für Ihr Unternehmen',

    brandHome: 'Karecik Startseite',
    selectLanguage: 'Sprache wählen',
    signIn: 'Anmelden',
    startFree: 'Jetzt kostenlos starten',
    startFreeShort: 'Loslegen',

    heading: 'QR-Menü-Service für Ihr Unternehmen',
    description: 'Digitalisieren Sie Ihren Verkauf einfach und praktisch mit unserem QR-Menü-Service.',

    badgeFree: 'Völlig kostenlos',
    badgeSetup: 'Schnelle Einrichtung',
    badgeMobile: 'Mobil optimiert',

    demoTitle: 'Beispiel-Cafémenü',
    demoPlaceholderTitle: 'Hier erscheint Ihr Menü',
    demoPlaceholderText: 'Kategorien, Produkte, Preise und Ihr eigenes Logo – alles auf dem Smartphone.',

    copyright: '© 2026 Karecik',
    footerPlatform: 'QR-Menü-Plattform',
    footerSetup: 'Keine Einrichtungsgebühr',
    footerCard: 'Keine Kreditkarte nötig',

    close: 'Schließen',
    dismissNotification: 'Benachrichtigung schließen',
    signUpTitle: 'Kostenloses Konto erstellen',
    signUpDescription: 'Starten Sie Ihr QR-Menü in wenigen Sekunden.',
    signUpBusinessName: 'Name des Betriebs',
    signUpBusinessNamePlaceholder: 'z. B. Café am Markt',
    signUpEmail: 'E-Mail',
    signUpEmailPlaceholder: 'name@ihr-betrieb.de',
    signUpPassword: 'Passwort',
    signUpPasswordPlaceholder: 'Mindestens 8 Zeichen',
    signUpPasswordHelp: 'Ihr Passwort muss mindestens 8 Zeichen lang sein.',
    signUpShowPassword: 'Passwort anzeigen',
    signUpHidePassword: 'Passwort verbergen',
    signUpSubmit: 'Konto erstellen',
    signUpSubmitting: 'Konto wird erstellt...',
    signUpHaveAccount: 'Sie haben bereits ein Konto?',
    signUpSignInLink: 'Anmelden',
    signUpWelcome: 'Willkommen! Sie können jetzt mit Ihrem Menü beginnen.',

    signUpErrorBusinessNameRequired: 'Geben Sie den Namen Ihres Betriebs ein.',
    signUpErrorBusinessNameShort: 'Der Name muss mindestens 2 Zeichen lang sein.',
    signUpErrorBusinessNameLong: 'Der Name darf höchstens 100 Zeichen lang sein.',
    signUpErrorEmailRequired: 'Geben Sie Ihre E-Mail-Adresse ein.',
    signUpErrorEmailInvalid: 'Geben Sie eine gültige E-Mail-Adresse ein.',
    signUpErrorPasswordRequired: 'Legen Sie ein Passwort fest.',
    signUpErrorPasswordShort: 'Das Passwort muss mindestens 8 Zeichen lang sein.',
    signUpErrorPasswordLong: 'Das Passwort ist zu lang: erlaubt sind höchstens 72 Byte, Umlaute wie ä oder ü zählen als 2 Byte.',

    signUpErrorEmailTaken: 'Diese E-Mail-Adresse ist bereits registriert. Versuchen Sie, sich anzumelden.',
    signUpErrorNetwork: 'Der Server ist nicht erreichbar. Prüfen Sie Ihre Internetverbindung und versuchen Sie es erneut.',
    signUpErrorRateLimited: 'Zu viele Versuche. Bitte warten Sie einen Moment und versuchen Sie es erneut.',
    signUpErrorServer: 'Auf unserer Seite ist ein Fehler aufgetreten. Bitte versuchen Sie es erneut.',
    signUpErrorGeneric: 'Ihr Konto konnte nicht erstellt werden. Bitte prüfen Sie Ihre Angaben und versuchen Sie es erneut.',
  },
}

/** Returns the string dictionary for a language, falling back to Turkish. */
export function landingText(language) {
  return LANDING_STRINGS[language] || LANDING_STRINGS[DEFAULT_LANDING_LANGUAGE]
}

/* ------------------------------------------------------------ sign-up rules */

/* The limits the server enforces on POST /api/auth/register (handlers/auth.go).
   The dialog checks them before sending, so every refusal a visitor can
   realistically run into is worded in their language rather than in the
   server's Turkish. The numbers are spelled out in the messages above; change
   both together. */
const BUSINESS_NAME_MIN = 2
// Characters (code points), the way the server counts them: len([]rune(name)).
const BUSINESS_NAME_MAX = 100
const PASSWORD_MIN = 8
// Bytes, not characters: bcrypt stops at 72 bytes, and "ş" or "ü" take two.
const PASSWORD_MAX_BYTES = 72

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/

/**
 * Client-side check of the sign-up form.
 *
 * Returns DICTIONARY KEYS, not sentences: the dialog looks them up while it
 * renders, so a message always reads in the language on screen now rather than
 * the one that was on screen when the button was pressed.
 *
 * @returns {{businessName?: string, email?: string, password?: string}}
 *          field -> key; empty when the form may be sent
 */
export function validateSignUp({ businessName = '', email = '', password = '' } = {}) {
  const found = {}

  const name = String(businessName).trim()
  const nameLength = [...name].length
  if (!name) {
    found.businessName = 'signUpErrorBusinessNameRequired'
  } else if (nameLength < BUSINESS_NAME_MIN) {
    found.businessName = 'signUpErrorBusinessNameShort'
  } else if (nameLength > BUSINESS_NAME_MAX) {
    found.businessName = 'signUpErrorBusinessNameLong'
  }

  const address = String(email).trim()
  if (!address) {
    found.email = 'signUpErrorEmailRequired'
  } else if (!EMAIL_PATTERN.test(address)) {
    found.email = 'signUpErrorEmailInvalid'
  }

  const secret = String(password)
  if (!secret) {
    found.password = 'signUpErrorPasswordRequired'
  } else if (secret.length < PASSWORD_MIN) {
    found.password = 'signUpErrorPasswordShort'
  } else if (new TextEncoder().encode(secret).length > PASSWORD_MAX_BYTES) {
    found.password = 'signUpErrorPasswordLong'
  }

  return found
}

/**
 * Which dictionary key describes a failed sign-up request.
 *
 * The server words its errors in Turkish (utils/response.go), and api.js hands
 * that sentence over as error.message — so showing it verbatim left an English
 * or German visitor with a Turkish error in an otherwise translated dialog.
 * The status and code are language-free, and they are what is read here.
 *
 * @param {Error & {status?: number, code?: string}} error - An ApiError, or anything thrown
 * @returns {string} A key of the landing dictionary
 */
export function signUpErrorKey(error) {
  const status = error?.status
  // api.js reports an unreachable server as status 0 / NETWORK_ERROR. Its own
  // message is written for a developer ("is the backend running?"), not for a
  // café owner signing up.
  if (error?.code === 'NETWORK_ERROR' || status === 0) return 'signUpErrorNetwork'
  if (status === 409) return 'signUpErrorEmailTaken'
  if (status === 429) return 'signUpErrorRateLimited'
  if (typeof status === 'number' && status >= 500) return 'signUpErrorServer'
  return 'signUpErrorGeneric'
}

/**
 * The sentence to show for a failed sign-up request, in the visitor's language.
 *
 * One exception keeps a detail the dictionary cannot: for a 4xx the dialog did
 * not foresee (a 422 on a rule the browser does not check), a Turkish visitor
 * gets the server's own sentence, which is Turkish already and names the field.
 * Only a real HTTP answer qualifies — a JavaScript error's message is English
 * developer text and never reaches the screen.
 */
export function signUpErrorMessage(error, language) {
  const key = signUpErrorKey(error)
  const status = error?.status
  const fromServer =
    typeof status === 'number' && status >= 400 && status < 500 && typeof error?.message === 'string'
  if (key === 'signUpErrorGeneric' && language === 'tr' && fromServer && error.message.trim()) {
    return error.message
  }
  return landingText(language)[key]
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
