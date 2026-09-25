import { useEffect, useState } from 'react'
import { Link, Navigate } from 'react-router-dom'

import Header from '../components/landing/Header.jsx'
import PhoneFrame from '../components/landing/PhoneFrame.jsx'
import Logo from '../components/ui/Logo.jsx'
import { DEMO_BUSINESS_SLUG } from '../lib/env'
import SignUpModal from '../components/landing/SignUpModal.jsx'
import { useAuth } from '../lib/auth.jsx'
import { landingText, readSavedLanguage, saveLanguage } from '../locales/landing.js'

/**
 * Karecik landing page.
 *
 * RULE: this page and everything under components/landing contains no animation.
 * No animation library is used, and the transition-*, animate-*, duration-* and
 * hover:scale-* classes are avoided. Only instant hover colour changes are used.
 */
/* Where the phone mockup stops being a live iframe and becomes a picture.
   1024px is the same breakpoint the hero uses to go two-column, so the desktop
   layout and the live demo arrive together. */
const LIVE_PREVIEW_QUERY = '(min-width: 1024px)'

/**
 * True on screens wide enough to be worth loading the live demo into.
 *
 * The point is NOT to hide the iframe on phones — a `hidden` iframe still
 * downloads the whole app a second time, which is exactly the cost being
 * avoided. This decides whether the element is rendered at all.
 */
function useLivePreview() {
  const [live, setLive] = useState(() => window.matchMedia(LIVE_PREVIEW_QUERY).matches)

  useEffect(() => {
    const query = window.matchMedia(LIVE_PREVIEW_QUERY)
    const onChange = (event) => setLive(event.matches)
    query.addEventListener('change', onChange)
    // Re-read once on mount: the first render may have raced a resize.
    setLive(query.matches)
    return () => query.removeEventListener('change', onChange)
  }, [])

  return live
}

/**
 * Puts the landing language on the document while the page is mounted: the
 * `lang` of <html> and the tab title. Both are restored on the way out.
 *
 * `lang` is not decoration. index.html ships lang="tr", and a page that says it
 * is Turkish while showing English makes the browser offer to "translate from
 * Turkish", makes screen readers read English words with Turkish pronunciation
 * and hyphenates by the wrong rules.
 *
 * `dir` is pinned to ltr for the same span. Every landing language is written
 * left to right, and a `dir="rtl"` left on <html> by a page visited before
 * (an Arabic customer menu, in this single-page app) would otherwise mirror the
 * header and send the language menu to the wrong edge.
 *
 * Restoring puts the attributes back exactly as they were — removed if they
 * were absent — so the next page finds the document untouched.
 */
function useDocumentLanguage(language, title) {
  useEffect(() => {
    const root = document.documentElement
    const previousLang = root.getAttribute('lang')
    const previousDir = root.getAttribute('dir')
    const previousTitle = document.title

    root.setAttribute('lang', language)
    root.setAttribute('dir', 'ltr')
    if (title) document.title = title

    return () => {
      restoreAttribute(root, 'lang', previousLang)
      restoreAttribute(root, 'dir', previousDir)
      document.title = previousTitle
    }
  }, [language, title])
}

function restoreAttribute(element, name, value) {
  if (value === null) element.removeAttribute(name)
  else element.setAttribute(name, value)
}

