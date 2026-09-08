// Karecik API client.
//
// BASE URL. Empty means "same origin", and it should stay empty everywhere: the
// Vite dev proxy forwards /api to the Go server locally, and in production that
// same Go server is what served this page.
//
// VITE_API_URL exists for a deployment that puts the API on its own host, and
// pointing it at one here breaks two things at once — the session cookie is not
// sent cross-origin, so the panel 401s on everything, and customer menus stop
// resolving, because the tenant is read from the request's Host header.
//
// Vite inlines it into the bundle, so it is a BUILD-time value: changing it
// means rebuilding, not restarting.
//
// AUTHENTICATION. There is no token here any more. The session lives in an
// HttpOnly cookie the browser attaches by itself, which is precisely why this
// file can no longer read it: a token in localStorage was readable by any
// script that reached the page, and that is the class of bug this removes.
// What the client must do instead is ask for the cookie to be sent —
// `credentials: 'include'` below — because fetch omits credentials on a
// cross-origin request unless told otherwise.
//
// NOTE: error messages are Turkish on purpose — they are shown to the user.

const API_BASE = import.meta.env.VITE_API_URL || ''

/* --------------------------------------------------------------- error */

export class ApiError extends Error {
  constructor(message, status, code) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

/* ------------------------------------------------------------ requests */

async function request(path, { method = 'GET', body, isForm = false } = {}) {
  const headers = {}

  if (!isForm && body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }

  let response
  try {
    response = await fetch(`${API_BASE}${path}`, {
      method,
      headers,
      // Sent on EVERY request, public ones included. Same-origin requests would
      // carry the cookie anyway, but a cross-origin one drops it silently
      // without this — and "silently" is the problem: the request succeeds,
      // arrives unauthenticated, and comes back 401 with nothing to point at.
      credentials: 'include',
      body: isForm ? body : body !== undefined ? JSON.stringify(body) : undefined,
    })
  } catch {
    throw new ApiError(
      'Sunucuya ulaşılamadı. Backend çalışıyor mu? (cd backend && go run ./cmd/api)',
      0,
      'NETWORK_ERROR',
    )
  }

  if (response.status === 204) return null

  const text = await response.text()
  let data = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = null
    }
  }

  if (!response.ok) {
    const message = data?.error || `Beklenmeyen bir hata oluştu (${response.status}).`
    // Nothing to clear on a 401 any more: the cookie is HttpOnly, and the server
    // already expires it when it refuses the session. The auth context notices
    // the 401 and sends the user to the login screen.
    throw new ApiError(message, response.status, data?.code)
  }

  return data
}

const qs = (params) => {
  const search = new URLSearchParams()
  Object.entries(params || {}).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') search.set(key, value)
  })
  const query = search.toString()
  return query ? `?${query}` : ''
}

/* ----------------------------------------------------------------- API */

export const api = {
  /* static catalogues: currencies, themes, fonts, allergens, languages */
  meta: () => request('/api/meta'),
  health: () => request('/api/health'),

  /* authentication */
  register: (payload) =>
    request('/api/auth/register', { method: 'POST', body: payload }),
  login: (payload) => request('/api/auth/login', { method: 'POST', body: payload }),
  logout: () => request('/api/auth/logout', { method: 'POST' }),
  // Answers the same way whether or not the address is registered — see
  // handlers.ForgotPassword. Do not add a branch here that assumes otherwise.
  forgotPassword: (email) =>
    request('/api/auth/forgot-password', { method: 'POST', body: { email } }),
  resetPassword: (token, password) =>
    request('/api/auth/reset-password', { method: 'POST', body: { token, password } }),
  changePassword: (currentPassword, newPassword) =>
    request('/api/auth/change-password', {
      method: 'POST',
      body: { current_password: currentPassword, new_password: newPassword },
    }),
  me: () => request('/api/auth/me'),

  /* business (the slim account record — every setting lives on a menu) */
  getBusiness: () => request('/api/business'),
  // Exactly two editable fields: `name` and `slug`. The slug is the tenant
  // subdomain, so it is globally unique and reserved-checked by the server,
  // which answers 409 when another business already owns it.
  updateBusiness: (payload) => request('/api/business', { method: 'PUT', body: payload }),

  /* categories */
  // `params` may carry { menu_id } to scope the list to one menu; no argument
  // returns every category of the business.
  listCategories: (params) => request(`/api/categories${qs(params)}`),
  createCategory: (payload) => request('/api/categories', { method: 'POST', body: payload }),
  updateCategory: (id, payload) =>
    request(`/api/categories/${id}`, { method: 'PUT', body: payload }),
  deleteCategory: (id) => request(`/api/categories/${id}`, { method: 'DELETE' }),
  reorderCategories: (ids) =>
    request('/api/categories/reorder', { method: 'PUT', body: { ids } }),

  /* products */
  listProducts: (params) => request(`/api/products${qs(params)}`),
  createProduct: (payload) => request('/api/products', { method: 'POST', body: payload }),
  updateProduct: (id, payload) => request(`/api/products/${id}`, { method: 'PUT', body: payload }),
  updateProductPrice: (id, price) =>
    request(`/api/products/${id}/price`, { method: 'PATCH', body: { price } }),
  deleteProduct: (id) => request(`/api/products/${id}`, { method: 'DELETE' }),
  reorderProducts: (categoryId, ids) =>
    request('/api/products/reorder', {
      method: 'PUT',
      body: { category_id: categoryId, ids },
    }),
  bulkPrice: (payload) => request('/api/products/bulk-price', { method: 'POST', body: payload }),

  /* image upload */
  upload: (file) => {
    const form = new FormData()
    form.append('file', file)
    return request('/api/uploads', { method: 'POST', body: form, isForm: true })
  },

  /* menus — the primary entity: a menu owns its own branding and settings */
  listMenus: () => request('/api/menus'),
  createMenu: (payload) => request('/api/menus', { method: 'POST', body: payload }),
  updateMenu: (id, payload) => request(`/api/menus/${id}`, { method: 'PUT', body: payload }),
  deleteMenu: (id) => request(`/api/menus/${id}`, { method: 'DELETE' }),

  /* customer menu payloads */
  // `params` may carry { menu }: the slug of the menu to preview, scoped to the
  // caller's own business. Without it the backend resolves the single active
  // menu, or answers 200 with menu_resolved: false when there is no such menu.
  previewMenu: (lang, params) =>
    request(`/api/preview/menu${qs({ lang, ...(params || {}) })}`),
  // Path form. The business slug identifies the tenant and the optional menu
  // slug picks one of its menus; menu slugs are unique only within a business,
  // so the business segment is never optional. With no menu slug the backend
  // resolves the only active menu or returns the directory payload.
  publicMenu: (businessSlug, menuSlug, lang) =>
    request(`/api/public/menu/${encodeURIComponent(businessSlug)}${menuSlug ? `/${encodeURIComponent(menuSlug)}` : ''}${qs({ lang })}`),
  // Host form: the subdomain identifies the tenant, ?menu= picks the menu.
  publicMenuByHost: (menuSlug, lang) =>
    request(`/api/public/menu${qs({ menu: menuSlug, lang })}`),
}

export default api
