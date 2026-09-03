// Karecik API client.
//
// In development requests go through the Vite proxy to the Go backend (:8080),
// so the default base URL is empty (same origin). To target a separate server,
// put VITE_API_URL=https://api.karecik.com in frontend/.env.
//
// NOTE: error messages are Turkish on purpose — they are shown to the user.

const API_BASE = import.meta.env.VITE_API_URL || ''
const TOKEN_KEY = 'karecik_token'

/* --------------------------------------------------------------- token */

export function getToken() {
  try {
    return localStorage.getItem(TOKEN_KEY)
  } catch {
    return null
  }
}

export function setToken(token) {
  try {
    if (token) localStorage.setItem(TOKEN_KEY, token)
    else localStorage.removeItem(TOKEN_KEY)
  } catch {
    /* localStorage may be unavailable in private windows */
  }
}

export function clearToken() {
  setToken(null)
}

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

async function request(path, { method = 'GET', body, auth = true, isForm = false } = {}) {
  const headers = {}

  if (auth) {
    const token = getToken()
    if (token) headers.Authorization = `Bearer ${token}`
  }
  if (!isForm && body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }

  let response
  try {
    response = await fetch(`${API_BASE}${path}`, {
      method,
      headers,
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
    // Session expired: drop the token so ProtectedRoute redirects to login.
    if (response.status === 401) clearToken()
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
  meta: () => request('/api/meta', { auth: false }),
  health: () => request('/api/health', { auth: false }),

  /* authentication */
  register: (payload) =>
    request('/api/auth/register', { method: 'POST', body: payload, auth: false }),
  login: (payload) => request('/api/auth/login', { method: 'POST', body: payload, auth: false }),
  me: () => request('/api/auth/me'),

  /* business */
  getBusiness: () => request('/api/business'),
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

  /* menus (a business may publish several menus) */
  listMenus: () => request('/api/menus'),
  createMenu: (payload) => request('/api/menus', { method: 'POST', body: payload }),
  updateMenu: (id, payload) => request(`/api/menus/${id}`, { method: 'PUT', body: payload }),
  deleteMenu: (id) => request(`/api/menus/${id}`, { method: 'DELETE' }),

  /* branches */
  listBranches: () => request('/api/branches'),
  createBranch: (payload) => request('/api/branches', { method: 'POST', body: payload }),
  updateBranch: (id, payload) => request(`/api/branches/${id}`, { method: 'PUT', body: payload }),
  deleteBranch: (id) => request(`/api/branches/${id}`, { method: 'DELETE' }),
  setBranchMenus: (id, menuIds, defaultMenuId) =>
    request(`/api/branches/${id}/menus`, {
      method: 'PUT',
      body: { menu_ids: menuIds, default_menu_id: defaultMenuId },
    }),

  /* branch-specific prices and availability */
  listBranchPrices: (id) => request(`/api/branches/${id}/prices`),
  setBranchPrice: (id, productId, payload) =>
    request(`/api/branches/${id}/prices/${productId}`, { method: 'PUT', body: payload }),
  clearBranchPrice: (id, productId) =>
    request(`/api/branches/${id}/prices/${productId}`, { method: 'DELETE' }),

  /* customer menu payloads */
  // `params` may carry { branch, menu } slugs; both are optional.
  previewMenu: (lang, params) =>
    request(`/api/preview/menu${qs({ lang, ...(params || {}) })}`),
  publicMenu: (slug, lang, menuSlug) =>
    request(`/api/public/menu/${slug}${qs({ lang, menu: menuSlug })}`, { auth: false }),
  publicMenuByHost: (lang, menuSlug) =>
    request(`/api/public/menu${qs({ lang, menu: menuSlug })}`, { auth: false }),
  publicMenuByBranch: (branchSlug, menuSlug, lang) =>
    request(
      `/api/public/b/${branchSlug}${menuSlug ? `/${menuSlug}` : ''}${qs({ lang })}`,
      { auth: false },
    ),
}

export default api
