/**
 * Saf CSS ile çizilmiş iPhone çerçevesi (mockup).
 *
 * İçerik ekran alanını tamamen doldurur, taşan kısımlar kırpılır.
 * Landing kuralı gereği hiçbir animasyon/geçiş yoktur; yalnızca durağan gölge.
 *
 * @param {node}   children  - Ekranın içine yerleşecek içerik
 * @param {string} className - Dış kapsayıcıya eklenecek sınıflar
 */
export default function IPhoneCerceve({ children, className = '' }) {
  return (
    <div className={`relative mx-auto w-full max-w-[340px] ${className}`}>
      {/* sol kenar: sessize alma ve ses tuşları */}
      <div
        className="absolute left-[-3px] top-[16%] h-8 w-[3px] rounded-l bg-gray-800"
        aria-hidden="true"
      />
      <div
        className="absolute left-[-3px] top-[26%] h-12 w-[3px] rounded-l bg-gray-800"
        aria-hidden="true"
      />
      <div
        className="absolute left-[-3px] top-[38%] h-12 w-[3px] rounded-l bg-gray-800"
        aria-hidden="true"
      />

      {/* sağ kenar: güç tuşu */}
      <div
        className="absolute right-[-3px] top-[30%] h-16 w-[3px] rounded-r bg-gray-800"
        aria-hidden="true"
      />

      {/* dış gövde */}
      <div className="relative aspect-[390/844] w-full rounded-[3rem] bg-gray-900 p-3 shadow-2xl">
        {/* ekran alanı */}
        <div className="relative h-full w-full overflow-hidden rounded-[2.5rem] bg-white">
          {/*
            Güvenli alan (safe area): Dynamic Island top-2 (8px) konumunda ve
            26px yüksekliğinde, yani alt hizası 34px. İçeriği 44px aşağıdan
            başlatarak çentiğin altına/arkasına taşmasını engelliyoruz —
            gerçek iOS'un üst güvenli alan payıyla aynı değer.
          */}
          <div className="h-full w-full pt-[44px]">{children}</div>

          {/* dynamic island */}
          <div
            className="pointer-events-none absolute left-1/2 top-2 z-10 h-[26px] w-[92px] -translate-x-1/2 rounded-full bg-black"
            aria-hidden="true"
          />
        </div>
      </div>
    </div>
  )
}
