// Menü dilleri, alerjen etiketleri ve müşteri tarafı arayüz metinleri.
// Kodlar backend'deki internal/utils/appearance.go ile birebir aynıdır.

export const DILLER = [
  { code: 'tr', label: 'Türkçe', flag: '🇹🇷' },
  { code: 'en', label: 'English', flag: '🇬🇧' },
  { code: 'de', label: 'Deutsch', flag: '🇩🇪' },
  { code: 'ru', label: 'Русский', flag: '🇷🇺' },
  { code: 'ar', label: 'العربية', flag: '🇸🇦' },
  { code: 'fr', label: 'Français', flag: '🇫🇷' },
]

export function dilBul(code) {
  return DILLER.find((dil) => dil.code === code) || DILLER[0]
}

/** Sağdan sola yazılan diller. */
export function sagdanSolaMi(code) {
  return code === 'ar'
}

/* ------------------------------------------------------------- alerjenler */

export const ALERJENLER = [
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

export function alerjenBul(code) {
  return ALERJENLER.find((a) => a.code === code) || null
}

export function alerjenEtiketi(code, dil = 'tr') {
  const alerjen = alerjenBul(code)
  if (!alerjen) return code
  return dil === 'tr' ? alerjen.tr : alerjen.en
}

/* ------------------------------------------------- müşteri menüsü metinleri */

const METINLER = {
  tr: {
    menu: 'Menü',
    ara: 'Menüde ara...',
    sonucYok: 'Aramanızla eşleşen ürün bulunamadı.',
    bosKategori: 'Bu kategoride henüz ürün yok.',
    bosMenu: 'Menü hazırlanıyor.',
    tumu: 'Tümü',
    oneCikan: 'Öne çıkan',
    icindekiler: 'İçindekiler',
    alerjenler: 'Alerjen bilgisi',
    kapat: 'Kapat',
    wifi: 'Wi-Fi şifresi',
    adres: 'Adres',
    telefon: 'Telefon',
    poweredBy: 'Karecik ile hazırlandı',
    yukleniyor: 'Menü yükleniyor...',
    bulunamadi: 'Menü bulunamadı',
    bulunamadiAciklama: 'Bu adrese ait bir menü yok. Adresi kontrol edin.',
  },
  en: {
    menu: 'Menu',
    ara: 'Search the menu...',
    sonucYok: 'No products matched your search.',
    bosKategori: 'No products in this category yet.',
    bosMenu: 'The menu is being prepared.',
    tumu: 'All',
    oneCikan: 'Featured',
    icindekiler: 'Ingredients',
    alerjenler: 'Allergen information',
    kapat: 'Close',
    wifi: 'Wi-Fi password',
    adres: 'Address',
    telefon: 'Phone',
    poweredBy: 'Made with Karecik',
    yukleniyor: 'Loading menu...',
    bulunamadi: 'Menu not found',
    bulunamadiAciklama: 'There is no menu at this address. Please check the URL.',
  },
}

/** Müşteri menüsü arayüz metni. Bilinmeyen dillerde İngilizce'ye düşer. */
export function metin(anahtar, dil = 'tr') {
  const sozluk = METINLER[dil] || METINLER.en
  return sozluk[anahtar] ?? METINLER.tr[anahtar] ?? anahtar
}
