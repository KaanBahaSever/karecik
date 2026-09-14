import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'

import api from './api'
import { useAuth } from './auth'

/**
 * Shares the menu being edited across the dashboard: the active-menu bar, the
 * menu editor, the settings page and the QR hub all read the same list and the
 * same selection from here.
 *
 * A menu owns every setting now — branding, splash, contact, pricing — so the
 * selection made here decides what the whole dashboard edits.
 *
 * There is no default menu, and a business may own none at all: `menus === []`
 * with `activeMenu === null` is a legitimate, permanent state that every
 * dashboard surface renders as an empty state.
 *
 * It is mounted inside <ProtectedRoute> only — the customer menu and the
 * landing page have no session, and fetching there would only produce 401s.
 */

const MENU_KEY = 'karecik_active_menu'

const MenuContext = createContext(null)

/* --------------------------------------------------------- localStorage */

// Private windows throw on access, not only on write — every call is guarded.

function readStored(key) {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function writeStored(key, value) {
  try {
    if (value) localStorage.setItem(key, value)
    else localStorage.removeItem(key)
  } catch {
    /* localStorage may be unavailable in private windows */
  }
}

/* --------------------------------------------------------------- helpers */

const byId = (list, id) => (id ? list.find((item) => item.id === id) || null : null)

/**
 * The menu shown first when nothing is remembered: the lowest position, ties
 * resolved by the order the server sent. There is no default menu to prefer.
 */
function firstByPosition(menus) {
  return menus.reduce(
    (first, menu) => (!first || (menu.position ?? 0) < (first.position ?? 0) ? menu : first),
    null,
  )
}

/** The remembered menu when it still exists, otherwise the first one, otherwise null. */
function pickMenu(menus, preferredId) {
  return byId(menus, preferredId) || firstByPosition(menus)
}

/* -------------------------------------------------------------- provider */

export function MenuProvider({ children }) {
  const { business } = useAuth()
  const businessID = business?.id || null

  const [menus, setMenus] = useState([])
  const [loading, setLoading] = useState(Boolean(businessID))
  const [error, setError] = useState('')
  const [menuID, setMenuID] = useState(() => readStored(MENU_KEY))

  // The selection is mirrored into a ref so that load() can read the current id
  // without being rebuilt on every selection change.
  const menuIDRef = useRef(menuID)

  // Guards against a slow first fetch overwriting a newer reload().
  const requestRef = useRef(0)

  /*
    ORDERING. Three things put menu data into the list: load() (the whole list),
    refreshMenu() (one menu, after a price edit) and saveActiveMenu() (one menu,
    from the settings page). Each is numbered from sequenceRef — a load and a
    refresh when they START, because their answer shows the server as of that
    moment; a save when it LANDS, because its response is the saved truth.

    - A one-menu answer is applied only when it is newer than the last one
      applied to that menu. Two refreshes landing out of order, or a refresh that
      was still on the wire when a save landed, cannot put older data back; and
      when the newer of two refreshes fails, the older one's answer still counts.
    - A load keeps any one-menu answer newer than itself instead of its own copy
      of that menu, so a slow reload cannot revert a price date or a save.
    - A refresh is dropped once a load that started after it has been APPLIED —
      that list is newer. It is not dropped merely because such a load started:
      the load may still fail, and the refresh's answer would be lost with it.
    - Nothing that started before a logout lands after it.
  */
  const sequenceRef = useRef(0)
  const latestMenuRef = useRef(new Map()) // menu id -> { number, menu }, newest one-menu answer applied
  const appliedLoadRef = useRef(0) // number of the newest load whose list was applied
  const sessionRef = useRef(0) // bumped on logout

  /** Stores one selection in state, in the ref and in localStorage. */
  const applySelection = useCallback((menu) => {
    const nextID = menu?.id || null
    menuIDRef.current = nextID
    setMenuID(nextID)
    writeStored(MENU_KEY, nextID)
  }, [])

  /** Refetches the menu list, keeping the current selection while it is still valid. */
  const load = useCallback(async () => {
    if (!businessID) return

    const request = requestRef.current + 1
    requestRef.current = request
    const number = sequenceRef.current
    setLoading(true)

    try {
      const list = await api.listMenus()
      if (request !== requestRef.current) return

      appliedLoadRef.current = Math.max(appliedLoadRef.current, number)
      const nextMenus = (Array.isArray(list) ? list : []).map((menu) => {
        const single = latestMenuRef.current.get(menu.id)
        return single && single.number > number
          ? { ...menu, ...single.menu, category_count: menu.category_count }
          : menu
      })
      setMenus(nextMenus)
      setError('')
      applySelection(pickMenu(nextMenus, menuIDRef.current))
    } catch (err) {
      if (request !== requestRef.current) return
      setError(err.message || 'Menüler yüklenemedi.')
    } finally {
      if (request === requestRef.current) setLoading(false)
    }
  }, [businessID, applySelection])

  useEffect(() => {
    if (!businessID) {
      // Logged out: drop the list, but keep the remembered id for the next session.
      requestRef.current += 1
      sessionRef.current += 1
      latestMenuRef.current.clear()
      appliedLoadRef.current = 0
      setMenus([])
      setError('')
      setLoading(false)
      return
    }
    load()
  }, [businessID, load])

  // Derived from the list, so a stored id pointing at a deleted menu silently
  // falls back to the first remaining menu. With no menus at all it stays null
  // — the dashboard renders its empty states instead.
  const activeMenu = useMemo(() => pickMenu(menus, menuID), [menus, menuID])
  const activeMenuId = activeMenu?.id || null
  const hasMenus = menus.length > 0

  const setActiveMenuId = useCallback(
    (id) => {
      applySelection(pickMenu(menus, id))
    },
    [menus, applySelection],
  )

  /**
   * Creates a menu, refreshes the list and selects the new menu. The server
   * resolves slug collisions inside the business, so the created record — with
   * the address it actually got — is returned to the caller.
   */
  const createMenu = useCallback(
    async (payload) => {
      const created = await api.createMenu(payload)

      // Selecting before the refresh means load() finds the id already parked
      // in the ref and keeps it, instead of falling back to the first menu.
      if (created?.id) applySelection(created)
      await load()
      return created
    },
    [applySelection, load],
  )

  /**
   * Deletes a menu and re-selects the first remaining one, or nothing at all.
   * Deleting the last menu is allowed: a business may own zero menus.
   */
  const deleteMenu = useCallback(
    async (id) => {
      await api.deleteMenu(id)
      if (menuIDRef.current === id) applySelection(null)
      await load()
    },
    [applySelection, load],
  )

  /**
   * Persists the active menu and swaps the server's canonical response into the
   * local list, so the editor, the live preview and the QR hub all see it at
   * once. The updated menu is returned — the settings page saves through this.
   */
  const saveActiveMenu = useCallback(
    async (payload) => {
      if (!activeMenuId) throw new Error('Düzenlenecek menü bulunamadı.')

      const updated = await api.updateMenu(activeMenuId, payload)
      if (updated?.id) {
        // Numbered when it lands; see ORDERING above.
        const number = sequenceRef.current + 1
        sequenceRef.current = number
        latestMenuRef.current.set(updated.id, { number, menu: updated })

        // `category_count` is computed by the list endpoint only, so the update
        // response always carries 0. Saving a setting never adds or removes a
        // category, which makes the count already in state the correct one —
        // without this the menu switcher reports "0 kategori" after every save.
        setMenus((current) =>
          current.map((menu) =>
            menu.id === updated.id
              ? { ...menu, ...updated, category_count: menu.category_count }
              : menu,
          ),
        )
      }
      return updated
    },
    [activeMenuId],
  )

  /**
   * Refetches ONE menu and merges it into the list.
   *
   * Price edits in the menu editor can move `price_updated_at` on the server,
   * and the settings page prints that date from this list — without a refetch
   * it would stay stale until a full reload. `category_count` is kept from
   * state for the same reason saveActiveMenu keeps it.
   *
   * `loading` is deliberately left alone. MenuEditor waits for that flag before
   * it loads, so toggling it would reload the whole editor behind a spinner
   * after every price edit.
   *
   * Nothing is thrown. The edit that asked for the refresh has already
   * succeeded, so a failure here is worth a console warning, not a toast that
   * would read as if the edit itself had failed.
   */
  const refreshMenu = useCallback(async (id) => {
    if (!id) return

    const number = sequenceRef.current + 1
    sequenceRef.current = number
    const session = sessionRef.current

    try {
      const fresh = await api.getMenu(id)
      if (fresh?.id !== id || sessionRef.current !== session) return

      // Out of date: a list requested after this refresh started has already
      // been applied, or a newer answer for this menu (a later refresh, or a
      // save) is already in place. See ORDERING above.
      if (appliedLoadRef.current >= number) return
      const applied = latestMenuRef.current.get(id)
      if (applied && applied.number > number) return

      latestMenuRef.current.set(id, { number, menu: fresh })
      setMenus((current) =>
        current.map((menu) =>
          menu.id === id ? { ...menu, ...fresh, category_count: menu.category_count } : menu,
        ),
      )
    } catch (err) {
      console.warn('[karecik] Could not refresh the menu after a price change:', err)
    }
  }, [])

  const value = useMemo(
    () => ({
      menus,
      loading,
      error,
      activeMenu,
      activeMenuId,
      hasMenus,
      setActiveMenuId,
      reload: load,
      createMenu,
      deleteMenu,
      saveActiveMenu,
      refreshMenu,
    }),
    [
      menus,
      loading,
      error,
      activeMenu,
      activeMenuId,
      hasMenus,
      setActiveMenuId,
      load,
      createMenu,
      deleteMenu,
      saveActiveMenu,
      refreshMenu,
    ],
  )

  return <MenuContext.Provider value={value}>{children}</MenuContext.Provider>
}

export function useActiveMenu() {
  const context = useContext(MenuContext)
  if (!context) {
    throw new Error('useActiveMenu can only be used inside <MenuProvider>.')
  }
  return context
}
