// Menu languages, allergen labels and customer-facing interface strings.
// The codes mirror backend/internal/utils/appearance.go exactly.
//
// NOTE: every label and string below is user-facing, so the wording in each
// language is data — not something to translate at the code level.
//
// EVERY visible or announced string of the customer menu lives here: button
// labels, headings, empty states, errors, placeholders, aria-labels and titles
// alike. A string left in the JSX stays Turkish whatever language the visitor
// picked, which is exactly the half-translated screen this file exists to
// prevent. tests/locales.test.mjs keeps the six dictionaries in step: every
// language must carry exactly the keys of 'tr', none of them empty.
//
// Words that are never translated, in any language:
//   Karecik       the product's name
//   Instagram     a brand
//   Wi-Fi         a trademark — except in German, where "WLAN" is the word
//                 every sign in a café uses, so the German strings say that
//   Yerli Üretim  the name of an official Turkish certification mark. It is
//                 printed as-is everywhere (see MenuFooter); only the
//                 descriptive title next to it is localized.
// The venue's own content — product names such as "Latte" or "Melly Beef" —
// comes from the payload and is never touched here.

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

/** The six language codes, in picker order. */
export const LANGUAGE_CODES = LANGUAGES.map((language) => language.code)

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

/** The `dir` attribute value for a language. */
export function languageDir(code) {
  return isRtl(code) ? 'rtl' : 'ltr'
}

/** A letter of a right-to-left script: Hebrew, Arabic and its supplements, Syriac, Thaana, N'Ko. */
const RTL_LETTER = /[֐-ࣿיִ-﷿ﹰ-ﻼ]/

/**
 * The `dir` of the isolate (<bdi>) around one of the owner's own texts — a
 * product, category, option or badge name, a description — in the menu of
 * `language`.
 *
 * 'auto' gives a text the direction of its first strong letter, which is what
 * an untranslated Turkish fallback needs inside the Arabic menu. But Arabic
 * translations keep loanwords and item names in Latin letters, and a text
 * that OPENS with one — "Frozen بالفراولة", "Cheesecake بمربى الحليب" — would
 * then be laid out left to right: the Arabic reader, starting at the right,
 * meets the Arabic words first and "Frozen" last. So in a right-to-left menu a
 * text with any right-to-left letter in it is 'rtl'; everything else, in every
 * menu, stays 'auto'.
 *
 * @param {unknown} text
 * @param {string}  language
 * @returns {'rtl'|'auto'}
 */
export function textDir(text, language) {
  return isRtl(language) && typeof text === 'string' && RTL_LETTER.test(text) ? 'rtl' : 'auto'
}

/* -------------------------------------------------------------- allergens */

