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
    setLoading(true)

    try {
      const list = await api.listMenus()
      if (request !== requestRef.current) return

      const nextMenus = Array.isArray(list) ? list : []
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
