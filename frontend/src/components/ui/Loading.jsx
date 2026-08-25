import { Loader2 } from 'lucide-react'

/**
 * Yükleniyor göstergesi.
 *
 * @param {boolean} tamEkran - Ekranı kaplayan sürüm
 * @param {string}  metin    - Altta gösterilecek açıklama
 * @param {string}  className
 */
export default function Yukleniyor({ tamEkran = false, metin = '', className = '' }) {
  const icerik = (
    <div className={`flex flex-col items-center justify-center gap-3 ${className}`}>
      <Loader2 className="h-6 w-6 animate-spin text-marka-600" aria-hidden="true" />
      {metin ? <p className="text-sm text-gray-500">{metin}</p> : null}
      <span className="sr-only">Yükleniyor</span>
    </div>
  )

  if (tamEkran) {
    return <div className="flex min-h-screen items-center justify-center bg-white">{icerik}</div>
  }
  return <div className="py-12">{icerik}</div>
}
