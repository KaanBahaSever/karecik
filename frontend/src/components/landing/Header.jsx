import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { Check, ChevronDown, Globe } from 'lucide-react'

import { LANDING_LANGUAGES, landingText } from '../../locales/landing.js'
import Logo from '../ui/Logo.jsx'

/**
 * Top bar of the landing page.
 *
 * Note: per the landing page rule there is no animation or transition here.
 * Only instant hover colour changes are used, and the dropdown appears and
 * disappears without any transition.
 *
 * @param {Function} onSignUpClick    - Opens the sign-up dialog
 * @param {string}   language         - Selected language code (tr | en | de)
 * @param {Function} onLanguageChange - Called when the language changes
 */
export default function Header({ onSignUpClick, language = 'tr', onLanguageChange }) {
  const t = landingText(language)

  return (
    <header className="sticky top-0 z-40 border-b border-gray-200 bg-white">
      {/*
        ONE ROW at every width.

        It used to wrap onto a second row on phones — the brand needs ~124px and
        the three controls needed ~286px, against 328px of usable width at 360px,
        so the nav dropped below the wordmark and the bar grew to 102px tall.
        Wrapping was the old fix; it just looked broken.

        What actually buys the space, measured at 360px rather than guessed:

          the wordmark shrinks         22px -> 18px       ~14px
          the language chevron goes    (below sm)         ~14px
          "Giriş Yap" leaves the bar   (below sm)         ~71px
          the CTA uses a short label   see startFreeShort ~50px

        Signing in is not lost with the link gone: the hero below carries the
        same "Giriş Yap" button, and it is the returning visitor's obvious
        target on a page this short. Everything comes back at sm and above,
        where there is room for it.
      */}
      <div className="mx-auto flex h-16 max-w-content flex-nowrap items-center justify-between gap-2 px-4 sm:h-24 sm:gap-4 sm:px-6">
        <Link
          to="/"
          className="flex min-w-0 shrink items-center gap-2 sm:gap-3"
          aria-label={t.brandHome}
        >
          <BrandMark />
          <span className="truncate text-lg font-bold leading-none tracking-tight text-gray-900 sm:text-[2.125rem]">
            Karecik
          </span>
        </Link>

        <nav className="flex shrink-0 flex-nowrap items-center gap-1 sm:gap-2">
          <LanguagePicker
            language={language}
            onLanguageChange={onLanguageChange}
            label={t.selectLanguage}
          />

          <Link
            to="/giris"
            className="btn-ghost hidden whitespace-nowrap px-2.5 py-2 text-xs sm:inline-flex sm:px-4 sm:py-2.5 sm:text-sm"
          >
            {t.signIn}
          </Link>
          <button
            type="button"
            onClick={onSignUpClick}
            className="btn-primary whitespace-nowrap px-3 py-2 text-xs sm:px-4 sm:py-2.5 sm:text-sm"
          >
            {/* Two labels rather than a JS breakpoint: CSS decides, so there is
                no resize listener and nothing to get wrong before first paint. */}
            <span className="sm:hidden">{t.startFreeShort}</span>
            <span className="hidden sm:inline">{t.startFree}</span>
          </button>
        </nav>
      </div>
    </header>
  )
}

/**
 * Minimalist language picker.
 *
 * It shows short codes (TR / EN / DE) rather than flag emoji: Windows cannot
 * render those and prints the country code instead, which made English appear
 * as "GB".
 */
function LanguagePicker({ language, onLanguageChange, label }) {
  const [open, setOpen] = useState(false)
  const containerRef = useRef(null)

  const selected = LANDING_LANGUAGES.find((item) => item.code === language) || LANDING_LANGUAGES[0]

  // Close on outside click and on Escape
  useEffect(() => {
    if (!open) return undefined

    function onClickOutside(event) {
      if (containerRef.current && !containerRef.current.contains(event.target)) {
        setOpen(false)
      }
    }
    function onKeyDown(event) {
      if (event.key === 'Escape') setOpen(false)
    }

    document.addEventListener('mousedown', onClickOutside)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('mousedown', onClickOutside)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [open])

  function select(code) {
    onLanguageChange?.(code)
    setOpen(false)
  }

  return (
    <div className="relative" ref={containerRef}>
      <button
        type="button"
        onClick={() => setOpen((previous) => !previous)}
        aria-label={label}
        aria-haspopup="listbox"
        aria-expanded={open}
        className="flex items-center gap-1 rounded-lg px-1.5 py-2 text-xs font-medium text-gray-600 hover:bg-gray-100 hover:text-gray-900 sm:gap-1.5 sm:px-2.5 sm:text-sm"
      >
        <Globe className="h-4 w-4 shrink-0" aria-hidden="true" />
        <span>{selected.short}</span>
        {/* The chevron is affordance, not information — the globe and the code
            already read as a picker. It is the cheapest thing to drop when the
            phone navbar is short of room. */}
        <ChevronDown
          className="hidden h-3.5 w-3.5 shrink-0 text-gray-400 sm:block"
          aria-hidden="true"
        />
      </button>

      {open ? (
        <ul
          role="listbox"
          aria-label={label}
          className="absolute right-0 top-full z-50 mt-1 w-40 overflow-hidden rounded-lg border border-gray-200 bg-white py-1 shadow-panel"
        >
          {LANDING_LANGUAGES.map((option) => {
            const isSelected = option.code === language
            return (
              <li key={option.code}>
                <button
                  type="button"
                  role="option"
                  aria-selected={isSelected}
                  onClick={() => select(option.code)}
                  className={`flex w-full items-center justify-between gap-2 px-3 py-2 text-left text-sm hover:bg-gray-50 ${
                    isSelected ? 'font-medium text-brand-700' : 'text-gray-700'
                  }`}
                >
                  <span className="flex items-center gap-2">
                    <span className="w-6 shrink-0 text-xs font-semibold text-gray-400">
                      {option.short}
                    </span>
                    <span>{option.name}</span>
                  </span>
                  {isSelected ? (
                    <Check className="h-4 w-4 shrink-0 text-brand-600" aria-hidden="true" />
                  ) : null}
                </button>
              </li>
            )
          })}
        </ul>
      ) : null}
    </div>
  )
}

/**
 * Brand mark at the landing-page size (32px on phones, 56px from sm up).
 * The artwork itself is the shared logo — see components/ui/Logo.jsx.
 */
function BrandMark() {
  return <Logo className="h-8 w-8 shrink-0 text-brand-600 sm:h-14 sm:w-14" title="" />
}
