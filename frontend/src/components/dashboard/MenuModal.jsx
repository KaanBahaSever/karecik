import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'

import api from '../../lib/api'
import { useAuth } from '../../lib/auth.jsx'
import Modal from '../ui/Modal.jsx'
import { useToast } from '../ui/Toast.jsx'

/* The suffix shown next to every public address (mirrors Settings.jsx) */
const MENU_SUFFIX = '.karecik.com'

/* ------------------------------------------------------- shared form parts */

// cleanSlug, finalizeSlug and ToggleField are exported because BranchModal
// needs exactly the same address rules and the same toggle. There is no shared
// module for dashboard form pieces yet, so they live next to their first user
// instead of being written out twice.

/* ASCII equivalents of Turkish letters (mirrors pages/dashboard/Settings.jsx) */
const TURKISH_LETTERS = {
  ç: 'c',
  Ç: 'c',
  ğ: 'g',
  Ğ: 'g',
  ı: 'i',
  I: 'i',
  İ: 'i',
  ö: 'o',
  Ö: 'o',
  ş: 's',
  Ş: 's',
  ü: 'u',
  Ü: 'u',
  â: 'a',
  Â: 'a',
  î: 'i',
  Î: 'i',
  û: 'u',
  Û: 'u',
}

/**
 * Cleans a public address while the user types.
 * A trailing dash is kept so typing can continue; it is dropped on save.
 */
export function cleanSlug(input) {
  return String(input ?? '')
    .replace(/[çÇğĞıIİöÖşŞüÜâÂîÎûÛ]/g, (letter) => TURKISH_LETTERS[letter] || letter)
    .replace(/&/g, '-ve-')
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, '-')
    .replace(/-{2,}/g, '-')
    .replace(/^-+/, '')
    .slice(0, 60)
}

/** Final address to store: leading and trailing dashes removed. */
export function finalizeSlug(input) {
  return cleanSlug(input).replace(/-+$/, '')
}

/**
 * Bordered on/off row — the toggle idiom the other dashboard dialogs use.
 *
 * @param {boolean}  checked
 * @param {Function} onChange     - (next: boolean) => void
 * @param {string}   label
 * @param {string}   description  - Small caption below the label
 * @param {boolean}  disabled
 */
export function ToggleField({ checked, onChange, label, description, disabled = false }) {
  return (
    <label
      className={`flex items-start gap-3 rounded-lg border border-gray-200 p-3 ${
        disabled ? 'cursor-not-allowed bg-gray-50' : 'cursor-pointer'
      }`}
    >
      <input
        type="checkbox"
        className="mt-0.5 h-4 w-4 shrink-0 rounded border-gray-300 text-brand-600 focus:ring-brand-500"
        checked={checked}
        disabled={disabled}
        onChange={(event) => onChange(event.target.checked)}
      />
      <span className="min-w-0">
        <span className="block text-sm font-medium text-gray-700">{label}</span>
        {description ? <span className="block text-xs text-gray-500">{description}</span> : null}
      </span>
    </label>
  )
}

/* -------------------------------------------------------------------- modal */

/**
 * Create / edit dialog for a menu.
 *
 * @param {boolean}     open
 * @param {Function}    onClose
 * @param {object|null} menu    - null creates a new record
 * @param {Function}    onSaved - (menu) => void, called after a successful save
 */