/*
  One label per menu language. `tr` and `en` are also read directly by the
  dashboard (ProductModal lists allergen.tr), so those two property names are
  part of the shape and must stay.

  Where the EU allergen regulation has a settled term the label uses it
  ("Schalenfrüchte", "Fruits à coque"), because that is the word a visitor with
  an allergy is used to looking for on a menu.
*/
export const ALLERGENS = [
  { code: 'gluten', emoji: '🌾', tr: 'Gluten', en: 'Gluten', de: 'Gluten', ru: 'Глютен', ar: 'غلوتين', fr: 'Gluten' },
  { code: 'sut', emoji: '🥛', tr: 'Süt', en: 'Milk', de: 'Milch', ru: 'Молоко', ar: 'حليب', fr: 'Lait' },
  { code: 'yumurta', emoji: '🥚', tr: 'Yumurta', en: 'Egg', de: 'Ei', ru: 'Яйца', ar: 'بيض', fr: 'Œufs' },
  {
    code: 'findik',
    emoji: '🌰',
    tr: 'Fındık / Kuruyemiş',
    en: 'Nuts',
    de: 'Schalenfrüchte',
    ru: 'Орехи',
    ar: 'مكسرات',
    fr: 'Fruits à coque',
  },
  {
    code: 'yer_fistigi',
    emoji: '🥜',
    tr: 'Yer Fıstığı',
    en: 'Peanuts',
    de: 'Erdnüsse',
    ru: 'Арахис',
    ar: 'فول سوداني',
    fr: 'Arachides',
  },
  { code: 'soya', emoji: '🫘', tr: 'Soya', en: 'Soy', de: 'Soja', ru: 'Соя', ar: 'صويا', fr: 'Soja' },
  { code: 'balik', emoji: '🐟', tr: 'Balık', en: 'Fish', de: 'Fisch', ru: 'Рыба', ar: 'سمك', fr: 'Poisson' },
  {
    code: 'kabuklu_deniz',
    emoji: '🦐',
    tr: 'Kabuklu Deniz Ürünü',
    en: 'Shellfish',
    de: 'Schalentiere',
    ru: 'Моллюски и ракообразные',
    ar: 'المحار والقشريات',
    fr: 'Crustacés et mollusques',
  },
  { code: 'susam', emoji: '🌱', tr: 'Susam', en: 'Sesame', de: 'Sesam', ru: 'Кунжут', ar: 'سمسم', fr: 'Sésame' },
  { code: 'hardal', emoji: '🟡', tr: 'Hardal', en: 'Mustard', de: 'Senf', ru: 'Горчица', ar: 'خردل', fr: 'Moutarde' },
  { code: 'kereviz', emoji: '🥬', tr: 'Kereviz', en: 'Celery', de: 'Sellerie', ru: 'Сельдерей', ar: 'كرفس', fr: 'Céleri' },
  { code: 'sulfit', emoji: '🧪', tr: 'Sülfit', en: 'Sulphites', de: 'Sulfite', ru: 'Сульфиты', ar: 'كبريتيت', fr: 'Sulfites' },
  { code: 'aci', emoji: '🌶️', tr: 'Acı', en: 'Spicy', de: 'Scharf', ru: 'Острое', ar: 'حار', fr: 'Épicé' },
  {
    code: 'vejetaryen',
    emoji: '🥗',
    tr: 'Vejetaryen',
    en: 'Vegetarian',
    de: 'Vegetarisch',
    ru: 'Вегетарианское',
    ar: 'نباتي',
    fr: 'Végétarien',
  },
  { code: 'vegan', emoji: '🌿', tr: 'Vegan', en: 'Vegan', de: 'Vegan', ru: 'Веганское', ar: 'نباتي صرف', fr: 'Végan' },
  {
    code: 'alkol',
    emoji: '🍷',
    tr: 'Alkol İçerir',
    en: 'Contains Alcohol',
    de: 'Enthält Alkohol',
    ru: 'Содержит алкоголь',
    ar: 'يحتوي على كحول',
    fr: "Contient de l'alcool",
  },
  { code: 'kafein', emoji: '☕', tr: 'Kafein', en: 'Caffeine', de: 'Koffein', ru: 'Кофеин', ar: 'كافيين', fr: 'Caféine' },
]

export function findAllergen(code) {
  return ALLERGENS.find((allergen) => allergen.code === code) || null
}

/**
 * The allergen's label in `language`: English for a language the table does
 * not know, and the code itself for an allergen it does not know.
 */
export function allergenLabel(code, language = 'tr') {
  const allergen = findAllergen(code)
  if (!allergen) return code
  return allergen[language] || allergen.en || allergen.tr
}

/* ------------------------------------------- customer menu interface copy */

