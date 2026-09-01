import { useEffect, useState } from 'react'
import { ArrowRight, Info, Loader2, Percent } from 'lucide-react'

import api from '../../lib/api'
import { formatPrice } from '../../lib/format'
import Modal from '../ui/Modal.jsx'
import { useToast } from '../ui/Toast.jsx'

/** Must match the constants in backend/internal/utils/pricing.go exactly. */
const ROUNDING_OPTIONS = [
  { value: 'none', label: 'Yuvarlama yok' },
  { value: 'integer', label: 'Tam sayıya (148)' },
  { value: 'nearest_5', label: "5'in katına (150)" },
  { value: 'nearest_10', label: "10'un katına (150)" },
  { value: 'ends_50', label: "0,50'nin katına (147,50)" },
  { value: 'ends_95', label: '…,95 ile bitir (147,95)' },
  { value: 'ends_99', label: '…,99 ile bitir (147,99)' },
]

const INCREASE_SHORTCUTS = [5, 10, 15, 20]
const DISCOUNT_SHORTCUTS = [-5, -10]

const MAX_PREVIEW_ROWS = 50

/** Extracts a category name from its translations. */
function categoryName(category) {
  const translations = category?.translations || {}
  return translations.tr?.name || Object.values(translations)[0]?.name || 'Adsız kategori'
}

/**
 * Bulk price update dialog (increase / discount plus rounding).
 *
 * @param {boolean}  open
 * @param {Function} onClose
 * @param {Array}    categories
 * @param {string}   currency
 * @param {Function} onApplied - lets the caller refresh its list afterwards
 */
