import { useEffect, useState } from 'react'
import QRCodeLib from 'qrcode'
import { Copy, Download, Info, Loader2, QrCode as QrCodeIcon } from 'lucide-react'

import { useAuth } from '../../lib/auth.jsx'
import { menuPathUrl, menuUrl } from '../../lib/subdomain'
import { useToast } from '../../components/ui/Toast.jsx'

/* When printing, everything except the QR card is hidden */
const PRINT_STYLE = `
  @media print {
    .print-hide { display: none; }
    html, body { height: auto; background: #ffffff; }
    .print-card { border: none; box-shadow: none; }
  }
`

export default function QrCode() {
  const { business } = useAuth()
  const toast = useToast()

  const [dataUrl, setDataUrl] = useState('')
  const [generating, setGenerating] = useState(true)

  const slug = business?.slug || ''
  const address = menuUrl(slug)
  const fallbackAddress = menuPathUrl(slug)

  useEffect(() => {
    let cancelled = false

    async function generate() {
      if (!address) {
        setGenerating(false)
        return
      }
      setGenerating(true)
      try {
        const result = await QRCodeLib.toDataURL(address, {
          width: 1024,
          margin: 2,
          color: { dark: '#111827', light: '#ffffff' },
          errorCorrectionLevel: 'H',
        })
        if (!cancelled) setDataUrl(result)
      } catch (err) {
        if (!cancelled) toast.error(err.message || 'QR kodu oluşturulamadı.')
      } finally {
        if (!cancelled) setGenerating(false)
      }
    }

    generate()
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [address])

  async function copy(text) {
    if (!text) return
    try {
      await navigator.clipboard.writeText(text)
      toast.success('Adres kopyalandı.')
    } catch {
      toast.error('Adres kopyalanamadı. Metni elle seçip kopyalayabilirsiniz.')
    }
  }

  function print() {
    window.print()
  }

  return (
    <div className="mx-auto max-w-content">
      <style>{PRINT_STYLE}</style>

      <div className="print-hide mb-6">
        <h2 className="text-xl font-semibold text-gray-900">QR Kod</h2>
        <p className="mt-1 text-sm text-gray-500">
          Müşterileriniz bu kodu okutarak menünüze anında ulaşır.
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* ---------------------------------------------------- LEFT: QR card */}
        <div className="print-card card flex flex-col items-center gap-5 p-6 sm:p-8">
          <div className="flex w-full max-w-xs items-center justify-center">
            {generating ? (
              <div className="flex aspect-square w-full items-center justify-center rounded-xl border border-dashed border-gray-200">
                <Loader2 className="h-6 w-6 animate-spin text-brand-600" aria-hidden="true" />
              </div>
            ) : dataUrl ? (
              <img
                src={dataUrl}
                alt={`${business?.name || 'İşletme'} menü QR kodu`}
                className="aspect-square w-full rounded-xl bg-white object-contain"
              />
            ) : (
              <div className="flex aspect-square w-full flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-gray-200 text-gray-400">
                <QrCodeIcon className="h-8 w-8" aria-hidden="true" />
                <span className="text-xs">QR kodu oluşturulamadı</span>
              </div>
            )}
          </div>

          <div className="text-center">
            <p className="text-lg font-semibold text-gray-900">{business?.name || 'İşletmeniz'}</p>
            <p className="mt-1 text-sm text-gray-500">Menümüz için okutun</p>
          </div>
        </div>

        {/* ------------------------------------------ RIGHT: addresses, actions */}
        <div className="print-hide space-y-6">
          <div className="card p-5">
            <h3 className="text-sm font-semibold text-gray-900">Menü adresiniz</h3>
            <p className="help-text">Müşterilerinizle paylaşabileceğiniz asıl adres.</p>

            <div className="mt-3 flex flex-col gap-2 sm:flex-row">
              <input
                type="text"
                readOnly
                value={address}
                onFocus={(event) => event.target.select()}
                className="input font-mono text-xs sm:text-sm"
                aria-label="Menü adresi"
              />
              <button
                type="button"
                onClick={() => copy(address)}
                className="btn-secondary shrink-0"
                disabled={!address}
              >
                <Copy className="h-4 w-4" />
                Kopyala
              </button>
            </div>

            <div className="mt-5 border-t border-gray-100 pt-4">
              <h4 className="text-sm font-medium text-gray-700">Yerel test adresi</h4>
              <p className="help-text">
                Alt alan adı ayarı olmayan bilgisayarlarda menüyü açmak için kullanın.
              </p>

              <div className="mt-3 flex flex-col gap-2 sm:flex-row">
                <input
                  type="text"
                  readOnly
                  value={fallbackAddress}
                  onFocus={(event) => event.target.select()}
                  className="input font-mono text-xs sm:text-sm"
                  aria-label="Yerel test adresi"
                />
                <button
                  type="button"
                  onClick={() => copy(fallbackAddress)}
                  className="btn-secondary shrink-0"
                  disabled={!fallbackAddress}
                >
                  <Copy className="h-4 w-4" />
                  Kopyala
                </button>
              </div>
            </div>
          </div>

          <div className="card p-5">
            <h3 className="text-sm font-semibold text-gray-900">QR kodunu kullanın</h3>
            <p className="help-text">
              Yüksek çözünürlüklü (1024 px) görseli indirin veya doğrudan yazdırın.
            </p>

            <div className="mt-3 flex flex-col gap-2 sm:flex-row">
              {dataUrl ? (
                <a
                  href={dataUrl}
                  download={`karecik-qr-${slug || 'menu'}.png`}
                  className="btn-primary"
                >
                  <Download className="h-4 w-4" />
                  PNG olarak indir
                </a>
              ) : (
                <button type="button" className="btn-primary" disabled>
                  <Download className="h-4 w-4" />
                  PNG olarak indir
                </button>
              )}

              <button type="button" onClick={print} className="btn-secondary" disabled={!dataUrl}>
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
