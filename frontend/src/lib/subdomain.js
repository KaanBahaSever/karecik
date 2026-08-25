// Subdomain çözümlemesi.
//
// Canlı:      kahve-duragi.karecik.com  -> "kahve-duragi"
// Geliştirme: kahve-duragi.localhost    -> "kahve-duragi"
// Yol tabanlı: localhost:5173/m/kahve-duragi
//
// Kök alan adlarını frontend/.env üzerinden değiştirebilirsin:
//   VITE_APP_DOMAIN=karecik.com
//   VITE_ROOT_DOMAIN=localhost

const APP_DOMAIN = import.meta.env.VITE_APP_DOMAIN || 'karecik.com'
const ROOT_DOMAIN = import.meta.env.VITE_ROOT_DOMAIN || 'localhost'

const AYRILMIS = new Set(['www', 'api', 'app', 'panel', 'admin'])

/** Adres çubuğundaki subdomain'i döndürür; yoksa null. */
export function subdomainAl(hostname = window.location.hostname) {
  const host = String(hostname || '').toLowerCase()
  if (!host) return null

  // Ham IP adreslerinde subdomain yoktur
  if (/^\d{1,3}(\.\d{1,3}){3}$/.test(host)) return null

  for (const kok of [APP_DOMAIN, ROOT_DOMAIN]) {
    const son = `.${String(kok).toLowerCase()}`
    if (host.endsWith(son)) {
      const alt = host.slice(0, -son.length).split('.')[0]
      if (!alt || AYRILMIS.has(alt)) return null
      return alt
    }
  }
  return null
}

/** Verilen slug için müşteri menüsünün tam adresini üretir. */
export function menuAdresi(slug) {
  if (!slug) return ''

  const { protocol, port, hostname } = window.location
  const yerel =
    hostname === 'localhost' ||
    hostname.endsWith(`.${ROOT_DOMAIN}`) ||
    hostname === ROOT_DOMAIN ||
    /^\d{1,3}(\.\d{1,3}){3}$/.test(hostname)

  if (yerel) {
    const portEki = port ? `:${port}` : ''
    return `${protocol}//${slug}.${ROOT_DOMAIN}${portEki}`
  }
  return `https://${slug}.${APP_DOMAIN}`
}

/** Yol tabanlı yedek adres — hosts dosyası ayarı gerektirmez. */
export function menuYoluAdresi(slug) {
  if (!slug) return ''
  return `${window.location.origin}/m/${slug}`
}

export { APP_DOMAIN, ROOT_DOMAIN }
