import { Navigate, Route, Routes } from 'react-router-dom'

import { useAuth } from './lib/auth.jsx'
import { subdomainAl } from './lib/subdomain'
import Yukleniyor from './components/ui/Yukleniyor.jsx'

import Landing from './pages/Landing.jsx'
import Giris from './pages/Giris.jsx'
import Kayit from './pages/Kayit.jsx'
import MusteriMenusu from './pages/menu/MusteriMenusu.jsx'

import PanelDuzeni from './pages/dashboard/PanelDuzeni.jsx'
import MenuEditoru from './pages/dashboard/MenuEditoru.jsx'
import Tasarim from './pages/dashboard/Tasarim.jsx'
import Ayarlar from './pages/dashboard/Ayarlar.jsx'
import QrKod from './pages/dashboard/QrKod.jsx'

/** Oturum gerektiren sayfaları korur. */
function KorumaliYol({ children }) {
  const { isAuthenticated, loading } = useAuth()

  if (loading) return <Yukleniyor tamEkran metin="Oturum kontrol ediliyor..." />
  if (!isAuthenticated) return <Navigate to="/giris" replace />
  return children
}

export default function App() {
  // kahve-duragi.karecik.com gibi bir adresle gelindiyse tüm yollar
  // doğrudan o işletmenin müşteri menüsünü gösterir.
  const altAlan = subdomainAl()

  if (altAlan) {
    return (
      <Routes>
        <Route path="*" element={<MusteriMenusu slug={altAlan} />} />
      </Routes>
    )
  }

  return (
    <Routes>
      <Route path="/" element={<Landing />} />
      <Route path="/giris" element={<Giris />} />
      <Route path="/kayit" element={<Kayit />} />

      {/* Yol tabanlı menü erişimi — hosts ayarı gerektirmez */}
      <Route path="/m/:slug" element={<MusteriMenusu />} />

      {/* Landing page'deki iPhone çerçevesinin içinde çalışan demo menü */}
      <Route path="/demo" element={<MusteriMenusu slug="demo-kafe" gomulu />} />

      <Route
        path="/panel"
        element={
          <KorumaliYol>
            <PanelDuzeni />
          </KorumaliYol>
        }
      >
        <Route index element={<MenuEditoru />} />
        <Route path="tasarim" element={<Tasarim />} />
        <Route path="ayarlar" element={<Ayarlar />} />
        <Route path="qr" element={<QrKod />} />
      </Route>

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
