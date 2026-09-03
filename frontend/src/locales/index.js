// Menu languages, allergen labels and customer-facing interface strings.
// The codes mirror backend/internal/utils/appearance.go exactly.
//
// NOTE: every label and string below is user-facing, so the Turkish and
// English wording is data — not something to translate at the code level.

// We deliberately avoid flag emoji (🇬🇧) in the interface: Windows cannot render
// them and prints the country code instead, which made English show up as "GB".
// Each language therefore carries its own short code (TR / EN / DE), which looks
// identical and correct on every operating system.
export const LANGUAGES = [
  { code: 'tr', short: 'TR', label: 'Türkçe', flag: '🇹🇷' },
  { code: 'en', short: 'EN', label: 'English', flag: '🇬🇧' },
  { code: 'de', short: 'DE', label: 'Deutsch', flag: '🇩🇪' },
  { code: 'ru', short: 'RU', label: 'Русский', flag: '🇷🇺' },
  { code: 'ar', short: 'AR', label: 'العربية', flag: '🇸🇦' },
  { code: 'fr', short: 'FR', label: 'Français', flag: '🇫🇷' },
]

export function findLanguage(code) {
  return (
    LANGUAGES.find((language) => language.code === code) || {
      code,
      short: String(code || '').toUpperCase(),
      label: code,
      flag: '',
    }
  )
}

/** Short code shown in the interface: "tr" -> "TR", "en" -> "EN". */
export function languageShort(code) {
  return findLanguage(code).short
}

/** Languages written right to left. */
export function isRtl(code) {
  return code === 'ar'
}

/* -------------------------------------------------------------- allergens */

export const ALLERGENS = [
  { code: 'gluten', emoji: '🌾', tr: 'Gluten', en: 'Gluten' },
  { code: 'sut', emoji: '🥛', tr: 'Süt', en: 'Milk' },
  { code: 'yumurta', emoji: '🥚', tr: 'Yumurta', en: 'Egg' },
  { code: 'findik', emoji: '🌰', tr: 'Fındık / Kuruyemiş', en: 'Nuts' },
  { code: 'yer_fistigi', emoji: '🥜', tr: 'Yer Fıstığı', en: 'Peanuts' },
  { code: 'soya', emoji: '🫘', tr: 'Soya', en: 'Soy' },
  { code: 'balik', emoji: '🐟', tr: 'Balık', en: 'Fish' },
  { code: 'kabuklu_deniz', emoji: '🦐', tr: 'Kabuklu Deniz Ürünü', en: 'Shellfish' },
  { code: 'susam', emoji: '🌱', tr: 'Susam', en: 'Sesame' },
  { code: 'hardal', emoji: '🟡', tr: 'Hardal', en: 'Mustard' },
  { code: 'kereviz', emoji: '🥬', tr: 'Kereviz', en: 'Celery' },
  { code: 'sulfit', emoji: '🧪', tr: 'Sülfit', en: 'Sulphites' },
  { code: 'aci', emoji: '🌶️', tr: 'Acı', en: 'Spicy' },
  { code: 'vejetaryen', emoji: '🥗', tr: 'Vejetaryen', en: 'Vegetarian' },
  { code: 'vegan', emoji: '🌿', tr: 'Vegan', en: 'Vegan' },
  { code: 'alkol', emoji: '🍷', tr: 'Alkol İçerir', en: 'Contains Alcohol' },
  { code: 'kafein', emoji: '☕', tr: 'Kafein', en: 'Caffeine' },
]

export function findAllergen(code) {
  return ALLERGENS.find((allergen) => allergen.code === code) || null
}

export function allergenLabel(code, language = 'tr') {
  const allergen = findAllergen(code)
  if (!allergen) return code
  return language === 'tr' ? allergen.tr : allergen.en
}

/* ------------------------------------------- customer menu interface copy */

const STRINGS = {
  tr: {
    menu: 'Menü',
    search: 'Menüde ara...',
    noResults: 'Aramanızla eşleşen ürün bulunamadı.',
    emptyCategory: 'Bu kategoride henüz ürün yok.',
    emptyMenu: 'Menü hazırlanıyor.',
    all: 'Tümü',
    featured: 'Öne çıkan',
    ingredients: 'İçindekiler',
    allergens: 'Alerjen bilgisi',
    calories: 'kalori',
    kcal: 'kcal',
    close: 'Kapat',
    wifi: 'Wi-Fi şifresi',
    wifiName: 'Wi-Fi ağı',
    wifiPassword: 'Wi-Fi şifresi',
    copy: 'Kopyala',
    copied: 'Kopyalandı',
    copyFailed: 'Kopyalanamadı',
    address: 'Adres',
    phone: 'Telefon',
    branch: 'Şube',
    menuLabel: 'Menü',
    poweredBy: 'Karecik ile hazırlandı',
    loading: 'Menü yükleniyor...',
    notFound: 'Menü bulunamadı',
    notFoundDetail: 'Bu adrese ait bir menü yok. Adresi kontrol edin.',
  },
  en: {
    menu: 'Menu',
    search: 'Search the menu...',
    noResults: 'No products matched your search.',
    emptyCategory: 'No products in this category yet.',
    emptyMenu: 'The menu is being prepared.',
    all: 'All',
    featured: 'Featured',
    ingredients: 'Ingredients',
    allergens: 'Allergen information',
    calories: 'calories',
    kcal: 'kcal',
    close: 'Close',
    wifi: 'Wi-Fi password',
    wifiName: 'Wi-Fi network',
    wifiPassword: 'Wi-Fi password',
    copy: 'Copy',
    copied: 'Copied',
    copyFailed: 'Could not copy',
    address: 'Address',
    phone: 'Phone',
    branch: 'Branch',
    menuLabel: 'Menu',
    poweredBy: 'Made with Karecik',
    loading: 'Loading menu...',
    notFound: 'Menu not found',
    notFoundDetail: 'There is no menu at this address. Please check the URL.',
  },
}

/** Customer menu interface string. Unknown languages fall back to English. */
export function t(key, language = 'tr') {
  const dictionary = STRINGS[language] || STRINGS.en
  return dictionary[key] ?? STRINGS.tr[key] ?? key
}
