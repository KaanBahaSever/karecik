import { useEffect, useState } from 'react'
import QRCodeLib from 'qrcode'
import {
  Copy,
  Download,
  ExternalLink,
  FileCode2,
  Info,
  Loader2,
  Printer,
  QrCode as QrCodeIcon,
} from 'lucide-react'

import api from '../../lib/api'
import { useAuth } from '../../lib/auth.jsx'
import { menuPathUrl, menuUrl } from '../../lib/subdomain'
import EmptyState from '../../components/ui/EmptyState.jsx'
import Loading from '../../components/ui/Loading.jsx'
import { useToast } from '../../components/ui/Toast.jsx'

/**
 * Every public address of the business in one place — one card per menu.
 *
 * A business can print several QR codes: one per menu, each encoding the exact
 * address `https://{business-slug}.karecik.com/{menu-slug}`. The subdomain is
 * the same on every card — it names the business — and only the path segment
 * changes, so both halves are needed to build a card's address.
 */

/* When printing, everything except the QR card is hidden (lifted from the
   single-business QR page this hub replaces). */
const PRINT_STYLE = `
  @media print {
    .print-hide { display: none !important; }
    html, body { height: auto; background: #ffffff; }
    .print-card { border: none; box-shadow: none; }
  }
`

/* The raster options QrCode.jsx used, so every export keeps printing the same. */
const QR_OPTIONS = {
  margin: 2,
  color: { dark: '#111827', light: '#ffffff' },
  errorCorrectionLevel: 'H',
}

/* The vector export is intentionally colourless: an SVG is edited downstream. */
const QR_SVG_OPTIONS = {
  type: 'svg',
  margin: 2,
  errorCorrectionLevel: 'H',
}

/** Clamp to two lines without needing the Tailwind line-clamp plugin. */
const TWO_LINES = {
  display: '-webkit-box',
  WebkitLineClamp: 2,
  WebkitBoxOrient: 'vertical',
  overflow: 'hidden',
}

/** Clicks a throwaway anchor so the browser saves `href` under `fileName`. */
function triggerDownload(href, fileName) {
  const link = document.createElement('a')
  link.href = href
  link.download = fileName
  document.body.appendChild(link)
  link.click()
  link.remove()
}

/* --------------------------------------------------------------- one menu */

/**
 * @param {string}   businessSlug   - the tenant subdomain, shared by every card
 * @param {object}   menu           - models.Menu (name, slug, description, …)
 * @param {boolean}  hiddenInPrint  - another card is being printed alone
 * @param {function} onPrint        - asks the page to print this card only
 */
