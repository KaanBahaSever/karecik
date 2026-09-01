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
import { menuUrl } from '../../lib/subdomain'
import Loading from '../../components/ui/Loading.jsx'

/* Sidebar links */
const NAV_ITEMS = [
  { to: '/panel', label: 'Menü Yönetimi', icon: LayoutList, end: true },
  { to: '/panel/tasarim', label: 'Tasarım', icon: Palette },
  { to: '/panel/qr', label: 'QR Kod', icon: QrCode },
  { to: '/panel/ayarlar', label: 'Ayarlar', icon: Settings },
]

/** NavLink class name — the active item is highlighted in the brand colour. */
function navLinkClass({ isActive }) {
  const base = 'flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm'
  return isActive
    ? `${base} bg-brand-50 text-brand-700 font-medium`
    : `${base} text-gray-600 hover:bg-gray-100 hover:text-gray-900`
}

/** Nav body shared by the desktop sidebar and the mobile drawer. */
function NavBody({ onNavigate }) {
  return (
    <nav className="flex-1 space-y-1 p-3">
      {NAV_ITEMS.map((item) => {
        const Icon = item.icon
        return (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.end}
            className={navLinkClass}
            onClick={onNavigate}
          >
            <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
            <span>{item.label}</span>
          </NavLink>
        )
      })}
    </nav>
  )
}

export default function DashboardLayout() {
  const { business, logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [drawerOpen, setDrawerOpen] = useState(false)

  // Close the mobile drawer whenever the route changes
  useEffect(() => {
    setDrawerOpen(false)
  }, [location.pathname])

  if (!business) {
    return <Loading fullScreen text="Panel hazırlanıyor..." />
  }

  const publicMenuUrl = business.menu_url || menuUrl(business.slug)

  function openPublicMenu() {
    if (!publicMenuUrl) return
    window.open(publicMenuUrl, '_blank', 'noopener,noreferrer')
  }

  function signOut() {
    logout()
    navigate('/')
  }

  return (
    <div className="flex h-screen overflow-hidden bg-gray-50">
      {/* --------------------------------------------------- desktop sidebar */}
      <aside className="print-hide hidden w-60 shrink-0 flex-col border-r border-gray-200 bg-white lg:flex">
        <div className="flex h-16 items-center gap-2 border-b border-gray-200 px-5">
          <span className="flex h-7 w-7 items-center justify-center rounded-md bg-brand-600 text-xs font-bold text-white">
            K
          </span>
          <span className="text-lg font-semibold text-gray-900">Karecik</span>
        </div>
        <NavBody />
      </aside>

      {/* ----------------------------------------------------- mobile drawer */}
      {drawerOpen ? (
        <div className="print-hide fixed inset-0 z-50 lg:hidden">
          <div
            className="absolute inset-0 bg-black/40"
            onClick={() => setDrawerOpen(false)}
            aria-hidden="true"
          />
          <div className="absolute inset-y-0 left-0 flex w-60 flex-col bg-white shadow-panel">
            <div className="flex h-16 items-center justify-between border-b border-gray-200 px-4">
              <div className="flex items-center gap-2">
                <span className="flex h-7 w-7 items-center justify-center rounded-md bg-brand-600 text-xs font-bold text-white">
                  K
                </span>
                <span className="text-lg font-semibold text-gray-900">Karecik</span>
              </div>
              <button
                type="button"
                onClick={() => setDrawerOpen(false)}
                className="btn-ghost btn-sm"
                aria-label="Menüyü kapat"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
            <NavBody onNavigate={() => setDrawerOpen(false)} />
          </div>
        </div>
      ) : null}

      {/* --------------------------------------------------------- main area */}
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="print-hide flex h-16 shrink-0 items-center gap-3 border-b border-gray-200 bg-white px-4 sm:px-6">
          <button
            type="button"
            onClick={() => setDrawerOpen(true)}
            className="btn-ghost btn-sm lg:hidden"
            aria-label="Menüyü aç"
          >
            <Menu className="h-5 w-5" />
          </button>

          <h1 className="min-w-0 flex-1 truncate text-base font-semibold text-gray-900">
            {business.name}
          </h1>

          <button
            type="button"
            onClick={openPublicMenu}
            className="btn-secondary btn-sm"
            title={publicMenuUrl}
          >
            <ExternalLink className="h-4 w-4" />
            <span className="hidden sm:inline">Menüyü Gör</span>
          </button>

          <button type="button" onClick={signOut} className="btn-ghost btn-sm">
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