/*
  `{name}` in a string is a placeholder t() fills from its third argument —
  priceValidFrom's `{date}` is the one in use. Each language puts the
  placeholder wherever its own grammar wants it, which is the whole reason the
  sentence is not glued together from pieces in the component.
*/
export const STRINGS = {
  tr: {
    search: 'Menüde ara...',
    searchLabel: 'Menüde ara',
    noResults: 'Aramanızla eşleşen ürün bulunamadı.',
    emptyCategory: 'Bu kategoride henüz ürün yok.',
    emptyMenu: 'Menü hazırlanıyor.',
    featured: 'Öne çıkan',
    hidden: 'Gizli',
    hasOptions: '+ Seçenekler',
    ingredients: 'İçindekiler',
    allergens: 'Alerjen bilgisi',
    kcal: 'kcal',
    required: 'Zorunlu',
    total: 'Toplam',
    close: 'Kapat',
    productDetails: 'Ürün detayı',
    categories: 'Kategoriler',
    backToCategories: 'Kategorilere dön',
    wifiName: 'Wi-Fi ağı',
    wifiPassword: 'Wi-Fi şifresi',
    wifiChip: 'Wi-Fi',
    showPassword: 'Şifreyi göster',
    copy: 'Kopyala',
    copied: 'Kopyalandı',
    copyFailed: 'Kopyalanamadı',
    phone: 'Telefon',
    call: 'Ara',
    contact: 'İletişim',
    instagram: 'Instagram',
    openInstagram: "Instagram'da aç",
    openLink: 'Aç',
    menuLabel: 'Menüler',
    language: 'Dil',
    priceValidFrom: 'Fiyatlarımız {date} tarihinden itibaren geçerlidir.',
    vatIncluded: 'Fiyatlarımıza KDV dahildir.',
    yerliUretimTitle: 'Yerli Üretim logosu',
    poweredBy: 'Karecik ile hazırlandı',
    loading: 'Menü yükleniyor...',
    notFound: 'Menü bulunamadı',
    notFoundDetail: 'Bu adrese ait bir menü yok. Adresi kontrol edin.',
    loadFailed: 'Menü yüklenemedi',
    loadFailedDetail: 'Bağlantınızı kontrol edip yeniden deneyin.',
    renderError: 'Menü görüntülenirken bir sorun oluştu.',
    retry: 'Yeniden dene',
    reload: 'Sayfayı yenile',
    skipSplash: 'Karşılama ekranını geç',
    chooseMenu: 'Bir menü seçin',
    noActiveMenus: 'Yayında menü yok',
    noActiveMenusDetail: 'Bu işletmenin şu anda yayında bir menüsü bulunmuyor.',
    untitledCategory: 'Adsız kategori',
    itemUnavailable: 'Bu ürün şu anda gösterilemiyor.',
  },
  en: {
    search: 'Search the menu...',
    searchLabel: 'Search the menu',
    noResults: 'No items match your search.',
    emptyCategory: 'There are no items in this category yet.',
    emptyMenu: 'The menu is being prepared.',
    featured: 'Featured',
    hidden: 'Hidden',
    hasOptions: '+ Options',
    ingredients: 'Ingredients',
    allergens: 'Allergen information',
    kcal: 'kcal',
    required: 'Required',
    total: 'Total',
    close: 'Close',
    productDetails: 'Item details',
    categories: 'Categories',
    backToCategories: 'Back to categories',
    wifiName: 'Wi-Fi network',
    wifiPassword: 'Wi-Fi password',
    wifiChip: 'Wi-Fi',
    showPassword: 'Show password',
    copy: 'Copy',
    copied: 'Copied',
    copyFailed: "Couldn't copy",
    phone: 'Phone',
    call: 'Call',
    contact: 'Contact',
    instagram: 'Instagram',
    openInstagram: 'Open in Instagram',
    openLink: 'Open',
    menuLabel: 'Menus',
    language: 'Language',
    priceValidFrom: 'Prices valid from {date}.',
    vatIncluded: 'All prices include VAT.',
    yerliUretimTitle: 'Yerli Üretim: Turkish domestic production mark',
    poweredBy: 'Powered by Karecik',
    loading: 'Loading the menu...',
    notFound: 'Menu not found',
    notFoundDetail: 'There is no menu at this address. Please check the link.',
    loadFailed: "Couldn't load the menu",
    loadFailedDetail: 'Check your connection and try again.',
    renderError: 'Something went wrong while showing the menu.',
    retry: 'Try again',
    reload: 'Reload page',
    skipSplash: 'Skip the welcome screen',
    chooseMenu: 'Choose a menu',
    noActiveMenus: 'No menus available',
    noActiveMenusDetail: "This venue doesn't have a published menu right now.",
    untitledCategory: 'Untitled category',
    itemUnavailable: "This item can't be shown right now.",
  },
  de: {
    search: 'Speisekarte durchsuchen...',
    searchLabel: 'Speisekarte durchsuchen',
    noResults: 'Zu Ihrer Suche wurde nichts gefunden.',
    emptyCategory: 'In dieser Kategorie gibt es noch keine Einträge.',
    emptyMenu: 'Die Speisekarte wird gerade vorbereitet.',
    featured: 'Empfehlung',
    hidden: 'Ausgeblendet',
    hasOptions: '+ Optionen',
    ingredients: 'Zutaten',
    allergens: 'Allergene',
    kcal: 'kcal',
    required: 'Pflichtauswahl',
    total: 'Gesamt',
    close: 'Schließen',
    productDetails: 'Produktdetails',
    categories: 'Kategorien',
    backToCategories: 'Zurück zu den Kategorien',
    wifiName: 'WLAN-Netzwerk',
    wifiPassword: 'WLAN-Passwort',
    wifiChip: 'WLAN',
    showPassword: 'Passwort anzeigen',
    copy: 'Kopieren',
    copied: 'Kopiert',
    copyFailed: 'Kopieren fehlgeschlagen',
    phone: 'Telefon',
    call: 'Anrufen',
    contact: 'Kontakt',
    instagram: 'Instagram',
    openInstagram: 'Auf Instagram öffnen',
    openLink: 'Öffnen',
    menuLabel: 'Speisekarten',
    language: 'Sprache',
    priceValidFrom: 'Preise gültig ab {date}.',
    vatIncluded: 'Alle Preise inkl. MwSt.',
    yerliUretimTitle: 'Yerli Üretim: Kennzeichen für türkische Inlandsproduktion',
    poweredBy: 'Bereitgestellt von Karecik',
    loading: 'Speisekarte wird geladen...',
    notFound: 'Speisekarte nicht gefunden',
    notFoundDetail: 'Unter dieser Adresse gibt es keine Speisekarte. Bitte prüfen Sie den Link.',
    loadFailed: 'Speisekarte konnte nicht geladen werden',
    loadFailedDetail: 'Bitte prüfen Sie Ihre Verbindung und versuchen Sie es erneut.',
    renderError: 'Beim Anzeigen der Speisekarte ist ein Fehler aufgetreten.',
    retry: 'Erneut versuchen',
    reload: 'Seite neu laden',
    skipSplash: 'Begrüßungsbildschirm überspringen',
    chooseMenu: 'Bitte wählen Sie eine Speisekarte',
    noActiveMenus: 'Keine Speisekarten verfügbar',
    noActiveMenusDetail: 'Dieser Betrieb hat derzeit keine veröffentlichte Speisekarte.',
    untitledCategory: 'Unbenannte Kategorie',
    itemUnavailable: 'Dieser Eintrag kann gerade nicht angezeigt werden.',
  },
  ru: {
    search: 'Поиск по меню...',
    searchLabel: 'Поиск по меню',
    noResults: 'По вашему запросу ничего не найдено.',
    emptyCategory: 'В этой категории пока нет позиций.',
    emptyMenu: 'Меню готовится.',
    featured: 'Рекомендуем',
    hidden: 'Скрыто',
    hasOptions: '+ Варианты',
    ingredients: 'Состав',
    allergens: 'Аллергены',
    kcal: 'ккал',
    required: 'Обязательно',
    total: 'Итого',
    close: 'Закрыть',
    productDetails: 'Подробнее о позиции',
    categories: 'Категории',
    backToCategories: 'Назад к категориям',
    wifiName: 'Сеть Wi-Fi',
    wifiPassword: 'Пароль Wi-Fi',
    wifiChip: 'Wi-Fi',
    showPassword: 'Показать пароль',
    copy: 'Копировать',
    copied: 'Скопировано',
    copyFailed: 'Не удалось скопировать',
    phone: 'Телефон',
    call: 'Позвонить',
    contact: 'Контакты',
    instagram: 'Instagram',
    openInstagram: 'Открыть в Instagram',
    openLink: 'Открыть',
    menuLabel: 'Меню',
    language: 'Язык',
    priceValidFrom: 'Цены действительны с {date}.',
    vatIncluded: 'Все цены указаны с учётом НДС.',
    yerliUretimTitle: 'Yerli Üretim: знак отечественного производства Турции',
    poweredBy: 'Работает на Karecik',
    loading: 'Загрузка меню...',
    notFound: 'Меню не найдено',
    notFoundDetail: 'По этому адресу меню нет. Проверьте ссылку.',
    loadFailed: 'Не удалось загрузить меню',
    loadFailedDetail: 'Проверьте подключение к интернету и попробуйте ещё раз.',
    renderError: 'При отображении меню произошла ошибка.',
    retry: 'Повторить',
    reload: 'Обновить страницу',
    skipSplash: 'Пропустить заставку',
    chooseMenu: 'Выберите меню',
    noActiveMenus: 'Нет доступных меню',
    noActiveMenusDetail: 'У этого заведения сейчас нет опубликованного меню.',
    untitledCategory: 'Категория без названия',
    itemUnavailable: 'Эту позицию сейчас невозможно показать.',
  },
  ar: {
    search: 'ابحث في القائمة...',
    searchLabel: 'ابحث في القائمة',
    noResults: 'لا توجد أصناف تطابق بحثك.',
    emptyCategory: 'لا توجد أصناف في هذه الفئة بعد.',
    emptyMenu: 'يجري إعداد القائمة.',
    featured: 'مميّز',
    hidden: 'مخفي',
    hasOptions: '+ خيارات',
    ingredients: 'المكوّنات',
    allergens: 'مسبّبات الحساسية',
    kcal: 'سعرة',
    required: 'إلزامي',
    total: 'الإجمالي',
    close: 'إغلاق',
    productDetails: 'تفاصيل الصنف',
    categories: 'الفئات',
    backToCategories: 'العودة إلى الفئات',
    wifiName: 'شبكة Wi-Fi',
    wifiPassword: 'كلمة مرور Wi-Fi',
    wifiChip: 'Wi-Fi',
    showPassword: 'إظهار كلمة المرور',
    copy: 'نسخ',
    copied: 'تم النسخ',
    copyFailed: 'تعذّر النسخ',
    phone: 'الهاتف',
    call: 'اتصال',
    contact: 'التواصل',
    instagram: 'Instagram',
    openInstagram: 'فتح في Instagram',
    openLink: 'فتح',
    menuLabel: 'القوائم',
    language: 'اللغة',
    priceValidFrom: 'الأسعار سارية اعتبارًا من {date}.',
    vatIncluded: 'جميع الأسعار شاملة ضريبة القيمة المضافة.',
    yerliUretimTitle: 'Yerli Üretim: علامة الإنتاج المحلي التركي',
    poweredBy: 'مدعوم من Karecik',
    loading: 'جارٍ تحميل القائمة...',
    notFound: 'لم يتم العثور على القائمة',
    notFoundDetail: 'لا توجد قائمة على هذا العنوان. يرجى التحقق من الرابط.',
    loadFailed: 'تعذّر تحميل القائمة',
    loadFailedDetail: 'تحقّق من اتصالك وحاول مرة أخرى.',
    renderError: 'حدثت مشكلة أثناء عرض القائمة.',
    retry: 'إعادة المحاولة',
    reload: 'إعادة تحميل الصفحة',
    skipSplash: 'تخطي شاشة الترحيب',
    chooseMenu: 'اختر قائمة',
    noActiveMenus: 'لا توجد قوائم متاحة',
    noActiveMenusDetail: 'لا توجد لدى هذا المكان قائمة منشورة حاليًا.',
    untitledCategory: 'فئة بلا اسم',
    itemUnavailable: 'لا يمكن عرض هذا الصنف حاليًا.',
  },
  fr: {
    search: 'Rechercher dans le menu...',
    searchLabel: 'Rechercher dans le menu',
    noResults: 'Aucun article ne correspond à votre recherche.',
    emptyCategory: 'Aucun article dans cette catégorie pour le moment.',
    emptyMenu: 'Le menu est en cours de préparation.',
    featured: 'Recommandé',
    hidden: 'Masqué',
    hasOptions: '+ Options',
    ingredients: 'Ingrédients',
    allergens: 'Allergènes',
    kcal: 'kcal',
    required: 'Obligatoire',
    total: 'Total',
    close: 'Fermer',
    productDetails: "Détails de l'article",
    categories: 'Catégories',
    backToCategories: 'Retour aux catégories',
    wifiName: 'Réseau Wi-Fi',
    wifiPassword: 'Mot de passe Wi-Fi',
    wifiChip: 'Wi-Fi',
    showPassword: 'Afficher le mot de passe',
    copy: 'Copier',
    copied: 'Copié',
    copyFailed: 'Copie impossible',
    phone: 'Téléphone',
    call: 'Appeler',
    contact: 'Contact',
    instagram: 'Instagram',
    openInstagram: 'Ouvrir dans Instagram',
    openLink: 'Ouvrir',
    menuLabel: 'Menus',
    language: 'Langue',
    priceValidFrom: 'Prix en vigueur à partir du {date}.',
    vatIncluded: 'Tous nos prix sont TTC.',
    yerliUretimTitle: 'Yerli Üretim : label turc de production nationale',
    poweredBy: 'Propulsé par Karecik',
    loading: 'Chargement du menu...',
    notFound: 'Menu introuvable',
    notFoundDetail: "Il n'y a aucun menu à cette adresse. Vérifiez le lien.",
    loadFailed: 'Impossible de charger le menu',
    loadFailedDetail: 'Vérifiez votre connexion et réessayez.',
    renderError: "Un problème est survenu lors de l'affichage du menu.",
    retry: 'Réessayer',
    reload: 'Recharger la page',
    skipSplash: "Passer l'écran d'accueil",
    chooseMenu: 'Choisissez un menu',
    noActiveMenus: 'Aucun menu disponible',
    noActiveMenusDetail: "Cet établissement n'a aucun menu publié pour le moment.",
    untitledCategory: 'Catégorie sans nom',
    itemUnavailable: 'Cet article ne peut pas être affiché pour le moment.',
  },
}

/**
 * Customer menu interface string.
 *
 * An unknown language reads the English dictionary; a key a dictionary lacks
 * falls back to English, then to Turkish, and finally to the key itself, so a
 * missing translation degrades to readable text instead of a blank.
 *
 * `values` fills `{name}` placeholders: t('priceValidFrom', 'en', { date })
 * -> "Prices valid from 24 August 2026." A placeholder with no value is left
 * as written, which makes the omission visible rather than silent.
 *
 * @param {string} key
 * @param {string} language
 * @param {object} [values]
 * @returns {string}
 */
export function t(key, language = 'tr', values) {
  const dictionary = Object.prototype.hasOwnProperty.call(STRINGS, language)
    ? STRINGS[language]
    : STRINGS.en
  const text = dictionary[key] ?? STRINGS.en[key] ?? STRINGS.tr[key] ?? key
  if (!values) return text

  return text.replace(/\{(\w+)\}/g, (placeholder, name) =>
    Object.prototype.hasOwnProperty.call(values, name) ? String(values[name]) : placeholder,
  )
}