export default function BulkPriceModal({ open, onClose, categories, currency, onApplied }) {
  const toast = useToast()
  const categoryList = Array.isArray(categories) ? categories : []

  const [percentage, setPercentage] = useState('')
  const [rounding, setRounding] = useState('none')
  const [allProducts, setAllProducts] = useState(true)
  const [selectedCategories, setSelectedCategories] = useState([])
  const [preview, setPreview] = useState(null)
  const [busy, setBusy] = useState(false)
  const [busyAction, setBusyAction] = useState(null) // 'preview' | 'apply'
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open) return
    setPercentage('')
    setRounding('none')
    setAllProducts(true)
    setSelectedCategories([])
    setPreview(null)
    setBusy(false)
    setBusyAction(null)
    setError('')
  }, [open])

  const percentageValue = Number(String(percentage).replace(',', '.'))
  const percentageValid = String(percentage).trim() !== '' && Number.isFinite(percentageValue)

  function toggleCategory(id) {
    setPreview(null)
    setSelectedCategories((previous) =>
      previous.includes(id) ? previous.filter((item) => item !== id) : [...previous, id],
    )
  }

  function requestBody(apply) {
    return {
      percentage: percentageValue,
      rounding,
      category_ids: allProducts ? [] : selectedCategories,
      apply,
    }
  }

  function validate() {
    if (!percentageValid) {
      setError('Bir yüzde değeri girin.')
      return false
    }
    if (percentageValue < -90 || percentageValue > 1000) {
      setError('Yüzde değeri -90 ile 1000 arasında olmalıdır.')
      return false
    }
    setError('')
    return true
  }

  async function runPreview() {
    if (!validate()) return

    setBusy(true)
    setBusyAction('preview')
    try {
      const result = await api.bulkPrice(requestBody(false))
      setPreview(result)
      if (!result?.affected) {
        toast.info('Bu ayarlarla değişecek bir fiyat bulunamadı.')
      }
    } catch (err) {
      toast.error(err.message)
    } finally {
      setBusy(false)
      setBusyAction(null)
    }
  }

  async function apply() {
    if (!validate()) return

    setBusy(true)
    setBusyAction('apply')
    try {
      const result = await api.bulkPrice(requestBody(true))
      toast.success(`${result?.affected ?? 0} ürünün fiyatı güncellendi.`)
      onApplied?.()
      onClose?.()
    } catch (err) {
      toast.error(err.message)
    } finally {
      setBusy(false)
      setBusyAction(null)
    }
  }

  const rows = Array.isArray(preview?.preview) ? preview.preview : []
  const visibleRows = rows.slice(0, MAX_PREVIEW_ROWS)
  const remaining = rows.length - visibleRows.length

  return (
    <Modal
      open={open}
      onClose={busy ? () => {} : onClose}
      title="Toplu Fiyat Güncelleme"
      description="Tüm menüye ya da seçtiğiniz kategorilere tek seferde zam veya indirim uygulayın."
      width="max-w-2xl"
      footer={
        <>
          <button type="button" className="btn-secondary" onClick={onClose} disabled={busy}>
            İptal
          </button>
          <button type="button" className="btn-secondary" onClick={runPreview} disabled={busy}>
            {busyAction === 'preview' ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
            Önizle
          </button>
          <button type="button" className="btn-primary" onClick={apply} disabled={busy}>
            {busyAction === 'apply' ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" /> Uygulanıyor
              </>
            ) : (
              'Uygula'
            )}
          </button>
        </>
      }
    >
      <div className="space-y-5">
        {/* ------------------------------------------------------ live summary */}
        {percentageValid && percentageValue !== 0 ? (
          <div className="rounded-lg border border-brand-200 bg-brand-50 px-4 py-3 text-sm font-medium text-brand-700">
            {percentageValue > 0
              ? `Tüm fiyatlara %${Math.abs(percentageValue)} zam uygulanacak.`
              : `Tüm fiyatlara %${Math.abs(percentageValue)} indirim uygulanacak.`}
          </div>
        ) : null}

        {/* -------------------------------------------------------- percentage */}
        <div>
          <label className="label" htmlFor="bulk-percentage">
            Değişim yüzdesi <span className="text-red-500">*</span>
          </label>
          <div className="relative">
            <input
              id="bulk-percentage"
              type="number"
              inputMode="decimal"
              min={-90}
              max={1000}
              step="0.1"
              className="input pr-9"
              value={percentage}
              onChange={(event) => {
                setPercentage(event.target.value)
                setPreview(null)
                setError('')
              }}
              placeholder="Örn. 10"
            />
            <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-gray-400">
              <Percent className="h-4 w-4" aria-hidden="true" />
            </span>
          </div>
          {error ? (
            <p className="error-text">{error}</p>
          ) : (
            <p className="help-text">-90 ile 1000 arasında bir değer girin. Eksi değer indirimdir.</p>
          )}

          <div className="mt-3 flex flex-wrap items-center gap-2">
            <span className="text-xs font-medium text-gray-500">Zam:</span>
            {INCREASE_SHORTCUTS.map((value) => (
              <button
                key={`increase-${value}`}
                type="button"
                disabled={busy}
                onClick={() => {
                  setPercentage(String(value))
                  setPreview(null)
                  setError('')
                }}
                className={`rounded-lg border px-3 py-1.5 text-xs font-medium ${
                  percentageValid && percentageValue === value
                    ? 'border-brand-600 bg-brand-50 text-brand-700'
                    : 'border-gray-300 text-gray-600 hover:bg-gray-50'
                }`}
              >
                +{value}
              </button>
            ))}

            <span className="ml-2 text-xs font-medium text-gray-500">İndirim:</span>
            {DISCOUNT_SHORTCUTS.map((value) => (
              <button
                key={`discount-${value}`}
                type="button"
                disabled={busy}
                onClick={() => {
                  setPercentage(String(value))
                  setPreview(null)
                  setError('')
                }}
                className={`rounded-lg border px-3 py-1.5 text-xs font-medium ${
                  percentageValid && percentageValue === value
                    ? 'border-brand-600 bg-brand-50 text-brand-700'
                    : 'border-gray-300 text-gray-600 hover:bg-gray-50'
                }`}
              >
                {value}
              </button>
            ))}
          </div>
        </div>

        {/* ---------------------------------------------------------- rounding */}
        <div>
          <label className="label" htmlFor="bulk-rounding">
            Fiyat yuvarlama
          </label>
          <select
            id="bulk-rounding"
            className="input"
            value={rounding}
            onChange={(event) => {
              setRounding(event.target.value)
              setPreview(null)
            }}
          >
            {ROUNDING_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
          <p className="help-text">
            Zam/indirim hesaplandıktan sonra fiyatlar bu kurala göre yuvarlanır.
          </p>
        </div>

        {/* ------------------------------------------------------------- scope */}
        <div>
          <span className="label">Kapsam</span>
          <div className="space-y-2">
            <label className="flex cursor-pointer items-center gap-3 rounded-lg border border-gray-200 px-3 py-2.5">
              <input
                type="radio"
                name="bulk-scope"
                className="h-4 w-4 border-gray-300 text-brand-600 focus:ring-brand-500"
                checked={allProducts}
                onChange={() => {
                  setAllProducts(true)
                  setPreview(null)
                }}
              />
              <span className="text-sm font-medium text-gray-700">Tüm ürünler</span>
            </label>

            <label className="flex cursor-pointer items-center gap-3 rounded-lg border border-gray-200 px-3 py-2.5">
              <input
                type="radio"
                name="bulk-scope"
                className="h-4 w-4 border-gray-300 text-brand-600 focus:ring-brand-500"
                checked={!allProducts}
                onChange={() => {
                  setAllProducts(false)
                  setPreview(null)
                }}
              />
              <span className="text-sm font-medium text-gray-700">Belirli kategoriler</span>
            </label>
          </div>

          {!allProducts ? (
            <div className="mt-2 max-h-48 space-y-1 overflow-y-auto rounded-lg border border-gray-200 p-2">
              {categoryList.length === 0 ? (
                <p className="px-1 py-2 text-sm text-gray-500">Henüz kategori yok.</p>
              ) : (
                categoryList.map((category) => (
                  <label
                    key={category.id}
                    className="flex cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 hover:bg-gray-50"
                  >
                    <input
                      type="checkbox"
                      className="h-4 w-4 shrink-0 rounded border-gray-300 text-brand-600 focus:ring-brand-500"
                      checked={selectedCategories.includes(category.id)}
                      onChange={() => toggleCategory(category.id)}
                    />
                    <span className="min-w-0 flex-1 truncate text-sm text-gray-700">
                      {category.icon ? `${category.icon} ` : ''}
                      {categoryName(category)}
                    </span>
                    {typeof category.product_count === 'number' ? (
                      <span className="shrink-0 text-xs text-gray-400">
                        {category.product_count} ürün
                      </span>
                    ) : null}
                  </label>
                ))
              )}
              <p className="px-1 pt-1 text-xs text-gray-500">
                Hiçbiri seçilmezse güncelleme tüm ürünlere uygulanır.
              </p>
            </div>
          ) : null}
        </div>

        {/* ---------------------------------------------------------- notice */}
        <div className="flex items-start gap-3 rounded-lg border border-blue-200 bg-blue-50 px-4 py-3">
          <Info className="mt-0.5 h-4 w-4 shrink-0 text-blue-600" aria-hidden="true" />
          <p className="text-sm leading-snug text-blue-800">
            Fiyat güncellemesinden sonra menünüzdeki “Fiyatlarımız … tarihinden itibaren
            geçerlidir” ibaresi otomatik olarak bugüne güncellenir.
          </p>
        </div>

        {/* --------------------------------------------------------- preview */}
        {preview ? (
          <div>
            <div className="mb-2 flex items-center justify-between gap-3">
              <p className="text-sm font-medium text-gray-700">
                {preview.affected ?? 0} üründe fiyat değişecek.
              </p>
              <span className="text-xs text-gray-400">{rows.length} ürün tarandı</span>
            </div>

            {rows.length === 0 ? (
              <p className="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 text-sm text-gray-500">
                Bu kapsamda güncellenecek ürün bulunamadı.
              </p>
            ) : (
              <div className="overflow-hidden rounded-lg border border-gray-200">
                <div className="max-h-72 overflow-y-auto">
                  <table className="w-full text-sm">
                    <thead className="sticky top-0 bg-gray-50 text-xs uppercase text-gray-500">
                      <tr>
                        <th className="px-3 py-2 text-left font-medium">Ürün</th>
                        <th className="px-3 py-2 text-right font-medium">Eski Fiyat</th>
                        <th className="px-3 py-2 text-right font-medium">Yeni Fiyat</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100">
                      {visibleRows.map((row) => (
                        <tr key={row.id}>
                          <td className="max-w-[220px] truncate px-3 py-2 text-gray-700">
                            {row.name}
                          </td>
                          <td className="whitespace-nowrap px-3 py-2 text-right text-gray-500">
                            {formatPrice(row.old_price, currency)}
                          </td>
                          <td className="whitespace-nowrap px-3 py-2 text-right font-medium text-brand-600">
                            <span className="inline-flex items-center justify-end gap-1">
                              <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
                              {formatPrice(row.new_price, currency)}
                            </span>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>

                {remaining > 0 ? (
                  <p className="border-t border-gray-200 bg-gray-50 px-3 py-2 text-xs text-gray-500">
                    ve {remaining} ürün daha
                  </p>
                ) : null}
              </div>
            )}
          </div>
        ) : null}
      </div>
    </Modal>
  )
}
