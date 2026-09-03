import { useEffect, useMemo, useRef, useState } from 'react'
import { Loader2, MapPin, Phone, Wifi } from 'lucide-react'

import api from '../../lib/api'
import Modal from '../ui/Modal.jsx'
import { useToast } from '../ui/Toast.jsx'
// The address rules and the toggle row are shared with the menu dialog; they
// live there because that is where they were needed first.
import { ToggleField, cleanSlug, finalizeSlug } from './MenuModal.jsx'

/* A branch address is a subdomain, exactly like the business address */
const BRANCH_SUFFIX = '.karecik.com'

/**
 * The menu ids of a branch in the order the API returned them. The first one is
 * the branch' own default menu: this dialog always sends the default first and
 * the backend stores that order, so it can be read back the same way.
 */
function storedMenuIDs(branch) {
  return Array.isArray(branch?.menu_ids) ? branch.menu_ids : []
}

/**
 * Create / edit dialog for a branch, including the menus it serves.
 *
 * The menu assignment is a separate endpoint (PUT /api/branches/:id/menus), so
 * it is saved only after the create/update call resolves — a new branch has no
 * id before that.
 *
 * @param {boolean}     open
 * @param {Function}    onClose
 * @param {object|null} branch     - null creates a new record
 * @param {Array}       menus      - every menu of the business
 * @param {boolean}     focusMenus - scroll straight to the menu assignment
 * @param {Function}    onSaved    - (branch) => void, after a successful save
 */
