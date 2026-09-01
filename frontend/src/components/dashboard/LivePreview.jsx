import { useEffect, useState } from 'react'
import { AlertCircle, Eye } from 'lucide-react'

import api from '../../lib/api'
import { currencySymbol } from '../../lib/format'
import { loadFont } from '../../themes/fonts'
import { findLanguage } from '../../locales/index.js'
import Loading from '../ui/Loading.jsx'
import MenuContent from '../menu/MenuContent.jsx'

/**
 * Phone-shaped preview that shows how dashboard changes look in the customer menu.
 *
 * The menu content comes from the server (api.previewMenu), while the appearance
 * settings come from the `business` prop — which may still be unsaved. That is
 * what lets a theme, font or colour change show up before it is persisted.
 *
 * @param {object} business - Draft (possibly unsaved) business settings
 * @param {number} refresh  - Bump this value to refetch the menu
 * @param {string} className
 */
export default function LivePreview({ business, refresh = 0, className = '' }) {
  const [language, setLanguage] = useState(business?.default_language || 'tr')
  const [menu, setMenu] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const languages =
    Array.isArray(business?.languages) && business.languages.length ? business.languages : ['tr']

  // If the selected language is removed from the business, fall back to the first.
  useEffect(() => {
    if (!languages.includes(language)) setLanguage(languages[0])
  }, [languages.join(','), language]) // eslint-disable-line react-hooks/exhaustive-deps

  // Load the selected font right away.
  useEffect(() => {
    if (business?.font_family) loadFont(business.font_family)
  }, [business?.font_family])

  // Fetch the menu content.
  useEffect(() => {
    let cancelled = false

    async function loadMenu() {
      setLoading(true)
      setError('')
      try {
        const data = await api.previewMenu(language)
        if (cancelled) return
        setMenu(data)
      } catch (err) {
        if (cancelled) return
        // No toast here: a preview failure should not be noisy.
        setError(err.message || 'Önizleme yüklenemedi.')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    loadMenu()
    return () => {
      cancelled = true
    }
  }, [refresh, language])

  // Merge the draft settings over the business object returned by the server.
  const previewMenu = menu
    ? {
        ...menu,
        business: {
          ...menu.business,
          ...(business || {}),
          currency_symbol: currencySymbol(business?.currency),
        },
      }
    : null

  return (
    <div className={className}>
      {/* header row */}
      <div className="mb-3 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2 text-sm font-medium text-gray-700">
          <Eye className="h-4 w-4 text-brand-600" aria-hidden="true" />
          <span>Canlı Önizleme</span>
        </div>

        {languages.length > 1 ? (
          <select
            value={language}
            onChange={(event) => setLanguage(event.target.value)}
            aria-label="Önizleme dili"
            className="rounded-lg border border-gray-300 bg-white px-2 py-1 text-xs text-gray-700 outline-none focus:border-brand-600 focus:ring-2 focus:ring-brand-100"
          >
            {languages.map((code) => {
              const info = findLanguage(code)
              return (
                <option key={code} value={code}>
                  {info.short} {info.label}
                </option>
              )
            })}
          </select>
        ) : null}
      </div>

      {/* phone frame */}
      <div className="rounded-[2.25rem] bg-gray-900 p-2.5 shadow-panel">
        <div className="relative h-[640px] overflow-hidden rounded-[1.75rem] bg-white">
          {/* notch */}
          <div className="pointer-events-none absolute left-1/2 top-2 z-10 h-1.5 w-20 -translate-x-1/2 rounded-full bg-gray-900/15" />

          <div className="no-scrollbar h-full overflow-y-auto overflow-x-hidden">
            {loading ? (
              <Loading text="Önizleme hazırlanıyor..." />
            ) : error ? (
              <div className="flex h-full items-center justify-center p-6">
                <div className="flex items-start gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2.5 text-left">
                  <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-red-600" aria-hidden="true" />
                  <div>
                    <p className="text-xs font-medium text-red-800">Önizleme yüklenemedi</p>
                    <p className="mt-0.5 text-[11px] leading-snug text-red-700">{error}</p>
                  </div>
                </div>
              </div>
            ) : previewMenu ? (
              <MenuContent
                menu={previewMenu}
                language={language}
                onLanguageChange={(next) => setLanguage(next)}
                embedded
              />
            ) : null}
          </div>
        </div>
      </div>

      <p className="mt-3 text-center text-xs text-gray-500">
        Değişiklikler burada anında görünür, kaydedene kadar müşterilere yansımaz.
      </p>
    </div>
  )
}
