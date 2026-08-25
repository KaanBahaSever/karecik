import { useEffect, useMemo, useState } from 'react'
import { ArrowLeft, Search, Star } from 'lucide-react'

import { fiyatBicimle } from '../../lib/format'
import { temaDegiskenleri } from '../../themes/themes'
import { fontStack, fontYukle } from '../../themes/fonts'
import { alerjenBul, dilBul, metin, sagdanSolaMi } from '../../locales/index.js'
import UrunDetayModal from './UrunDetayModal.jsx'
import MenuFooter from './MenuFooter.jsx'

/**
 * Müşteri menüsünün asıl ekranı.
 *
 * İki bileşen tarafından kullanılır:
 *   1. pages/menu/MusteriMenusu.jsx  — QR ile açılan gerçek menü
 *   2. components/dashboard/CanliOnizleme.jsx — paneldeki canlı önizleme
 * Bu yüzden prop imzası SABİTTİR, değiştirme.
 *
 * Renkler Tailwind sınıflarıyla değil, temadan üretilen CSS değişkenleriyle
 * verilir (--menu-bg, --menu-text, --menu-primary ...). Böylece altı temanın
 * tamamı tek bir bileşen ağacıyla çalışır.
 *
 * @param {object}   menu       - { business, categories, footer }
 * @param {string}   dil        - Aktif dil kodu
 * @param {function} dilDegisti - Dil değiştirildiğinde çağrılır
 * @param {boolean}  gomulu     - Dar alanda (panel önizlemesi / iframe) kompakt render
 */

/** Vurgu rengi üzerinde okunaklı metin rengi seçer. */
function okunakliMetinRengi(hex) {
  const temiz = String(hex || '').trim().replace('#', '')
  let r
  let g
  let b

  if (temiz.length === 3) {
    r = Number.parseInt(temiz[0] + temiz[0], 16)
    g = Number.parseInt(temiz[1] + temiz[1], 16)
    b = Number.parseInt(temiz[2] + temiz[2], 16)
  } else if (temiz.length === 6) {
    r = Number.parseInt(temiz.slice(0, 2), 16)
    g = Number.parseInt(temiz.slice(2, 4), 16)
    b = Number.parseInt(temiz.slice(4, 6), 16)
  } else {
    return '#ffffff'
  }

  if ([r, g, b].some(Number.isNaN)) return '#ffffff'

  const parlaklik = (0.299 * r + 0.587 * g + 0.114 * b) / 255
  return parlaklik > 0.6 ? '#111827' : '#ffffff'
}

/** Türkçe duyarlı küçük harfe çevirme (arama için). */
function kucult(metinDegeri) {
  return String(metinDegeri || '').toLocaleLowerCase('tr')
}

/** İki satırdan sonra kırpma — Tailwind eklentisi olmadan. */
const IKI_SATIR = {
  display: '-webkit-box',
  WebkitLineClamp: 2,
  WebkitBoxOrient: 'vertical',
  overflow: 'hidden',
}