export default function Landing() {
  const { isAuthenticated } = useAuth()
  const [signUpOpen, setSignUpOpen] = useState(false)
  const [language, setLanguage] = useState(readSavedLanguage)
  const livePreview = useLivePreview()

  const t = landingText(language)

  // Declared before the redirect below: hooks may not sit behind an early return.
  useDocumentLanguage(language, t.documentTitle)

  function changeLanguage(next) {
    setLanguage(next)
    saveLanguage(next)
  }

  // Already signed in: go straight to the dashboard.
  if (isAuthenticated) return <Navigate to="/panel" replace />

  return (
    <div className="flex min-h-screen flex-col bg-white">
      <Header
        onSignUpClick={() => setSignUpOpen(true)}
        language={language}
        onLanguageChange={changeLanguage}
      />

      <main className="flex-1">
        {/*
          The right column is sized to its content (auto), so the leftover space
          goes to the left column and the gap between them. That pushes the phone
          against the right edge and widens the breathing room in the middle.
        */}
        <section className="mx-auto grid max-w-content grid-cols-1 items-center gap-14 px-4 py-16 sm:px-6 lg:grid-cols-[minmax(0,1fr)_auto] lg:gap-24 lg:py-24 lg:pr-2 xl:gap-28">
          {/* left column: the pitch */}
          <div className="max-w-xl">
            <h1 className="text-4xl font-bold tracking-tight text-gray-900 sm:text-5xl lg:text-6xl">
              {t.heading}
            </h1>

            <p className="mt-6 text-lg text-gray-600">{t.description}</p>

            {/* The bars are drawn separators, not words: hidden from screen
                readers, which would otherwise announce "vertical line". */}
            <div className="mt-5 flex flex-wrap items-center gap-3 text-sm text-gray-500">
              <span>{t.badgeFree}</span>
              <span className="text-gray-300" aria-hidden="true">|</span>
              <span>{t.badgeSetup}</span>
              <span className="text-gray-300" aria-hidden="true">|</span>
              <span>{t.badgeMobile}</span>
            </div>

            <div className="mt-9 flex flex-wrap items-center gap-3">
              <button
                type="button"
                onClick={() => setSignUpOpen(true)}
                className="btn-primary px-6 py-3 text-base"
              >
                {t.startFree}
              </button>
              <Link to="/giris" className="btn-secondary px-6 py-3 text-base">
                {t.signIn}
              </Link>
            </div>
          </div>

          {/* right column: a real sample menu built on the platform */}
          <div className="flex justify-center lg:justify-end">
            <PhoneFrame>
              {/*
                No `scrolling="no"` here: it would take the touch scrolling with
                it, and the whole point of the mockup is that the menu inside is
                real and can be browsed. The bar is hidden instead — `no-scrollbar`
                on the frame and on the iframe element, plus the same class that
                CustomerMenu puts on the embedded document while it is embedded.
              */}
              {!livePreview ? (
                /* Phones and tablets get a picture of the same menu.
                   The iframe boots a second copy of the whole app — another
                   ~470 kB of JavaScript plus an API round trip — to show
                   something the visitor mostly just looks at. This is 67 kB and
                   paints immediately.

                   The dimensions are the frame's own content box at 2x, so the
                   image is pixel-crisp on a phone and object-cover has almost
                   nothing to crop. Stating them keeps the layout from jumping
                   while it loads. */
                <img
                  src="/demo-onizleme.png"
                  alt={t.demoTitle}
                  width={632}
                  height={1336}
                  loading="lazy"
                  decoding="async"
                  className="h-full w-full object-cover object-top"
                />
              ) : DEMO_BUSINESS_SLUG ? (
                /* `?lang=` makes the sample menu follow this page's language
                   picker (CustomerMenu reads it as an explicit choice, never
                   remembered). Without it the menu would open in the browser's
                   own language — German inside a Turkish landing page, or the
                   other way round. A new language reloads the frame, which is
                   the price of it being a real, separate page. */
                <iframe
                  src={`/demo?lang=${encodeURIComponent(language)}`}
                  title={t.demoTitle}
                  className="no-scrollbar h-full w-full border-0"
                />
              ) : (
                /* No sample tenant is deployed. Rendering the iframe anyway
                   would put "menu bulunamadı" inside the phone on the page that
                   is selling the product, so the frame shows a neutral placeholder
                   instead. Set VITE_DEMO_BUSINESS to a real tenant slug to turn
                   the live preview back on. */
                <div className="flex h-full flex-col items-center justify-center gap-3 bg-gray-50 px-8 text-center">
                  <Logo className="h-10 w-10 text-brand-600" title="" />
                  <p className="text-sm font-medium text-gray-700">{t.demoPlaceholderTitle}</p>
                  <p className="text-xs leading-relaxed text-gray-500">{t.demoPlaceholderText}</p>
                </div>
              )}
            </PhoneFrame>
          </div>
        </section>
      </main>

      <footer className="border-t border-gray-200">
        <div className="mx-auto flex max-w-content flex-col items-center justify-between gap-3 px-4 py-6 sm:flex-row sm:px-6">
          <p className="text-sm text-gray-500">{t.copyright}</p>
          <div className="flex flex-wrap items-center justify-center gap-3 text-xs text-gray-400">
            <span>{t.footerPlatform}</span>
            <span className="text-gray-300" aria-hidden="true">|</span>
            <span>{t.footerSetup}</span>
            <span className="text-gray-300" aria-hidden="true">|</span>
            <span>{t.footerCard}</span>
          </div>
        </div>
      </footer>

      <SignUpModal
        open={signUpOpen}
        onClose={() => setSignUpOpen(false)}
        language={language}
      />
    </div>
  )
}
