import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { Check, ChevronDown, Globe } from 'lucide-react'

import { LANDING_LANGUAGES, landingText } from '../../locales/landing.js'

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
        Two-tier layout:

        phones (below sm) : TWO ROWS. Logo on top, language picker and buttons
                            below. On a single row the buttons take about 300px,
                            leaving no space for the "Karecik" wordmark, which
                            then got clipped. Moving them to a second row keeps
                            the wordmark fully visible without shrinking the logo.
        sm and above      : ONE ROW, logo at 56px / 34px (~1.55x the old size).
      */}
      <div className="mx-auto flex max-w-content flex-wrap items-center justify-between gap-x-2 gap-y-2.5 px-4 py-3 sm:h-24 sm:flex-nowrap sm:gap-4 sm:px-6 sm:py-0">
        <Link to="/" className="flex shrink-0 items-center gap-2.5 sm:gap-3" aria-label={t.brandHome}>
          <BrandMark />
          <span className="text-[1.375rem] font-bold leading-none tracking-tight text-gray-900 sm:text-[2.125rem]">
            Karecik
          </span>
        </Link>

        <nav className="flex w-full flex-wrap items-center justify-end gap-1.5 sm:w-auto sm:flex-nowrap sm:gap-2">
          <LanguagePicker
            language={language}
            onLanguageChange={onLanguageChange}
            label={t.selectLanguage}
          />

          <Link
            to="/giris"
            className="btn-ghost whitespace-nowrap px-2.5 py-2 text-xs sm:px-4 sm:py-2.5 sm:text-sm"
          >
            {t.signIn}
          </Link>
          <button
            type="button"
            onClick={onSignUpClick}
            className="btn-primary whitespace-nowrap px-2.5 py-2 text-xs sm:px-4 sm:py-2.5 sm:text-sm"
          >
            {t.startFree}
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
        className="flex items-center gap-1 rounded-lg px-2 py-2 text-xs font-medium text-gray-600 hover:bg-gray-100 hover:text-gray-900 sm:gap-1.5 sm:px-2.5 sm:text-sm"
      >
        <Globe className="h-4 w-4 shrink-0" aria-hidden="true" />
        <span>{selected.short}</span>
        <ChevronDown className="h-3.5 w-3.5 shrink-0 text-gray-400" aria-hidden="true" />
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
 * Brand mark: a plain four-block symbol reminiscent of a QR code.
 * The shape is unchanged; only the colour follows the corporate blue.
 */
function BrandMark() {
  return (
    <svg
      viewBox="0 0 24 24"
      className="h-9 w-9 shrink-0 sm:h-14 sm:w-14"
      fill="none"
      aria-hidden="true"
      focusable="false"
    >
      <rect x="1" y="1" width="22" height="22" rx="5.5" fill="#1d4ed8" />
      <rect x="5.5" y="5.5" width="5.5" height="5.5" rx="1.4" fill="#ffffff" />
      <rect x="13" y="5.5" width="5.5" height="5.5" rx="1.4" fill="#ffffff" fillOpacity="0.55" />
      <rect x="5.5" y="13" width="5.5" height="5.5" rx="1.4" fill="#ffffff" fillOpacity="0.55" />
      <rect x="13" y="13" width="5.5" height="5.5" rx="1.4" fill="#ffffff" />
    </svg>
  )
}