export default function BranchModal({
  open,
  onClose,
  branch,
  menus,
  focusMenus = false,
  onSaved,
}) {
  const toast = useToast()

  const menuList = useMemo(() => (Array.isArray(menus) ? menus : []), [menus])
  // A stable dependency for the refill effect: a fresh array on every render of
  // the page would otherwise reset the form while the user is typing in it.
  const menuKey = menuList.map((menu) => menu.id).join(',')

  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [slugTouched, setSlugTouched] = useState(false)
  const [phone, setPhone] = useState('')
  const [address, setAddress] = useState('')
  const [wifiSSID, setWifiSSID] = useState('')
  const [wifiPassword, setWifiPassword] = useState('')
  const [isActive, setIsActive] = useState(true)
  const [isDefault, setIsDefault] = useState(false)
  const [selectedIDs, setSelectedIDs] = useState([])
  const [defaultMenuID, setDefaultMenuID] = useState('')
  const [slugError, setSlugError] = useState('')
  const [saving, setSaving] = useState(false)

  const menuSectionRef = useRef(null)

  /* Refill the form from the incoming branch every time the dialog opens. */
  useEffect(() => {
    if (!open) return

    setName(branch?.name || '')
    setSlug(branch?.slug || '')
    // An existing address is never rewritten from the name behind the user's
    // back — that would silently invalidate the QR codes already in print.
    setSlugTouched(Boolean(branch))
    setPhone(branch?.phone || '')
    setAddress(branch?.address || '')
    setWifiSSID(branch?.wifi_ssid || '')
    setWifiPassword(branch?.wifi_password || '')
    setIsActive(branch?.is_active !== false)
    setIsDefault(Boolean(branch?.is_default))
    setSlugError('')
    setSaving(false)

    // A new branch starts on the default menu of the business, which is also
    // what the API links for it when the assignment is left untouched.
    const stored = storedMenuIDs(branch).filter((id) =>
      menuList.some((menu) => menu.id === id),
    )
    const initial = stored.length
      ? stored
      : menuList.filter((menu) => menu.is_default).map((menu) => menu.id)

    // Keep the checked ids in the order of the menu list, so the checkbox list
    // and the branch chips read the same way.
    setSelectedIDs(menuList.filter((menu) => initial.includes(menu.id)).map((menu) => menu.id))
    setDefaultMenuID(initial[0] || '')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, branch, menuKey])

  /* "Menüleri seç" opens the same dialog, only scrolled to the assignment. */
  useEffect(() => {
    if (!open || !focusMenus) return
    menuSectionRef.current?.scrollIntoView({ block: 'start' })
  }, [open, focusMenus])

  /** The address follows the name until the user edits it by hand. */
  function updateName(value) {
    setName(value)
    if (!slugTouched) setSlug(cleanSlug(value))
  }

  /** Checks or unchecks one menu, keeping the default pointing at a checked one. */
  function toggleMenu(menuID) {
    const next = selectedIDs.includes(menuID)
      ? selectedIDs.filter((id) => id !== menuID)
      : [...selectedIDs, menuID]

    const ordered = menuList.filter((menu) => next.includes(menu.id)).map((menu) => menu.id)
    setSelectedIDs(ordered)
    if (!ordered.includes(defaultMenuID)) setDefaultMenuID(ordered[0] || '')
  }

  const alreadyDefault = Boolean(branch?.is_default)
  const displaySlug = finalizeSlug(slug) || 'sube-adresi'
  const selectedMenus = menuList.filter((menu) => selectedIDs.includes(menu.id))

  async function save() {
    if (saving) return
    setSlugError('')

    const finalName = name.trim()
    if (finalName.length < 2 || finalName.length > 60) {
      toast.error('Şube adı 2 ile 60 karakter arasında olmalıdır.')
      return
    }

    const finalSlug = finalizeSlug(slug)
    if (finalSlug.length < 2) {
      setSlugError(
        'Şube adresi en az 2 karakter olmalı ve yalnızca harf, rakam ve tire içerebilir.',
      )
      return
    }

    // The API answers an empty list with 422; catching it here keeps the
    // branch itself from being written before the assignment is usable.
    if (selectedIDs.length === 0) {
      toast.error('Şubeye en az bir menü seçmelisiniz.')
      return
    }

    // An empty override means "inherit from the business", which is NULL in the
    // database — an empty string would store a blank value instead.
    const payload = {
      name: finalName,
      slug: finalSlug,
      phone: phone.trim() || null,
      address: address.trim() || null,
      wifi_ssid: wifiSSID.trim() || null,
      wifi_password: wifiPassword.trim() || null,
      is_active: isActive,
    }
    // is_default is never sent as false: the business must keep a default
    // branch, and the flag is moved by promoting another branch instead.
    if (isDefault) payload.is_default = true

    setSaving(true)

    let saved
    try {
      saved = branch
        ? await api.updateBranch(branch.id, payload)
        : await api.createBranch(payload)
    } catch (error) {
      // A taken or reserved address comes back as 409 and belongs on the field
      // itself, not in a toast that disappears before it can be acted on.
      if (error.status === 409) setSlugError(error.message)
      else toast.error(error.message)
      setSaving(false)
      return
    }

    // The branch default menu is sent first, because the backend stores the
    // list order and the dashboard reads menu_ids[0] back as that default.
    const finalDefaultID = selectedIDs.includes(defaultMenuID) ? defaultMenuID : selectedIDs[0]
    const orderedIDs = [finalDefaultID, ...selectedIDs.filter((id) => id !== finalDefaultID)]

    try {
      const linked = await api.setBranchMenus(saved.id, orderedIDs, finalDefaultID)
      if (linked) saved = linked
      toast.success('Şube kaydedildi.')
    } catch (error) {
      // The branch itself is already stored; only the assignment failed, so the
      // dialog still reports what was saved instead of pretending nothing was.
      toast.error(error.message)
    } finally {
      setSaving(false)
    }

    onSaved?.(saved)
    onClose?.()
  }

  return (
    <Modal
      open={open}
      onClose={saving ? () => {} : onClose}
      title={branch ? 'Şubeyi Düzenle' : 'Yeni Şube'}
      description="Şube adresi, iletişim bilgileri ve yayınlanan menüler."
      width="max-w-xl"
      footer={
        <>
          <button type="button" className="btn-secondary" onClick={onClose} disabled={saving}>
            İptal
          </button>
          <button type="button" className="btn-primary" onClick={save} disabled={saving}>
            {saving ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" /> Kaydediliyor
              </>
            ) : (
              'Kaydet'
            )}
          </button>
        </>
      }
    >
      <div className="space-y-5">
        {/* ------------------------------------------------------------ name */}
        <div>
          <label className="label" htmlFor="branch-name">
            Şube adı <span className="text-red-500">*</span>
          </label>
          <input
            id="branch-name"
            type="text"
            className="input"
            value={name}
            maxLength={60}
            placeholder="Örn. Kadıköy Şubesi"
            autoComplete="off"
            onChange={(event) => updateName(event.target.value)}
          />
          <p className="help-text">2 ile 60 karakter arasında olmalıdır.</p>
        </div>

        {/* ------------------------------------------------------------ slug */}
        <div>
          <label className="label" htmlFor="branch-slug">
            Şube adresi (subdomain)
          </label>
          <div className="flex">
            <input
              id="branch-slug"
              type="text"
              className="input rounded-r-none"
              value={slug}
              placeholder="kadikoy"
              autoComplete="off"
              spellCheck={false}
              onChange={(event) => {
                setSlugTouched(true)
                setSlugError('')
                setSlug(cleanSlug(event.target.value))
              }}
            />
            <span className="inline-flex shrink-0 items-center rounded-r-lg border border-l-0 border-gray-300 bg-gray-100 px-3 text-sm text-gray-500">
              {BRANCH_SUFFIX}
            </span>
          </div>

          {slugError ? <p className="error-text">{slugError}</p> : null}

          <p className="help-text">
            Şube menüsü şu adresten açılacak:{' '}
            <b className="text-gray-700">
              {displaySlug}
              {BRANCH_SUFFIX}
            </b>
          </p>
          <p className="help-text">
            Adresi değiştirirseniz bu şubenin eski QR kodları çalışmaya devam ETMEZ. Değişiklikten
            sonra QR kodlarını yeniden indirin.
          </p>
        </div>

        {/* ------------------------------------------------------- overrides */}
        <div className="rounded-lg border border-gray-200 p-3">
          <p className="text-sm font-medium text-gray-900">Şube bilgileri (opsiyonel)</p>
          <p className="help-text">Boş bıraktığınız alanlar işletme bilgilerinden alınır.</p>

          <div className="mt-4 space-y-4">
            <div>
              <label className="label" htmlFor="branch-phone">
                <span className="inline-flex items-center gap-1.5">
                  <Phone className="h-3.5 w-3.5 text-gray-400" aria-hidden="true" />
                  Telefon
                </span>
              </label>
              <input
                id="branch-phone"
                type="tel"
                className="input"
                value={phone}
                placeholder="+90 555 000 00 00"
                onChange={(event) => setPhone(event.target.value)}
              />
            </div>

            <div>
              <label className="label" htmlFor="branch-address">
                <span className="inline-flex items-center gap-1.5">
                  <MapPin className="h-3.5 w-3.5 text-gray-400" aria-hidden="true" />
                  Adres
                </span>
              </label>
              <textarea
                id="branch-address"
                className="input resize-none"
                rows={2}
                value={address}
                placeholder="Mahalle, cadde, no"
                onChange={(event) => setAddress(event.target.value)}
              />
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <div>
                <label className="label" htmlFor="branch-wifi-ssid">
                  <span className="inline-flex items-center gap-1.5">
                    <Wifi className="h-3.5 w-3.5 text-gray-400" aria-hidden="true" />
                    Wi-Fi ağı
                  </span>
                </label>
                <input
                  id="branch-wifi-ssid"
                  type="text"
                  className="input"
                  value={wifiSSID}
                  placeholder="Kahve_Duragi"
                  autoComplete="off"
                  onChange={(event) => setWifiSSID(event.target.value)}
                />
              </div>

              <div>
                <label className="label" htmlFor="branch-wifi-password">
                  Wi-Fi şifresi
                </label>
                <input
                  id="branch-wifi-password"
                  type="text"
                  className="input"
                  value={wifiPassword}
                  placeholder="misafir2024"
                  autoComplete="off"
                  onChange={(event) => setWifiPassword(event.target.value)}
                />
              </div>
            </div>
          </div>
        </div>

        {/* -------------------------------------------------- menu assignment */}
        <div ref={menuSectionRef} className="rounded-lg border border-gray-200 p-3">
          <p className="text-sm font-medium text-gray-900">Bu şubede yayınlanan menüler</p>
          <p className="help-text">En az bir menü seçmelisiniz.</p>

          {menuList.length === 0 ? (
            <p className="mt-3 rounded-lg border border-dashed border-gray-300 bg-gray-50 px-3 py-4 text-center text-sm text-gray-500">
              Henüz menü yok. Önce bir menü ekleyin.
            </p>
          ) : (
            <div className="mt-3 space-y-2">
              {menuList.map((menu) => (
                <label
                  key={menu.id}
                  className="flex cursor-pointer items-center gap-3 rounded-lg border border-gray-200 px-3 py-2"
                >
                  <input
                    type="checkbox"
                    className="h-4 w-4 shrink-0 rounded border-gray-300 text-brand-600 focus:ring-brand-500"
                    checked={selectedIDs.includes(menu.id)}
                    onChange={() => toggleMenu(menu.id)}
                  />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium text-gray-700">
                      {menu.name}
                    </span>
                    <span className="block truncate text-xs text-gray-500">/{menu.slug}</span>
                  </span>
                  {menu.is_active === false ? (
                    <span className="badge shrink-0 bg-gray-100 text-gray-600">Pasif</span>
                  ) : null}
                </label>
              ))}
            </div>
          )}

          {selectedMenus.length > 0 ? (
            <div className="mt-4">
              <label className="label" htmlFor="branch-default-menu">
                Şubenin varsayılan menüsü
              </label>
              <select
                id="branch-default-menu"
                className="input"
                value={selectedIDs.includes(defaultMenuID) ? defaultMenuID : selectedIDs[0]}
                onChange={(event) => setDefaultMenuID(event.target.value)}
              >
                {selectedMenus.map((menu) => (
                  <option key={menu.id} value={menu.id}>
                    {menu.name}
                  </option>
                ))}
              </select>
              <p className="help-text">
                <b className="text-gray-700">
                  {displaySlug}
                  {BRANCH_SUFFIX}
                </b>{' '}
                adresi doğrudan bu menüyü açar; diğer menüler adresin sonuna kendi adlarıyla
                eklenir.
              </p>
            </div>
          ) : null}
        </div>

        {/* --------------------------------------------------------- toggles */}
        <div className="space-y-3">
          <ToggleField
            checked={isActive}
            onChange={setIsActive}
            label="Şube yayında"
            description="Kapatırsanız şube adresi müşterilere açılmaz."
          />

          <ToggleField
            checked={isDefault}
            onChange={setIsDefault}
            disabled={alreadyDefault}
            label="Varsayılan şube"
            description={
              alreadyDefault
                ? 'Bu şube zaten varsayılan. Değiştirmek için başka bir şubeyi varsayılan yapın.'
                : 'İşletmenin yalnızca bir varsayılan şubesi olur; açarsanız varsayılan bu şubeye taşınır.'
            }
          />
        </div>
      </div>
    </Modal>
  )
}
