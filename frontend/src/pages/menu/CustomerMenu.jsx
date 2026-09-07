import { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

import api from '../../lib/api'
import { getSubdomain } from '../../lib/subdomain'
import { t } from '../../locales/index.js'
import Loading from '../../components/ui/Loading.jsx'
import MenuContent from '../../components/menu/MenuContent.jsx'
import MenuDirectory from '../../components/menu/MenuDirectory.jsx'
import SplashScreen from '../../components/menu/SplashScreen.jsx'

/**
 * The customer menu opened by scanning a QR code.
 *
 *   {business-slug}.karecik.com / {menu-slug}
 *    └── identifies the tenant   └── identifies one menu within that tenant
 *
 * Two slugs reach this page and either of them may be missing:
 *   business — a prop (landing iframe), /m/:businessSlug, or the host subdomain
 *   menu     — a prop, /:menuSlug under a subdomain, /m/:businessSlug/:menuSlug
 *
 * A menu slug on its own is meaningless: menu slugs are unique only within a
 * business, so the two segments always travel together.
 *
 * When no menu was chosen the backend answers in one of three ways, and each
 * one gets its own screen here:
 *
 *   exactly one active menu  the backend resolves and serves it, so the menu is
 *                            already on screen and only the address is wrong —
 *                            it is replaced with that menu's own URL, which is
 *                            the link the visitor copies, bookmarks or shares
 *   two or more              `menu_resolved: false` -> <MenuDirectory />
 *   none                     `menu_resolved: false` with an empty list -> the
 *                            "no active menus" placeholder, which MenuDirectory
 *                            draws in the same layout
 *
 * @param {string}  businessSlug - Forces a tenant (optional)
 * @param {string}  menuSlug     - Forces one of its menus (optional)
 * @param {boolean} embedded     - Rendered inside an iframe / narrow container
 */
export default function CustomerMenu({
  businessSlug: businessSlugProp,
  menuSlug: menuSlugProp,
  embedded = false,
}) {
  const params = useParams()
  const navigate = useNavigate()

  const businessSlug = businessSlugProp || params.businessSlug || getSubdomain() || ''
  const menuSlug = menuSlugProp || params.menuSlug || ''

  // Stays empty until the visitor picks a language; the backend then serves the
  // menu's own default language.
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
        // With a business slug the path form is exact; without one the backend
        // resolves the tenant from the request host itself.
        const data = businessSlug
          ? await api.publicMenu(businessSlug, menuSlug, selectedLanguage)
          : await api.publicMenuByHost(menuSlug, selectedLanguage)
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
  }, [businessSlug, menuSlug, selectedLanguage])

  /* ------------------------------------------ one menu: fix the address bar */
  // The bare tenant address with a single active menu is served directly, so the
  // menu is already rendered and this navigation changes nothing on screen — but
  // the address is what gets shared, so it has to become the real one.
  //
  // The loop guard is the URL itself: the target carries a menu slug, so on the
  // very next render `menuSlug` is set and the condition below is false. A mount
  // that never read the URL must never navigate either — hence the `embedded`
  // bail-out, which is what keeps the landing page's iframe intact.
  useEffect(() => {
    if (embedded || menuSlug || !menu) return
    if (menu.menu_resolved === false) return

    const menus = Array.isArray(menu.menus) ? menu.menus : []
    if (menus.length !== 1) return

    const slug = menus[0]?.slug
    if (!slug) return

    // On a tenant subdomain the host already names the business; the path form
    // has to carry the business slug, so without one there is no address to go
    // to and the menu simply stays on the address the visitor used.
    if (getSubdomain()) {
      navigate(`/${slug}`, { replace: true })
    } else if (businessSlug) {
      navigate(`/m/${businessSlug}/${slug}`, { replace: true })
    }
  }, [embedded, menuSlug, menu, businessSlug, navigate])

  /* --------------------------------------------- scrollbar in embedded mode */
  /* The landing page renders this route inside an iframe, and a scrollbar down
     the side of the "phone" breaks the illusion completely.

     Hiding it with `no-scrollbar` alone was not enough. That class only asks the
     browser not to PAINT the document's scrollbar, and whether it obeys depends
     on the engine, on whether html or body is the scrolling element, and on the
     platform's overlay-scrollbar setting. The bar kept coming back.

     So the document is taken out of the scrolling business entirely: html and
     body are pinned to the viewport with `overflow: hidden`, and the wrapper
     below becomes the scroller instead. There is then no document scrollbar to
     suppress — it cannot exist — and the inner one is hidden by a class on an
     ordinary element, which every engine honours. Touch and wheel scrolling are
     untouched.

     Only the iframe reaches this: LivePreview hands `embedded` straight to
     MenuContent and never renders this component. */
  useEffect(() => {
    if (!embedded) return undefined

    const root = document.documentElement
    const previousRoot = root.style.cssText
    const previousBody = document.body.style.cssText

    root.classList.add('no-scrollbar')
    document.body.classList.add('no-scrollbar')
    root.style.height = '100%'
    root.style.overflow = 'hidden'
    document.body.style.height = '100%'
    document.body.style.overflow = 'hidden'

    return () => {
      root.classList.remove('no-scrollbar')
      document.body.classList.remove('no-scrollbar')
      root.style.cssText = previousRoot
      document.body.style.cssText = previousBody
    }
  }, [embedded])

  /* ---------------------------------------------------------- splash screen */
  useEffect(() => {
    if (embedded || !menu?.business?.splash_enabled) return
    if (splashShown.current) return

    // Both segments are in the key: menu slugs repeat across businesses, so a
    // menu slug on its own would let one tenant suppress another tenant's
    // splash screen.
    const tenantKey = menu.business.business_slug || businessSlug
    const key = `karecik_splash_${tenantKey}_${menu.business.menu_slug || menuSlug}`
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
  }, [menu, embedded, businessSlug, menuSlug])

  /* -------------------------------------------------------------- tab title */
  useEffect(() => {
    // `name` is the MENU name; the directory has no menu, so there the tenant
    // name is what the tab should read.
    const title = menu?.business?.name || menu?.business?.business_name
    if (embedded || !title) return undefined

    const previousTitle = document.title
    document.title = title
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

  // The tenant was found but no single menu was. The directory lists what it
  // has, and renders the "no active menus" placeholder when that list is empty.
  if (menu.menu_resolved === false) {
    return (
      <MenuDirectory
        business={menu.business}
        menus={menu.menus}
        language={activeLanguage}
        embedded={embedded}
      />
    )
  }

  const content = (
    <>
      {showSplash ? (
        <SplashScreen business={menu.business} onDone={() => setShowSplash(false)} />
      ) : null}

      {/* A URL that names a menu is a request for that one menu, so the in-menu
          switcher — a list of the tenant's other menus — is dropped there. The
          bare tenant address keeps it: nothing was chosen yet, and the backend
          resolved a single menu on the visitor's behalf. */}
      <MenuContent
        menu={menu}
        language={activeLanguage}
        onLanguageChange={(next) => setSelectedLanguage(next)}
        embedded={embedded}
        showMenuSwitcher={!menuSlug}
      />
    </>
  )

  /* Embedded, this wrapper is the scroller: the effect above pinned the
     document so it cannot scroll, and the bar now belongs to an ordinary
     element, where `no-scrollbar` is reliable across engines. Standalone the
     same tree is returned unwrapped — a real menu on a real phone scrolls the
     document, which is what it should do.

     It is a plain conditional rather than a wrapper component defined here: a
     component declared inside render is a new type on every render, so React
     would unmount and remount the whole menu on each keystroke in the search
     box, throwing away the selected category and the scroll position with it. */
  if (!embedded) return content

  return (
    <div className="no-scrollbar h-screen w-full overflow-y-auto overflow-x-hidden">
      {content}
    </div>
  )
}
