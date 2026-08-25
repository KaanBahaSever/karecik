import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import api, { clearToken, getToken, setToken } from './api'

const AuthContext = createContext(null)

/**
 * Oturum durumunu (kullanıcı + işletme) uygulama genelinde paylaşır.
 * Token localStorage'da tutulur; sayfa yenilendiğinde /api/auth/me ile doğrulanır.
 */
export function AuthProvider({ children }) {
  const [user, setUser] = useState(null)
  const [business, setBusiness] = useState(null)
  const [loading, setLoading] = useState(Boolean(getToken()))

  useEffect(() => {
    let iptal = false

    async function oturumuYukle() {
      if (!getToken()) {
        setLoading(false)
        return
      }
      try {
        const data = await api.me()
        if (iptal) return
        setUser(data.user)
        setBusiness(data.business)
      } catch {
        clearToken()
        if (!iptal) {
          setUser(null)
          setBusiness(null)
        }
      } finally {
        if (!iptal) setLoading(false)
      }
    }

    oturumuYukle()
    return () => {
      iptal = true
    }
  }, [])

  const login = useCallback(async (email, password) => {
    const data = await api.login({ email, password })
    setToken(data.token)
    setUser(data.user)
    setBusiness(data.business)
    return data
  }, [])

  const register = useCallback(async (businessName, email, password) => {
    const data = await api.register({
      business_name: businessName,
      email,
      password,
    })
    setToken(data.token)
    setUser(data.user)
    setBusiness(data.business)
    return data
  }, [])

  const logout = useCallback(() => {
    clearToken()
    setUser(null)
    setBusiness(null)
  }, [])

  /** İşletme ayarlarını günceller ve context'i tazeler (canlı önizleme buna bakar). */
  const saveBusiness = useCallback(async (payload) => {
    const updated = await api.updateBusiness(payload)
    setBusiness(updated)
    return updated
  }, [])

  const refreshBusiness = useCallback(async () => {
    const updated = await api.getBusiness()
    setBusiness(updated)
    return updated
  }, [])

  const value = useMemo(
    () => ({
      user,
      business,
      loading,
      isAuthenticated: Boolean(user),
      login,
      register,
      logout,
      saveBusiness,
      refreshBusiness,
    }),
    [user, business, loading, login, register, logout, saveBusiness, refreshBusiness],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth yalnızca <AuthProvider> içinde kullanılabilir.')
  }
  return context
}