export default function MenuIcerik({ menu, dil = 'tr', dilDegisti, gomulu = false }) {
  const business = menu?.business || {}
  const kategoriler = useMemo(() => menu?.categories || [], [menu])

  const [seciliKategoriId, setSeciliKategoriId] = useState(null)
  const [arama, setArama] = useState('')
  const [seciliUrun, setSeciliUrun] = useState(null)

  // Seçilen yazı tipini yükle
  useEffect(() => {
    if (business.font_family) fontYukle(business.font_family)
  }, [business.font_family])

  // Menü değişirse (dil değişimi vb.) seçili kategori geçersiz kalmasın
  useEffect(() => {
    if (seciliKategoriId && !kategoriler.some((k) => k.id === seciliKategoriId)) {
      setSeciliKategoriId(null)
    }
  }, [kategoriler, seciliKategoriId])

  const stil = temaDegiskenleri(
    business.theme,
    business.primary_color,
    fontStack(business.font_family),
  )

  const vurguUzeriMetin = okunakliMetinRengi(business.primary_color)
  const diller = Array.isArray(business.languages) ? business.languages : []
  const aramaMetni = arama.trim()
  const aramaAktif = aramaMetni.length > 0

  const urunAdedi = (n) => (dil === 'tr' ? `${n} ürün` : `${n} items`)

  /* ------------------------------------------------------------ arama */

  const aramaSonuclari = useMemo(() => {
    if (!aramaAktif) return []
    const anahtar = kucult(aramaMetni)

    return kategoriler.flatMap((kategori) =>
      (kategori.products || [])
        .filter((urun) =>
          [urun.name, urun.description, urun.ingredients].some((alan) =>
            kucult(alan).includes(anahtar),
          ),
        )
        .map((urun) => ({ urun, kategoriAdi: kategori.name })),
    )
  }, [aramaAktif, aramaMetni, kategoriler])

  const seciliKategori = kategoriler.find((k) => k.id === seciliKategoriId) || null

  /* ------------------------------------------------------- alt parçalar */

  function UrunSatiri({ urun, kategoriAdi }) {
    const alerjenler = Array.isArray(urun.allergens) ? urun.allergens : []

    return (
      <button
        type="button"
        onClick={() => setSeciliUrun(urun)}
        className="flex w-full items-start gap-3 p-3 text-left"
        style={{
          backgroundColor: 'var(--menu-surface)',
          borderRadius: 'var(--menu-radius)',
          border: '1px solid var(--menu-border)',
          boxShadow: 'var(--menu-shadow)',
          opacity: urun.is_active === false ? 0.5 : 1,
        }}
      >
        {urun.image_url ? (
          <img
            src={urun.image_url}
            alt=""
            className="h-[72px] w-[72px] shrink-0 object-cover"
            style={{ borderRadius: 'calc(var(--menu-radius) * 0.7)' }}
          />
        ) : null}

        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="font-medium leading-snug" style={{ color: 'var(--menu-text)' }}>
                {urun.name}
              </p>
              {kategoriAdi ? (
                <p className="mt-0.5 text-[11px]" style={{ color: 'var(--menu-muted)' }}>
                  {kategoriAdi}
                </p>
              ) : null}
            </div>

            <div className="shrink-0 text-right">
              {urun.compare_price ? (
                <p className="text-[11px] line-through" style={{ color: 'var(--menu-muted)' }}>
                  {fiyatBicimle(urun.compare_price, business.currency)}
                </p>
              ) : null}
              <p className="font-semibold" style={{ color: 'var(--menu-primary)' }}>
                {fiyatBicimle(urun.price, business.currency)}
              </p>
            </div>
          </div>

          {urun.description ? (
            <p
              className="mt-1 text-xs leading-relaxed"
              style={{ ...IKI_SATIR, color: 'var(--menu-muted)' }}
            >
              {urun.description}
            </p>
          ) : null}

          {(alerjenler.length > 0 || urun.is_featured || urun.is_active === false) && (
            <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
              {urun.is_featured ? (
                <span
                  className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium"
                  style={{
                    backgroundColor: 'var(--menu-primary)',
                    color: vurguUzeriMetin,
                  }}
                >
                  <Star className="h-2.5 w-2.5" aria-hidden="true" />
                  {metin('oneCikan', dil)}
                </span>
              ) : null}

              {urun.is_active === false ? (
                <span
                  className="rounded-full px-2 py-0.5 text-[10px] font-medium"
                  style={{ border: '1px solid var(--menu-border)', color: 'var(--menu-muted)' }}
                >
                  Gizli
                </span>
              ) : null}

              {alerjenler.map((kod) => {
                const bilgi = alerjenBul(kod)
                if (!bilgi) return null
                return (
                  <span key={kod} title={dil === 'tr' ? bilgi.tr : bilgi.en} className="text-xs">
                    {bilgi.emoji}
                  </span>
                )
              })}
            </div>
          )}
        </div>
      </button>
    )
  }

  function KategoriSeridi() {
    return (
      <div className="kaydirma-gizli -mx-4 mb-4 flex gap-2 overflow-x-auto px-4 pb-1">
        {kategoriler.map((kategori) => {
          const secili = kategori.id === seciliKategoriId
          return (
            <button
              key={kategori.id}
              type="button"
              onClick={() => setSeciliKategoriId(kategori.id)}
              className="shrink-0 whitespace-nowrap rounded-full px-3.5 py-1.5 text-sm"
              style={
                secili
                  ? { backgroundColor: 'var(--menu-primary)', color: vurguUzeriMetin }
                  : {
                      backgroundColor: 'var(--menu-surface)',
                      color: 'var(--menu-text)',
                      border: '1px solid var(--menu-border)',
                    }
              }
            >
              {kategori.icon ? `${kategori.icon} ` : ''}
              {kategori.name}
            </button>
          )
        })}
      </div>
    )
  }

  function BosMetin({ yazi }) {
    return (
      <p className="py-10 text-center text-sm" style={{ color: 'var(--menu-muted)' }}>
        {yazi}
      </p>
    )
  }

  /* -------------------------------------------------------------- render */

  return (
    <div
      className={gomulu ? 'min-h-full w-full' : 'min-h-screen w-full'}
      style={stil}
      dir={sagdanSolaMi(dil) ? 'rtl' : 'ltr'}
    >
      <div className="mx-auto max-w-lg px-4 py-5">
        {/* ---------------------------------------------- başlık: logo SOL ÜSTTE */}
        <header className="flex items-start justify-between gap-3">
          <div className="flex min-w-0 items-center gap-3">
            {business.logo_url ? (
              <img
                src={business.logo_url}
                alt=""
                className="h-12 w-12 shrink-0 object-contain"
                style={{ borderRadius: 'calc(var(--menu-radius) * 0.6)' }}
              />
            ) : (
              <div
                className="flex h-12 w-12 shrink-0 items-center justify-center text-lg font-semibold"
                style={{
                  backgroundColor: 'var(--menu-primary)',
                  color: vurguUzeriMetin,
                  borderRadius: 'calc(var(--menu-radius) * 0.6)',
                }}
                aria-hidden="true"
              >
                {String(business.name || '•').charAt(0).toLocaleUpperCase('tr')}
              </div>
            )}

            <div className="min-w-0">
              <h1
                className="truncate text-lg font-semibold leading-tight"
                style={{ color: 'var(--menu-text)' }}
              >
                {business.name}
              </h1>
              {business.address ? (
                <p className="truncate text-xs" style={{ color: 'var(--menu-muted)' }}>
                  {business.address}
                </p>
              ) : null}
            </div>
          </div>

          {diller.length > 1 ? (
            <div className="flex shrink-0 items-center gap-1">
              {diller.map((kod) => {
                const bilgi = dilBul(kod)
                const secili = kod === dil
                return (
                  <button
                    key={kod}
                    type="button"
                    onClick={() => dilDegisti?.(kod)}
                    aria-label={bilgi.label}
                    title={bilgi.label}
                    className="rounded-full px-2 py-1 text-xs font-semibold leading-none"
                    style={{
                      opacity: secili ? 1 : 0.45,
                      border: secili ? '1px solid var(--menu-border)' : '1px solid transparent',
                    }}
                  >
                    {/* Bayrak emojisi yerine dil kodu: Windows bayrakları
                        çizemediği için İngilizce "GB" olarak görünüyordu. */}
                    {bilgi.kisa}
                  </button>
                )
              })}
            </div>
          ) : null}
        </header>

        {/* ------------------------------------------------------------ arama */}
        {kategoriler.length > 0 ? (
          <div className="relative mt-4">
            <Search
              className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2"
              style={{ color: 'var(--menu-muted)' }}
              aria-hidden="true"
            />
            <input
              type="search"
              value={arama}
              onChange={(olay) => setArama(olay.target.value)}
              placeholder={metin('ara', dil)}
              className="w-full py-2.5 pl-9 pr-3 text-sm outline-none"
              style={{
                backgroundColor: 'var(--menu-surface)',
                color: 'var(--menu-text)',
                border: '1px solid var(--menu-border)',
                borderRadius: 'var(--menu-radius)',
              }}
            />
          </div>
        ) : null}

        <div className="mt-4">
          {/* ------------------------------------------------- 1) arama sonuçları */}
          {aramaAktif ? (
            aramaSonuclari.length === 0 ? (
              <BosMetin yazi={metin('sonucYok', dil)} />
            ) : (
              <div className="flex flex-col gap-2.5">
                {aramaSonuclari.map(({ urun, kategoriAdi }) => (
                  <UrunSatiri key={urun.id} urun={urun} kategoriAdi={kategoriAdi} />
                ))}
              </div>
            )
          ) : seciliKategori ? (
            /* ------------------------------------------- 2) ürün listesi görünümü */
            <>
              <div className="mb-3 flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => setSeciliKategoriId(null)}
                  aria-label={dil === 'tr' ? 'Kategorilere dön' : 'Back to categories'}
                  className="flex h-8 w-8 shrink-0 items-center justify-center"
                  style={{
                    backgroundColor: 'var(--menu-surface)',
                    border: '1px solid var(--menu-border)',
                    borderRadius: 'calc(var(--menu-radius) * 0.7)',
                    color: 'var(--menu-text)',
                  }}
                >
                  <ArrowLeft className="h-4 w-4" style={{ transform: sagdanSolaMi(dil) ? 'scaleX(-1)' : 'none' }} />
                </button>

                <h2 className="truncate text-base font-semibold" style={{ color: 'var(--menu-text)' }}>
                  {seciliKategori.icon ? `${seciliKategori.icon} ` : ''}
                  {seciliKategori.name}
                </h2>
              </div>

              <KategoriSeridi />

              {seciliKategori.description ? (
                <p className="mb-3 text-xs" style={{ color: 'var(--menu-muted)' }}>
                  {seciliKategori.description}
                </p>
              ) : null}

              {(seciliKategori.products || []).length === 0 ? (
                <BosMetin yazi={metin('bosKategori', dil)} />
              ) : (
                <div className="flex flex-col gap-2.5">
                  {seciliKategori.products.map((urun) => (
                    <UrunSatiri key={urun.id} urun={urun} />
                  ))}
                </div>
              )}
            </>
          ) : kategoriler.length === 0 ? (
            /* ---------------------------------------------------- 3) menü boş */
            <BosMetin yazi={metin('bosMenu', dil)} />
          ) : (
            /* ------------------------------------------------ 4) kategori kartları */
            <div className="grid grid-cols-2 gap-3">
              {kategoriler.map((kategori) => (
                <button
                  key={kategori.id}
                  type="button"
                  onClick={() => setSeciliKategoriId(kategori.id)}
                  className="flex flex-col overflow-hidden text-left"
                  style={{
                    backgroundColor: 'var(--menu-surface)',
                    border: '1px solid var(--menu-border)',
                    borderRadius: 'var(--menu-radius)',
                    boxShadow: 'var(--menu-shadow)',
                    opacity: kategori.is_active === false ? 0.5 : 1,
                  }}
                >
                  {kategori.image_url ? (
                    <img
                      src={kategori.image_url}
                      alt=""
                      className={gomulu ? 'h-20 w-full object-cover' : 'h-24 w-full object-cover'}
                    />
                  ) : (
                    <div
                      className={`flex w-full items-center justify-center text-4xl ${
                        gomulu ? 'h-20' : 'h-24'
                      }`}
                    >
                      {kategori.icon || '🍽️'}
                    </div>
                  )}

                  <div className="px-3 py-2.5">
                    <p
                      className="truncate text-sm font-medium"
                      style={{ color: 'var(--menu-text)' }}
                    >
                      {kategori.name}
                    </p>
                    <p className="mt-0.5 text-[11px]" style={{ color: 'var(--menu-muted)' }}>
                      {urunAdedi((kategori.products || []).length)}
                    </p>
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>

        {/*
          Ana sayfada (kategori kartları) yalnızca "Karecik ile hazırlandı"
          görünür. Fiyat tarihi, KDV ibaresi ve iletişim bilgileri ürünlerin
          listelendiği ekranlara taşındı.
        */}
        <MenuFooter
          business={business}
          footer={menu?.footer}
          dil={dil}
          kapsam={aramaAktif || seciliKategori ? 'urunler' : 'anasayfa'}
        />
      </div>

      <UrunDetayModal
        urun={seciliUrun}
        business={business}
        dil={dil}
        kapat={() => setSeciliUrun(null)}
      />
    </div>
  )
}
