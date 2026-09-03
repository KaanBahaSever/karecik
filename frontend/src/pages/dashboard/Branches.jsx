import { useMemo, useState } from 'react'
import {
  AlertCircle,
  LayoutList,
  ListChecks,
  MapPin,
  Pencil,
  Phone,
  Plus,
  Store,
  Trash2,
  Wifi,
} from 'lucide-react'

import api from '../../lib/api'
import { useBranchMenu } from '../../lib/branchContext.jsx'

import { useToast } from '../../components/ui/Toast.jsx'
import ConfirmModal from '../../components/ui/ConfirmModal.jsx'
import EmptyState from '../../components/ui/EmptyState.jsx'
import Loading from '../../components/ui/Loading.jsx'

import BranchLinkCard from '../../components/dashboard/BranchLinkCard.jsx'
import BranchModal from '../../components/dashboard/BranchModal.jsx'
import MenuModal from '../../components/dashboard/MenuModal.jsx'

/* The suffix shown next to every public address (mirrors Settings.jsx) */
const BRANCH_SUFFIX = '.karecik.com'

/** Card section heading (mirrors pages/dashboard/Settings.jsx). */
function SectionHeading({ icon: Icon, title, description, action }) {
  return (
    <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <Icon className="h-4 w-4 text-brand-600" aria-hidden="true" />
          <h2 className="text-base font-semibold text-gray-900">{title}</h2>
        </div>
        {description ? <p className="help-text">{description}</p> : null}
      </div>
      {action ? <div className="shrink-0">{action}</div> : null}
    </div>
  )
}

/**
 * The menus a branch serves, in the order the API returned them.
 *
 * The first id is the branch' own default menu: BranchModal always sends the
 * default first and the backend stores that order, so it can be read back the
 * same way. A branch with no assignment at all serves nothing — that is shown
 * as such rather than guessed at.
 */
function servedMenus(branch, menuByID) {
  const ids = Array.isArray(branch?.menu_ids) ? branch.menu_ids : []
  return ids.map((id) => menuByID[id]).filter(Boolean)
}

/** The overrides a branch sets, as icon + text pairs. */
function branchOverrides(branch) {
  return [
    branch.phone ? { key: 'phone', Icon: Phone, text: branch.phone } : null,
    branch.address ? { key: 'address', Icon: MapPin, text: branch.address } : null,
    branch.wifi_ssid ? { key: 'wifi', Icon: Wifi, text: branch.wifi_ssid } : null,
  ].filter(Boolean)
}

/* -------------------------------------------------------------------- page */

