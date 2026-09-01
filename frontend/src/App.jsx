import { Navigate, Route, Routes } from 'react-router-dom'

import { useAuth } from './lib/auth.jsx'
import { getSubdomain } from './lib/subdomain'
import Loading from './components/ui/Loading.jsx'

import Landing from './pages/Landing.jsx'
import Login from './pages/Login.jsx'
import SignUp from './pages/SignUp.jsx'
import CustomerMenu from './pages/menu/CustomerMenu.jsx'

import DashboardLayout from './pages/dashboard/DashboardLayout.jsx'
import MenuEditor from './pages/dashboard/MenuEditor.jsx'
import Design from './pages/dashboard/Design.jsx'
import Settings from './pages/dashboard/Settings.jsx'
import QrCode from './pages/dashboard/QrCode.jsx'

/** Guards routes that require an active session. */
function ProtectedRoute({ children }) {
  const { isAuthenticated, loading } = useAuth()

  if (loading) return <Loading fullScreen text="Oturum kontrol ediliyor..." />
  if (!isAuthenticated) return <Navigate to="/giris" replace />
  return children
}

export default function App() {
  // When the visitor arrives through a subdomain such as
  // kahve-duragi.karecik.com, every path renders that business' customer menu.
  const subdomain = getSubdomain()

  if (subdomain) {
    return (
      <Routes>
        <Route path="*" element={<CustomerMenu slug={subdomain} />} />
      </Routes>
    )
  }

  return (
    <Routes>
      <Route path="/" element={<Landing />} />
      <Route path="/giris" element={<Login />} />
      <Route path="/kayit" element={<SignUp />} />

      {/* Path-based menu access — needs no hosts file entry */}
      <Route path="/m/:slug" element={<CustomerMenu />} />

      {/* Demo menu rendered inside the iPhone frame on the landing page */}
      <Route path="/demo" element={<CustomerMenu slug="demo-kafe" embedded />} />

      <Route
        path="/panel"
        element={
          <ProtectedRoute>
            <DashboardLayout />
          </ProtectedRoute>
        }
      >
        <Route index element={<MenuEditor />} />
        <Route path="tasarim" element={<Design />} />
        <Route path="ayarlar" element={<Settings />} />
        <Route path="qr" element={<QrCode />} />
      </Route>

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