function MenuQrCard({ businessSlug, menu, hiddenInPrint, onPrint }) {
  const toast = useToast()

  const [preview, setPreview] = useState('')
  const [generating, setGenerating] = useState(true)
  const [downloading, setDownloading] = useState('') // '' | 'png' | 'svg'

  const slug = menu?.slug || ''
  const name = menu?.name || 'Menü'
  const description = String(menu?.description || '').trim()

  // menuUrl() answers with {business}.karecik.com/{menu} in production and with
  // the local {business}.localhost:5173/{menu} form in development.
  const address = menuUrl(businessSlug, slug)
  // Path fallback for machines without a hosts file entry for the subdomain.
  const fallbackAddress = menuPathUrl(businessSlug, slug)
  const fileName = `karecik-qr-${slug || 'menu'}`

  // The preview is generated as the card mounts — a business with a dozen menus
  // never pays for codes nobody scrolled to until its card is on screen.
  useEffect(() => {
    let cancelled = false

    async function generate() {
      if (!address) {
        setPreview('')
        setGenerating(false)
        return
      }
      setGenerating(true)
      try {
        // Shown in a 256 px box; 512 px keeps it crisp on high density screens.
        const result = await QRCodeLib.toDataURL(address, { ...QR_OPTIONS, width: 512 })
        if (!cancelled) setPreview(result)
      } catch (error) {
        if (!cancelled) toast.error(error.message || 'QR kodu oluşturulamadı.')
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

  async function downloadPng() {
    if (!address || downloading) return

    setDownloading('png')
    try {
      const dataUrl = await QRCodeLib.toDataURL(address, { ...QR_OPTIONS, width: 1024 })
      triggerDownload(dataUrl, `${fileName}.png`)
      toast.success('QR kodu PNG olarak indirildi.')
    } catch (error) {
      toast.error(error.message || 'QR kodu oluşturulamadı.')
    } finally {
      setDownloading('')
    }
  }

  async function downloadSvg() {
    if (!address || downloading) return

    setDownloading('svg')
    let objectUrl = ''
    try {
      const svg = await QRCodeLib.toString(address, QR_SVG_OPTIONS)
      objectUrl = URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml;charset=utf-8' }))
      triggerDownload(objectUrl, `${fileName}.svg`)
      toast.success('QR kodu SVG olarak indirildi.')
    } catch (error) {
      toast.error(error.message || 'QR kodu oluşturulamadı.')
    } finally {
      // Without the revoke the blob is held for the whole page session; the
      // zero delay lets the anchor click be dispatched first.
      if (objectUrl) setTimeout(() => URL.revokeObjectURL(objectUrl), 0)
      setDownloading('')
    }
  }

  return (
    <div
      className={`print-card card flex flex-col p-5 ${hiddenInPrint ? 'print-hide' : ''}`}
    >
      {/* ------------------------------------------------------- identity */}
      <div className="mb-4">
        <div className="flex items-start gap-2">
          <h3 className="min-w-0 flex-1 truncate text-base font-semibold text-gray-900">{name}</h3>
          {menu?.is_active === false ? (
            <span className="badge shrink-0 bg-gray-100 text-gray-600">Pasif</span>
          ) : null}
        </div>
        {description ? (
          <p className="mt-1 text-sm text-gray-500" style={TWO_LINES}>
            {description}
          </p>
        ) : null}
      </div>

      {/* ------------------------------------------------------ QR preview */}
      <div className="mb-4 flex justify-center">
        {generating ? (
          <div className="flex h-64 w-64 items-center justify-center rounded-xl border border-dashed border-gray-200">
            <Loader2 className="h-6 w-6 animate-spin text-brand-600" aria-hidden="true" />
          </div>
        ) : preview ? (
          <img
            src={preview}
            alt={`${name} menü QR kodu`}
            width={256}
            height={256}
            className="h-64 w-64 rounded-xl bg-white object-contain"
          />
        ) : (
          <div className="flex h-64 w-64 flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-gray-200 text-gray-400">
            <QrCodeIcon className="h-8 w-8" aria-hidden="true" />
            <span className="text-xs">QR kodu oluşturulamadı</span>
          </div>
        )}
      </div>

      {/* -------------------------------------------------------- addresses */}
      <div className="print-hide space-y-3">
        <div>
          <label className="label mb-1" htmlFor={`qr-address-${menu?.id || slug}`}>
            Menü adresi
          </label>
          <div className="flex flex-col gap-2 sm:flex-row">
            <input
              id={`qr-address-${menu?.id || slug}`}
              type="text"
              readOnly
              value={address}
              onFocus={(event) => event.target.select()}
              className="input font-mono text-xs"
              aria-label={`${name} menü adresi`}
            />
            <button
              type="button"
              onClick={() => copy(address)}
              className="btn-secondary btn-sm shrink-0"
              disabled={!address}
            >
              <Copy className="h-3.5 w-3.5" aria-hidden="true" />
              Bağlantıyı kopyala
            </button>
          </div>
        </div>

        <div>
          <label className="label mb-1" htmlFor={`qr-path-${menu?.id || slug}`}>
            Yerel test adresi
          </label>
          <div className="flex flex-col gap-2 sm:flex-row">
            <input
              id={`qr-path-${menu?.id || slug}`}
              type="text"
              readOnly
              value={fallbackAddress}
              onFocus={(event) => event.target.select()}
              className="input font-mono text-xs"
              aria-label={`${name} yerel test adresi`}
            />
            <button
              type="button"
              onClick={() => copy(fallbackAddress)}
              className="btn-secondary btn-sm shrink-0"
              disabled={!fallbackAddress}
            >
              <Copy className="h-3.5 w-3.5" aria-hidden="true" />
              Kopyala
            </button>
          </div>
        </div>
      </div>

      {/* ----------------------------------------------------------- actions */}
      <div className="print-hide mt-4 flex flex-wrap gap-2 border-t border-gray-100 pt-4">
        <button
          type="button"
          onClick={downloadPng}
          className="btn-primary btn-sm"
          disabled={!address || Boolean(downloading)}
        >
          {downloading === 'png' ? (
            <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden="true" />
          ) : (
            <Download className="h-3.5 w-3.5" aria-hidden="true" />
          )}
          PNG indir
        </button>

        <button
          type="button"
          onClick={downloadSvg}
          className="btn-secondary btn-sm"
          disabled={!address || Boolean(downloading)}
        >
          {downloading === 'svg' ? (
            <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden="true" />
          ) : (
            <FileCode2 className="h-3.5 w-3.5" aria-hidden="true" />
          )}
          SVG indir
        </button>

        <button
          type="button"
          onClick={() => onPrint?.(menu?.id)}
          className="btn-secondary btn-sm"
          disabled={!preview}
        >
          <Printer className="h-3.5 w-3.5" aria-hidden="true" />
          Yazdır
        </button>

        {address ? (
          <a
            href={address}
            target="_blank"
            rel="noopener noreferrer"
            className="btn-ghost btn-sm ml-auto"
          >
            <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
            Menüyü aç
          </a>
        ) : null}
      </div>
    </div>
  )
}

/* -------------------------------------------------------------------- page */

export default function QrHub() {
  const { business } = useAuth()
  const toast = useToast()

  // The subdomain half of every address on this page. A menu slug alone does
  // not identify a menu: two businesses may both own "kahvalti".
  const businessSlug = business?.slug || ''

  const [menus, setMenus] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // The id of the card being printed on its own; null means "print the page".
  const [printingID, setPrintingID] = useState(null)

  useEffect(() => {
    let cancelled = false

    async function load() {
      setLoading(true)
      try {
        const list = await api.listMenus()
        if (cancelled) return
        setMenus(Array.isArray(list) ? list : [])
        setError('')
      } catch (err) {
        if (cancelled) return
        const message = err.message || 'Menüler yüklenemedi.'
        setError(message)
        toast.error(message)
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    load()
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Printing one card means hiding the others first, so the dialog only opens
  // after React has painted that layout.
  useEffect(() => {
    if (!printingID) return undefined

    const timer = setTimeout(() => {
      window.print()
      setPrintingID(null)
    }, 60)
    return () => clearTimeout(timer)
  }, [printingID])

  return (
    <div className="mx-auto max-w-content">
      <style>{PRINT_STYLE}</style>

      <div className="print-hide mb-6">
        <h2 className="text-xl font-semibold text-gray-900">QR Kodlar</h2>
        <p className="mt-1 text-sm text-gray-500">
          Her menünün kendi adresi ve kendi QR kodu vardır. Menüde yaptığınız değişiklikler anında
          yansır, kodu tekrar bastırmanıza gerek yoktur.
        </p>
      </div>

      {loading ? (
        <Loading text="Menüler yükleniyor" />
      ) : error ? (
        <div className="print-hide rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">
          {error}
        </div>
      ) : menus.length === 0 ? (
        <EmptyState
          className="print-hide"
          icon={QrCodeIcon}
          title="Henüz bir menünüz yok"
          description="QR kodlarını görüntülemek için önce bir menü oluşturun."
        />
      ) : (
        <>
          <div className="grid gap-6 lg:grid-cols-2 2xl:grid-cols-3">
            {menus.map((menu) => (
              <MenuQrCard
                key={menu.id}
                businessSlug={businessSlug}
                menu={menu}
                hiddenInPrint={Boolean(printingID) && printingID !== menu.id}
                onPrint={setPrintingID}
              />
            ))}
          </div>

          <div className="print-hide mt-6 flex items-start gap-3 rounded-xl border border-blue-200 bg-blue-50 p-4 text-sm text-blue-800">
            <Info className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
            <p className="leading-relaxed">
              QR kodlarını masalarınıza, menü standlarınıza veya kasanıza yerleştirebilirsiniz. Baskı
              için PNG (1024 px) yeterlidir; tabela ve büyük ölçekli baskılarda SVG dosyasını
              kullanın. Menü adresini değiştirirseniz basılı kodlar çalışmaz.
            </p>
          </div>
        </>
      )}
    </div>
  )
}
