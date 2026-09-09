// The locally remembered account, so the panel can draw itself on load without
// asking the server who is signed in.
//
// WHAT THIS IS NOT
//
// It is not a credential, and nothing here decides whether a request succeeds.
// The session is an HttpOnly cookie this file cannot read or write; the server
// is the only thing that knows whether it is valid. What is stored here is the
// PROFILE — a name, an e-mail, a business slug — so that the first paint after
// a refresh shows the right sidebar instead of a spinner.
//
// So yes: anyone can open devtools and write whatever they like into this key,
// and the panel shell will draw for them. That buys them an empty frame in
// their own browser. Every request it makes comes back 401, the interceptor in
// api.js clears this key, and they land on the login page. No data crosses.
// Treating this as a display cache rather than as proof is the whole design.
//
// WHY IT EXISTS AT ALL
//
// The alternative — asking /api/auth/me on every page load — put an
// authenticated request on the critical path of pages that have nothing to do
// with authentication. A QR menu is opened by a stranger on a phone, and it was
// making an auth call before it could show a coffee.

export const STORED_SESSION_KEY = 'krc_user'

/**
 * The remembered account, or null.
 *
 * Anything unexpected in storage is treated as "nobody" rather than trusted:
 * the value is hand-editable, may be left over from an older shape of the app,
 * and reading it must never be able to throw during a synchronous state
 * initialiser — that would take the whole page down instead of showing a login
 * screen.
 */
export function readStoredSession() {
  try {
    const raw = localStorage.getItem(STORED_SESSION_KEY)
    if (!raw) return null

    const parsed = JSON.parse(raw)
    // A user with no id is not a user. This also rejects `null`, arrays and
    // primitives, all of which JSON.parse will happily return.
    if (!parsed || typeof parsed !== 'object' || !parsed.user?.id) return null

    return { user: parsed.user, business: parsed.business ?? null }
  } catch {
    // Private windows can refuse storage outright, and a half-written value
    // throws on parse. Either way: nobody is signed in.
    return null
  }
}

/** Remembers the account. A missing user clears the key instead. */
export function writeStoredSession(user, business) {
  try {
    if (!user?.id) {
      localStorage.removeItem(STORED_SESSION_KEY)
      return
    }
    localStorage.setItem(STORED_SESSION_KEY, JSON.stringify({ user, business: business ?? null }))
  } catch {
    // Storage full or refused. The app still works for this tab — only the
    // convenience of surviving a refresh is lost.
  }
}

/** Forgets the account. Called on logout and on any expired-session 401. */
export function clearStoredSession() {
  try {
    localStorage.removeItem(STORED_SESSION_KEY)
  } catch {
    /* nothing to do; the key is unreachable either way */
  }
}
