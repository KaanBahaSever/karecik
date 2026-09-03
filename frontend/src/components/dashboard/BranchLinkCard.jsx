import { useState } from 'react'
import QRCodeLib from 'qrcode'
import { Copy, Download, Loader2, QrCode as QrCodeIcon } from 'lucide-react'

import { menuUrl } from '../../lib/subdomain'
import { useToast } from '../ui/Toast.jsx'

/* One read-only address with its copy button. */
function AddressRow({ id, label, value, onCopy, children }) {
  return (
    <div>
      <label className="label mb-1" htmlFor={id}>
        {label}
      </label>
      <div className="flex flex-col gap-2 sm:flex-row">
        <input
          id={id}
          type="text"
          readOnly
          value={value}
          onFocus={(event) => event.target.select()}
          className="input font-mono text-xs"
          aria-label={label}
        />
        <div className="flex shrink-0 gap-2">
          <button
            type="button"
            onClick={() => onCopy(value)}
            className="btn-secondary btn-sm"
            disabled={!value}
          >
            <Copy className="h-3.5 w-3.5" aria-hidden="true" />
            Kopyala
          </button>
          {children}
        </div>
      </div>
    </div>
  )
}

/**
 * The public addresses of one (branch, menu) pair plus its QR code.
 *
 * The QR is generated on click and never on mount: a business with eight
 * branches and three menus would otherwise render twenty-four codes nobody
 * asked for.
 *
 * @param {object}  branch
 * @param {object}  menu
 * @param {boolean} isBranchDefault - true when the bare branch address opens
 *                                    this menu, so its slug is not appended
 */
export default function BranchLinkCard({ branch, menu, isBranchDefault = false }) {
  const toast = useToast()

  const [generating, setGenerating] = useState(false)

  const branchSlug = branch?.slug || ''
  const menuSlug = menu?.slug || ''

  // menuUrl() already answers with the subdomain form in production and with
  // the local <slug>.localhost:5173 form in development.
  const base = branchSlug ? menuUrl(branchSlug) : ''
  const address = base ? `${base}${isBranchDefault ? '' : `/${menuSlug}`}` : ''

  // Path fallback for machines without a hosts file entry for the subdomain.
  const pathAddress =
    branchSlug && menuSlug ? `${window.location.origin}/b/${branchSlug}/${menuSlug}` : ''

  const fieldID = `branch-link-${branch?.id || 'x'}-${menu?.id || 'x'}`

  async function copy(text) {
    if (!text) return
    try {
      await navigator.clipboard.writeText(text)
      toast.success('Adres kopyalandı.')
    } catch {
      toast.error('Adres kopyalanamadı.')
    }
  }

  async function downloadQr() {
    if (!address || generating) return

    setGenerating(true)
    try {
      // The same options the QR page uses, so both produce identical prints.
      const dataUrl = await QRCodeLib.toDataURL(address, {
        width: 1024,
        margin: 2,
        color: { dark: '#111827', light: '#ffffff' },
        errorCorrectionLevel: 'H',
      })

      const link = document.createElement('a')
      link.href = dataUrl
      link.download = `karecik-qr-${branchSlug}-${menuSlug}.png`
      document.body.appendChild(link)
      link.click()
      link.remove()

      toast.success('QR kodu indirildi.')
    } catch (error) {
      toast.error(error.message || 'QR kodu oluşturulamadı.')
    } finally {
      setGenerating(false)
    }
  }

  return (
    <div className="rounded-lg border border-gray-200 bg-gray-50/60 p-3">
      <div className="mb-3 flex items-center gap-2">
        <QrCodeIcon className="h-4 w-4 shrink-0 text-gray-400" aria-hidden="true" />
        <span className="truncate text-sm font-semibold text-gray-900">
          {menu?.name || 'Menü'}
        </span>
        {isBranchDefault ? (
          <span className="badge shrink-0 bg-brand-50 text-brand-700">Şube varsayılanı</span>
        ) : null}
        {menu?.is_active === false ? (
          <span className="badge shrink-0 bg-gray-100 text-gray-600">Pasif</span>
        ) : null}
      </div>

      <div className="space-y-3">
        <AddressRow
          id={`${fieldID}-main`}
          label="Menü adresi"
          value={address}
          onCopy={copy}
        >
          <button
            type="button"
            onClick={downloadQr}
            className="btn-secondary btn-sm"
            disabled={!address || generating}
          >
            {generating ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden="true" />
            ) : (
              <Download className="h-3.5 w-3.5" aria-hidden="true" />
            )}
            QR indir
          </button>
        </AddressRow>

        <AddressRow
          id={`${fieldID}-path`}
          label="Yerel test adresi"
          value={pathAddress}
          onCopy={copy}
        />
      </div>
    </div>
  )
}
