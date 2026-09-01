import { useState } from 'react'
import { Link, Navigate } from 'react-router-dom'

import Header from '../components/landing/Header.jsx'
import PhoneFrame from '../components/landing/PhoneFrame.jsx'
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
export default function Landing() {
  const { isAuthenticated } = useAuth()
  const [signUpOpen, setSignUpOpen] = useState(false)
  const [language, setLanguage] = useState(readSavedLanguage)

  const t = landingText(language)

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

            <div className="mt-5 flex flex-wrap items-center gap-3 text-sm text-gray-500">
              <span>{t.badgeFree}</span>
              <span className="text-gray-300">|</span>
              <span>{t.badgeSetup}</span>
              <span className="text-gray-300">|</span>
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
              <iframe src="/demo" title={t.demoTitle} className="h-full w-full border-0" />
            </PhoneFrame>
          </div>
        </section>
      </main>

      <footer className="border-t border-gray-200">
        <div className="mx-auto flex max-w-content flex-col items-center justify-between gap-3 px-4 py-6 sm:flex-row sm:px-6">
          <p className="text-sm text-gray-500">{t.copyright}</p>
          <div className="flex flex-wrap items-center justify-center gap-3 text-xs text-gray-400">
            <span>{t.footerPlatform}</span>
            <span className="text-gray-300">|</span>
            <span>{t.footerSetup}</span>
            <span className="text-gray-300">|</span>
            <span>{t.footerCard}</span>
          </div>
        </div>
      </footer>

      <SignUpModal open={signUpOpen} onClose={() => setSignUpOpen(false)} />
    </div>
  )
}
