// Açılış sayfası (landing) ve üst çubuk metinleri.
//
// Header'daki dil seçici bu sözlüğü kullanır; seçim localStorage'da saklanır.
// Panel ve müşteri menüsü ayrı sözlüklerle çalışır (bkz. locales/index.js).

export const LANDING_DILLERI = [
  { kod: 'tr', kisa: 'TR', ad: 'Türkçe' },
  { kod: 'en', kisa: 'EN', ad: 'English' },
  { kod: 'de', kisa: 'DE', ad: 'Deutsch' },
]

export const VARSAYILAN_LANDING_DILI = 'tr'

const SOZLUK = {
  tr: {
    markaAnaSayfa: 'Karecik ana sayfa',
    dilSec: 'Dil seçin',
    girisYap: 'Giriş Yap',
    ucretsizBasla: 'Hemen Ücretsiz Başla',

    baslik: 'İşletmeniz için QR menü servisi',
    aciklama: 'QR menü hizmetimizle satışlarınızı kolay ve pratik bir şekilde dijitalleştirin.',

    rozetUcretsiz: 'Tamamen ücretsiz',
    rozetKurulum: 'Hızlı kurulum',
    rozetMobil: 'Mobil uyum',

    demoBaslik: 'Örnek kafe menüsü',

    telif: '© 2026 Karecik',
    altPlatform: 'QR menü platformu',
    altKurulum: 'Kurulum ücreti yok',
    altKart: 'Kredi kartı gerekmez',
  },

  en: {
    markaAnaSayfa: 'Karecik home page',
    dilSec: 'Select language',
    girisYap: 'Sign In',
    ucretsizBasla: 'Start Free Now',

    baslik: 'A QR menu service for your business',
    aciklama: 'Digitise your sales easily and practically with our QR menu service.',

    rozetUcretsiz: 'Completely free',
    rozetKurulum: 'Quick setup',
    rozetMobil: 'Mobile friendly',

    demoBaslik: 'Sample cafe menu',

    telif: '© 2026 Karecik',
    altPlatform: 'QR menu platform',
    altKurulum: 'No setup fee',
    altKart: 'No credit card required',
  },

  de: {
    markaAnaSayfa: 'Karecik Startseite',
    dilSec: 'Sprache wählen',
    girisYap: 'Anmelden',
    ucretsizBasla: 'Jetzt kostenlos starten',

    baslik: 'QR-Menü-Service für Ihr Unternehmen',
    aciklama: 'Digitalisieren Sie Ihren Verkauf einfach und praktisch mit unserem QR-Menü-Service.',

    rozetUcretsiz: 'Völlig kostenlos',
    rozetKurulum: 'Schnelle Einrichtung',
    rozetMobil: 'Mobil optimiert',

    demoBaslik: 'Beispiel-Cafémenü',

    telif: '© 2026 Karecik',
    altPlatform: 'QR-Menü-Plattform',
    altKurulum: 'Keine Einrichtungsgebühr',
    altKart: 'Keine Kreditkarte nötig',
  },
}

/** Seçilen dilin metin sözlüğünü döndürür; bilinmeyen dilde Türkçe'ye düşer. */
export function landingMetni(dil) {
  return SOZLUK[dil] || SOZLUK[VARSAYILAN_LANDING_DILI]
}

const ANAHTAR = 'karecik_landing_dili'

/** Kullanıcının önceki dil seçimini okur. */
export function kayitliDiliOku() {
  try {
    const deger = localStorage.getItem(ANAHTAR)
    return LANDING_DILLERI.some((d) => d.kod === deger) ? deger : VARSAYILAN_LANDING_DILI
  } catch {
    return VARSAYILAN_LANDING_DILI
  }
}

/** Dil seçimini saklar (gizli sekmede sessizce yok sayılır). */
export function diliKaydet(dil) {
  try {
    localStorage.setItem(ANAHTAR, dil)
  } catch {
    /* localStorage kapalı olabilir */
  }
}
