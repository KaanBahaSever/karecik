import { useState } from 'react'
import { Check, Copy, Layers, Link2, Loader2, Plus, Repeat, Trash2 } from 'lucide-react'

import { useAuth } from '../../lib/auth.jsx'
import { useActiveMenu } from '../../lib/menuContext.jsx'
import { menuUrl } from '../../lib/subdomain'
import ConfirmModal from '../ui/ConfirmModal.jsx'
import EmptyState from '../ui/EmptyState.jsx'
import Modal from '../ui/Modal.jsx'
import { useToast } from '../ui/Toast.jsx'
import MenuModal from './MenuModal.jsx'

/**
 * The bar that says — unmistakably — which menu the dashboard is editing.
 *
 * A menu owns every setting now, so the selection made here decides what the
 * editor, the settings page and the live preview act on. It is deliberately a
 * full-width card and not a small dropdown in the topbar: the previous switcher
 * was so discreet that people edited the wrong menu without noticing.
 *
 * It renders with a single menu too — that is the point. With no menu at all it
 * becomes an empty state: a business may own zero menus, and nothing is
 * selected then.
 *
 * The switch modal is also the only place a menu can be deleted, which is why
 * it opens for a single menu as well: otherwise a business owning exactly one
 * menu could never remove it.
 */

/* ------------------------------------------------------------- helpers */

/**
 * The public address of a menu: `{business-slug}.karecik.com/{menu-slug}`.
 * `menu_url` is the same address computed by the server; the local builder is
 * the fallback for a payload that predates it.
 */
function addressOf(businessSlug, menu) {
  if (!menu) return ''
  return menu.menu_url || menuUrl(businessSlug, menu.slug)
}

