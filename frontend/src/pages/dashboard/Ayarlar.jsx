import { useEffect, useMemo, useState } from 'react'
import {
  AlertCircle,
  Eye,
  EyeOff,
  Globe,
  Info,
  Instagram,
  Languages,
  MapPin,
  Percent,
  Phone,
  Save,
  Store,
  Wifi,
} from 'lucide-react'

import { useAuth } from '../../lib/auth.jsx'
import { PARA_BIRIMI_LISTESI, fiyatBicimle, tarihBicimle } from '../../lib/format'
import { DILLER } from '../../locales/index.js'
import Yukleniyor from '../../components/ui/Yukleniyor.jsx'
import GorselYukleyici from '../../components/ui/GorselYukleyici.jsx'
import { useBildirim } from '../../components/ui/Bildirim.jsx'

/* Bu sayfanın yönettiği işletme alanları */
const AYAR_ALANLARI = [
  'logo_url',
  'name',
  'slug',
  'phone',
  'address',
  'instagram',
  'wifi_password',
  'currency',
  'languages',
  'default_language',
  'show_price_date',
  'show_vat_note',
  'vat_note_text',
  'is_active',
]

/* Boş bırakılabilen (boşsa null gönderilen) metin alanları */
const BOSALTILABILIR = ['logo_url', 'phone', 'address', 'instagram', 'wifi_password']

const VARSAYILAN_KDV_METNI = 'Fiyatlarımıza KDV dahildir.'
const MENU_SON_EKI = '.karecik.com'

/* Sistem tarafından ayrılmış menü adresleri (backend utils/slug.go ile aynı) */
const AYRILMIS_ADRESLER = [
  'www',
  'api',
  'admin',
  'app',
  'panel',
  'mail',
  'ftp',
  'blog',
  'help',
  'destek',
  'karecik',
  'static',
  'cdn',
  'assets',
  'dashboard',
  'login',
  'register',
  'demo',
]

/* Türkçe karakterlerin ASCII karşılıkları */
const TURKCE_HARFLER = {
  ç: 'c',
  Ç: 'c',
  ğ: 'g',
  Ğ: 'g',
  ı: 'i',
  I: 'i',
  İ: 'i',
  ö: 'o',
  Ö: 'o',
  ş: 's',
  Ş: 's',
  ü: 'u',
  Ü: 'u',
  â: 'a',
  Â: 'a',
  î: 'i',
  Î: 'i',
  û: 'u',
  Û: 'u',
}

/**
 * Kullanıcı yazarken menü adresini temizler.
 * Sondaki tire, yazmaya devam edilebilsin diye korunur; kayıtta atılır.
 */
function slugTemizle(girdi) {
  return String(girdi ?? '')
    .replace(/[çÇğĞıIİöÖşŞüÜâÂîÎûÛ]/g, (harf) => TURKCE_HARFLER[harf] || harf)
    .replace(/&/g, '-ve-')
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, '-')
    .replace(/-{2,}/g, '-')
    .replace(/^-+/, '')
    .slice(0, 60)
}

/** Kaydedilecek nihai adres: baştaki ve sondaki tireler atılır. */
function slugKesinlestir(girdi) {
  return slugTemizle(girdi).replace(/-+$/, '')
}

/** İşletme nesnesinden düzenlenebilir taslak üretir. */
function taslakOlustur(isletme) {
  const diller =
    Array.isArray(isletme.languages) && isletme.languages.length > 0
      ? [...isletme.languages]
      : ['tr']

  return {
    logo_url: isletme.logo_url || null,
    name: isletme.name || '',
    slug: isletme.slug || '',
    phone: isletme.phone || '',
    address: isletme.address || '',
    instagram: String(isletme.instagram || '').replace(/^@+/, ''),
    wifi_password: isletme.wifi_password || '',
    currency: isletme.currency || 'TRY',
    languages: diller,
    default_language: diller.includes(isletme.default_language)
      ? isletme.default_language
      : diller[0],
    show_price_date: Boolean(isletme.show_price_date),
    show_vat_note: Boolean(isletme.show_vat_note),
    vat_note_text: isletme.vat_note_text || VARSAYILAN_KDV_METNI,
    is_active: Boolean(isletme.is_active),
  }
}

