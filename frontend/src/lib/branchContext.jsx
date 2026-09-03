import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'

import api from './api'
import { useAuth } from './auth'

/**
 * Shares the active branch / menu selection across the dashboard: the topbar
 * switcher, the menu editor and the branches page all read the same lists and
 * the same selection from here.
 *
 * It is mounted inside <ProtectedRoute> only — the customer menu and the
 * landing page have no session, and fetching there would only produce 401s.
 */

const BRANCH_KEY = 'karecik_active_branch'
const MENU_KEY = 'karecik_active_menu'

const BranchMenuContext = createContext(null)

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

/** The remembered branch when it still exists, otherwise the default, otherwise the first. */
function pickBranch(branches, preferredId) {
  return (
    byId(branches, preferredId) ||
    branches.find((branch) => branch.is_default) ||
    branches[0] ||
    null
  )
}

/**
 * The menus a branch actually serves, in the branch's own order. A branch with
 * no assignment yet falls back to every menu of the business, so the editor is
 * never left with nothing to edit.
 */
function servedMenus(menus, branch) {
  const ids = branch?.menu_ids
  if (!Array.isArray(ids) || ids.length === 0) return menus
  const served = ids.map((id) => byId(menus, id)).filter(Boolean)
  return served.length ? served : menus
}

/**
 * The menu to select for a branch: the remembered one when that branch serves
 * it, otherwise the branch's default menu, otherwise its first one.
 *
 * A branch with its own assignment lists its default menu FIRST - that is the
 * order BranchModal sends and SetBranchMenus stores - and that menu is the one
 * the branch address opens. It therefore wins over the business-wide default,
 * which this branch may not even serve.
 */
function pickMenu(menus, branch, preferredId) {
  const pool = servedMenus(menus, branch)
  const remembered = byId(pool, preferredId)
  if (remembered) return remembered

  const assigned = Array.isArray(branch?.menu_ids) && branch.menu_ids.length > 0
  if (assigned) return pool[0] || null

  return pool.find((menu) => menu.is_default) || pool[0] || null
}

/* -------------------------------------------------------------- provider */

export function BranchMenuProvider({ children }) {
  const { business } = useAuth()
  const businessID = business?.id || null

  const [branches, setBranches] = useState([])
  const [menus, setMenus] = useState([])
  const [loading, setLoading] = useState(Boolean(businessID))
  const [error, setError] = useState('')

  const [branchID, setBranchID] = useState(() => readStored(BRANCH_KEY))
  const [menuID, setMenuID] = useState(() => readStored(MENU_KEY))

  // The selection is mirrored into refs so that load() and the setters can read
  // the current ids without being rebuilt on every selection change.
  const branchIDRef = useRef(branchID)
  const menuIDRef = useRef(menuID)

  // Guards against a slow first fetch overwriting a newer reload().
  const requestRef = useRef(0)

  /** Stores one (branch, menu) pair in state, in the refs and in localStorage. */
  const applySelection = useCallback((branch, menu) => {
    const nextBranchID = branch?.id || null
    const nextMenuID = menu?.id || null

    branchIDRef.current = nextBranchID
    menuIDRef.current = nextMenuID
    setBranchID(nextBranchID)
    setMenuID(nextMenuID)
    writeStored(BRANCH_KEY, nextBranchID)
    writeStored(MENU_KEY, nextMenuID)
  }, [])

  /** Refetches both lists, keeping the current selection while it is still valid. */
  const load = useCallback(async () => {
    if (!businessID) return

    const request = requestRef.current + 1
    requestRef.current = request
    setLoading(true)

    try {
      const [branchList, menuList] = await Promise.all([api.listBranches(), api.listMenus()])
      if (request !== requestRef.current) return

      const nextBranches = Array.isArray(branchList) ? branchList : []
      const nextMenus = Array.isArray(menuList) ? menuList : []
      setBranches(nextBranches)
      setMenus(nextMenus)
      setError('')

      const branch = pickBranch(nextBranches, branchIDRef.current)
      applySelection(branch, pickMenu(nextMenus, branch, menuIDRef.current))
    } catch (err) {
      if (request !== requestRef.current) return
      setError(err.message || 'Şubeler ve menüler yüklenemedi.')
    } finally {
      if (request === requestRef.current) setLoading(false)
    }
  }, [businessID, applySelection])

  useEffect(() => {
    if (!businessID) {
      // Logged out: drop the lists, but keep the remembered ids for the next session.
      requestRef.current += 1
      setBranches([])
      setMenus([])
      setError('')
      setLoading(false)
      return
    }
    load()
  }, [businessID, load])

  const activeBranch = useMemo(() => pickBranch(branches, branchID), [branches, branchID])
  const activeMenu = useMemo(
    () => pickMenu(menus, activeBranch, menuID),
    [menus, activeBranch, menuID],
  )

  /** Switching branch re-points the menu: a branch only serves its own menus. */
  const setActiveBranchId = useCallback(
    (id) => {
      const branch = pickBranch(branches, id)
      applySelection(branch, pickMenu(menus, branch, menuIDRef.current))
    },
    [branches, menus, applySelection],
  )

  const setActiveMenuId = useCallback(
    (id) => {
      const branch = pickBranch(branches, branchIDRef.current)
      applySelection(branch, pickMenu(menus, branch, id))
    },
    [branches, menus, applySelection],
  )

  const value = useMemo(
    () => ({
      branches,
      menus,
      loading,
      error,
      activeBranch,
      activeMenu,
      setActiveBranchId,
      setActiveMenuId,
      reload: load,
    }),
    [
      branches,
      menus,
      loading,
      error,
      activeBranch,
      activeMenu,
      setActiveBranchId,
      setActiveMenuId,
      load,
    ],
  )

  return <BranchMenuContext.Provider value={value}>{children}</BranchMenuContext.Provider>
}

export function useBranchMenu() {
  const context = useContext(BranchMenuContext)
  if (!context) {
    throw new Error('useBranchMenu can only be used inside <BranchMenuProvider>.')
  }
  return context
}
