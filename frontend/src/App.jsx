import { Navigate, Route, Routes } from 'react-router-dom'

import { useAuth } from './lib/auth.jsx'
import { MenuProvider } from './lib/menuContext.jsx'
import { getSubdomain } from './lib/subdomain'
import Loading from './components/ui/Loading.jsx'

import Landing from './pages/Landing.jsx'
import Login from './pages/Login.jsx'
import SignUp from './pages/SignUp.jsx'
import ForgotPassword from './pages/ForgotPassword.jsx'
import ResetPassword from './pages/ResetPassword.jsx'
import CustomerMenu from './pages/menu/CustomerMenu.jsx'

import DashboardLayout from './pages/dashboard/DashboardLayout.jsx'
import MenuEditor from './pages/dashboard/MenuEditor.jsx'
import MenuSettings from './pages/dashboard/MenuSettings.jsx'
import QrHub from './pages/dashboard/QrHub.jsx'
import Account from './pages/dashboard/Account.jsx'
import { DEMO_BUSINESS_SLUG } from './lib/env'

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

      {/* Password reset. Both are public: someone who cannot log in is exactly
          who needs them. The reset page reads its token from ?token= rather
          than from the path, so the address stays the same whether or not the
          link carries one and the page can explain a missing token itself. */}
      <Route path="/sifremi-unuttum" element={<ForgotPassword />} />
      <Route path="/sifre-sifirla" element={<ResetPassword />} />

      {/* Path-based menu access — needs no hosts file entry. Both segments are
          required to name a menu; the first one alone is the tenant address. */}
      <Route path="/m/:businessSlug" element={<CustomerMenu />} />
      <Route path="/m/:businessSlug/:menuSlug" element={<CustomerMenu />} />

      {/* The sample venue inside the iPhone frame on the landing page.

          The tenant is NOT hardcoded any more. Automatic seeding is gone, so
          there is no venue this route can assume exists — a hardcoded slug would
          render "menu not found" inside the marketing page's phone on any
          deployment that was never seeded, which is every real one.

          VITE_DEMO_BUSINESS names it instead, and Landing.jsx simply omits the
          frame when it is unset. */}
      <Route
        path="/demo"
        element={<CustomerMenu businessSlug={DEMO_BUSINESS_SLUG} embedded />}
      />

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
        <Route path="hesap" element={<Account />} />
      </Route>

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
