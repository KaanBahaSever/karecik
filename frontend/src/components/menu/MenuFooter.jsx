import { Instagram, MapPin, Phone, Wifi } from 'lucide-react'

import { metin } from '../../locales/index.js'

/**
 * Müşteri menüsünün altı: iletişim bilgileri ve sistemin otomatik ürettiği
 * yasal ibareler.
 *
 * footer.price_note ve footer.vat_note metinleri BACKEND tarafından üretilir
 * (bkz. repository/menu.go -> buildFooter). Burada yalnızca gösterilir:
 *   "Fiyatlarımız 24.08.2026 tarihinden itibaren geçerlidir."
 *   "Fiyatlarımıza KDV dahildir."
 * İşletme bu ibareleri panelden kapatırsa alanlar boş gelir ve satır hiç çizilmez.
 *
 * @param {object} business - PublicMenu.business
 * @param {object} footer   - PublicMenu.footer { price_note, vat_note, powered_by }
 * @param {string} dil      - Aktif dil kodu
 */
export default function MenuFooter({ business, footer, dil = 'tr' }) {
  const adres = business?.address?.trim()
  const telefon = business?.phone?.trim()
  const wifi = business?.wifi_password?.trim()
  const instagram = business?.instagram?.trim().replace(/^@/, '')

  const iletisimVar = Boolean(adres || telefon || wifi || instagram)

  const fiyatNotu = footer?.price_note?.trim()
  const kdvNotu = footer?.vat_note?.trim()
  const imza = footer?.powered_by?.trim() || metin('poweredBy', dil)

  return (
    <footer
      className="mt-10 pt-6 text-center"
      style={{ borderTop: '1px solid var(--menu-border)', color: 'var(--menu-muted)' }}
    >
      {iletisimVar ? (
        <div className="mb-5 flex flex-col items-center gap-2 text-xs">
          {adres ? (
            <p className="flex items-start justify-center gap-1.5">
              <MapPin className="mt-px h-3.5 w-3.5 shrink-0" aria-hidden="true" />
              <span className="max-w-xs">{adres}</span>
            </p>
          ) : null}

          {telefon ? (
            <p className="flex items-center justify-center gap-1.5">
              <Phone className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
              <span>{telefon}</span>
            </p>
          ) : null}

          {wifi ? (
            <p className="flex items-center justify-center gap-1.5">
              <Wifi className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
              <span>
                {metin('wifi', dil)}:{' '}
                <span style={{ color: 'var(--menu-text)' }} className="font-medium">
                  {wifi}
                </span>
              </span>
            </p>
          ) : null}

          {instagram ? (
            <p className="flex items-center justify-center gap-1.5">
              <Instagram className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
              <span>@{instagram}</span>
            </p>
          ) : null}
        </div>
      ) : null}

      {/* Sistem tarafından otomatik güncellenen yasal ibareler */}
      {fiyatNotu || kdvNotu ? (
        <div className="mb-4 space-y-1 text-[11px] leading-relaxed">
          {fiyatNotu ? <p>{fiyatNotu}</p> : null}
          {kdvNotu ? <p>{kdvNotu}</p> : null}
        </div>
      ) : null}

      <p className="pb-6 text-[11px]" style={{ opacity: 0.7 }}>
        {imza}
      </p>
    </footer>
  )
}
