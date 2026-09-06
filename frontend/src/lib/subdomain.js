// Business subdomain resolution and public address building.
//
//   {business-slug}.karecik.com / {menu-slug}
//    └── identifies the tenant   └── identifies one menu within that tenant
//
// Production : kahve-duragi.karecik.com/kahvalti
// Development: kahve-duragi.localhost:5173/kahvalti
// Path based : localhost:5173/m/kahve-duragi/kahvalti
//
// Menu slugs are unique only within a business, so every menu address needs
// both segments — the subdomain alone never identifies a menu.
//
// The root domains can be overridden through frontend/.env:
//   VITE_APP_DOMAIN=karecik.com
//   VITE_ROOT_DOMAIN=localhost

const APP_DOMAIN = import.meta.env.VITE_APP_DOMAIN || 'karecik.com'
const ROOT_DOMAIN = import.meta.env.VITE_ROOT_DOMAIN || 'localhost'

const RESERVED = new Set(['www', 'api', 'app', 'panel', 'admin'])

/** Returns the business slug in the current address, or null when there is none. */
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

/** True while the app is served from a development host. */
function isLocalHost(hostname) {
  return (
    hostname === 'localhost' ||
    hostname === ROOT_DOMAIN ||
    hostname.endsWith(`.${ROOT_DOMAIN}`) ||
    /^\d{1,3}(\.\d{1,3}){3}$/.test(hostname)
  )
}

/**
 * The tenant's root address, with no menu path:
 * https://{business}.karecik.com — http://{business}.localhost:5173 locally.
 */
export function businessUrl(businessSlug) {
  if (!businessSlug) return ''

  const { protocol, port, hostname } = window.location
  if (isLocalHost(hostname)) {
    const portSuffix = port ? `:${port}` : ''
    return `${protocol}//${businessSlug}.${ROOT_DOMAIN}${portSuffix}`
  }
  return `https://${businessSlug}.${APP_DOMAIN}`
}

/**
 * The full customer menu URL: https://{business}.karecik.com/{menu}.
 * Both segments are required — the menu slug alone is meaningless outside its
 * business. Without a menu slug this degrades to the tenant's root address.
 */
export function menuUrl(businessSlug, menuSlug) {
  const base = businessUrl(businessSlug)
  if (!base) return ''
  return menuSlug ? `${base}/${menuSlug}` : base
}

/** Path-based fallback URL — needs no hosts file entry. */
export function menuPathUrl(businessSlug, menuSlug) {
  if (!businessSlug) return ''
  const base = `${window.location.origin}/m/${businessSlug}`
  return menuSlug ? `${base}/${menuSlug}` : base
}

export { APP_DOMAIN, ROOT_DOMAIN }
