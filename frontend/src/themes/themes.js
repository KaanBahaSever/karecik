// Müşteri menüsü temaları.
// Kimlikler (id) backend'deki internal/utils/appearance.go ile birebir aynıdır.

export const TEMALAR = [
  {
    id: 'modern-light',
    label: 'Modern Açık',
    description: 'Beyaz zemin, mavi vurgu',
    dark: false,
    renkler: {
      primary: '#1d4ed8',
      background: '#ffffff',
      surface: '#f6f7f8',
      text: '#111827',
      muted: '#6b7280',
      border: '#e5e7eb',
    },
    stil: { radius: '0.875rem', kartGolge: '0 1px 3px rgb(0 0 0 / 0.07)' },
  },
  {
    id: 'modern-dark',
    label: 'Modern Koyu',
    description: 'Koyu zemin, gece kullanımı',
    dark: true,
    renkler: {
      primary: '#60a5fa',
      background: '#0f172a',
      surface: '#1e293b',
      text: '#f1f5f9',
      muted: '#94a3b8',
      border: '#334155',
    },
    stil: { radius: '0.875rem', kartGolge: '0 1px 3px rgb(0 0 0 / 0.4)' },
  },
  {
    id: 'zarif',
    label: 'Zarif',
    description: 'Krem zemin, serif başlıklar',
    dark: false,
    renkler: {
      primary: '#8a6a3c',
      background: '#faf6ef',
      surface: '#f2ead9',
      text: '#3b2f22',
      muted: '#8a7a66',
      border: '#e3d7c1',
    },
    stil: { radius: '0.25rem', kartGolge: 'none' },
  },
  {
    id: 'sicak',
    label: 'Sıcak',
    description: 'Kiremit tonları, samimi',
    dark: false,
    renkler: {
      primary: '#c2410c',
      background: '#fffbf6',
      surface: '#ffedd5',
      text: '#431407',
      muted: '#9a6b52',
      border: '#fed7aa',
    },
    stil: { radius: '1.25rem', kartGolge: '0 2px 8px rgb(194 65 12 / 0.08)' },
  },
  {
    id: 'minimal',
    label: 'Minimal',
    description: 'Siyah-beyaz, sade tipografi',
    dark: false,
    renkler: {
      primary: '#111111',
      background: '#ffffff',
      surface: '#fafafa',
      text: '#111111',
      muted: '#737373',
      border: '#e5e5e5',
    },
    stil: { radius: '0rem', kartGolge: 'none' },
  },
  {
    id: 'canli',
    label: 'Canlı',
    description: 'Mor vurgu, yüksek kontrast',
    dark: false,
    renkler: {
      primary: '#7c3aed',
      background: '#ffffff',
      surface: '#f5f3ff',
      text: '#1e1b4b',
      muted: '#6d6a8a',
      border: '#ddd6fe',
    },
    stil: { radius: '1rem', kartGolge: '0 2px 10px rgb(124 58 237 / 0.10)' },
  },
]

export const VARSAYILAN_TEMA = TEMALAR[0]

/** Kimliğe göre temayı bulur; bulunamazsa varsayılanı döndürür. */
export function temaBul(id) {
  return TEMALAR.find((tema) => tema.id === id) || VARSAYILAN_TEMA
}

/**
 * Temayı CSS değişkenlerine çevirir. Menü kapsayıcısına style olarak verilir,
 * içerideki tüm bileşenler var(--menu-*) üzerinden okur.
 *
 * vurguRengi: işletmenin seçtiği özel ana renk (primary_color). Verilirse
 * temanın kendi primary değerinin yerine geçer.
 */
export function temaDegiskenleri(tema, vurguRengi, fontStack) {
  const t = typeof tema === 'string' ? temaBul(tema) : tema || VARSAYILAN_TEMA
  const primary = vurguRengi || t.renkler.primary

  return {
    '--menu-primary': primary,
    '--menu-bg': t.renkler.background,
    '--menu-surface': t.renkler.surface,
    '--menu-text': t.renkler.text,
    '--menu-muted': t.renkler.muted,
    '--menu-border': t.renkler.border,
    '--menu-radius': t.stil.radius,
    '--menu-shadow': t.stil.kartGolge,
    '--menu-font': fontStack || "'Inter', system-ui, sans-serif",
    backgroundColor: t.renkler.background,
    color: t.renkler.text,
    fontFamily: fontStack || "'Inter', system-ui, sans-serif",
  }
}
