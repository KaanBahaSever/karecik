import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import api, { clearToken, getToken, setToken } from './api'

const AuthContext = createContext(null)

/**
 * Shares the session state (user + business) across the whole application.
 * The token lives in localStorage and is re-validated through /api/auth/me
 * whenever the page reloads.
 */
export function AuthProvider({ children }) {
  const [user, setUser] = useState(null)
  const [business, setBusiness] = useState(null)
  const [loading, setLoading] = useState(Boolean(getToken()))

  useEffect(() => {
    let cancelled = false

    async function loadSession() {
      if (!getToken()) {
        setLoading(false)
        return
      }
      try {
        const data = await api.me()
        if (cancelled) return
        setUser(data.user)
        setBusiness(data.business)
      } catch {
        clearToken()
        if (!cancelled) {
          setUser(null)
          setBusiness(null)
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    loadSession()
    return () => {
      cancelled = true
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

  /** Persists business settings and refreshes the context (live preview reads this). */
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
    throw new Error('useAuth can only be used inside <AuthProvider>.')
  }
  return context
}