/** İki alan değerini karşılaştırır (dizilerde sıra önemsizdir). */
function esitMi(a, b) {
  if (Array.isArray(a) || Array.isArray(b)) {
    const birinci = (Array.isArray(a) ? a : []).slice().sort()
    const ikinci = (Array.isArray(b) ? b : []).slice().sort()
    return birinci.length === ikinci.length && birinci.every((deger, i) => deger === ikinci[i])
  }
  if (typeof a === 'boolean' || typeof b === 'boolean') return Boolean(a) === Boolean(b)
  return String(a ?? '').trim() === String(b ?? '').trim()
}

/* ------------------------------------------------------- küçük bileşenler */

/** Aç/kapa anahtarı. */
function Anahtar({ acik, degisti, etiket, aciklama }) {
  return (
    <div className="flex items-start justify-between gap-4">
      <div className="min-w-0">
        <p className="text-sm font-medium text-gray-900">{etiket}</p>
        {aciklama ? <p className="yardim">{aciklama}</p> : null}
      </div>

      <button
        type="button"
        role="switch"
        aria-checked={acik}
        aria-label={etiket}
        onClick={() => degisti(!acik)}
        className={`relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-marka-100 ${
          acik ? 'bg-marka-600' : 'bg-gray-300'
        }`}
      >
        <span
          className={`inline-block h-5 w-5 rounded-full bg-white shadow transition-transform ${
            acik ? 'translate-x-[22px]' : 'translate-x-0.5'
          }`}
        />
      </button>
    </div>
  )
}

/** Kart bölümü başlığı. */
function BolumBasligi({ ikon: Ikon, baslik, aciklama }) {
  return (
    <div className="mb-4">
      <div className="flex items-center gap-2">
        <Ikon className="h-4 w-4 text-marka-600" aria-hidden="true" />
        <h2 className="text-base font-semibold text-gray-900">{baslik}</h2>
      </div>
      {aciklama ? <p className="yardim">{aciklama}</p> : null}
    </div>
  )
}

/* ------------------------------------------------------------------ sayfa */

