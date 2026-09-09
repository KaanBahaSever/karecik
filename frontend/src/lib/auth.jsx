import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import api, { onSessionExpired } from './api'
import { clearStoredSession, readStoredSession, writeStoredSession } from './session'

const AuthContext = createContext(null)

/**
 * Shares the signed-in account across the application.
 *
 * OPTIMISTIC, NOT VERIFIED — and the distinction is the whole point.
 *
 * This provider used to open every page load with GET /api/auth/me: the session
 * is an HttpOnly cookie the browser will not show us, so the only way to know
 * whether it was valid was to ask. Correct, and the wrong trade. It sits at the
 * root of the tree, so it fired on routes that have nothing to do with being
 * signed in — the landing page, and every customer menu behind a QR code.
 * Someone scanning a code in a cafe was waiting on an authentication round trip
 * before they could see a coffee, and the answer was always "no session".
 *
 * So the question is not asked any more. The account is read synchronously out
 * of localStorage and BELIEVED, and the belief is corrected the first time it
 * matters: a protected request comes back 401 SESSION_EXPIRED, api.js clears
 * the stored account and calls the handler registered below.
 *
 * That is not weaker than before. It was never this client that decided whether
 * a request was allowed — the server reads the cookie and answers. All that has
 * changed is WHEN the client finds out it was wrong: on the first request that
 * needed a session, instead of on a request made specially to ask.
 */
export function AuthProvider({ children }) {
  // Read once, synchronously, during the first render. There is no asynchronous
  // gap to cover, so there is no `loading` flag and no spinner on boot: the
  // panel either draws immediately or the login page does.
  const [session, setSession] = useState(readStoredSession)
  const navigate = useNavigate()

  const user = session?.user ?? null
  const business = session?.business ?? null

  const remember = useCallback((nextUser, nextBusiness) => {
    writeStoredSession(nextUser, nextBusiness)
    setSession(nextUser?.id ? { user: nextUser, business: nextBusiness ?? null } : null)
  }, [])

  const forget = useCallback(() => {
    clearStoredSession()
    setSession(null)
  }, [])

  /* ------------------------------------------------- the expired-session hook */
  /*
     Registered here rather than at module scope because the response needs
     `navigate`: a soft navigation keeps the single-page app intact, where
     window.location.assign would throw away a perfectly good page to move one
     route. api.js falls back to the hard version when nothing is registered.

     `state.expired` is what lets the login screen say WHY the visitor is
     suddenly looking at it. Without it the panel simply vanishes mid-click,
     which reads as a crash rather than as a session ending.
  */
  useEffect(() => {
    return onSessionExpired(() => {
      // api.js has already cleared storage; this drops the matching React state.
      setSession(null)
      navigate('/giris', { replace: true, state: { expired: true } })
    })
  }, [navigate])

  const login = useCallback(
    async (email, password) => {
      // The response carries no token: the server set the cookie on this very
      // response and the browser stored it before this line runs. What comes
      // back is the profile, which is what gets remembered.
      const data = await api.login({ email, password })
      remember(data.user, data.business)
      return data
    },
    [remember],
  )

  const register = useCallback(
    async (businessName, email, password) => {
      const data = await api.register({
        business_name: businessName,
        email,
        password,
      })
      remember(data.user, data.business)
      return data
    },
    [remember],
  )

  /* Logging out is a REQUEST, not a local erase. Only the server can delete the
     session from its memory and only the server can clear an HttpOnly cookie,
     so a purely client-side logout would leave a working credential behind.

     The local clear happens whatever the request does: if the network is down
     the user still expects the screen to log them out, and the stale session
     either expires on its own or is revoked from another device. */
  const logout = useCallback(async () => {
    try {
      await api.logout()
    } catch {
      /* already signed out, or offline — the local clear below still applies */
    }
    forget()
  }, [forget])

  /* Changes the password and reports how many OTHER sessions were signed out,
     so the form can tell the user their other devices are now logged out. */
  const changePassword = useCallback(
    (currentPassword, newPassword) => api.changePassword(currentPassword, newPassword),
    [],
  )

  /* Persists the account record — name and slug, the tenant subdomain. Every
     other setting belongs to a menu and is saved through the menu API.

     The stored copy is updated too, or the sidebar would show the old business
     name until the next login. */
  const saveBusiness = useCallback(
    async (payload) => {
      const updated = await api.updateBusiness(payload)
      remember(user, updated)
      return updated
    },
    [remember, user],
  )

  const refreshBusiness = useCallback(
    async () => {
      const updated = await api.getBusiness()
      remember(user, updated)
      return updated
    },
    [remember, user],
  )

  const value = useMemo(
    () => ({
      user,
      business,
      isAuthenticated: Boolean(user),
      login,
      register,
      logout,
      changePassword,
      saveBusiness,
      refreshBusiness,
    }),
    [user, business, login, register, logout, changePassword, saveBusiness, refreshBusiness],
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
