import { useEffect, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'

import api from '../../lib/api'
import { getSubdomain } from '../../lib/subdomain'
import { t } from '../../locales/index.js'
import Loading from '../../components/ui/Loading.jsx'
import MenuContent from '../../components/menu/MenuContent.jsx'
import SplashScreen from '../../components/menu/SplashScreen.jsx'

/**
 * The customer menu opened by scanning a QR code.
 *
 * The slug can arrive in three ways:
 *   1. as a prop     -> <CustomerMenu slug="demo-kafe" />  (landing page iframe)
 *   2. from the path -> /m/:slug                            (fallback address)
 *   3. from the host -> kahve-duragi.karecik.com            (primary address)
 *
 * @param {string}  slug     - Forces a specific business (optional)
 * @param {boolean} embedded - Rendered inside an iframe / narrow container
 */
export default function CustomerMenu({ slug: slugProp, embedded = false }) {
  const { slug: slugParam } = useParams()
  const slug = slugProp || slugParam || getSubdomain() || ''

  // Stays empty until the visitor picks a language; the backend then serves the
  // business' own default language.
  const [selectedLanguage, setSelectedLanguage] = useState('')
  const [menu, setMenu] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showSplash, setShowSplash] = useState(false)

  // Keeps the splash screen from replaying when the language changes.
  const splashShown = useRef(false)

  const activeLanguage = selectedLanguage || menu?.business?.default_language || 'tr'

  /* ------------------------------------------------------------ load menu */
  useEffect(() => {
    let cancelled = false

    async function loadMenu() {
      setLoading(true)
      setError('')
      try {
        const data = slug
          ? await api.publicMenu(slug, selectedLanguage)
          : await api.publicMenuByHost(selectedLanguage)
        if (cancelled) return
        setMenu(data)
      } catch (err) {
        if (cancelled) return
        setError(err.message || 'Menü yüklenemedi.')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    loadMenu()
    return () => {
      cancelled = true
    }
  }, [slug, selectedLanguage])

  /* --------------------------------------------- scrollbar in embedded mode */
  // The phone frame on the landing page renders this page inside an iframe.
  // The iframe's own document scrollbar spoils the device illusion, so it is
  // hidden while embedded — scrolling itself keeps working.
  useEffect(() => {
    if (!embedded) return undefined

    const root = document.documentElement
    root.classList.add('no-scrollbar')
    document.body.classList.add('no-scrollbar')

    return () => {
      root.classList.remove('no-scrollbar')
      document.body.classList.remove('no-scrollbar')
    }
  }, [embedded])

  /* ---------------------------------------------------------- splash screen */
  useEffect(() => {
    if (embedded || !menu?.business?.splash_enabled) return
    if (splashShown.current) return

    const key = `karecik_splash_${menu.business.slug || slug}`
    try {
      if (sessionStorage.getItem(key)) {
        splashShown.current = true
        return
      }
      sessionStorage.setItem(key, '1')
    } catch {
      /* sessionStorage may be disabled in private windows — show it once anyway */
    }

    splashShown.current = true
    setShowSplash(true)
  }, [menu, embedded, slug])

  /* -------------------------------------------------------------- tab title */
  useEffect(() => {
    if (embedded || !menu?.business?.name) return undefined

    const previousTitle = document.title
    document.title = menu.business.name
    return () => {
      document.title = previousTitle
    }
  }, [embedded, menu])

  /* ----------------------------------------------------------------- render */

  if (loading && !menu) {
    return (
      <div
        className={`flex items-center justify-center bg-white ${
          embedded ? 'h-full' : 'min-h-screen'
        }`}
      >
        <Loading text={t('loading', activeLanguage)} />
      </div>
    )
  }

  if (!menu) {
    return (
      <div
        className={`flex items-center justify-center bg-white px-6 ${
          embedded ? 'h-full' : 'min-h-screen'
        }`}
      >
        <div className="w-full max-w-sm rounded-2xl border border-gray-200 p-8 text-center">
          <p className="text-4xl" aria-hidden="true">
            🔍
          </p>
          <h1 className="mt-4 text-lg font-semibold text-gray-900">
            {t('notFound', activeLanguage)}
          </h1>
          <p className="mt-1.5 text-sm text-gray-600">{t('notFoundDetail', activeLanguage)}</p>
          {error ? <p className="mt-4 text-xs text-gray-400">{error}</p> : null}
        </div>
      </div>
    )
  }

  return (
    <>
      {showSplash ? (
        <SplashScreen business={menu.business} onDone={() => setShowSplash(false)} />
      ) : null}

      <MenuContent
        menu={menu}
        language={activeLanguage}
        onLanguageChange={(next) => setSelectedLanguage(next)}
        embedded={embedded}
      />
    </>
  )
}