export default function Ayarlar() {
  const { business, saveBusiness } = useAuth()
  const bildirim = useBildirim()

  const [taslak, setTaslak] = useState(null)
  const [kaydediliyor, setKaydediliyor] = useState(false)
  const [slugHatasi, setSlugHatasi] = useState('')
  const [sifreGorunur, setSifreGorunur] = useState(false)

  // Sunucudaki kayıtlı hâl (karşılaştırma için).
  const kayitli = useMemo(() => (business ? taslakOlustur(business) : null), [business])

  // İşletme değiştiğinde (ilk yükleme veya kayıt sonrası) taslağı tazele.
  useEffect(() => {
    setTaslak(kayitli ? { ...kayitli } : null)
    setSlugHatasi('')
  }, [kayitli])

  if (!business || !kayitli || !taslak) {
    return <Yukleniyor metin="Ayarlar yükleniyor..." />
  }

  /* ----------------------------------------------------------- yardımcılar */

  const guncelle = (alan, deger) => {
    setTaslak((onceki) => ({ ...onceki, [alan]: deger }))
  }

  const degisenAlanlar = AYAR_ALANLARI.filter((alan) => !esitMi(taslak[alan], kayitli[alan]))
  const degisiklikVar = degisenAlanlar.length > 0

  const sonSlug = slugKesinlestir(taslak.slug)
  const gosterilecekSlug = sonSlug || 'menu-adresiniz'
  const fiyatTarihi = tarihBicimle(business.price_updated_at) || tarihBicimle(new Date())

  /** Dil çipine tıklandığında seçimi değiştirir. */
  const diliDegistir = (kod) => {
    const secili = taslak.languages.includes(kod)

    if (secili && taslak.languages.length <= 1) {
      bildirim.hata('En az bir menü dili seçili kalmalıdır.')
      return
    }

    const secilenler = secili
      ? taslak.languages.filter((dil) => dil !== kod)
      : [...taslak.languages, kod]

    // DILLER sırasına göre düzenli tut.
    const yeniDiller = DILLER.filter((dil) => secilenler.includes(dil.code)).map((dil) => dil.code)

    setTaslak((onceki) => ({
      ...onceki,
      languages: yeniDiller,
      default_language: yeniDiller.includes(onceki.default_language)
        ? onceki.default_language
        : yeniDiller[0],
    }))
  }

  /** Yalnızca değişen alanları toplayıp kaydeder. */
  const kaydet = async () => {
    if (!degisiklikVar || kaydediliyor) return

    setSlugHatasi('')

    const ad = taslak.name.trim()
    if (ad.length < 2 || ad.length > 100) {
      bildirim.hata('İşletme adı 2 ile 100 karakter arasında olmalıdır.')
      return
    }
    if (sonSlug.length < 2) {
      setSlugHatasi(
        'Menü adresi en az 2 karakter olmalı ve yalnızca harf, rakam ve tire içerebilir.',
      )
      return
    }
    if (AYRILMIS_ADRESLER.includes(sonSlug)) {
      setSlugHatasi('Bu menü adresi sistem tarafından ayrılmıştır. Lütfen başka bir adres deneyin.')
      return
    }
    if (taslak.languages.length === 0) {
      bildirim.hata('En az bir menü dili seçmelisiniz.')
      return
    }
    if (taslak.vat_note_text.trim().length > 200) {
      bildirim.hata('KDV ibaresi en fazla 200 karakter olabilir.')
      return
    }

    const govde = {}
    degisenAlanlar.forEach((alan) => {
      if (alan === 'slug') {
        govde.slug = sonSlug
        return
      }
      if (alan === 'name') {
        govde.name = ad
        return
      }
      if (alan === 'vat_note_text') {
        govde.vat_note_text = taslak.vat_note_text.trim()
        return
      }
      if (BOSALTILABILIR.includes(alan)) {
        const deger = String(taslak[alan] ?? '').trim()
        govde[alan] = deger === '' ? null : deger
        return
      }
      govde[alan] = taslak[alan]
    })

    // Yalnızca sondaki tire temizlendiyse gönderilecek bir şey kalmayabilir.
    if (govde.slug === kayitli.slug) delete govde.slug
    if (Object.keys(govde).length === 0) {
      setTaslak((onceki) => ({ ...onceki, slug: sonSlug }))
      return
    }

    setKaydediliyor(true)
    try {
      await saveBusiness(govde)
      bildirim.basari('Ayarlar kaydedildi.')
    } catch (hata) {
      if (hata.status === 409) {
        setSlugHatasi(hata.message)
      } else {
        bildirim.hata(hata.message)
      }
    } finally {
      setKaydediliyor(false)
    }
  }

  /* --------------------------------------------------------------- görünüm */

  return (
    <div className="mx-auto max-w-3xl pb-10">
      {/* Yapışkan başlık çubuğu */}
      <div className="sticky top-0 z-10 -mx-4 -mt-4 mb-6 border-b border-gray-200 bg-gray-50 px-4 pb-4 pt-4 sm:-mx-6 sm:-mt-6 sm:px-6 sm:pt-6">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0">
            <h1 className="text-2xl font-semibold text-gray-900">Ayarlar</h1>
            <p className="mt-1 text-sm text-gray-500">
              İşletme bilgilerinizi ve menü ayarlarınızı yönetin.
            </p>
          </div>

          <button
            type="button"
            className="btn-birincil shrink-0"
            onClick={kaydet}
            disabled={!degisiklikVar || kaydediliyor}
          >
            <Save className="h-4 w-4" aria-hidden="true" />
            {kaydediliyor ? 'Kaydediliyor...' : 'Kaydet'}
          </button>
        </div>
      </div>

      <div className="space-y-6">
        {/* ------------------------------------------- 1. İşletme Bilgileri */}
        <section className="kart p-5">
          <BolumBasligi ikon={Store} baslik="İşletme Bilgileri" />

          <div className="space-y-5">
            <GorselYukleyici
              deger={taslak.logo_url}
              degisti={(url) => guncelle('logo_url', url)}
              etiket="Logo"
              ipucu="Kare veya yuvarlak logo önerilir · en fazla 5 MB"
              yuvarlak
            />

            <div>
              <label className="etiket" htmlFor="isletme-adi">
                İşletme Adı
              </label>
              <input
                id="isletme-adi"
                type="text"
                className="girdi"
                value={taslak.name}
                maxLength={100}
                placeholder="Kahve Durağı"
                onChange={(olay) => guncelle('name', olay.target.value)}
              />
              <p className="yardim">2 ile 100 karakter arasında olmalıdır.</p>
            </div>

            <div>
              <label className="etiket" htmlFor="menu-adresi">
                Menü Adresi (subdomain)
              </label>
              <div className="flex">
                <input
                  id="menu-adresi"
                  type="text"
                  className="girdi rounded-r-none"
                  value={taslak.slug}
                  placeholder="kahve-duragi"
                  autoComplete="off"
                  spellCheck={false}
                  onChange={(olay) => {
                    setSlugHatasi('')
                    guncelle('slug', slugTemizle(olay.target.value))
                  }}
                />
                <span className="inline-flex shrink-0 items-center rounded-r-lg border border-l-0 border-gray-300 bg-gray-100 px-3 text-sm text-gray-500">
                  {MENU_SON_EKI}
                </span>
              </div>

              {slugHatasi ? <p className="hata-metni">{slugHatasi}</p> : null}

              <p className="yardim">
                Menünüz şu adresten açılacak:{' '}
                <b className="text-gray-700">
                  {gosterilecekSlug}
                  {MENU_SON_EKI}
                </b>
              </p>
              <p className="yardim">
                Adresi değiştirirseniz eski QR kodlarınız çalışmaya devam ETMEZ. Değişiklikten sonra
                QR kodunuzu yeniden indirin.
              </p>
            </div>

            <div>
              <label className="etiket" htmlFor="telefon">
                <span className="inline-flex items-center gap-1.5">
                  <Phone className="h-3.5 w-3.5 text-gray-400" aria-hidden="true" />
                  Telefon
                </span>
              </label>
              <input
                id="telefon"
                type="tel"
                className="girdi"
                value={taslak.phone}
                placeholder="+90 555 000 00 00"
                onChange={(olay) => guncelle('phone', olay.target.value)}
              />
            </div>

            <div>
              <label className="etiket" htmlFor="adres">
                <span className="inline-flex items-center gap-1.5">
                  <MapPin className="h-3.5 w-3.5 text-gray-400" aria-hidden="true" />
                  Adres
                </span>
              </label>
              <textarea
                id="adres"
                rows={3}
                className="girdi resize-y"
                value={taslak.address}
                placeholder="Caferağa Mah. Moda Cad. No: 12, Kadıköy / İstanbul"
                onChange={(olay) => guncelle('address', olay.target.value)}
              />
            </div>

            <div>
              <label className="etiket" htmlFor="instagram">
                <span className="inline-flex items-center gap-1.5">
                  <Instagram className="h-3.5 w-3.5 text-gray-400" aria-hidden="true" />
                  Instagram kullanıcı adı
                </span>
              </label>
              <div className="flex">
                <span className="inline-flex shrink-0 items-center rounded-l-lg border border-r-0 border-gray-300 bg-gray-100 px-3 text-sm text-gray-500">
                  @
                </span>
                <input
                  id="instagram"
                  type="text"
                  className="girdi rounded-l-none"
                  value={taslak.instagram}
                  placeholder="kahveduragi"
                  autoComplete="off"
                  spellCheck={false}
                  onChange={(olay) => guncelle('instagram', olay.target.value.replace(/[@\s]/g, ''))}
                />
              </div>
              <p className="yardim">Menünün altında Instagram bağlantısı olarak görünür.</p>
            </div>

            <div>
              <label className="etiket" htmlFor="wifi-sifresi">
                <span className="inline-flex items-center gap-1.5">
                  <Wifi className="h-3.5 w-3.5 text-gray-400" aria-hidden="true" />
                  Wi-Fi şifresi
                </span>
              </label>
              <div className="relative">
                <input
                  id="wifi-sifresi"
                  type={sifreGorunur ? 'text' : 'password'}
                  className="girdi pr-11"
                  value={taslak.wifi_password}
                  placeholder="kahve2026"
                  autoComplete="off"
                  onChange={(olay) => guncelle('wifi_password', olay.target.value)}
                />
                <button
                  type="button"
                  onClick={() => setSifreGorunur((onceki) => !onceki)}
                  className="absolute inset-y-0 right-0 flex w-11 items-center justify-center text-gray-400 hover:text-gray-600"
                  aria-label={sifreGorunur ? 'Şifreyi gizle' : 'Şifreyi göster'}
                >
                  {sifreGorunur ? (
                    <EyeOff className="h-4 w-4" aria-hidden="true" />
                  ) : (
                    <Eye className="h-4 w-4" aria-hidden="true" />
                  )}
                </button>
              </div>
              <p className="yardim">Bu şifre menüde müşterilerinize gösterilir.</p>
            </div>
          </div>
        </section>

        {/* ------------------------------------------------- 2. Para Birimi */}
        <section className="kart p-5">
          <BolumBasligi
            ikon={Globe}
            baslik="Para Birimi"
            aciklama="Menüdeki tüm fiyatlar bu para birimiyle gösterilir."
          />

          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            {PARA_BIRIMI_LISTESI.map((birim) => {
              const secili = taslak.currency === birim.code
              return (
                <button
                  key={birim.code}
                  type="button"
                  onClick={() => guncelle('currency', birim.code)}
                  aria-pressed={secili}
                  className={`rounded-xl border p-3 text-center hover:border-marka-400 ${
                    secili ? 'border-marka-600 ring-2 ring-marka-600' : 'border-gray-200'
                  }`}
                >
                  <span className="block text-2xl leading-none text-gray-900" aria-hidden="true">
                    {birim.symbol}
                  </span>
                  <span className="mt-2 block text-sm font-medium text-gray-900">{birim.code}</span>
                  <span className="mt-0.5 block text-xs leading-snug text-gray-500">
                    {birim.label}
                  </span>
                </button>
              )
            })}
          </div>

          <p className="mt-4 rounded-lg bg-gray-50 px-3 py-2.5 text-sm text-gray-600">
            Fiyatlar şöyle görünecek:{' '}
            <b className="text-gray-900">{fiyatBicimle(145, taslak.currency)}</b>
          </p>
        </section>

        {/* ------------------------------------------------ 3. Menü Dilleri */}
        <section className="kart p-5">
          <BolumBasligi ikon={Languages} baslik="Menü Dilleri" />

          <div className="flex flex-wrap gap-2">
            {DILLER.map((dil) => {
              const secili = taslak.languages.includes(dil.code)
              return (
                <button
                  key={dil.code}
                  type="button"
                  onClick={() => diliDegistir(dil.code)}
                  aria-pressed={secili}
                  className={`inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-sm ${
                    secili
                      ? 'border-marka-600 bg-marka-50 font-medium text-marka-700'
                      : 'border-gray-300 bg-white text-gray-600 hover:bg-gray-50'
                  }`}
                >
                  <span aria-hidden="true">{dil.flag}</span>
                  {dil.label}
                </button>
              )
            })}
          </div>

          <div className="mt-5 max-w-xs">
            <label className="etiket" htmlFor="varsayilan-dil">
              Varsayılan dil
            </label>
            <select
              id="varsayilan-dil"
              className="girdi"
              value={taslak.default_language}
              onChange={(olay) => guncelle('default_language', olay.target.value)}
            >
              {DILLER.filter((dil) => taslak.languages.includes(dil.code)).map((dil) => (
                <option key={dil.code} value={dil.code}>
                  {dil.flag} {dil.label}
                </option>
              ))}
            </select>
          </div>

          <p className="yardim mt-3">
            Birden fazla dil seçerseniz ürün eklerken her dil için ayrı ad ve açıklama
            girebilirsiniz. Müşteriler menüde dil değiştirebilir.
          </p>
        </section>

        {/* ---------------------------- 4. Menü Altı Bilgiler (Yasal İbareler) */}
        <section className="kart p-5">
          <BolumBasligi ikon={Percent} baslik="Menü Altı Bilgiler (Yasal İbareler)" />

          <div className="space-y-5">
            <div>
              <Anahtar
                acik={taslak.show_price_date}
                degisti={(deger) => guncelle('show_price_date', deger)}
                etiket="Fiyat geçerlilik tarihini göster"
              />

              <p className="mt-3 rounded-lg bg-gray-50 px-3 py-2.5 text-sm text-gray-600">
                Fiyatlarımız <b className="text-gray-900">{fiyatTarihi}</b> tarihinden itibaren
                geçerlidir.
              </p>

              <p className="yardim flex items-start gap-1.5">
                <Info className="mt-0.5 h-3.5 w-3.5 shrink-0 text-gray-400" aria-hidden="true" />
                <span>
                  Bu tarih, toplu fiyat güncellemesi yaptığınızda otomatik olarak yenilenir.
                </span>
              </p>
            </div>

            <div className="border-t border-gray-100 pt-5">
              <Anahtar
                acik={taslak.show_vat_note}
                degisti={(deger) => guncelle('show_vat_note', deger)}
                etiket="KDV ibaresini göster"
              />

              <div className="mt-3">
                <label className="etiket" htmlFor="kdv-metni">
                  KDV ibaresi metni
                </label>
                <input
                  id="kdv-metni"
                  type="text"
                  className="girdi"
                  value={taslak.vat_note_text}
                  maxLength={200}
                  placeholder={VARSAYILAN_KDV_METNI}
                  disabled={!taslak.show_vat_note}
                  onChange={(olay) => guncelle('vat_note_text', olay.target.value)}
                />
                <p className="yardim">{taslak.vat_note_text.length} / 200 karakter</p>
              </div>
            </div>
          </div>
        </section>

        {/* ------------------------------------------------- 5. Menü Durumu */}
        <section className="kart p-5">
          <BolumBasligi ikon={Eye} baslik="Menü Durumu" />

          <Anahtar
            acik={taslak.is_active}
            degisti={(deger) => guncelle('is_active', deger)}
            etiket="Menü yayında"
            aciklama="Kapattığınızda menüyü panelden düzenlemeye devam edebilirsiniz, müşteriler göremez."
          />

          {!taslak.is_active ? (
            <div className="mt-4 flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2.5">
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600" aria-hidden="true" />
              <p className="text-sm text-amber-800">
                Menünüz kapalı. Müşteriler QR kodu okuttuğunda menüyü göremez.
              </p>
            </div>
          ) : null}
        </section>
      </div>
    </div>
  )
}
