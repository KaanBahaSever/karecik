// Subdomain resolution.
//
// Production : kahve-duragi.karecik.com  -> "kahve-duragi"
// Development: kahve-duragi.localhost    -> "kahve-duragi"
// Path based : localhost:5173/m/kahve-duragi
//
// The root domains can be overridden through frontend/.env:
//   VITE_APP_DOMAIN=karecik.com
//   VITE_ROOT_DOMAIN=localhost

const APP_DOMAIN = import.meta.env.VITE_APP_DOMAIN || 'karecik.com'
const ROOT_DOMAIN = import.meta.env.VITE_ROOT_DOMAIN || 'localhost'

const RESERVED = new Set(['www', 'api', 'app', 'panel', 'admin'])

/** Returns the subdomain of the current address, or null when there is none. */
export function getSubdomain(hostname = window.location.hostname) {
  const host = String(hostname || '').toLowerCase()
  if (!host) return null

  // A bare IP address has no subdomain
  if (/^\d{1,3}(\.\d{1,3}){3}$/.test(host)) return null

  for (const root of [APP_DOMAIN, ROOT_DOMAIN]) {
    const suffix = `.${String(root).toLowerCase()}`
    if (host.endsWith(suffix)) {
      const sub = host.slice(0, -suffix.length).split('.')[0]
      if (!sub || RESERVED.has(sub)) return null
      return sub
    }
  }
  return null
}

/** Builds the full customer menu URL for a slug. */
export function menuUrl(slug) {
  if (!slug) return ''

  const { protocol, port, hostname } = window.location
  const isLocal =
    hostname === 'localhost' ||
    hostname.endsWith(`.${ROOT_DOMAIN}`) ||
    hostname === ROOT_DOMAIN ||
    /^\d{1,3}(\.\d{1,3}){3}$/.test(hostname)

  if (isLocal) {
    const portSuffix = port ? `:${port}` : ''
    return `${protocol}//${slug}.${ROOT_DOMAIN}${portSuffix}`
  }
  return `https://${slug}.${APP_DOMAIN}`
}

/** Path-based fallback URL — needs no hosts file entry. */
export function menuPathUrl(slug) {
  if (!slug) return ''
  return `${window.location.origin}/m/${slug}`
}

export { APP_DOMAIN, ROOT_DOMAIN }
