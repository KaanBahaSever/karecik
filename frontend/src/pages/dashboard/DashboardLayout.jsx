import { useEffect, useState } from 'react'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import {
  ExternalLink,
  LayoutList,
  LogOut,
  Menu,
  Palette,
  QrCode,
  Settings,
  X,
} from 'lucide-react'

import { useAuth } from '../../lib/auth.jsx'
import { menuAdresi } from '../../lib/subdomain'
import Yukleniyor from '../../components/ui/Yukleniyor.jsx'

/* Sol menüdeki bağlantılar */
const MENU_OGELERI = [
  { to: '/panel', etiket: 'Menü Yönetimi', ikon: LayoutList, end: true },
  { to: '/panel/tasarim', etiket: 'Tasarım', ikon: Palette },
  { to: '/panel/qr', etiket: 'QR Kod', ikon: QrCode },
  { to: '/panel/ayarlar', etiket: 'Ayarlar', ikon: Settings },
]

/** NavLink sınıfı — aktif olan marka rengiyle vurgulanır. */
function baglantiSinifi({ isActive }) {
  const temel = 'flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm'
  return isActive
    ? `${temel} bg-marka-50 text-marka-700 font-medium`
    : `${temel} text-gray-600 hover:bg-gray-100 hover:text-gray-900`
}

/** Hem masaüstü kenar çubuğunda hem mobil çekmecede kullanılan menü gövdesi. */
function MenuGovdesi({ tiklandi }) {
  return (
    <nav className="flex-1 space-y-1 p-3">
      {MENU_OGELERI.map((oge) => {
        const Ikon = oge.ikon
        return (
          <NavLink
            key={oge.to}
            to={oge.to}
            end={oge.end}
            className={baglantiSinifi}
            onClick={tiklandi}
          >
            <Ikon className="h-4 w-4 shrink-0" aria-hidden="true" />
            <span>{oge.etiket}</span>
          </NavLink>
        )
      })}
    </nav>
  )
}

export default function PanelDuzeni() {
  const { business, logout } = useAuth()
  const navigate = useNavigate()
  const konum = useLocation()
  const [cekmeceAcik, setCekmeceAcik] = useState(false)

  // Sayfa değişince mobil çekmece kapansın
  useEffect(() => {
    setCekmeceAcik(false)
  }, [konum.pathname])

  if (!business) {
    return <Yukleniyor tamEkran metin="Panel hazırlanıyor..." />
  }

  const menuAdres = business.menu_url || menuAdresi(business.slug)

  function menuyuGor() {
    if (!menuAdres) return
    window.open(menuAdres, '_blank', 'noopener,noreferrer')
  }

  function cikisYap() {
    logout()
    navigate('/')
  }

  return (
    <div className="flex h-screen overflow-hidden bg-gray-50">
      {/* ---------------------------------------------- masaüstü kenar çubuğu */}
      <aside className="yazdirma-gizle hidden w-60 shrink-0 flex-col border-r border-gray-200 bg-white lg:flex">
        <div className="flex h-16 items-center gap-2 border-b border-gray-200 px-5">
          <span className="flex h-7 w-7 items-center justify-center rounded-md bg-marka-600 text-xs font-bold text-white">
            K
          </span>
          <span className="text-lg font-semibold text-gray-900">Karecik</span>
        </div>
        <MenuGovdesi />
      </aside>

      {/* ------------------------------------------------- mobil çekmece menü */}
      {cekmeceAcik ? (
        <div className="yazdirma-gizle fixed inset-0 z-50 lg:hidden">
          <div
            className="absolute inset-0 bg-black/40"
            onClick={() => setCekmeceAcik(false)}
            aria-hidden="true"
          />
          <div className="absolute inset-y-0 left-0 flex w-60 flex-col bg-white shadow-panel">
            <div className="flex h-16 items-center justify-between border-b border-gray-200 px-4">
              <div className="flex items-center gap-2">
                <span className="flex h-7 w-7 items-center justify-center rounded-md bg-marka-600 text-xs font-bold text-white">
                  K
                </span>
                <span className="text-lg font-semibold text-gray-900">Karecik</span>
              </div>
              <button
                type="button"
                onClick={() => setCekmeceAcik(false)}
                className="btn-sessiz btn-kucuk"
                aria-label="Menüyü kapat"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
            <MenuGovdesi tiklandi={() => setCekmeceAcik(false)} />
          </div>
        </div>
      ) : null}

      {/* ------------------------------------------------------ sağ ana bölüm */}
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="yazdirma-gizle flex h-16 shrink-0 items-center gap-3 border-b border-gray-200 bg-white px-4 sm:px-6">
          <button
            type="button"
            onClick={() => setCekmeceAcik(true)}
            className="btn-sessiz btn-kucuk lg:hidden"
            aria-label="Menüyü aç"
          >
            <Menu className="h-5 w-5" />
          </button>

          <h1 className="min-w-0 flex-1 truncate text-base font-semibold text-gray-900">
            {business.name}
          </h1>

          <button
            type="button"
            onClick={menuyuGor}
            className="btn-ikincil btn-kucuk"
            title={menuAdres}
          >
            <ExternalLink className="h-4 w-4" />
            <span className="hidden sm:inline">Menüyü Gör</span>
          </button>

          <button type="button" onClick={cikisYap} className="btn-sessiz btn-kucuk">
            <LogOut className="h-4 w-4" />
            <span className="hidden sm:inline">Çıkış</span>
          </button>
        </header>

        <main className="flex-1 overflow-y-auto bg-gray-50 p-4 sm:p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
