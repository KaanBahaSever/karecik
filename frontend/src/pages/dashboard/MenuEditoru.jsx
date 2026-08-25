import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
} from '@dnd-kit/core'
import {
  SortableContext,
  arrayMove,
  sortableKeyboardCoordinates,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { LayoutList, Percent, Plus } from 'lucide-react'

import api from '../../lib/api'
import { useAuth } from '../../lib/auth.jsx'
import { dilBul } from '../../locales/index.js'

import { useBildirim } from '../../components/ui/Bildirim.jsx'
import BosDurum from '../../components/ui/BosDurum.jsx'
import OnayModal from '../../components/ui/OnayModal.jsx'
import Yukleniyor from '../../components/ui/Yukleniyor.jsx'

import KategoriSatiri from '../../components/dashboard/KategoriSatiri.jsx'
import UrunSatiri from '../../components/dashboard/UrunSatiri.jsx'
import KategoriModal from '../../components/dashboard/KategoriModal.jsx'
import UrunModal from '../../components/dashboard/UrunModal.jsx'
import TopluFiyatModal from '../../components/dashboard/TopluFiyatModal.jsx'
import CanliOnizleme from '../../components/dashboard/CanliOnizleme.jsx'

/** Ürün dizisini category_id'ye göre gruplar ve her grubu position'a göre sıralar. */
function urunleriGrupla(urunler) {
  const harita = {}
  const liste = Array.isArray(urunler) ? urunler : []

  liste.forEach((urun) => {
    const anahtar = String(urun.category_id)
    if (!harita[anahtar]) harita[anahtar] = []
    harita[anahtar].push(urun)
  })

  Object.keys(harita).forEach((anahtar) => {
    harita[anahtar].sort((a, b) => (a.position ?? 0) - (b.position ?? 0))
  })

  return harita
}

/** Seçili dilde kategori adı; yoksa Türkçesine düşer. */
function kategoriAdiAl(kategori, dil) {
  if (!kategori) return ''
  return (
    kategori.translations?.[dil]?.name || kategori.translations?.tr?.name || 'İsimsiz kategori'
  )
}

/** Seçili dilde ürün adı; yoksa Türkçesine düşer. */
function urunAdiAl(urun, dil) {
  if (!urun) return ''
  return urun.translations?.[dil]?.name || urun.translations?.tr?.name || 'İsimsiz ürün'
}

export default function MenuEditoru() {
  const { business } = useAuth()
  const bildirim = useBildirim()

  /* ------------------------------------------------------------- işletme */

  const diller = useMemo(() => {
    const liste = business?.languages
    return Array.isArray(liste) && liste.length > 0 ? liste : ['tr']
  }, [business])

  const varsayilanDil = business?.default_language || diller[0] || 'tr'
  const paraBirimi = business?.currency || 'TRY'

  const [dil, setDil] = useState(varsayilanDil)

  useEffect(() => {
    if (!diller.includes(dil)) setDil(varsayilanDil)
  }, [diller, dil, varsayilanDil])

  /* --------------------------------------------------------------- veri */

  const [yukleniyor, setYukleniyor] = useState(true)
  const [kategoriler, setKategoriler] = useState([])
  const [urunHaritasi, setUrunHaritasi] = useState({})
  const [acikIdler, setAcikIdler] = useState([])
  const [onizlemeSayaci, setOnizlemeSayaci] = useState(0)

  /* -------------------------------------------------------------- modal */

  const [kategoriModalAcik, setKategoriModalAcik] = useState(false)
  const [duzenlenenKategori, setDuzenlenenKategori] = useState(null)

  const [urunModalAcik, setUrunModalAcik] = useState(false)
  const [duzenlenenUrun, setDuzenlenenUrun] = useState(null)
  const [seciliKategoriId, setSeciliKategoriId] = useState(null)

  const [topluFiyatAcik, setTopluFiyatAcik] = useState(false)

  const [silinecekKategori, setSilinecekKategori] = useState(null)
  const [silinecekUrun, setSilinecekUrun] = useState(null)
  const [silmeIslemde, setSilmeIslemde] = useState(false)

  const onizlemeYenile = useCallback(() => {
    setOnizlemeSayaci((onceki) => onceki + 1)
  }, [])

  /* ------------------------------------------------------------ yükleme */

  const veriYukle = useCallback(async () => {
    setYukleniyor(true)
    try {
      const [gelenKategoriler, gelenUrunler] = await Promise.all([
        api.listCategories(),
        api.listProducts(),
      ])

      const liste = Array.isArray(gelenKategoriler) ? gelenKategoriler : []
      setKategoriler(liste)
      setUrunHaritasi(urunleriGrupla(gelenUrunler))
      setAcikIdler((oncekiler) =>
        oncekiler.length > 0 ? oncekiler : liste.slice(0, 1).map((k) => k.id),
      )
    } catch (err) {
      bildirim.hata(err.message)
    } finally {
      setYukleniyor(false)
    }
  }, [bildirim])

  useEffect(() => {
    veriYukle()
  }, [veriYukle])

  const urunleriAl = useCallback(
    (kategoriId) => urunHaritasi[String(kategoriId)] || [],
    [urunHaritasi],
  )

  const urunBul = useCallback(
    (urunId) => {
      const gruplar = Object.values(urunHaritasi)
      for (let i = 0; i < gruplar.length; i += 1) {
        const bulunan = gruplar[i].find((u) => u.id === urunId)
        if (bulunan) return bulunan
      }
      return null
    },
    [urunHaritasi],
  )

  /** Tek bir ürünün alanlarını yerel olarak günceller (iyimser güncelleme). */
  const urunuYerelGuncelle = useCallback((urunId, degisiklikler) => {
    setUrunHaritasi((oncekiler) => {
      const yeni = {}
      Object.keys(oncekiler).forEach((anahtar) => {
        yeni[anahtar] = oncekiler[anahtar].map((u) =>
          u.id === urunId ? { ...u, ...degisiklikler } : u,
        )
      })
      return yeni
    })
  }, [])

  /* ------------------------------------------------------- sürükle-bırak */

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  /** Kategori sırası değişti: önce yerel state, sonra sunucu. */
  async function kategoriSiralandi(olay) {
    const { active, over } = olay
    if (!over || active.id === over.id) return

    const eskiSira = kategoriler
    const eskiIndex = kategoriler.findIndex((k) => k.id === active.id)
    const yeniIndex = kategoriler.findIndex((k) => k.id === over.id)
    if (eskiIndex < 0 || yeniIndex < 0) return

    const yeniSira = arrayMove(kategoriler, eskiIndex, yeniIndex)
    setKategoriler(yeniSira)
    onizlemeYenile()

    try {
      await api.reorderCategories(yeniSira.map((k) => k.id))
    } catch (err) {
      setKategoriler(eskiSira)
      onizlemeYenile()
      bildirim.hata(err.message)
    }
  }

  /** Bir kategorinin ürün sırası değişti. */
  async function urunSiralandi(kategoriId, olay) {
    const { active, over } = olay
    if (!over || active.id === over.id) return

    const anahtar = String(kategoriId)
    const eskiListe = urunHaritasi[anahtar] || []
    const eskiIndex = eskiListe.findIndex((u) => u.id === active.id)
    const yeniIndex = eskiListe.findIndex((u) => u.id === over.id)
    if (eskiIndex < 0 || yeniIndex < 0) return

    const yeniListe = arrayMove(eskiListe, eskiIndex, yeniIndex)
    setUrunHaritasi((oncekiler) => ({ ...oncekiler, [anahtar]: yeniListe }))
    onizlemeYenile()

    try {
      await api.reorderProducts(kategoriId, yeniListe.map((u) => u.id))
    } catch (err) {
      setUrunHaritasi((oncekiler) => ({ ...oncekiler, [anahtar]: eskiListe }))
      onizlemeYenile()
      bildirim.hata(err.message)
    }
  }

  /* ------------------------------------------------------- satır işlemleri */

  function kategoriAcKapat(kategoriId) {
    setAcikIdler((oncekiler) =>
      oncekiler.includes(kategoriId)
        ? oncekiler.filter((id) => id !== kategoriId)
        : [...oncekiler, kategoriId],
    )
  }

  function kategoriEkleAc() {
    setDuzenlenenKategori(null)
    setKategoriModalAcik(true)
  }

  function kategoriDuzenleAc(kategori) {
    setDuzenlenenKategori(kategori)
    setKategoriModalAcik(true)
  }

  function urunEkleAc(kategoriId) {
    setDuzenlenenUrun(null)
    setSeciliKategoriId(kategoriId)
    setUrunModalAcik(true)
  }

  function urunDuzenleAc(urun) {
    setDuzenlenenUrun(urun)
    setSeciliKategoriId(urun.category_id)
    setUrunModalAcik(true)
  }

  function kategoriKaydedildi(kategori) {
    setKategoriModalAcik(false)
    setDuzenlenenKategori(null)

    if (!kategori || !kategori.id) {
      veriYukle()
      onizlemeYenile()
      return
    }

    setKategoriler((oncekiler) => {
      const varMi = oncekiler.some((k) => k.id === kategori.id)
      return varMi
        ? oncekiler.map((k) => (k.id === kategori.id ? kategori : k))
        : [...oncekiler, kategori]
    })
    setAcikIdler((oncekiler) =>
      oncekiler.includes(kategori.id) ? oncekiler : [...oncekiler, kategori.id],
    )
    onizlemeYenile()
  }

  function urunKaydedildi(urun) {
    setUrunModalAcik(false)
    setDuzenlenenUrun(null)

    if (!urun || !urun.id) {
      veriYukle()
      onizlemeYenile()
      return
    }

    setUrunHaritasi((oncekiler) => {
      const yeni = {}
      // Ürün başka kategoriye taşınmış olabilir: önce her yerden çıkar.
      Object.keys(oncekiler).forEach((anahtar) => {
        yeni[anahtar] = oncekiler[anahtar].filter((u) => u.id !== urun.id)
      })

      const anahtar = String(urun.category_id)
      const liste = [...(yeni[anahtar] || []), urun]
      liste.sort((a, b) => (a.position ?? 0) - (b.position ?? 0))
      yeni[anahtar] = liste
      return yeni
    })

    setAcikIdler((oncekiler) =>
      oncekiler.includes(urun.category_id) ? oncekiler : [...oncekiler, urun.category_id],
    )
    onizlemeYenile()
  }

  /** Satır içi hızlı fiyat düzenleme. */
  async function urunFiyatiDegisti(urunId, yeniFiyat) {
    const mevcut = urunBul(urunId)
    const eskiFiyat = mevcut ? mevcut.price : 0

    urunuYerelGuncelle(urunId, { price: yeniFiyat })
    onizlemeYenile()

    try {
      const guncel = await api.updateProductPrice(urunId, yeniFiyat)
      if (guncel && guncel.id) urunuYerelGuncelle(urunId, guncel)
      bildirim.basari('Fiyat güncellendi.')
    } catch (err) {
      urunuYerelGuncelle(urunId, { price: eskiFiyat })
      onizlemeYenile()
      bildirim.hata(err.message)
    }
  }

  /** Göz ikonu ile menüde göster / gizle. */
  async function urunAktifligiDegisti(urunId, yeniDurum) {
    urunuYerelGuncelle(urunId, { is_active: yeniDurum })
    onizlemeYenile()

    try {
      const guncel = await api.updateProduct(urunId, { is_active: yeniDurum })
      if (guncel && guncel.id) urunuYerelGuncelle(urunId, guncel)
    } catch (err) {
      urunuYerelGuncelle(urunId, { is_active: !yeniDurum })
      onizlemeYenile()
      bildirim.hata(err.message)
    }
  }

  async function kategoriSilOnayla() {
    if (!silinecekKategori) return
    const hedef = silinecekKategori
    setSilmeIslemde(true)

    try {
      await api.deleteCategory(hedef.id)

      setKategoriler((oncekiler) => oncekiler.filter((k) => k.id !== hedef.id))
      setUrunHaritasi((oncekiler) => {
        const yeni = { ...oncekiler }
        delete yeni[String(hedef.id)]
        return yeni
      })
      setAcikIdler((oncekiler) => oncekiler.filter((id) => id !== hedef.id))

      setSilinecekKategori(null)
      onizlemeYenile()
      bildirim.basari('Kategori silindi.')
    } catch (err) {
      bildirim.hata(err.message)
    } finally {
      setSilmeIslemde(false)
    }
  }

  async function urunSilOnayla() {
    if (!silinecekUrun) return
    const hedef = silinecekUrun
    setSilmeIslemde(true)

    try {
      await api.deleteProduct(hedef.id)

      setUrunHaritasi((oncekiler) => {
        const yeni = {}
        Object.keys(oncekiler).forEach((anahtar) => {
          yeni[anahtar] = oncekiler[anahtar].filter((u) => u.id !== hedef.id)
        })
        return yeni
      })

      setSilinecekUrun(null)
      onizlemeYenile()
      bildirim.basari('Ürün silindi.')
    } catch (err) {
      bildirim.hata(err.message)
    } finally {
      setSilmeIslemde(false)
    }
  }

  function topluFiyatUygulandi() {
    setTopluFiyatAcik(false)
    veriYukle()
    onizlemeYenile()
  }

  /* --------------------------------------------------------------- render */

  return (
    <div>
      {/* --------------------------------------------------------- başlık */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <h1 className="text-xl font-semibold text-gray-900">Menü Yönetimi</h1>
          <p className="mt-1 text-sm text-gray-500">
            Kategorileri ve ürünleri sürükleyerek sıralayabilirsiniz.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <button
            type="button"
            className="btn-ikincil"
            onClick={() => setTopluFiyatAcik(true)}
          >
            <Percent className="h-4 w-4" aria-hidden="true" />
            Toplu Fiyat Güncelle
          </button>

          <button type="button" className="btn-birincil" onClick={kategoriEkleAc}>
            <Plus className="h-4 w-4" aria-hidden="true" />
            Kategori Ekle
          </button>
        </div>
      </div>

      {/* --------------------------------------------------- dil seçici */}
      {diller.length > 1 ? (
        <div className="mt-4 inline-flex flex-wrap gap-1 rounded-lg border border-gray-200 bg-gray-50 p-1">
          {diller.map((kod) => {
            const bilgi = dilBul(kod)
            const seciliMi = kod === dil
            return (
              <button
                key={kod}
                type="button"
                onClick={() => setDil(kod)}
                aria-pressed={seciliMi}
                className={`rounded-md px-3 py-1.5 text-xs font-medium ${
                  seciliMi
                    ? 'bg-white text-gray-900 shadow-kart'
                    : 'text-gray-500 hover:text-gray-700'
                }`}
              >
                <span className="mr-1" aria-hidden="true">
                  {bilgi.kisa}
                </span>
                {bilgi.label}
              </button>
            )
          })}
        </div>
      ) : null}

      {/* ----------------------------------------------------- ana düzen */}
      <div className="mt-6 grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
        {/* ------------------------------------------------------ editör */}
        <div className="min-w-0">
          {yukleniyor ? (
            <Yukleniyor metin="Menü yükleniyor..." />
          ) : kategoriler.length === 0 ? (
            <BosDurum
              ikon={LayoutList}
              baslik="Henüz kategori eklemediniz"
              aciklama="Menünüzü oluşturmak için ilk kategoriyi ekleyin. Örn: Sıcak İçecekler, Tatlılar"
              aksiyon={
                <button type="button" className="btn-birincil" onClick={kategoriEkleAc}>
                  <Plus className="h-4 w-4" aria-hidden="true" />
                  Kategori Ekle
                </button>
              }
            />
          ) : (
            /* 1. DndContext — yalnızca kategori sıralaması */
            <DndContext
              sensors={sensors}
              collisionDetection={closestCenter}
              onDragEnd={kategoriSiralandi}
            >
              <SortableContext
                items={kategoriler.map((k) => k.id)}
                strategy={verticalListSortingStrategy}
              >
                <div className="flex flex-col gap-3">
                  {kategoriler.map((kategori) => {
                    const urunler = urunleriAl(kategori.id)
                    const acik = acikIdler.includes(kategori.id)

                    return (
                      <KategoriSatiri
                        key={kategori.id}
                        kategori={kategori}
                        urunler={urunler}
                        acik={acik}
                        acKapat={() => kategoriAcKapat(kategori.id)}
                        dil={dil}
                        duzenle={() => kategoriDuzenleAc(kategori)}
                        sil={() => setSilinecekKategori(kategori)}
                        urunEkle={() => urunEkleAc(kategori.id)}
                        cocuklar={
                          <div className="border-t border-gray-100 bg-gray-50/70 px-2 py-3 sm:px-3">
                            {urunler.length === 0 ? (
                              <p className="py-3 text-center text-sm text-gray-500">
                                Bu kategoride henüz ürün yok.
                              </p>
                            ) : (
                              /* 2. DndContext — yalnızca bu kategorinin ürünleri */
                              <DndContext
                                sensors={sensors}
                                collisionDetection={closestCenter}
                                onDragEnd={(olay) => urunSiralandi(kategori.id, olay)}
                              >
                                <SortableContext
                                  items={urunler.map((u) => u.id)}
                                  strategy={verticalListSortingStrategy}
                                >
                                  <div className="flex flex-col gap-2">
                                    {urunler.map((urun) => (
                                      <UrunSatiri
                                        key={urun.id}
                                        urun={urun}
                                        dil={dil}
                                        paraBirimi={paraBirimi}
                                        duzenle={() => urunDuzenleAc(urun)}
                                        sil={() => setSilinecekUrun(urun)}
                                        fiyatDegisti={urunFiyatiDegisti}
                                        aktiflikDegisti={urunAktifligiDegisti}
                                      />
                                    ))}
                                  </div>
                                </SortableContext>
                              </DndContext>
                            )}

                            <button
                              type="button"
                              className="btn-ikincil btn-kucuk mt-3 w-full"
                              onClick={() => urunEkleAc(kategori.id)}
                            >
                              <Plus className="h-3.5 w-3.5" aria-hidden="true" />
                              Ürün Ekle
                            </button>
                          </div>
                        }
                      />
                    )
                  })}
                </div>
              </SortableContext>
            </DndContext>
          )}
        </div>

        {/* -------------------------------------------------- canlı önizleme */}
        <div className="hidden xl:block">
          <div className="sticky top-6">
            <CanliOnizleme business={business} yenile={onizlemeSayaci} />
          </div>
        </div>
      </div>

      {/* --------------------------------------------------------- modaller */}
      <KategoriModal
        acik={kategoriModalAcik}
        kapat={() => {
          setKategoriModalAcik(false)
          setDuzenlenenKategori(null)
        }}
        kategori={duzenlenenKategori}
        diller={diller}
        varsayilanDil={varsayilanDil}
        kaydedildi={kategoriKaydedildi}
      />

      <UrunModal
        acik={urunModalAcik}
        kapat={() => {
          setUrunModalAcik(false)
          setDuzenlenenUrun(null)
        }}
        urun={duzenlenenUrun}
        kategoriler={kategoriler}
        seciliKategoriId={seciliKategoriId}
        diller={diller}
        varsayilanDil={varsayilanDil}
        paraBirimi={paraBirimi}
        kaydedildi={urunKaydedildi}
      />

      <TopluFiyatModal
        acik={topluFiyatAcik}
        kapat={() => setTopluFiyatAcik(false)}
        kategoriler={kategoriler}
        paraBirimi={paraBirimi}
        uygulandi={topluFiyatUygulandi}
      />

      <OnayModal
        acik={Boolean(silinecekKategori)}
        kapat={() => setSilinecekKategori(null)}
        onayla={kategoriSilOnayla}
        baslik="Kategoriyi sil"
        mesaj={
          silinecekKategori
            ? `"${kategoriAdiAl(silinecekKategori, dil)}" kategorisi ve içindeki ${
                urunleriAl(silinecekKategori.id).length
              } ürün kalıcı olarak silinecek.`
            : ''
        }
        onayMetni="Sil"
        islemde={silmeIslemde}
      />

      <OnayModal
        acik={Boolean(silinecekUrun)}
        kapat={() => setSilinecekUrun(null)}
        onayla={urunSilOnayla}
        baslik="Ürünü sil"
        mesaj={
          silinecekUrun ? `"${urunAdiAl(silinecekUrun, dil)}" ürünü kalıcı olarak silinecek.` : ''
        }
        onayMetni="Sil"
        islemde={silmeIslemde}
      />
    </div>
  )
}
