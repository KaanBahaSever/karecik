import { useEffect, useState } from 'react'
import { Info, Palette, Save } from 'lucide-react'

import { useAuth } from '../../lib/auth.jsx'
import { TEMALAR } from '../../themes/themes'
import { FONTLAR, tumFontlariYukle } from '../../themes/fonts'
import Yukleniyor from '../../components/ui/Yukleniyor.jsx'
import { useBildirim } from '../../components/ui/Bildirim.jsx'
import CanliOnizleme from '../../components/dashboard/CanliOnizleme.jsx'

/* Kaydedilecek (bu sayfanın yönettiği) alanlar */
const TASARIM_ALANLARI = [
  'theme',
  'font_family',
  'primary_color',
  'splash_enabled',
  'splash_duration',
  'splash_bg_color',
  'splash_text',
]

const HAZIR_RENKLER = [
  '#1a7f5a',
  '#0f766e',
  '#b45309',
  '#c2410c',
  '#be123c',
  '#7c3aed',
  '#1d4ed8',
  '#111827',
]

const HAZIR_KARSILAMA_RENKLERI = [
  '#0f172a',
  '#111827',
  '#1a7f5a',
  '#7c3aed',
  '#be123c',
  '#c2410c',
  '#faf6ef',
  '#ffffff',
]

/** #RRGGBB biçimini doğrular. */
function gecerliRenkMi(deger) {
  return /^#[0-9a-fA-F]{6}$/.test(String(deger || ''))
}

/** Geçersiz renkleri color girdisinin kabul edeceği bir değere düşürür. */
function guvenliRenk(deger, yedek) {
  return gecerliRenkMi(deger) ? deger : yedek
}

/** 1200 -> "1,2 saniye" */
function sureyiYaz(milisaniye) {
  const saniye = Number(milisaniye || 0) / 1000
  return `${saniye.toFixed(1).replace('.', ',')} saniye`
}

/** Koyu zeminde beyaz, açık zeminde koyu metin döndürür. */
function zeminUzeriMetinRengi(hex) {
  if (!gecerliRenkMi(hex)) return '#ffffff'
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  const parlaklik = (r * 299 + g * 587 + b * 114) / 1000
  return parlaklik > 150 ? '#111827' : '#ffffff'
}