export default function Branches() {
  const { branches, menus, loading, error, reload } = useBranchMenu()
  const toast = useToast()

  const [menuModalOpen, setMenuModalOpen] = useState(false)
  const [editingMenu, setEditingMenu] = useState(null)

  const [branchModalOpen, setBranchModalOpen] = useState(false)
  const [editingBranch, setEditingBranch] = useState(null)
  const [focusMenus, setFocusMenus] = useState(false)

  const [menuToDelete, setMenuToDelete] = useState(null)
  const [branchToDelete, setBranchToDelete] = useState(null)
  const [deleting, setDeleting] = useState(false)

  const menuByID = useMemo(() => {
    const map = {}
    menus.forEach((menu) => {
      map[menu.id] = menu
    })
    return map
  }, [menus])

  /* ---------------------------------------------------------------- menus */

  function openCreateMenu() {
    setEditingMenu(null)
    setMenuModalOpen(true)
  }

  function openEditMenu(menu) {
    setEditingMenu(menu)
    setMenuModalOpen(true)
  }

  async function confirmDeleteMenu() {
    if (!menuToDelete || deleting) return

    setDeleting(true)
    try {
      await api.deleteMenu(menuToDelete.id)
      toast.success('Menü silindi.')
      await reload()
    } catch (err) {
      // The API refuses the last menu of a business; its own Turkish message is
      // more precise than anything invented here.
      toast.error(err.message)
    } finally {
      setDeleting(false)
      setMenuToDelete(null)
    }
  }

  /* ------------------------------------------------------------- branches */

  function openCreateBranch() {
    setEditingBranch(null)
    setFocusMenus(false)
    setBranchModalOpen(true)
  }

  function openEditBranch(branch, jumpToMenus = false) {
    setEditingBranch(branch)
    setFocusMenus(jumpToMenus)
    setBranchModalOpen(true)
  }

  async function confirmDeleteBranch() {
    if (!branchToDelete || deleting) return

    setDeleting(true)
    try {
      await api.deleteBranch(branchToDelete.id)
      toast.success('Şube silindi.')
      await reload()
    } catch (err) {
      toast.error(err.message)
    } finally {
      setDeleting(false)
      setBranchToDelete(null)
    }
  }

  /* --------------------------------------------------------------- render */

  if (loading && menus.length === 0 && branches.length === 0) {
    return <Loading text="Şubeler ve menüler yükleniyor..." />
  }

  return (
    <div className="mx-auto w-full max-w-4xl pb-10">
      <div className="mb-6">
        <h1 className="text-xl font-semibold text-gray-900">Şubeler ve Menüler</h1>
        <p className="mt-1 text-sm text-gray-500">
          Şubelerinizi ve her şubede yayınlanan menüleri yönetin.
        </p>
      </div>

      {error ? (
        <div className="mb-6 flex items-start gap-3 rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-800">
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          <p className="leading-relaxed">{error}</p>
        </div>
      ) : null}

      <div className="space-y-6">
        {/* --------------------------------------------------- A. menus */}
        <section className="card p-5">
          <SectionHeading
            icon={LayoutList}
            title="Menüler"
            description="Her menünün kendi adresi ve kendi kategorileri vardır."
            action={
              <button type="button" className="btn-primary" onClick={openCreateMenu}>
                <Plus className="h-4 w-4" aria-hidden="true" />
                Menü Ekle
              </button>
            }
          />

          {menus.length === 0 ? (
            <EmptyState
              icon={LayoutList}
              title="Henüz menü yok"
              description="İlk menünüzü ekleyin; şubeleriniz bu menüleri yayınlar."
              action={
                <button type="button" className="btn-primary" onClick={openCreateMenu}>
                  <Plus className="h-4 w-4" aria-hidden="true" />
                  Menü Ekle
                </button>
              }
            />
          ) : (
            <div className="space-y-2">
              {menus.map((menu) => (
                <div
                  key={menu.id}
                  className="flex items-center gap-2 rounded-lg border border-gray-200 px-3 py-2.5"
                >
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="truncate text-sm font-semibold text-gray-900">
                        {menu.name}
                      </span>
                      {menu.is_default ? (
                        <span className="badge bg-brand-50 text-brand-700">Varsayılan</span>
                      ) : null}
                      {menu.is_active === false ? (
                        <span className="badge bg-gray-100 text-gray-600">Pasif</span>
                      ) : null}
                    </div>
                    <p className="mt-0.5 truncate text-xs text-gray-500">
                      <span className="font-mono">/{menu.slug}</span>
                      <span aria-hidden="true"> · </span>
                      {menu.category_count ?? 0} kategori
                    </p>
                  </div>

                  <div className="flex shrink-0 items-center">
                    <button
                      type="button"
                      onClick={() => openEditMenu(menu)}
                      className="rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-700"
                      aria-label={`${menu.name} menüsünü düzenle`}
                      title="Düzenle"
                    >
                      <Pencil className="h-4 w-4" aria-hidden="true" />
                    </button>

                    <button
                      type="button"
                      onClick={() => setMenuToDelete(menu)}
                      className="rounded-md p-1.5 text-gray-400 hover:bg-red-50 hover:text-red-600"
                      aria-label={`${menu.name} menüsünü sil`}
                      title="Sil"
                    >
                      <Trash2 className="h-4 w-4" aria-hidden="true" />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>

        {/* ------------------------------------------------ B. branches */}
        <section className="card p-5">
          <SectionHeading
            icon={Store}
            title="Şubeler"
            description="Her şubenin kendi adresi, kendi bilgileri ve kendi menüleri olur."
            action={
              <button type="button" className="btn-primary" onClick={openCreateBranch}>
                <Plus className="h-4 w-4" aria-hidden="true" />
                Şube Ekle
              </button>
            }
          />

          {branches.length === 0 ? (
            <EmptyState
              icon={Store}
              title="Henüz şube yok"
              description="Birden fazla noktanız varsa her biri için ayrı bir şube adresi oluşturun."
              action={
                <button type="button" className="btn-primary" onClick={openCreateBranch}>
                  <Plus className="h-4 w-4" aria-hidden="true" />
                  Şube Ekle
                </button>
              }
            />
          ) : (
            <div className="space-y-4">
              {branches.map((branch) => {
                const served = servedMenus(branch, menuByID)
                const overrides = branchOverrides(branch)

                return (
                  <div key={branch.id} className="rounded-xl border border-gray-200 p-4">
                    {/* ------------------------------------------- header */}
                    <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="truncate text-sm font-semibold text-gray-900">
                            {branch.name}
                          </span>
                          {branch.is_default ? (
                            <span className="badge bg-brand-50 text-brand-700">Varsayılan</span>
                          ) : null}
                          {branch.is_active === false ? (
                            <span className="badge bg-gray-100 text-gray-600">Pasif</span>
                          ) : null}
                        </div>
                        <p className="mt-0.5 truncate font-mono text-xs text-gray-500">
                          {branch.slug}
                          {BRANCH_SUFFIX}
                        </p>
                      </div>

                      <div className="flex shrink-0 flex-wrap items-center gap-2">
                        <button
                          type="button"
                          className="btn-secondary btn-sm"
                          onClick={() => openEditBranch(branch, true)}
                        >
                          <ListChecks className="h-3.5 w-3.5" aria-hidden="true" />
                          Menüleri seç
                        </button>

                        <button
                          type="button"
                          onClick={() => openEditBranch(branch)}
                          className="rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-700"
                          aria-label={`${branch.name} şubesini düzenle`}
                          title="Düzenle"
                        >
                          <Pencil className="h-4 w-4" aria-hidden="true" />
                        </button>

                        <button
                          type="button"
                          onClick={() => setBranchToDelete(branch)}
                          className="rounded-md p-1.5 text-gray-400 hover:bg-red-50 hover:text-red-600"
                          aria-label={`${branch.name} şubesini sil`}
                          title="Sil"
                        >
                          <Trash2 className="h-4 w-4" aria-hidden="true" />
                        </button>
                      </div>
                    </div>

                    {/* ---------------------------------------- overrides */}
                    <div className="mt-3">
                      {overrides.length === 0 ? (
                        <p className="text-xs text-gray-500">İşletme bilgileri kullanılıyor.</p>
                      ) : (
                        <ul className="flex flex-col gap-1">
                          {overrides.map(({ key, Icon, text }) => (
                            <li key={key} className="flex items-start gap-1.5 text-xs text-gray-600">
                              <Icon className="mt-0.5 h-3.5 w-3.5 shrink-0 text-gray-400" aria-hidden="true" />
                              <span className="min-w-0 break-words">{text}</span>
                            </li>
                          ))}
                        </ul>
                      )}
                    </div>

                    {/* ------------------------------------ served menus */}
                    {served.length === 0 ? (
                      <div className="mt-4 rounded-lg border border-dashed border-gray-300 bg-gray-50 px-3 py-4 text-center">
                        <p className="text-sm text-gray-600">
                          Bu şubede yayınlanan menü yok, adresi boş açılır.
                        </p>
                        <button
                          type="button"
                          className="btn-secondary btn-sm mt-3"
                          onClick={() => openEditBranch(branch, true)}
                        >
                          <ListChecks className="h-3.5 w-3.5" aria-hidden="true" />
                          Menüleri seç
                        </button>
                      </div>
                    ) : (
                      <>
                        <div className="mt-3 flex flex-wrap gap-1.5">
                          {served.map((menu, index) => (
                            <span
                              key={menu.id}
                              className={`badge ${
                                index === 0
                                  ? 'bg-brand-50 text-brand-700'
                                  : 'bg-gray-100 text-gray-600'
                              }`}
                            >
                              {menu.name}
                              {index === 0 ? ' · varsayılan' : ''}
                            </span>
                          ))}
                        </div>

                        <div className="mt-4 space-y-3">
                          {served.map((menu, index) => (
                            <BranchLinkCard
                              key={menu.id}
                              branch={branch}
                              menu={menu}
                              isBranchDefault={index === 0}
                            />
                          ))}
                        </div>
                      </>
                    )}
                  </div>
                )
              })}
            </div>
          )}
        </section>
      </div>

      {/* ------------------------------------------------------------ modals */}
      <MenuModal
        open={menuModalOpen}
        onClose={() => setMenuModalOpen(false)}
        menu={editingMenu}
        onSaved={reload}
      />

      <BranchModal
        open={branchModalOpen}
        onClose={() => setBranchModalOpen(false)}
        branch={editingBranch}
        menus={menus}
        focusMenus={focusMenus}
        onSaved={reload}
      />

      <ConfirmModal
        open={Boolean(menuToDelete)}
        onClose={() => setMenuToDelete(null)}
        onConfirm={confirmDeleteMenu}
        busy={deleting}
        title="Menüyü sil"
        message={`"${menuToDelete?.name || ''}" menüsü, içindeki kategoriler ve ürünler kalıcı olarak silinecek. Bu işlem geri alınamaz.`}
      />

      <ConfirmModal
        open={Boolean(branchToDelete)}
        onClose={() => setBranchToDelete(null)}
        onConfirm={confirmDeleteBranch}
        busy={deleting}
        title="Şubeyi sil"
        message={`"${branchToDelete?.name || ''}" şubesi, menü bağlantıları ve şubeye özel fiyatlar kalıcı olarak silinecek. Bu işlem geri alınamaz.`}
      />
    </div>
  )
}
