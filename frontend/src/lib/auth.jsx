import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import api from './api'

const AuthContext = createContext(null)

/**
 * Shares the session state (user + business) across the whole application.
 *
 * The session is an HttpOnly cookie, so this file cannot see it — and that is
 * the point. It means the client can no longer tell whether it is signed in by
 * looking at storage; the only way to know is to ASK, which is what the
 * /api/auth/me call below does on every page load. A 401 is the answer "no".
 */
export function AuthProvider({ children }) {
  const [user, setUser] = useState(null)
  const [business, setBusiness] = useState(null)
  // Starts true unconditionally: with the cookie invisible there is nothing to
  // check synchronously, so every load begins by asking the server. Starting
  // false would flash the login screen at someone who is already signed in.
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false

    async function loadSession() {
      try {
        const data = await api.me()
        if (cancelled) return
        setUser(data.user)
        setBusiness(data.business)
      } catch {
        // 401 for "no session", or the network is down. Either way there is
        // nobody signed in and nothing local to clean up.
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
    // The response carries no token: the server set the cookie on this very
    // response and the browser stored it before this line runs.
    const data = await api.login({ email, password })
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
    setUser(data.user)
    setBusiness(data.business)
    return data
  }, [])

  /* Logging out is a REQUEST now, not a local erase. Only the server can delete
     the session row, and only the server can clear an HttpOnly cookie — so a
     purely client-side logout would leave a working credential behind.

     The local state is cleared whatever the request does: if the network is
     down, the user still expects the screen to log them out, and the stale
     session either expires or is revoked from another device. */
  const logout = useCallback(async () => {
    try {
      await api.logout()
    } catch {
      /* already signed out, or offline — the local clear below still applies */
    }
    setUser(null)
    setBusiness(null)
  }, [])

  /* Changes the password and reports how many OTHER sessions were signed out,
     so the form can tell the user their other devices are now logged out. */
  const changePassword = useCallback(
    (currentPassword, newPassword) => api.changePassword(currentPassword, newPassword),
    [],
  )

  /* Persists the account record — name and slug, the tenant subdomain. Every
     other setting belongs to a menu and is saved through the menu API. */
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
      changePassword,
      saveBusiness,
      refreshBusiness,
    }),
    [user, business, loading, login, register, logout, changePassword, saveBusiness, refreshBusiness],
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
