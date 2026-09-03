import { useEffect, useMemo, useState } from 'react'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import {
  ExternalLink,
  LayoutList,
  LogOut,
  Menu,
  Palette,
  QrCode,
  Settings,
  Store,
  X,
} from 'lucide-react'

import { useAuth } from '../../lib/auth.jsx'
import { useBranchMenu } from '../../lib/branchContext.jsx'
import { menuUrl } from '../../lib/subdomain'
import Loading from '../../components/ui/Loading.jsx'
import Logo from '../../components/ui/Logo.jsx'

/* Sidebar links */
const NAV_ITEMS = [
  { to: '/panel', label: 'Menü Yönetimi', icon: LayoutList, end: true },
  { to: '/panel/subeler', label: 'Şubeler ve Menüler', icon: Store },
  { to: '/panel/tasarim', label: 'Tasarım', icon: Palette },
  { to: '/panel/qr', label: 'QR Kod', icon: QrCode },
  { to: '/panel/ayarlar', label: 'Ayarlar', icon: Settings },
]

/* Compact select used by the topbar switcher, in both of its layouts. */
const SWITCHER_SELECT_CLASS =
  'w-full min-w-0 rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-xs text-gray-700 outline-none focus:border-brand-600 focus:ring-2 focus:ring-brand-100'

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

/**
 * Compact branch / menu picker that sits in the topbar.
 *
 * A single-venue business never sees it: one branch and one menu need no
 * switcher, and the topbar has no space to give away for chrome nobody uses.
 *
 * The topbar is a single non-wrapping row, so below `md` the two selects move
 * into a small popover behind one icon button instead of pushing the row onto
 * a second line.
 */
function BranchMenuSwitcher() {
  const { branches, menus, activeBranch, activeMenu, setActiveBranchId, setActiveMenuId } =
    useBranchMenu()
  const [open, setOpen] = useState(false)

  /* The menus this branch actually serves, in the branch's own order. A branch
     without an assignment serves everything — the same fallback branchContext
     resolves the active menu against. */
  const servedMenus = useMemo(() => {
    const ids = activeBranch?.menu_ids
    if (!Array.isArray(ids) || ids.length === 0) return menus
    const served = ids.map((id) => menus.find((menu) => menu.id === id)).filter(Boolean)
    return served.length > 0 ? served : menus
  }, [menus, activeBranch])

  if (branches.length <= 1 && menus.length <= 1) return null

  const showBranches = branches.length > 1
  // Kept on screen whenever the business has several menus, even if this branch
  // serves only one: a select that appears and disappears while switching
  // branches makes the whole topbar jump.
  const showMenus = menus.length > 1

  const branchSelect = (
    <select
      value={activeBranch?.id || ''}
      onChange={(event) => setActiveBranchId(event.target.value)}
      aria-label="Şube seç"
      className={SWITCHER_SELECT_CLASS}
    >
      {branches.map((branch) => (
        <option key={branch.id} value={branch.id}>
          {branch.name}
        </option>
      ))}
    </select>
  )

  const menuSelect = (
    <select
      value={activeMenu?.id || ''}
      onChange={(event) => setActiveMenuId(event.target.value)}
      aria-label="Menü seç"
      className={SWITCHER_SELECT_CLASS}
    >
      {servedMenus.map((menu) => (
        <option key={menu.id} value={menu.id}>
          {menu.name}
        </option>
      ))}
    </select>
  )

  return (
    <>
      {/* wide layout — the selects sit straight in the bar */}
      <div className="hidden min-w-0 shrink items-center gap-2 md:flex">
        {showBranches ? <div className="w-[9rem] shrink">{branchSelect}</div> : null}
        {showMenus ? <div className="w-[9rem] shrink">{menuSelect}</div> : null}
      </div>

      {/* narrow layout — one button, the same two selects in a popover */}
      <div className="relative shrink-0 md:hidden">
        <button
          type="button"
          onClick={() => setOpen((previous) => !previous)}
          className="btn-ghost btn-sm"
          aria-label="Şube ve menü seç"
          aria-expanded={open}
        >
          <Store className="h-4 w-4" aria-hidden="true" />
        </button>

        {open ? (
          <>
            <div
              className="fixed inset-0 z-40"
              onClick={() => setOpen(false)}
              aria-hidden="true"
            />
            {/* React's onChange bubbles, so one handler closes the popover
                whichever of the two selects was used. */}
            <div
              onChange={() => setOpen(false)}
              className="absolute right-0 top-full z-50 mt-2 w-56 space-y-3 rounded-lg border border-gray-200 bg-white p-3 shadow-panel"
            >
              {showBranches ? (
                <div>
                  <span className="label">Şube</span>
                  {branchSelect}
                </div>
              ) : null}
              {showMenus ? (
                <div>
                  <span className="label">Menü</span>
                  {menuSelect}
                </div>
              ) : null}
            </div>
          </>
        ) : null}
      </div>
    </>
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
          <Logo className="h-7 w-7 shrink-0 text-brand-600" title="" />
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
                <Logo className="h-7 w-7 shrink-0 text-brand-600" title="" />
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
        {/* Sticky so the topbar never shifts or scrolls away between pages. */}
        <header className="print-hide sticky top-0 z-30 flex h-16 shrink-0 items-center gap-3 border-b border-gray-200 bg-white px-4 sm:px-6">
          <button
            type="button"
            onClick={() => setDrawerOpen(true)}
            className="btn-ghost btn-sm shrink-0 lg:hidden"
            aria-label="Menüyü aç"
          >
            <Menu className="h-5 w-5" />
          </button>

          <h1 className="min-w-0 flex-1 truncate text-base font-semibold text-gray-900">
            {business.name}
          </h1>

          <BranchMenuSwitcher />

          <button
            type="button"
            onClick={openPublicMenu}
            className="btn-secondary btn-sm shrink-0"
            title={publicMenuUrl}
          >
            <ExternalLink className="h-4 w-4" />
            <span className="hidden sm:inline">Menüyü Gör</span>
          </button>

          <button type="button" onClick={signOut} className="btn-ghost btn-sm shrink-0">
            <LogOut className="h-4 w-4" />
            <span className="hidden sm:inline">Çıkış</span>
          </button>
        </header>

        {/* scroll-pt-16 keeps anchored scrolling clear of the sticky topbar. */}
        <main className="flex-1 scroll-pt-16 overflow-y-auto bg-gray-50 p-4 sm:p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
