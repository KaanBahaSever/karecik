import { useEffect, useLayoutEffect, useMemo, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { Loader2 } from 'lucide-react'

import api from '../../lib/api'
import { trackMenuEvent } from '../../lib/analytics'
import {
  browserLanguages,
  readRememberedLanguage,
  rememberLanguage,
  resolveMenuLanguage,
} from '../../lib/language'
import { getSubdomain } from '../../lib/subdomain'
import { LANGUAGE_CODES, languageDir, t } from '../../locales/index.js'
import { themeVariables } from '../../themes/themes'
import ErrorBoundary from '../../components/ui/ErrorBoundary.jsx'
import MenuContent from '../../components/menu/MenuContent.jsx'
import MenuDirectory from '../../components/menu/MenuDirectory.jsx'
import MenuFooter from '../../components/menu/MenuFooter.jsx'
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
 * The menus whose `menu_view` this page load has already reported, as
 * "business/menu" keys.
 *
 * Module level on purpose: it is what "once per page load" means. The payload
 * is fetched again on every language switch, the component may be remounted
 * by a route change inside the same tenant, and React's development mode runs
 * every effect twice — none of those is a second visit, and none of them
 * outlives the page, which this Set does not either.
 */
const reportedMenuViews = new Set()

/**
 * Neutral --menu-* values for the footer of the status screens: the theme
 * defaults MenuDirectory draws with too. No menu data goes into them — on those
 * screens there is either no menu or one that failed to draw.
 */
const NEUTRAL_THEME = themeVariables(undefined, undefined, undefined)

/**
 * The language named by the address's `?lang=`, or '' when it names none the
 * interface is written in. Lowercased, so `?lang=DE` works as typed.
 *
 * @param {string|null} raw
 * @returns {string}
 */
function addressLanguage(raw) {
  const code = typeof raw === 'string' ? raw.trim().toLowerCase() : ''
  return LANGUAGE_CODES.includes(code) ? code : ''
}

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
 * The full-screen frame of the three states drawn without a menu: loading,
 * unavailable and the render error. Each is in the visitor's language and
 * direction, which here is all the page has to go on (see lib/language.js).
 *
 * `signature` adds the standard footer at the bottom in its directory scope:
 * "Powered by Karecik" alone, since there is no menu whose prices, VAT note or
 * badge it could state. The two screens a visitor can be left on carry it,
 * like every other public view; the loading screen does not, where it would
 * only flash for a moment before the menu's own footer takes its place.
 */
function StatusFrame({ embedded, language, signature = false, children }) {
  return (
    <div
      className={`flex flex-col bg-white px-6 ${embedded ? 'h-full' : 'min-h-screen'}`}
      dir={languageDir(language)}
      lang={language}
    >
      <div className="flex flex-1 items-center justify-center">{children}</div>
      {signature ? (
        <div className="mx-auto w-full max-w-lg" style={NEUTRAL_THEME}>
          <MenuFooter business={null} footer={null} language={language} scope="directory" />
        </div>
      ) : null}
    </div>
  )
}

/**
 * The loading state. Drawn here rather than with the shared Loading component,
 * whose screen-reader text is the dashboard's Turkish: a visitor reading the
 * menu in Arabic is told it is loading in Arabic. `role="status"` announces
 * the one visible line, so nothing is read twice.
 */
function MenuLoading({ embedded, language }) {
  return (
    <StatusFrame embedded={embedded} language={language}>
      <div role="status" className="flex flex-col items-center justify-center gap-3 py-12">
        <Loader2 className="h-6 w-6 animate-spin text-brand-600" aria-hidden="true" />
        <p className="text-sm text-gray-500">{t('loading', language)}</p>
      </div>
    </StatusFrame>
  )
}

/**
 * No menu to show. Two different answers get two different screens:
 *
 *   notFound  the server looked and there is no such menu (404): the address
 *             is wrong, and trying again would change nothing
 *   otherwise the request failed — the venue's Wi-Fi dropped, the server was
 *             restarting: the menu is probably fine, so the visitor gets a
 *             button to try again
 *
 * The server's own error text is Turkish and written for owners; it is not
 * shown to visitors.
 *
 * @param {boolean}  embedded
 * @param {string}   language
 * @param {boolean}  notFound
 * @param {Function} onRetry  - Fetches the menu again
 */
function MenuUnavailable({ embedded, language, notFound, onRetry }) {
  return (
    <StatusFrame embedded={embedded} language={language} signature>
      <div className="w-full max-w-sm rounded-2xl border border-gray-200 p-8 text-center">
        <p className="text-4xl" aria-hidden="true">
          🔍
        </p>
        <h1 className="mt-4 text-lg font-semibold text-gray-900">
          {t(notFound ? 'notFound' : 'loadFailed', language)}
        </h1>
        <p className="mt-1.5 text-sm text-gray-600">
          {t(notFound ? 'notFoundDetail' : 'loadFailedDetail', language)}
        </p>
        {notFound ? null : (
          <button
            type="button"
            onClick={onRetry}
            className="mt-6 w-full rounded-lg bg-gray-900 px-4 py-2.5 text-sm font-medium text-white hover:bg-gray-800"
          >
            {t('retry', language)}
          </button>
        )}
      </div>
    </StatusFrame>
  )
}

/**
 * Drawn in place of the menu when MenuContent throws while rendering.
 *
 * Plain Tailwind greys on purpose — nothing from the menu's theme. The menu's
 * own theme data may be exactly what broke, and a fallback that leaned on it
 * could fail the same way; the signature footer under it reads the neutral
 * defaults only. "Try again" renders the same payload once more; "Reload page"
 * reloads the page and fetches everything again.
 *
 * @param {boolean}  embedded - Fill the iframe instead of the screen
 * @param {string}   language - Active language code
 * @param {Function} onRetry  - Resets the error boundary
 */
function MenuRenderError({ embedded, language, onRetry }) {
  return (
    <StatusFrame embedded={embedded} language={language} signature>
      <div
        role="alert"
        className="w-full max-w-sm rounded-2xl border border-gray-200 p-8 text-center"
      >
        <p className="text-base font-semibold text-gray-900">{t('renderError', language)}</p>

        <div className="mt-6 flex flex-col gap-2">
          <button
            type="button"
            onClick={onRetry}
            className="w-full rounded-lg bg-gray-900 px-4 py-2.5 text-sm font-medium text-white hover:bg-gray-800"
          >
            {t('retry', language)}
          </button>
          <button
            type="button"
            onClick={() => window.location.reload()}
            className="w-full rounded-lg border border-gray-300 bg-white px-4 py-2.5 text-sm font-medium text-gray-700 hover:bg-gray-50"
          >
            {t('reload', language)}
          </button>
        </div>
      </div>
    </StatusFrame>
  )
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
  const [searchParams, setSearchParams] = useSearchParams()

  const businessSlug = businessSlugProp || params.businessSlug || getSubdomain() || ''
  const menuSlug = menuSlugProp || params.menuSlug || ''

  /* ------------------------------------------------------------- language */
  /*
    Which language the menu is shown in is lib/language.js's decision; see the
    order written out there. What this component holds is its inputs:

      selectedLanguage    a language the visitor tapped on this visit
      urlLanguage         a language the ADDRESS names with ?lang= — how the
                          landing page's demo iframe follows the language
                          picked on the landing page, and how a shared link
                          can open a menu in a given language. An explicit
                          request like a tap, but never remembered: nobody
                          chose it on this device. A tap rewrites it (see
                          chooseLanguage), so it never outranks a later pick
      rememberedLanguage  the one they tapped on an earlier visit to this
                          tenant, read from localStorage — and never on an
                          embedded surface, which stays side-effect free
      syncLanguage        see "an older server" below; normally empty

    The FIRST request carries the address's or the remembered pick when
    there is one, and no language otherwise, which lets the server answer in
    the browser's own language from Accept-Language. Either way the first
    paint is already in the right language: no second request, and no flash
    of another language.
  */
  const [selectedLanguage, setSelectedLanguage] = useState('')
  const urlLanguage = addressLanguage(searchParams.get('lang'))
  const rememberedLanguage = useMemo(
    () => (embedded ? '' : readRememberedLanguage(businessSlug)),
    [embedded, businessSlug],
  )
  const [syncLanguage, setSyncLanguage] = useState('')
  const requestLanguage = selectedLanguage || syncLanguage || urlLanguage || rememberedLanguage
  // The browser's preferences do not change while the page is open.
  const [preferredLanguages] = useState(() => browserLanguages())

  const [menu, setMenu] = useState(null)
  const [loading, setLoading] = useState(true)
  // null, or { notFound } — see MenuUnavailable.
  const [error, setError] = useState(null)
  // Bumped by the "try again" button to repeat the request.
  const [reloadKey, setReloadKey] = useState(0)

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

  /* The directory's payload carries no languages of its own (it is not a
     menu), so there, as on the loading and error screens, every language the
     interface is written in is on offer. */
  const menuLanguages = menu && menu.menu_resolved !== false ? menu.business?.languages : []
  const activeLanguage = resolveMenuLanguage({
    choice: selectedLanguage || urlLanguage || rememberedLanguage,
    served: menu?.language,
    preferred: preferredLanguages,
    available: menuLanguages,
    fallback: menu?.business?.default_language,
  })

  /* The tenant a pick is remembered for: the payload's own slug once there is
     one, the address's before that. */
  const tenantSlug = menu?.business?.business_slug || businessSlug

  /* An explicit pick is remembered for the next visit — here, in the event
     handler, never in render. Tapping the language already on screen changes
     nothing and fetches nothing, but it is still a choice worth keeping.

     An address that names a language has it replaced by the pick, in place,
     without a new history entry. Left alone, the ?lang= of the link the
     visitor came in on would outrank the pick they just made as soon as the
     page is reloaded; rewritten, a reload and a copied link both keep the
     language on screen. Embedded surfaces leave their address alone: the
     landing page owns its iframe's ?lang=. */
  function chooseLanguage(next) {
    if (!embedded) {
      rememberLanguage(tenantSlug, next)
      if (searchParams.has('lang') && searchParams.get('lang') !== next) {
        setSearchParams(
          (current) => {
            const updated = new URLSearchParams(current)
            updated.set('lang', next)
            return updated
          },
          { replace: true },
        )
      }
    }
    if (next !== activeLanguage) setSelectedLanguage(next)
  }

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
      setError(null)
      try {
        // With a business slug the path form is exact; without one the backend
        // resolves the tenant from the request host itself. An empty language
        // leaves the choice to the server's Accept-Language negotiation.
        const data = businessSlug
          ? await api.publicMenu(businessSlug, menuSlug, requestLanguage)
          : await api.publicMenuByHost(menuSlug, requestLanguage)
        if (cancelled) return
        setMenu(data)
      } catch (err) {
        if (cancelled) return
        setError({ notFound: err?.status === 404 })
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    loadMenu()
    return () => {
      cancelled = true
    }
  }, [businessSlug, menuSlug, requestLanguage, reloadKey])

  /* --------------------------------------------------- an older server */
  /* A server that does not report `language` resolved the texts in the
     requested language when the menu offers it, else in its default one — and
     then activeLanguage may have settled on the browser's language instead
     (step 3 of lib/language.js). The interface and the texts would disagree,
     so the menu is fetched once more in the language the interface chose.
     The current server always reports `language`, and this never runs. */
  useEffect(() => {
    if (!menu || menu.menu_resolved === false) return
    if (typeof menu.language === 'string' && menu.language !== '') return

    const languages = Array.isArray(menu.business?.languages) ? menu.business.languages : []
    const textLanguage = languages.includes(requestLanguage)
      ? requestLanguage
      : menu.business?.default_language
    if (textLanguage && activeLanguage !== textLanguage && languages.includes(activeLanguage)) {
      setSyncLanguage(activeLanguage)
    }
  }, [menu, requestLanguage, activeLanguage])

  /* ------------------------------------------- the document's language */
  /* Standalone, the page IS the menu, so the document itself takes the
     language and its direction: the browser's own UI (scrollbars, form
     controls, text selection handles) follows `dir`, screen readers and
     hyphenation follow `lang`, and anything a component ever portals out of
     the menu's tree inherits both. A layout effect, so a right-to-left menu is
     never painted left-to-right first. The previous values come back on the
     way out — the landing page is Turkish and left-to-right.

     Embedded surfaces never touch the document: the landing iframe's menu and
     the dashboard preview set `dir` and `lang` on their own container only. */
  useLayoutEffect(() => {
    if (embedded) return undefined

    const root = document.documentElement
    const previousLang = root.getAttribute('lang')
    const previousDir = root.getAttribute('dir')
    root.setAttribute('lang', activeLanguage)
    root.setAttribute('dir', languageDir(activeLanguage))

    return () => {
      if (previousLang === null) root.removeAttribute('lang')
      else root.setAttribute('lang', previousLang)
      if (previousDir === null) root.removeAttribute('dir')
      else root.setAttribute('dir', previousDir)
    }
  }, [embedded, activeLanguage])

  /* ------------------------------------------------------ visitor analytics */
  /* One `menu_view` per resolved menu per page load — see reportedMenuViews.
     Never from an embedded surface (the landing page's demo iframe), never for
     the tenant directory, which is not a menu. Category and product views are
     reported by MenuContent, under the same rules. */
  useEffect(() => {
    if (embedded || !menu || menu.menu_resolved === false) return

    const tenant = menu.business?.business_slug
    const slug = menu.business?.menu_slug
    if (!tenant || !slug) return

    const key = `${tenant}/${slug}`
    if (reportedMenuViews.has(key)) return
    reportedMenuViews.add(key)
    trackMenuEvent({ businessSlug: tenant, menuSlug: slug, type: 'menu_view', language: activeLanguage })
  }, [embedded, menu, activeLanguage])

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

    // The query string travels along: a ?lang= dropped here would switch the
    // menu back out of the language the link asked for, one render later.
    const search = searchParams.toString()
    const suffix = search ? `?${search}` : ''

    // On a tenant subdomain the host already names the business; the path form
    // has to carry the business slug, so without one there is no address to go
    // to and the menu simply stays on the address the visitor used.
    if (getSubdomain()) {
      navigate(`/${slug}${suffix}`, { replace: true })
    } else if (businessSlug) {
      navigate(`/m/${businessSlug}/${slug}${suffix}`, { replace: true })
    }
  }, [embedded, menuSlug, menu, businessSlug, navigate, searchParams])

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
  /* Until a menu is in, the tab says what the screen says, in the visitor's
     language, rather than index.html's Turkish title, which is the landing
     page's. Mirrors the render below: loading, then "not found" or "could not
     load". */
  let statusTitleKey = ''
  if (!menu) {
    if (loading) statusTitleKey = 'loading'
    else statusTitleKey = error && !error.notFound ? 'loadFailed' : 'notFound'
  }

  useEffect(() => {
    // `name` is the MENU name; the directory has no menu, so there the tenant
    // name is what the tab should read.
    const title = statusTitleKey
      ? t(statusTitleKey, activeLanguage)
      : menu?.business?.name || menu?.business?.business_name
    if (embedded || !title) return undefined

    const previousTitle = document.title
    document.title = title
    return () => {
      document.title = previousTitle
    }
  }, [embedded, menu, statusTitleKey, activeLanguage])

  /* ----------------------------------------------------------------- render */

  if (loading && !menu) {
    return <MenuLoading embedded={embedded} language={activeLanguage} />
  }

  if (!menu) {
    return (
      <MenuUnavailable
        embedded={embedded}
        language={activeLanguage}
        notFound={error ? error.notFound : true}
        onRetry={() => setReloadKey((key) => key + 1)}
      />
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
          language={activeLanguage}
          onDone={() => setSplash({ decided: true, open: false })}
        />
      ) : null}

      {/* The last line of defence. MenuContent already isolates every category
          card and every product row, but anything else it throws would unmount
          the whole tree and leave the visitor a blank white page. A new payload
          gets a fresh try as well. */}
      <ErrorBoundary
        resetKeys={[menu]}
        fallback={(_error, reset) => (
          <MenuRenderError embedded={embedded} language={activeLanguage} onRetry={reset} />
        )}
      >
        {/* A URL that names a menu is a request for that one menu, so the in-menu
            switcher — a list of the tenant's other menus — is dropped there. The
            bare tenant address keeps it: nothing was chosen yet, and the backend
            resolved a single menu on the visitor's behalf. */}
        <MenuContent
          menu={menu}
          language={activeLanguage}
          onLanguageChange={chooseLanguage}
          embedded={embedded}
          showMenuSwitcher={!menuSlug}
        />
      </ErrorBoundary>
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
