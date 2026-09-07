import { Navigate, Route, Routes } from 'react-router-dom'

import { useAuth } from './lib/auth.jsx'
import { MenuProvider } from './lib/menuContext.jsx'
import { getSubdomain } from './lib/subdomain'
import Loading from './components/ui/Loading.jsx'

import Landing from './pages/Landing.jsx'
import Login from './pages/Login.jsx'
import SignUp from './pages/SignUp.jsx'
import CustomerMenu from './pages/menu/CustomerMenu.jsx'

import DashboardLayout from './pages/dashboard/DashboardLayout.jsx'
import MenuEditor from './pages/dashboard/MenuEditor.jsx'
import MenuSettings from './pages/dashboard/MenuSettings.jsx'
import QrHub from './pages/dashboard/QrHub.jsx'

/** Guards routes that require an active session. */
function ProtectedRoute({ children }) {
  const { isAuthenticated, loading } = useAuth()

  if (loading) return <Loading fullScreen text="Oturum kontrol ediliyor..." />
  if (!isAuthenticated) return <Navigate to="/giris" replace />
  return children
}

export default function App() {
  //   {business-slug}.karecik.com / {menu-slug}
  //    └── identifies the tenant   └── identifies one menu within that tenant
  //
  // The subdomain names the BUSINESS, never a menu — menu slugs are unique only
  // within a business, so the path is what picks one out.
  const subdomain = getSubdomain()

  if (subdomain) {
    return (
      <Routes>
        {/* The bare tenant address. CustomerMenu decides from the payload:
            one menu -> replace the address with that menu's own URL,
            two or more -> the directory, none -> the empty placeholder. */}
        <Route path="/" element={<CustomerMenu businessSlug={subdomain} />} />

        {/* One menu of this tenant */}
        <Route path="/:menuSlug" element={<CustomerMenu businessSlug={subdomain} />} />

        {/* Nothing deeper than one segment exists under a tenant subdomain */}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    )
  }

  return (
    <Routes>
      <Route path="/" element={<Landing />} />
      <Route path="/giris" element={<Login />} />
      <Route path="/kayit" element={<SignUp />} />

      {/* Path-based menu access — needs no hosts file entry. Both segments are
          required to name a menu; the first one alone is the tenant address. */}
      <Route path="/m/:businessSlug" element={<CustomerMenu />} />
      <Route path="/m/:businessSlug/:menuSlug" element={<CustomerMenu />} />

      {/* The sample venue inside the iPhone frame on the landing page.

          It points at the FICTIONAL karecik-kafe, not at a real customer. It
          used to point at melly-coffee, and the day that tenant gained its
          second menu the iframe quietly stopped showing a menu at all: with no
          menu slug and two menus to choose from, the backend answers
          menu_resolved:false and the page renders the menu DIRECTORY. A picker
          in the shop window, on a page selling menus.

          karecik-kafe publishes exactly one menu, so the backend resolves it on
          its own and the frame always shows an actual menu. */}
      <Route path="/demo" element={<CustomerMenu businessSlug="karecik-kafe" embedded />} />

      <Route
        path="/panel"
        element={
          <ProtectedRoute>
            {/* The menu being edited is shared by every dashboard page, and only
                by them: the landing page and the customer menu have no session,
                so the provider must stay inside the guard. */}
            <MenuProvider>
              <DashboardLayout />
            </MenuProvider>
          </ProtectedRoute>
        }
      >
        <Route index element={<MenuEditor />} />
        <Route path="ayarlar" element={<MenuSettings />} />
        <Route path="qr" element={<QrHub />} />
      </Route>

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
