import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { AlertCircle, Eye, EyeOff } from 'lucide-react'

import Modal from '../ui/Modal.jsx'
import { useBildirim } from '../ui/Bildirim.jsx'
import { useAuth } from '../../lib/auth.jsx'

const EPOSTA_DESENI = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/

/**
 * Açılış sayfasındaki hızlı kayıt penceresi.
 *
 * @param {boolean}  acik
 * @param {Function} kapat
 */
export default function KayitModal({ acik, kapat }) {
  const { register } = useAuth()
  const bildirim = useBildirim()
  const gezin = useNavigate()

  const [isletmeAdi, setIsletmeAdi] = useState('')
  const [eposta, setEposta] = useState('')
  const [sifre, setSifre] = useState('')
  const [sifreGorunur, setSifreGorunur] = useState(false)
  const [hatalar, setHatalar] = useState({})
  const [genelHata, setGenelHata] = useState('')
  const [gonderiliyor, setGonderiliyor] = useState(false)

  // Pencere her açıldığında önceki hata izlerini temizle.
  useEffect(() => {
    if (!acik) return
    setHatalar({})
    setGenelHata('')
    setGonderiliyor(false)
    setSifreGorunur(false)
  }, [acik])

  /** İstemci tarafı doğrulama; alan adı -> hata metni sözlüğü döner. */
  function dogrula() {
    const yeniHatalar = {}

    if (!isletmeAdi.trim()) {
      yeniHatalar.isletmeAdi = 'İşletme adını girin.'
    } else if (isletmeAdi.trim().length < 2) {
      yeniHatalar.isletmeAdi = 'İşletme adı en az 2 karakter olmalı.'
    }

    if (!eposta.trim()) {
      yeniHatalar.eposta = 'E-posta adresinizi girin.'
    } else if (!EPOSTA_DESENI.test(eposta.trim())) {
      yeniHatalar.eposta = 'Geçerli bir e-posta adresi girin.'
    }

    if (!sifre) {
      yeniHatalar.sifre = 'Bir şifre belirleyin.'
    } else if (sifre.length < 8) {
      yeniHatalar.sifre = 'Şifre en az 8 karakter olmalı.'
    }

    return yeniHatalar
  }

  async function gonder(olay) {
    olay.preventDefault()
    if (gonderiliyor) return

    setGenelHata('')
    const yeniHatalar = dogrula()
    setHatalar(yeniHatalar)
    if (Object.keys(yeniHatalar).length > 0) return

    setGonderiliyor(true)
    try {
      await register(isletmeAdi.trim(), eposta.trim(), sifre)
      bildirim.basari('Hoş geldiniz! Menünüzü oluşturmaya başlayabilirsiniz.')
      kapat?.()
      gezin('/panel')
    } catch (hata) {
      setGenelHata(hata.message)
      bildirim.hata(hata.message)
      setGonderiliyor(false)
    }
  }

  return (
    <Modal
      acik={acik}
      kapat={gonderiliyor ? () => {} : kapat}
      baslik="Ücretsiz hesabınızı oluşturun"
      aciklama="Birkaç saniyede QR menünüzü hazırlamaya başlayın."
      genislik="max-w-md"
      altBilgi={
        <button
          type="submit"
          form="karecik-kayit-formu"
          disabled={gonderiliyor}
          className="btn-birincil w-full sm:w-auto"
        >
          {gonderiliyor ? 'Hesap oluşturuluyor...' : 'Hesabımı oluştur'}
        </button>
      }
    >
      <form id="karecik-kayit-formu" onSubmit={gonder} noValidate className="space-y-4">
        {genelHata ? (
          <div className="flex items-start gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2.5 text-sm text-red-700">
            <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
            <p className="leading-snug">{genelHata}</p>
          </div>
        ) : null}

        <div>
          <label className="etiket" htmlFor="kayit-isletme-adi">
            İşletme Adı
          </label>
          <input
            id="kayit-isletme-adi"
            type="text"
            className="girdi"
            placeholder="Örn. Kahve Durağı"
            autoComplete="organization"
            value={isletmeAdi}
            disabled={gonderiliyor}
            onChange={(olay) => setIsletmeAdi(olay.target.value)}
          />
          {hatalar.isletmeAdi ? <p className="hata-metni">{hatalar.isletmeAdi}</p> : null}
        </div>

        <div>
          <label className="etiket" htmlFor="kayit-eposta">
            E-posta
          </label>
          <input
            id="kayit-eposta"
            type="email"
            className="girdi"
            placeholder="ornek@isletmem.com"
            autoComplete="email"
            value={eposta}
            disabled={gonderiliyor}
            onChange={(olay) => setEposta(olay.target.value)}
          />
          {hatalar.eposta ? <p className="hata-metni">{hatalar.eposta}</p> : null}
        </div>

        <div>
          <label className="etiket" htmlFor="kayit-sifre">
            Şifre
          </label>
          <div className="relative">
            <input
              id="kayit-sifre"
              type={sifreGorunur ? 'text' : 'password'}
              className="girdi pr-11"
              placeholder="En az 8 karakter"
              autoComplete="new-password"
              value={sifre}
              disabled={gonderiliyor}
              onChange={(olay) => setSifre(olay.target.value)}
            />
            <button
              type="button"
              onClick={() => setSifreGorunur((onceki) => !onceki)}
              className="absolute right-1 top-1/2 -translate-y-1/2 rounded-lg p-2 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
              aria-label={sifreGorunur ? 'Şifreyi gizle' : 'Şifreyi göster'}
            >
              {sifreGorunur ? (
                <EyeOff className="h-4 w-4" aria-hidden="true" />
              ) : (
                <Eye className="h-4 w-4" aria-hidden="true" />
              )}
            </button>
          </div>
          {hatalar.sifre ? (
            <p className="hata-metni">{hatalar.sifre}</p>
          ) : (
            <p className="yardim">Şifreniz en az 8 karakter olmalı.</p>
          )}
        </div>

        <p className="border-t border-gray-200 pt-4 text-sm text-gray-600">
          Zaten hesabınız var mı?{' '}
          <Link
            to="/giris"
            onClick={() => kapat?.()}
            className="font-medium text-marka-600 hover:text-marka-700"
          >
            Giriş yapın
          </Link>
        </p>
      </form>
    </Modal>
  )
}
