import { Link } from 'react-router-dom'

/**
 * Açılış sayfasının üst çubuğu.
 *
 * Not: Landing page kuralı gereği burada hiçbir animasyon/geçiş yoktur.
 * Yalnızca anlık renk değişimi yapan hover sınıfları kullanılır.
 *
 * @param {Function} kayitAc - Kayıt modalını açan çağrı
 */
export default function Header({ kayitAc }) {
  return (
    <header className="sticky top-0 z-40 border-b border-gray-200 bg-white">
      <div className="mx-auto flex h-16 max-w-icerik items-center justify-between gap-2 px-4 sm:gap-4 sm:px-6">
        <Link to="/" className="flex shrink-0 items-center gap-2" aria-label="Karecik ana sayfa">
          <KarecikIsareti />
          <span className="text-lg font-bold tracking-tight text-gray-900">Karecik</span>
        </Link>

        <nav className="flex items-center gap-1.5 sm:gap-2">
          <Link
            to="/giris"
            className="btn-sessiz whitespace-nowrap px-2.5 py-2 text-xs sm:px-4 sm:py-2.5 sm:text-sm"
          >
            Giriş Yap
          </Link>
          <button
            type="button"
            onClick={kayitAc}
            className="btn-birincil whitespace-nowrap px-2.5 py-2 text-xs sm:px-4 sm:py-2.5 sm:text-sm"
          >
            Hemen Ücretsiz Başla
          </button>
        </nav>
      </div>
    </header>
  )
}

/** Marka işareti: QR karesini andıran dört bloklu sade simge. */
function KarecikIsareti() {
  return (
    <svg
      viewBox="0 0 24 24"
      className="h-7 w-7 shrink-0"
      fill="none"
      aria-hidden="true"
      focusable="false"
    >
      <rect x="1" y="1" width="22" height="22" rx="5.5" fill="#1a7f5a" />
      <rect x="5.5" y="5.5" width="5.5" height="5.5" rx="1.4" fill="#ffffff" />
      <rect x="13" y="5.5" width="5.5" height="5.5" rx="1.4" fill="#ffffff" fillOpacity="0.55" />
      <rect x="5.5" y="13" width="5.5" height="5.5" rx="1.4" fill="#ffffff" fillOpacity="0.55" />
      <rect x="13" y="13" width="5.5" height="5.5" rx="1.4" fill="#ffffff" />
    </svg>
  )
}