export default function Tasarim() {
  const { business, saveBusiness } = useAuth()
  const bildirim = useBildirim()

  const [taslak, setTaslak] = useState(null)
  const [kaydediliyor, setKaydediliyor] = useState(false)

  // Sunucudaki işletme değiştiğinde (ilk yükleme veya kayıt sonrası) taslağı tazele.
  useEffect(() => {
    setTaslak(business ? { ...business } : null)
  }, [business])

  // Yazı tipi kartları kendi fontlarıyla görünsün.
  useEffect(() => {
    tumFontlariYukle()
  }, [])

  if (!business || !taslak) return <Yukleniyor metin="Tasarım ayarları yükleniyor..." />

  /* ------------------------------------------------------------ yardımcılar */

  const guncelle = (alan, deger) => {
    setTaslak((onceki) => ({ ...onceki, [alan]: deger }))
  }

  const degisenAlanlar = TASARIM_ALANLARI.filter((alan) => taslak[alan] !== business[alan])
  const degisiklikVar = degisenAlanlar.length > 0

  const karsilamaAcik = Boolean(taslak.splash_enabled)
  const anaRenk = guvenliRenk(taslak.primary_color, '#1a7f5a')
  const karsilamaZemin = guvenliRenk(taslak.splash_bg_color, '#0f172a')
  const anaRenkGecersiz = !gecerliRenkMi(taslak.primary_color)
  const karsilamaZeminGecersiz = !gecerliRenkMi(taslak.splash_bg_color)

  const geriAl = () => {
    setTaslak({ ...business })
  }

  const kaydet = async () => {
    if (!degisiklikVar || kaydediliyor) return

    if (anaRenkGecersiz) {
      bildirim.hata('Ana renk #RRGGBB biçiminde olmalı (örn. #1a7f5a).')
      return
    }
    if (karsilamaZeminGecersiz) {
      bildirim.hata('Karşılama ekranı arka plan rengi #RRGGBB biçiminde olmalı.')
      return
    }

    // Yalnızca değişen alanları gönder.
    const govde = {}
    degisenAlanlar.forEach((alan) => {
      govde[alan] = alan === 'splash_duration' ? Number(taslak[alan]) : taslak[alan]
    })

    setKaydediliyor(true)
    try {
      await saveBusiness(govde)
      bildirim.basari('Tasarım kaydedildi.')
    } catch (err) {
      bildirim.hata(err.message)
    } finally {
      setKaydediliyor(false)
    }
  }

  /* ----------------------------------------------------------------- görünüm */

  return (
    <div className="pb-10">
      {/* Üst çubuk */}
      <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900">Tasarım</h1>
          <p className="mt-1 text-sm text-gray-500">Menünüzün görünümünü özelleştirin.</p>
        </div>

        <div className="flex shrink-0 items-center gap-2">
          <button
            type="button"
            className="btn-ikincil"
            onClick={geriAl}
            disabled={!degisiklikVar || kaydediliyor}
          >
            Değişiklikleri geri al
          </button>
          <button
            type="button"
            className="btn-birincil"
            onClick={kaydet}
            disabled={!degisiklikVar || kaydediliyor}
          >
            <Save className="h-4 w-4" aria-hidden="true" />
            {kaydediliyor ? 'Kaydediliyor...' : 'Kaydet'}
          </button>
        </div>
      </div>

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_380px]">
        {/* ---------------------------------------------------- sol sütun */}
        <div className="min-w-0 space-y-6">
          {/* 1. Tasarım Tarzı */}
          <section className="kart p-5">
            <div className="mb-4 flex items-center gap-2">
              <Palette className="h-4 w-4 text-marka-600" aria-hidden="true" />
              <h2 className="text-base font-semibold text-gray-900">Tasarım Tarzı</h2>
            </div>

            <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
              {TEMALAR.map((tema) => {
                const secili = taslak.theme === tema.id
                return (
                  <button
                    key={tema.id}
                    type="button"
                    onClick={() => guncelle('theme', tema.id)}
                    aria-pressed={secili}
                    className={`rounded-xl border p-2.5 text-left hover:border-marka-400 ${
                      secili ? 'border-marka-600 ring-2 ring-marka-600' : 'border-gray-200'
                    }`}
                  >
                    {/* renk şeridi */}
                    <div
                      className="flex h-14 gap-1 overflow-hidden rounded-lg border border-gray-200 p-1"
                      style={{ backgroundColor: tema.renkler.background }}
                    >
                      <div
                        className="flex-1 rounded"
                        style={{ backgroundColor: tema.renkler.background }}
                      />
                      <div
                        className="flex-1 rounded"
                        style={{ backgroundColor: tema.renkler.surface }}
                      />
                      <div
                        className="flex-1 rounded"
                        style={{ backgroundColor: tema.renkler.primary }}
                      />
                    </div>

                    <p className="mt-2 text-sm font-medium text-gray-900">{tema.label}</p>
                    <p className="mt-0.5 text-xs leading-snug text-gray-500">{tema.description}</p>
                  </button>
                )
              })}
            </div>
          </section>

          {/* 2. Yazı Tipi */}
          <section className="kart p-5">
            <div className="mb-1 flex items-center gap-2">
              <span className="text-base leading-none" aria-hidden="true">
                🅰️
              </span>
              <h2 className="text-base font-semibold text-gray-900">Yazı Tipi</h2>
            </div>
            <p className="yardim mb-4">Menüdeki tüm başlık ve metinler bu yazı tipiyle görünür.</p>

            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              {FONTLAR.map((font) => {
                const secili = taslak.font_family === font.id
                return (
                  <button
                    key={font.id}
                    type="button"
                    onClick={() => guncelle('font_family', font.id)}
                    aria-pressed={secili}
                    className={`rounded-xl border p-3 text-left hover:border-marka-400 ${
                      secili ? 'border-marka-600 ring-2 ring-marka-600' : 'border-gray-200'
                    }`}
                  >
                    <p className="text-xs font-medium uppercase tracking-wide text-gray-500">
                      {font.label}
                    </p>
                    <p
                      className="mt-1.5 truncate text-lg text-gray-900"
                      style={{ fontFamily: font.stack }}
                    >
                      Türk Kahvesi 145,00 ₺
                    </p>
                  </button>
                )
              })}
            </div>
          </section>

          {/* 3. Ana Renk */}
          <section className="kart p-5">
            <h2 className="text-base font-semibold text-gray-900">Ana Renk</h2>
            <p className="yardim mb-4">
              Butonlar, fiyatlar ve seçili kategori bu renkle vurgulanır.
            </p>

            <div className="flex flex-wrap items-start gap-3">
              <input
                type="color"
                value={anaRenk}
                onChange={(olay) => guncelle('primary_color', olay.target.value)}
                aria-label="Ana renk seçici"
                className="h-11 w-14 shrink-0 cursor-pointer rounded-lg border border-gray-300 bg-white p-1"
              />

              <div className="w-40">
                <input
                  type="text"
                  value={taslak.primary_color || ''}
                  onChange={(olay) => guncelle('primary_color', olay.target.value.trim())}
                  placeholder="#1a7f5a"
                  maxLength={7}
                  aria-label="Ana renk kodu"
                  className={`girdi font-mono uppercase ${
                    anaRenkGecersiz ? 'border-red-400 focus:border-red-500 focus:ring-red-100' : ''
                  }`}
                />
                {anaRenkGecersiz ? (
                  <p className="hata-metni">#RRGGBB biçiminde yazın.</p>
                ) : null}
              </div>
            </div>

            <div className="mt-4 flex flex-wrap gap-2">
              {HAZIR_RENKLER.map((renk) => {
                const secili = String(taslak.primary_color || '').toLowerCase() === renk
                return (
                  <button
                    key={renk}
                    type="button"
                    onClick={() => guncelle('primary_color', renk)}
                    title={renk}
                    aria-label={`Ana rengi ${renk} yap`}
                    className={`h-9 w-9 rounded-lg border ${
                      secili ? 'border-transparent ring-2 ring-marka-600' : 'border-gray-200'
                    }`}
                    style={{ backgroundColor: renk }}
                  />
                )
              })}
            </div>
          </section>

          {/* 4. Karşılama Ekranı */}
          <section className="kart p-5">
            <div className="flex items-start justify-between gap-4">
              <div>
                <h2 className="text-base font-semibold text-gray-900">Karşılama Ekranı</h2>
                <p className="yardim max-w-xl">
                  Menü açıldığında logonuzla birlikte kısa bir karşılama ekranı gösterilir.
                  Yalnızca müşteri menüsünde görünür, panelde çalışmaz.
                </p>
              </div>

              <button
                type="button"
                role="switch"
                aria-checked={karsilamaAcik}
                aria-label="Karşılama ekranını aç veya kapat"
                onClick={() => guncelle('splash_enabled', !karsilamaAcik)}
                className={`relative mt-1 inline-flex h-6 w-11 shrink-0 items-center rounded-full ${
                  karsilamaAcik ? 'bg-marka-600' : 'bg-gray-300'
                }`}
              >
                <span
                  className={`inline-block h-4 w-4 rounded-full bg-white shadow ${
                    karsilamaAcik ? 'translate-x-6' : 'translate-x-1'
                  }`}
                />
              </button>
            </div>

            <div className={`mt-5 grid gap-5 lg:grid-cols-2 ${karsilamaAcik ? '' : 'opacity-60'}`}>
              <div className="space-y-5">
                {/* Süre */}
                <div>
                  <div className="flex items-center justify-between gap-3">
                    <label htmlFor="karsilama-sure" className="etiket mb-0">
                      Gösterim süresi
                    </label>
                    <span className="text-sm font-medium text-gray-900">
                      {sureyiYaz(taslak.splash_duration)}
                    </span>
                  </div>
                  <input
                    id="karsilama-sure"
                    type="range"
                    min={300}
                    max={5000}
                    step={100}
                    value={Number(taslak.splash_duration) || 1200}
                    onChange={(olay) => guncelle('splash_duration', Number(olay.target.value))}
                    disabled={!karsilamaAcik}
                    className="mt-2 w-full accent-marka-600 disabled:cursor-not-allowed"
                  />
                  <div className="mt-1 flex justify-between text-[11px] text-gray-400">
                    <span>0,3 sn</span>
                    <span>5,0 sn</span>
                  </div>
                </div>

                {/* Arka plan rengi */}
                <div>
                  <span className="etiket">Arka plan rengi</span>
                  <div className="flex flex-wrap items-start gap-3">
                    <input
                      type="color"
                      value={karsilamaZemin}
                      onChange={(olay) => guncelle('splash_bg_color', olay.target.value)}
                      disabled={!karsilamaAcik}
                      aria-label="Karşılama arka plan rengi seçici"
                      className="h-11 w-14 shrink-0 cursor-pointer rounded-lg border border-gray-300 bg-white p-1 disabled:cursor-not-allowed"
                    />
                    <div className="w-40">
                      <input
                        type="text"
                        value={taslak.splash_bg_color || ''}
                        onChange={(olay) => guncelle('splash_bg_color', olay.target.value.trim())}
                        placeholder="#0f172a"
                        maxLength={7}
                        disabled={!karsilamaAcik}
                        aria-label="Karşılama arka plan renk kodu"
                        className={`girdi font-mono uppercase ${
                          karsilamaZeminGecersiz
                            ? 'border-red-400 focus:border-red-500 focus:ring-red-100'
                            : ''
                        }`}
                      />
                      {karsilamaZeminGecersiz ? (
                        <p className="hata-metni">#RRGGBB biçiminde yazın.</p>
                      ) : null}
                    </div>
                  </div>

                  <div className="mt-3 flex flex-wrap gap-2">
                    {HAZIR_KARSILAMA_RENKLERI.map((renk) => {
                      const secili = String(taslak.splash_bg_color || '').toLowerCase() === renk
                      return (
                        <button
                          key={renk}
                          type="button"
                          onClick={() => guncelle('splash_bg_color', renk)}
                          disabled={!karsilamaAcik}
                          title={renk}
                          aria-label={`Arka planı ${renk} yap`}
                          className={`h-9 w-9 rounded-lg border disabled:cursor-not-allowed ${
                            secili ? 'border-transparent ring-2 ring-marka-600' : 'border-gray-200'
                          }`}
                          style={{ backgroundColor: renk }}
                        />
                      )
                    })}
                  </div>
                </div>

                {/* Metin */}
                <div>
                  <label htmlFor="karsilama-metin" className="etiket">
                    Karşılama metni
                  </label>
                  <input
                    id="karsilama-metin"
                    type="text"
                    className="girdi"
                    value={taslak.splash_text || ''}
                    onChange={(olay) => guncelle('splash_text', olay.target.value.slice(0, 200))}
                    placeholder="Hoş geldiniz"
                    maxLength={200}
                    disabled={!karsilamaAcik}
                  />
                  <p className="yardim">{(taslak.splash_text || '').length} / 200 karakter</p>
                </div>
              </div>

              {/* Statik önizleme kutusu */}
              <div>
                <span className="etiket">Önizleme</span>
                <div
                  className="flex h-56 flex-col items-center justify-center gap-3 rounded-xl border border-gray-200 px-6 text-center"
                  style={{ backgroundColor: karsilamaZemin }}
                >
                  {taslak.logo_url ? (
                    <img
                      src={taslak.logo_url}
                      alt=""
                      className="h-16 w-16 rounded-full object-cover"
                    />
                  ) : (
                    <div
                      className="flex h-16 w-16 items-center justify-center rounded-full text-lg font-semibold"
                      style={{
                        backgroundColor: anaRenk,
                        color: zeminUzeriMetinRengi(anaRenk),
                      }}
                    >
                      {(taslak.name || 'K').trim().charAt(0).toUpperCase()}
                    </div>
                  )}

                  <p
                    className="text-base font-medium"
                    style={{ color: zeminUzeriMetinRengi(karsilamaZemin) }}
                  >
                    {taslak.splash_text || 'Hoş geldiniz'}
                  </p>
                </div>

                <div className="mt-3 flex items-start gap-2 rounded-lg border border-blue-200 bg-blue-50 px-3 py-2">
                  <Info className="mt-0.5 h-4 w-4 shrink-0 text-blue-600" aria-hidden="true" />
                  <p className="text-xs leading-snug text-blue-800">
                    Karşılama ekranı yalnızca müşteriler menüyü ilk açtığında görünür; sağdaki
                    canlı önizlemede gösterilmez.
                  </p>
                </div>
              </div>
            </div>
          </section>
        </div>

        {/* ---------------------------------------------------- sağ sütun */}
        <div className="min-w-0">
          <div className="xl:sticky xl:top-6">
            <CanliOnizleme business={taslak} yenile={0} />
          </div>
        </div>
      </div>
    </div>
  )
}
