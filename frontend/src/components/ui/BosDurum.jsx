/**
 * İçerik bulunmadığında gösterilen boş durum kutusu.
 *
 * @param {Component} ikon     - lucide-react bileşeni
 * @param {string}    baslik
 * @param {string}    aciklama
 * @param {node}      aksiyon  - Buton vb.
 */
export default function BosDurum({ ikon: Ikon, baslik, aciklama, aksiyon, className = '' }) {
  return (
    <div
      className={`flex flex-col items-center justify-center rounded-xl border border-dashed border-gray-300 bg-gray-50/60 px-6 py-12 text-center ${className}`}
    >
      {Ikon ? (
        <div className="mb-3 flex h-11 w-11 items-center justify-center rounded-full bg-white shadow-kart">
          <Ikon className="h-5 w-5 text-gray-400" aria-hidden="true" />
        </div>
      ) : null}

      <h3 className="text-sm font-semibold text-gray-900">{baslik}</h3>
      {aciklama ? <p className="mt-1 max-w-sm text-sm text-gray-500">{aciklama}</p> : null}
      {aksiyon ? <div className="mt-4">{aksiyon}</div> : null}
    </div>
  )
}
