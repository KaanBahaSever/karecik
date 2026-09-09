import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

import api from '../../lib/api'
import { getSubdomain } from '../../lib/subdomain'
import { t } from '../../locales/index.js'
import Loading from '../../components/ui/Loading.jsx'
import MenuContent from '../../components/menu/MenuContent.jsx'
import MenuDirectory from '../../components/menu/MenuDirectory.jsx'
import SplashScreen from '../../components/menu/SplashScreen.jsx'

/**
 * How long the splash may wait for the menu's hero images before giving up.
 *
 * There has to be a cap. A logo hosted on someone else's slow server — this
 * menu's is — would otherwise hold the curtain shut indefinitely, turning a
 * polish feature into a broken site. Two seconds is past the point where
 * waiting still feels like loading and starts to feel like nothing happened.
 */
const HERO_IMAGE_CAP_MS = 2000

/**
 * sessionStorage key for "this visitor has already seen this menu's splash".
 *
 * BOTH segments are in the key. Menu slugs are unique only within a business,
 * so keying on the menu slug alone would let one tenant's visit suppress
 * another tenant's splash screen.
 */
function splashKey(business, businessSlug, menuSlug) {
  const tenant = business?.business_slug || businessSlug
  return `karecik_splash_${tenant}_${business?.menu_slug || menuSlug}`
}

/**
 * Whether the splash should open, answered SYNCHRONOUSLY from the menu payload.
 *
 * Everything it needs is already in hand the moment the fetch resolves, which
 * is what lets the caller decide during render instead of in an effect. It only
 * reads storage; the matching write lives in an effect, because a render must
 * not have side effects.
 */
function shouldOpenSplash(menu, embedded, businessSlug, menuSlug) {
  // The dashboard's live preview drives SplashScreen itself, with its own
  // replay button; a second one opening here would fight it.
  if (embedded) return false
  if (!menu?.business?.splash_enabled) return false
  // The directory is a list of menus, not a menu. Nothing to introduce.
  if (menu.menu_resolved === false) return false

  try {
    return !sessionStorage.getItem(splashKey(menu.business, businessSlug, menuSlug))
  } catch {
    // Private windows can refuse storage entirely. Showing the splash once per
    // load is the better failure than never showing it at all.
    return true
  }
}

/**
 * True once every listed image has settled — loaded OR failed — or the cap ran
 * out. Never blocks on the outcome, only on the wait being over.
 *
 * A failed image counts as ready on purpose: a broken URL is a permanent state,
 * and holding the splash for something that is never going to arrive would
 * punish the visitor for the owner's bad link.
 *
 * The effect keys on the joined URL string rather than the array, because a new
 * array identity on every render would restart the loads forever.
 */
function useImagesReady(urls, capMs) {
  const key = urls.join('|')
  const [readyFor, setReadyFor] = useState(null)

  useEffect(() => {
    if (!key) {
      setReadyFor(key)
      return undefined
    }

    let live = true
    let pending = urls.length
    const finish = () => {
      if (live) {
        live = false
        setReadyFor(key)
      }
    }

    const timer = setTimeout(finish, capMs)
    const loaders = urls.map((src) => {
      const image = new Image()
      const settle = () => {
        pending -= 1
        if (pending <= 0) finish()
      }
      image.onload = settle
      image.onerror = settle
      image.src = src
      return image
    })

    return () => {
      live = false
      clearTimeout(timer)
      // Drop the handlers rather than the requests: the browser keeps the
      // downloads warm in its cache, which is the point of preloading them.
      loaders.forEach((image) => {
        image.onload = null
        image.onerror = null
      })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- `key` IS `urls`
  }, [key, capMs])

  return readyFor === key
}

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

  /* ---------------------------------------------------------- splash state */
  /*
    DECIDED DURING RENDER, NEVER IN AN EFFECT. This is the whole fix for the
    opening flash, so it is worth being precise about why.

    An effect runs AFTER React has committed the DOM, and the browser is free to
    paint in between. Turning the splash on from an effect therefore produced
    this sequence: the fetch resolves -> the menu renders -> the browser paints
    the bare menu -> the effect finally runs -> the splash slams down on top.

    On a fast desktop React usually flushes the effect before the paint, so the
    bug is invisible there — which is exactly why it survived. It is a RACE, and
    a phone loses it. Measured on this menu with the CPU throttled:

        4x   menu painted at 302ms, splash at 431ms    ->  129ms bare
        10x  menu painted at 657ms, splash at 1063ms   ->  406ms bare
        20x  menu painted at 1315ms, splash at 2530ms  -> 1215ms bare

    Adjusting state during render is React's own answer to "derive state from
    something you just received": React discards the in-progress render and
    re-runs the component immediately with the new state, before anything
    reaches the screen. There is no frame in between to leak the menu.

    `decided` is what stops the splash replaying. The menu object is refetched
    whenever the visitor switches language, and a new object identity must not
    reopen a screen they have already dismissed.
  */
  const [splash, setSplash] = useState({ decided: false, open: false })

  if (menu && !splash.decided) {
    setSplash({ decided: true, open: shouldOpenSplash(menu, embedded, businessSlug, menuSlug) })
  }

  const activeLanguage = selectedLanguage || menu?.business?.default_language || 'tr'

  /* The images the splash is covering for. Waiting on these is what makes the
     handover a reveal rather than a second flash — see useImagesReady. Product
     photographs are deliberately absent: they are below the fold and lazy, and
     blocking on them would hold the curtain for a screenful nobody has scrolled
     to yet. */
  const heroImages = useMemo(() => {
    const business = menu?.business
    if (!business) return []
    return [
      business.splash_logo_url,
      business.logo_url,
      business.cover_url,
      business.background_image_url,
    ].filter(Boolean)
  }, [menu])

  const heroReady = useImagesReady(heroImages, HERO_IMAGE_CAP_MS)

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

  /* ------------------------------------------- remember the splash was shown */
  /* The decision above only READS sessionStorage, which a render may do — it is
     external state, like a media query. The WRITE belongs here: a render must
     not have side effects, and React may run one and throw it away. */
  useEffect(() => {
    if (!splash.open) return
    try {
      sessionStorage.setItem(splashKey(menu?.business, businessSlug, menuSlug), '1')
    } catch {
      /* private windows can refuse storage; the splash simply shows again */
    }
  }, [splash.open, menu, businessSlug, menuSlug])

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
      {/* MenuContent is rendered UNDERNEATH this, not instead of it, and that is
          deliberate: the menu lays out and its images decode while the splash
          holds, so lifting the curtain reveals a finished screen instead of
          starting the work. `ready` is what closes the loop — the exit waits
          for those images, capped, so a slow logo cannot strand the visitor. */}
      {splash.open ? (
        <SplashScreen
          business={menu.business}
          ready={heroReady}
          onDone={() => setSplash({ decided: true, open: false })}
        />
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
