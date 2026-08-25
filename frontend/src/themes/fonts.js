// Menüde kullanılabilen yazı tipleri.
// Kimlikler backend'deki internal/utils/appearance.go ile birebir aynıdır.
//
// Fontlar sayfa açılışında değil, seçildiklerinde Google Fonts'tan yüklenir.

export const FONTLAR = [
  { id: 'inter', label: 'Inter', stack: "'Inter', system-ui, sans-serif", google: 'Inter:wght@400;500;600;700' },
  { id: 'poppins', label: 'Poppins', stack: "'Poppins', system-ui, sans-serif", google: 'Poppins:wght@400;500;600;700' },
  { id: 'montserrat', label: 'Montserrat', stack: "'Montserrat', system-ui, sans-serif", google: 'Montserrat:wght@400;500;600;700' },
  { id: 'nunito', label: 'Nunito', stack: "'Nunito', system-ui, sans-serif", google: 'Nunito:wght@400;600;700' },
  { id: 'playfair', label: 'Playfair Display', stack: "'Playfair Display', Georgia, serif", google: 'Playfair+Display:wght@400;600;700' },
  { id: 'lora', label: 'Lora', stack: "'Lora', Georgia, serif", google: 'Lora:wght@400;500;600' },
  { id: 'dm-serif', label: 'DM Serif Display', stack: "'DM Serif Display', Georgia, serif", google: 'DM+Serif+Display' },
  { id: 'space-grotesk', label: 'Space Grotesk', stack: "'Space Grotesk', system-ui, sans-serif", google: 'Space+Grotesk:wght@400;500;700' },
]

export const VARSAYILAN_FONT = FONTLAR[0]

/** Kimliğe göre fontu bulur; bulunamazsa Inter döner. */
export function fontBul(id) {
  return FONTLAR.find((font) => font.id === id) || VARSAYILAN_FONT
}

/** Font kimliğinden CSS font-family değeri. */
export function fontStack(id) {
  return fontBul(id).stack
}

const yuklenenler = new Set(['inter']) // Inter index.html'de zaten yüklü

/**
 * Seçilen yazı tipini Google Fonts'tan bir kez yükler.
 * Panelde font değiştirildiğinde canlı önizleme anında doğru fontu gösterir.
 */
export function fontYukle(id) {
  const font = fontBul(id)
  if (!font.google || yuklenenler.has(font.id)) return

  const link = document.createElement('link')
  link.rel = 'stylesheet'
  link.href = `https://fonts.googleapis.com/css2?family=${font.google}&display=swap`
  document.head.appendChild(link)
  yuklenenler.add(font.id)
}

/** Tüm menü fontlarını önceden yükler (tasarım sekmesinde önizleme için). */
export function tumFontlariYukle() {
  FONTLAR.forEach((font) => fontYukle(font.id))
}