export default function MenuModal({ open, onClose, menu, onSaved }) {
  const { business } = useAuth()
  const toast = useToast()

  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [slugTouched, setSlugTouched] = useState(false)
  const [description, setDescription] = useState('')
  const [isActive, setIsActive] = useState(true)
  const [isDefault, setIsDefault] = useState(false)
  const [slugError, setSlugError] = useState('')
  const [saving, setSaving] = useState(false)

  /* Refill the form from the incoming menu every time the dialog opens. */
  useEffect(() => {
    if (!open) return

    setName(menu?.name || '')
    setSlug(menu?.slug || '')
    // An existing address is never rewritten from the name behind the user's
    // back — that would silently invalidate the QR codes already in print.
    setSlugTouched(Boolean(menu))
    setDescription(menu?.description || '')
    setIsActive(menu?.is_active !== false)
    setIsDefault(Boolean(menu?.is_default))
    setSlugError('')
    setSaving(false)
  }, [open, menu])

  /** The address follows the name until the user edits it by hand. */
  function updateName(value) {
    setName(value)
    if (!slugTouched) setSlug(cleanSlug(value))
  }

  const alreadyDefault = Boolean(menu?.is_default)
  const businessSlug = business?.slug || 'menu-adresiniz'
  const displaySlug = finalizeSlug(slug) || 'menu-adresi'

  async function save() {
    if (saving) return
    setSlugError('')

    const finalName = name.trim()
    if (finalName.length < 2 || finalName.length > 60) {
      toast.error('Menü adı 2 ile 60 karakter arasında olmalıdır.')
      return
    }

    const finalSlug = finalizeSlug(slug)
    if (finalSlug.length < 2) {
      setSlugError(
        'Menü adresi en az 2 karakter olmalı ve yalnızca harf, rakam ve tire içerebilir.',
      )
      return
    }

    const finalDescription = description.trim()
    if (finalDescription.length > 200) {
      toast.error('Menü açıklaması en fazla 200 karakter olabilir.')
      return
    }

    setSaving(true)
    try {
      const payload = {
        name: finalName,
        slug: finalSlug,
        description: finalDescription,
        is_active: isActive,
      }

      let saved = menu
        ? await api.updateMenu(menu.id, payload)
        : await api.createMenu(payload)

      // POST /api/menus does not take is_default — the first menu of a business
      // becomes the default on its own — so promoting one is a second call. It
      // is never sent as false: the business must keep a default menu, and the
      // flag is moved by promoting another menu instead.
      if (isDefault && !saved.is_default) {
        saved = await api.updateMenu(saved.id, { is_default: true })
      }

      onSaved?.(saved)
      toast.success('Menü kaydedildi.')
      onClose?.()
    } catch (error) {
      // A taken or reserved address comes back as 409 and belongs on the field
      // itself, not in a toast that disappears before it can be acted on.
      if (error.status === 409) setSlugError(error.message)
      else toast.error(error.message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      open={open}
      onClose={saving ? () => {} : onClose}
      title={menu ? 'Menüyü Düzenle' : 'Yeni Menü'}
      description="Menü adı, adresi ve yayın durumu."
      width="max-w-lg"
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
          <label className="label" htmlFor="menu-name">
            Menü adı <span className="text-red-500">*</span>
          </label>
          <input
            id="menu-name"
            type="text"
            className="input"
            value={name}
            maxLength={60}
            placeholder="Örn. Kahvaltı Menüsü"
            autoComplete="off"
            onChange={(event) => updateName(event.target.value)}
          />
          <p className="help-text">2 ile 60 karakter arasında olmalıdır.</p>
        </div>

        {/* ------------------------------------------------------------ slug */}
        <div>
          <label className="label" htmlFor="menu-slug">
            Menü adresi
          </label>
          <input
            id="menu-slug"
            type="text"
            className="input"
            value={slug}
            placeholder="kahvalti"
            autoComplete="off"
            spellCheck={false}
            onChange={(event) => {
              setSlugTouched(true)
              setSlugError('')
              setSlug(cleanSlug(event.target.value))
            }}
          />

          {slugError ? <p className="error-text">{slugError}</p> : null}

          <p className="help-text">
            Menü şu adresten açılacak:{' '}
            <b className="text-gray-700">
              {businessSlug}
              {MENU_SUFFIX}/{displaySlug}
            </b>
          </p>
          <p className="help-text">
            Adresi değiştirirseniz bu menüyü gösteren eski QR kodları çalışmaya devam ETMEZ.
          </p>
        </div>

        {/* ----------------------------------------------------- description */}
        <div>
          <label className="label" htmlFor="menu-description">
            Açıklama (opsiyonel)
          </label>
          <textarea
            id="menu-description"
            className="input resize-none"
            rows={2}
            value={description}
            maxLength={200}
            placeholder="Menü hakkında kısa bir not"
            onChange={(event) => setDescription(event.target.value)}
          />
          <p className="help-text">En fazla 200 karakter.</p>
        </div>

        {/* --------------------------------------------------------- toggles */}
        <div className="space-y-3">
          <ToggleField
            checked={isActive}
            onChange={setIsActive}
            label="Menü yayında"
            description="Kapatırsanız menü müşterilere gösterilmez, adresi de açılmaz."
          />

          <ToggleField
            checked={isDefault}
            onChange={setIsDefault}
            disabled={alreadyDefault}
            label="Varsayılan menü"
            description={
              alreadyDefault
                ? 'Bu menü zaten varsayılan. Değiştirmek için başka bir menüyü varsayılan yapın.'
                : 'İşletmenin yalnızca bir varsayılan menüsü olur; açarsanız varsayılan bu menüye taşınır.'
            }
          />
        </div>
      </div>
    </Modal>
  )
}