/** "https://kahve-duragi.karecik.com/kahvalti" -> "kahve-duragi.karecik.com/kahvalti" */
function shortAddress(address) {
  return String(address || '').replace(/^https?:\/\//, '')
}

/** "3 kategori" — the count is computed by the server for every menu. */
function categoryLabel(menu) {
  return `${Number(menu?.category_count) || 0} kategori`
}

/* --------------------------------------------------------------- pills */

/** Status marker shown next to a menu name. */
function MenuPills({ menu }) {
  if (menu?.is_active !== false) return null
  return <span className="badge shrink-0 bg-gray-100 text-gray-600">Pasif</span>
}

/* ----------------------------------------------------------------- bar */

export default function ActiveMenuBar({ className = '' }) {
  const {
    menus,
    loading,
    error,
    activeMenu,
    activeMenuId,
    setActiveMenuId,
    reload,
    deleteMenu,
  } = useActiveMenu()
  const { business } = useAuth()
  const toast = useToast()

  const [switchOpen, setSwitchOpen] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)

  // The menu the confirmation dialog is asking about, and the flag that keeps
  // its buttons disabled while the request runs.
  const [menuToDelete, setMenuToDelete] = useState(null)
  const [deleting, setDeleting] = useState(false)

  // The subdomain half of every address on this bar. A menu slug alone is
  // meaningless: two businesses may both own a menu called "kahvalti".
  const businessSlug = business?.slug || ''

  const address = addressOf(businessSlug, activeMenu)

  async function copyAddress() {
    if (!address) return
    try {
      await navigator.clipboard.writeText(address)
      toast.success('Menü adresi kopyalandı.')
    } catch {
      toast.error('Menü adresi kopyalanamadı.')
    }
  }

  function choose(id) {
    setActiveMenuId(id)
    setSwitchOpen(false)
  }

  /**
   * Deletes the menu the confirmation dialog is about. The context re-selects
   * the first remaining menu on its own — or nothing at all when that was the
   * last one, and the bar renders its empty state instead.
   */
  async function confirmDelete() {
    if (!menuToDelete) return
    const target = menuToDelete

    // Read before the request: afterwards the list no longer holds the menu.
    const wasLast = menus.length <= 1
    setDeleting(true)

    try {
      await deleteMenu(target.id)

      setMenuToDelete(null)
      // Nothing is left to switch between, so the switcher must not stay armed
      // — it would pop back open the moment a new menu is created.
      if (wasLast) setSwitchOpen(false)
      toast.success('Menü silindi.')
    } catch (err) {
      toast.error(err.message)
    } finally {
      setDeleting(false)
    }
  }

  const wrapper = `card mb-6 p-4 ${className}`

  /* -------------------------------------------------------- loading */

  if (loading && menus.length === 0) {
    return (
      <section className={wrapper} aria-busy="true">
        <div className="flex items-center gap-2 text-sm text-gray-500">
          <Loader2 className="h-4 w-4 animate-spin text-brand-600" aria-hidden="true" />
          Menüler yükleniyor...
        </div>
      </section>
    )
  }

  /* ---------------------------------------------------------- empty */

  // No menu selected, because there is none to select: a legitimate state, not
  // a failure. The error case shares it — without a list there is nothing to
  // switch between — and adds the retry next to the create button.
  if (!activeMenu) {
    return (
      <>
        {/* Only an error is explained here: every page adds its own line about
            what it cannot show without a menu. */}
        <EmptyState
          className={`mb-6 py-8 ${className}`}
          icon={Layers}
          title={error ? 'Menüler yüklenemedi' : 'Menü seçilmedi'}
          description={error}
          action={
            <div className="flex flex-wrap items-center justify-center gap-2">
              {error ? (
                <button type="button" className="btn-secondary" onClick={reload}>
                  Yeniden dene
                </button>
              ) : null}
              <button type="button" className="btn-primary" onClick={() => setCreateOpen(true)}>
                <Plus className="h-4 w-4" aria-hidden="true" />
                Yeni Menü Oluştur
              </button>
            </div>
          }
        />

        <MenuModal open={createOpen} onClose={() => setCreateOpen(false)} menu={null} />
      </>
    )
  }

  /* ---------------------------------------------------------- ready */

  return (
    <>
      {/* --------------------------------------------- delete confirmation */}
      {/* Mounted before the switcher on purpose. Both dialogs lock <body>
          scrolling and put back whatever they found on the way in, so this one
          — which found the "hidden" the switcher underneath it had just set —
          has to be destroyed first, or it would restore that lock over the
          switcher's release. Deleting the last menu unmounts both in the same
          commit, and React destroys them in tree order. The z-index is what
          then keeps it on top, since the reading order no longer does. */}
      {menuToDelete ? (
        <div className="relative z-[60]">
          <ConfirmModal
            open
            onClose={() => setMenuToDelete(null)}
            onConfirm={confirmDelete}
            title="Menüyü sil"
            message={`"${menuToDelete.name}" menüsü ve içindeki tüm kategori ve ürünler kalıcı olarak silinecek.`}
            confirmText="Sil"
            busy={deleting}
          />
        </div>
      ) : null}

      <section className={wrapper}>
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          {/* ------------------------------------------- which menu */}
          <div className="flex min-w-0 items-center gap-3">
            <span
              className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-brand-50 text-brand-600"
              aria-hidden="true"
            >
              <Layers className="h-5 w-5" />
            </span>

            <div className="min-w-0">
              <p className="text-xs font-medium uppercase tracking-wide text-gray-500">
                Düzenlenen menü
              </p>
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <h2 className="truncate text-lg font-semibold text-gray-900">{activeMenu.name}</h2>
                <MenuPills menu={activeMenu} />
              </div>
            </div>
          </div>

          {/* --------------------------------------- address and actions */}
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <div className="flex min-w-0 items-center gap-1.5 rounded-lg border border-gray-200 bg-gray-50 px-2.5 py-1.5">
              <Link2 className="h-3.5 w-3.5 shrink-0 text-gray-400" aria-hidden="true" />
              <span className="truncate font-mono text-xs text-gray-600" title={address}>
                {shortAddress(address)}
              </span>
              <button
                type="button"
                onClick={copyAddress}
                disabled={!address}
                title="Menü adresini kopyala"
                aria-label="Menü adresini kopyala"
                className="shrink-0 rounded p-1 text-gray-400 hover:bg-white hover:text-gray-700 disabled:cursor-not-allowed"
              >
                <Copy className="h-3.5 w-3.5" aria-hidden="true" />
              </button>
            </div>

            <button
              type="button"
              className="btn-secondary btn-sm"
              onClick={() => setSwitchOpen(true)}
              title={
                menus.length < 2
                  ? 'Tek menünüz var. Buradan silebilir veya yeni bir menü oluşturabilirsiniz.'
                  : 'Başka bir menüye geç veya bir menüyü sil'
              }
            >
              <Repeat className="h-4 w-4" aria-hidden="true" />
              Menü Değiştir
            </button>

            <button
              type="button"
              className="btn-primary btn-sm"
              onClick={() => setCreateOpen(true)}
            >
              <Plus className="h-4 w-4" aria-hidden="true" />
              Yeni Menü
            </button>
          </div>
        </div>
      </section>

      {/* --------------------------------------------------- switcher */}
      <Modal
        open={switchOpen}
        onClose={() => setSwitchOpen(false)}
        title="Menü Değiştir"
        description="Panelde düzenlemek istediğiniz menüyü seçin veya bir menüyü silin."
        width="max-w-2xl"
        footer={
          <button type="button" className="btn-secondary" onClick={() => setSwitchOpen(false)}>
            Kapat
          </button>
        }
      >
        <div className="space-y-2">
          {menus.map((menu) => {
            const isSelected = menu.id === activeMenuId
            const menuAddress = addressOf(businessSlug, menu)

            return (
              // Choosing and deleting are two separate buttons — a button
              // nested inside a button is invalid markup — so the row itself is
              // the plain element that carries the border.
              <div
                key={menu.id}
                className={`flex items-start gap-2 rounded-xl border p-3 hover:border-brand-400 ${
                  isSelected ? 'border-brand-600 ring-2 ring-brand-600' : 'border-gray-200'
                }`}
              >
                <button
                  type="button"
                  onClick={() => choose(menu.id)}
                  aria-pressed={isSelected}
                  className="flex min-w-0 flex-1 items-start gap-3 text-left"
                >
                  <span
                    className={`mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full border ${
                      isSelected
                        ? 'border-brand-600 bg-brand-600 text-white'
                        : 'border-gray-300 text-transparent'
                    }`}
                    aria-hidden="true"
                  >
                    <Check className="h-3.5 w-3.5" />
                  </span>

                  <span className="min-w-0 flex-1">
                    <span className="flex flex-wrap items-center gap-2">
                      <span className="truncate text-sm font-semibold text-gray-900">
                        {menu.name}
                      </span>
                      <MenuPills menu={menu} />
                    </span>

                    <span className="mt-1 block truncate font-mono text-xs text-gray-500">
                      {shortAddress(menuAddress)}
                    </span>

                    <span className="mt-0.5 block text-xs text-gray-500">
                      {categoryLabel(menu)} · /{menu.slug}
                    </span>
                  </span>
                </button>

                {/* Deleting the last menu is allowed: a business may own none,
                    and the bar falls back to its empty state. */}
                <button
                  type="button"
                  onClick={() => setMenuToDelete(menu)}
                  className="shrink-0 rounded-md p-1.5 text-gray-400 hover:bg-red-50 hover:text-red-600"
                  aria-label={`${menu.name} menüsünü sil`}
                  title="Menüyü sil"
                >
                  <Trash2 className="h-4 w-4" aria-hidden="true" />
                </button>
              </div>
            )
          })}
        </div>
      </Modal>

      {/* ------------------------------------------------- new menu */}
      <MenuModal open={createOpen} onClose={() => setCreateOpen(false)} menu={null} />
    </>
  )
}
