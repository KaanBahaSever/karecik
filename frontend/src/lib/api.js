// Karecik API istemcisi.
//
// Geliştirmede istekler Vite proxy'si üzerinden Go backend'ine (8080) gider,
// bu yüzden varsayılan taban adres boştur (aynı origin). Ayrı bir sunucuya
// bağlanmak istersen frontend/.env içine VITE_API_URL=https://api.karecik.com yaz.

const API_BASE = import.meta.env.VITE_API_URL || ''
const TOKEN_KEY = 'karecik_token'

/* -------------------------------------------------------------- token */

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
    /* gizli sekmede localStorage kapalı olabilir */
  }
}

export function clearToken() {
  setToken(null)
}

/* --------------------------------------------------------------- hata */

export class ApiError extends Error {
  constructor(message, status, code) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

/* ------------------------------------------------------------ istekler */

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
    // Oturum düştüyse token'ı temizle; ProtectedRoute girişe yönlendirir.
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
  const s = search.toString()
  return s ? `?${s}` : ''
}

/* ---------------------------------------------------------------- API */

export const api = {
  /* sabit listeler: para birimleri, temalar, fontlar, alerjenler, diller */
  meta: () => request('/api/meta', { auth: false }),
  health: () => request('/api/health', { auth: false }),

  /* kimlik doğrulama */
  register: (payload) =>
    request('/api/auth/register', { method: 'POST', body: payload, auth: false }),
  login: (payload) => request('/api/auth/login', { method: 'POST', body: payload, auth: false }),
  me: () => request('/api/auth/me'),

  /* işletme */
  getBusiness: () => request('/api/business'),
  updateBusiness: (payload) => request('/api/business', { method: 'PUT', body: payload }),

  /* kategoriler */
  listCategories: () => request('/api/categories'),
  createCategory: (payload) => request('/api/categories', { method: 'POST', body: payload }),
  updateCategory: (id, payload) =>
    request(`/api/categories/${id}`, { method: 'PUT', body: payload }),
  deleteCategory: (id) => request(`/api/categories/${id}`, { method: 'DELETE' }),
  reorderCategories: (ids) =>
    request('/api/categories/reorder', { method: 'PUT', body: { ids } }),

  /* ürünler */
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

  /* görsel yükleme */
  upload: (file) => {
    const form = new FormData()
    form.append('file', file)
    return request('/api/uploads', { method: 'POST', body: form, isForm: true })
  },

  /* menüler */
  previewMenu: (lang) => request(`/api/preview/menu${qs({ lang })}`),
  publicMenu: (slug, lang) => request(`/api/public/menu/${slug}${qs({ lang })}`, { auth: false }),
  publicMenuByHost: (lang) => request(`/api/public/menu${qs({ lang })}`, { auth: false }),
}

export default api
