import { createContext, useCallback, useContext, useMemo, useState } from 'react'
import { AlertCircle, CheckCircle2, Info, X } from 'lucide-react'

const BildirimContext = createContext(null)

let sayac = 0

/**
 * Sağ altta beliren kısa bildirimler (toast).
 * Kullanım:
 *   const bildirim = useBildirim()
 *   bildirim.basari('Kaydedildi.')
 *   bildirim.hata(err.message)
 */
export function BildirimSaglayici({ children }) {
  const [bildirimler, setBildirimler] = useState([])

  const kaldir = useCallback((id) => {
    setBildirimler((oncekiler) => oncekiler.filter((b) => b.id !== id))
  }, [])

  const ekle = useCallback(
    (tur, mesaj, sure = 4000) => {
      if (!mesaj) return
      sayac += 1
      const id = sayac
      setBildirimler((oncekiler) => [...oncekiler, { id, tur, mesaj }])
      if (sure > 0) {
        setTimeout(() => kaldir(id), sure)
      }
    },
    [kaldir],
  )

  const value = useMemo(
    () => ({
      basari: (mesaj, sure) => ekle('basari', mesaj, sure),
      hata: (mesaj, sure) => ekle('hata', mesaj, sure ?? 6000),
      bilgi: (mesaj, sure) => ekle('bilgi', mesaj, sure),
      kaldir,
    }),
    [ekle, kaldir],
  )

  return (
    <BildirimContext.Provider value={value}>
      {children}

      <div
        className="pointer-events-none fixed bottom-4 right-4 z-[100] flex w-full max-w-sm flex-col gap-2"
        role="status"
        aria-live="polite"
      >
        {bildirimler.map((bildirim) => (
          <BildirimKutusu key={bildirim.id} bildirim={bildirim} kapat={() => kaldir(bildirim.id)} />
        ))}
      </div>
    </BildirimContext.Provider>
  )
}

const STILLER = {
  basari: { kutu: 'border-green-200 bg-green-50 text-green-800', Ikon: CheckCircle2 },
  hata: { kutu: 'border-red-200 bg-red-50 text-red-800', Ikon: AlertCircle },
  bilgi: { kutu: 'border-blue-200 bg-blue-50 text-blue-800', Ikon: Info },
}

function BildirimKutusu({ bildirim, kapat }) {
  const stil = STILLER[bildirim.tur] || STILLER.bilgi
  const { Ikon } = stil

  return (
    <div
      className={`pointer-events-auto flex items-start gap-3 rounded-lg border px-4 py-3 shadow-panel ${stil.kutu}`}
    >
      <Ikon className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
      <p className="flex-1 text-sm leading-snug">{bildirim.mesaj}</p>
      <button
        type="button"
        onClick={kapat}
        className="shrink-0 opacity-60 hover:opacity-100"
        aria-label="Bildirimi kapat"
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  )
}

export function useBildirim() {
  const context = useContext(BildirimContext)
  if (!context) {
    throw new Error('useBildirim yalnızca <BildirimSaglayici> içinde kullanılabilir.')
  }
  return context
}
