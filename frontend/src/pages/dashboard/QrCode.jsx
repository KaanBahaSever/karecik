import { useEffect, useState } from 'react'
import QRCode from 'qrcode'
import { Copy, Download, Info, Loader2, QrCode as QrKodIkonu } from 'lucide-react'

import { useAuth } from '../../lib/auth.jsx'
import { menuAdresi, menuYoluAdresi } from '../../lib/subdomain'
import { useBildirim } from '../../components/ui/Bildirim.jsx'

/* Yazdırırken QR kartı dışındaki her şey gizlenir */
const YAZDIRMA_STILI = `
  @media print {
    .yazdirma-gizle { display: none; }
    html, body { height: auto; background: #ffffff; }
    .yazdirma-kart { border: none; box-shadow: none; }
  }
`

export default function QrKod() {
  const { business } = useAuth()
  const bildirim = useBildirim()

  const [dataUrl, setDataUrl] = useState('')
  const [uretiliyor, setUretiliyor] = useState(true)

  const slug = business?.slug || ''
  const adres = menuAdresi(slug)
  const yedekAdres = menuYoluAdresi(slug)

  useEffect(() => {
    let iptal = false

    async function qrUret() {
      if (!adres) {
        setUretiliyor(false)
        return
      }
      setUretiliyor(true)
      try {
        const sonuc = await QRCode.toDataURL(adres, {
          width: 1024,
          margin: 2,
          color: { dark: '#111827', light: '#ffffff' },
          errorCorrectionLevel: 'H',
        })
        if (!iptal) setDataUrl(sonuc)
      } catch (err) {
        if (!iptal) bildirim.hata(err.message || 'QR kodu oluşturulamadı.')
      } finally {
        if (!iptal) setUretiliyor(false)
      }
    }

    qrUret()
    return () => {
      iptal = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [adres])

  async function kopyala(metin) {
    if (!metin) return
    try {
      await navigator.clipboard.writeText(metin)
      bildirim.basari('Adres kopyalandı.')
    } catch {
      bildirim.hata('Adres kopyalanamadı. Metni elle seçip kopyalayabilirsiniz.')
    }
  }

  function yazdir() {
    window.print()
  }

  return (
    <div className="mx-auto max-w-icerik">
      <style>{YAZDIRMA_STILI}</style>

      <div className="yazdirma-gizle mb-6">
        <h2 className="text-xl font-semibold text-gray-900">QR Kod</h2>
        <p className="mt-1 text-sm text-gray-500">
          Müşterileriniz bu kodu okutarak menünüze anında ulaşır.
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* ------------------------------------------------- SOL: QR kartı */}
        <div className="yazdirma-kart kart flex flex-col items-center gap-5 p-6 sm:p-8">
          <div className="flex w-full max-w-xs items-center justify-center">
            {uretiliyor ? (
              <div className="flex aspect-square w-full items-center justify-center rounded-xl border border-dashed border-gray-200">
                <Loader2 className="h-6 w-6 animate-spin text-marka-600" aria-hidden="true" />
              </div>
            ) : dataUrl ? (
              <img
                src={dataUrl}
                alt={`${business?.name || 'İşletme'} menü QR kodu`}
                className="aspect-square w-full rounded-xl bg-white object-contain"
              />
            ) : (
              <div className="flex aspect-square w-full flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-gray-200 text-gray-400">
                <QrKodIkonu className="h-8 w-8" aria-hidden="true" />
                <span className="text-xs">QR kodu oluşturulamadı</span>
              </div>
            )}
          </div>

          <div className="text-center">
            <p className="text-lg font-semibold text-gray-900">{business?.name || 'İşletmeniz'}</p>
            <p className="mt-1 text-sm text-gray-500">Menümüz için okutun</p>
          </div>
        </div>

        {/* ----------------------------------------- SAĞ: adresler ve işlemler */}
        <div className="yazdirma-gizle space-y-6">
          <div className="kart p-5">
            <h3 className="text-sm font-semibold text-gray-900">Menü adresiniz</h3>
            <p className="yardim">Müşterilerinizle paylaşabileceğiniz asıl adres.</p>

            <div className="mt-3 flex flex-col gap-2 sm:flex-row">
              <input
                type="text"
                readOnly
                value={adres}
                onFocus={(e) => e.target.select()}
                className="girdi font-mono text-xs sm:text-sm"
                aria-label="Menü adresi"
              />
              <button
                type="button"
                onClick={() => kopyala(adres)}
                className="btn-ikincil shrink-0"
                disabled={!adres}
              >
                <Copy className="h-4 w-4" />
                Kopyala
              </button>
            </div>

            <div className="mt-5 border-t border-gray-100 pt-4">
              <h4 className="text-sm font-medium text-gray-700">Yerel test adresi</h4>
              <p className="yardim">
                Alt alan adı ayarı olmayan bilgisayarlarda menüyü açmak için kullanın.
              </p>

              <div className="mt-3 flex flex-col gap-2 sm:flex-row">
                <input
                  type="text"
                  readOnly
                  value={yedekAdres}
                  onFocus={(e) => e.target.select()}
                  className="girdi font-mono text-xs sm:text-sm"
                  aria-label="Yerel test adresi"
                />
                <button
                  type="button"
                  onClick={() => kopyala(yedekAdres)}
                  className="btn-ikincil shrink-0"
                  disabled={!yedekAdres}
                >
                  <Copy className="h-4 w-4" />
                  Kopyala
                </button>
              </div>
            </div>
          </div>

          <div className="kart p-5">
            <h3 className="text-sm font-semibold text-gray-900">QR kodunu kullanın</h3>
            <p className="yardim">
              Yüksek çözünürlüklü (1024 px) görseli indirin veya doğrudan yazdırın.
            </p>

            <div className="mt-3 flex flex-col gap-2 sm:flex-row">
              {dataUrl ? (
                <a
                  href={dataUrl}
                  download={`karecik-qr-${slug || 'menu'}.png`}
                  className="btn-birincil"
                >
                  <Download className="h-4 w-4" />
                  PNG olarak indir
                </a>
              ) : (
                <button type="button" className="btn-birincil" disabled>
                  <Download className="h-4 w-4" />
                  PNG olarak indir
                </button>
              )}

              <button type="button" onClick={yazdir} className="btn-ikincil" disabled={!dataUrl}>
                <span aria-hidden="true">🖨️</span>
                Yazdır
              </button>
            </div>
          </div>

          <div className="flex items-start gap-3 rounded-xl border border-blue-200 bg-blue-50 p-4 text-sm text-blue-800">
            <Info className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
            <p className="leading-relaxed">
              QR kodu masalarınıza, menü standlarınıza veya kasanıza yerleştirebilirsiniz. Menüde
              yaptığınız değişiklikler anında yansır, QR kodu tekrar bastırmanıza gerek yoktur.
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}
