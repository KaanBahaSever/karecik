import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'

import api from '../../lib/api'
import { useAuth } from '../../lib/auth.jsx'
import { useActiveMenu } from '../../lib/menuContext.jsx'
import { cleanSlugInput, MIN_SLUG_LENGTH, slugify } from '../../lib/slugify'
import { APP_DOMAIN } from '../../lib/subdomain'
import Modal from '../ui/Modal.jsx'
import { useToast } from '../ui/Toast.jsx'

/* ------------------------------------------------------- shared form parts */

// The address rules live in lib/slugify.js, which mirrors the Go slugifier
// character for character — the preview below must promise the address the
// server will actually store.

/**
 * Bordered on/off row — the toggle idiom the other dashboard dialogs use.
 *
 * @param {boolean}  checked
 * @param {Function} onChange     - (next: boolean) => void
 * @param {string}   label
 * @param {string}   description  - Small caption below the label
 * @param {boolean}  disabled
 */
function ToggleField({ checked, onChange, label, description, disabled = false }) {
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
 * The public address is `{business-slug}.karecik.com/{menu-slug}`: the
 * subdomain names the business, the path names the menu. Menu slugs only have
 * to be unique inside their own business, so a taken address is not an error —
 * the server appends a number and answers with what it actually stored.
 *
 * @param {boolean}     open
 * @param {Function}    onClose
 * @param {object|null} menu    - null creates a new record
 * @param {Function}    onSaved - (menu) => void, called after a successful save
 */
export default function MenuModal({ open, onClose, menu, onSaved }) {
  const { business } = useAuth()
  const { createMenu } = useActiveMenu()
  const toast = useToast()

  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [customSlug, setCustomSlug] = useState(false)
  const [description, setDescription] = useState('')
  const [isActive, setIsActive] = useState(true)
  const [slugError, setSlugError] = useState('')
  const [saving, setSaving] = useState(false)

  /* Refill the form from the incoming menu every time the dialog opens. */
  useEffect(() => {
    if (!open) return

    setName(menu?.name || '')
    setSlug(menu?.slug || '')
    // An existing address is never rewritten from the name behind the user's
    // back — that would silently invalidate the QR codes already in print, so
    // an edit always opens with the address field visible.
    setCustomSlug(Boolean(menu))
    setDescription(menu?.description || '')
    setIsActive(menu?.is_active !== false)
    setSlugError('')
    setSaving(false)
  }, [open, menu])

  /** Opening the toggle seeds the field with the address shown until now. */
  function toggleCustomSlug(next) {
    setCustomSlug(next)
    setSlugError('')
    if (next && !slug) setSlug(slugify(name, ''))
  }

  const businessSlug = business?.slug || 'isletmeniz'

  // The address follows the name until the user takes it over. A custom address
  // left empty stays empty so the check in save() can refuse it; a derived one
  // falls back to "menu", exactly like utils.SlugifyWithFallback on the server.
  const requestedSlug = customSlug ? slugify(slug, '') : slugify(name, 'menu')
  const displaySlug = requestedSlug || 'menu-adresi'

  async function save() {
    if (saving) return
    setSlugError('')

    const finalName = name.trim()
    if (finalName.length < 2 || finalName.length > 60) {
      toast.error('Menü adı 2 ile 60 karakter arasında olmalıdır.')
      return
    }

    if (requestedSlug.length < MIN_SLUG_LENGTH) {
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
        slug: requestedSlug,
        description: finalDescription,
        is_active: isActive,
      }

      // Creating goes through the menu context, so the new menu is loaded and
      // selected as the active one before this dialog closes.
      const saved = menu ? await api.updateMenu(menu.id, payload) : await createMenu(payload)

      // The address was already taken in this business: the server appended a
      // number, and the user has to be told which address was really stored.
      const renamed = Boolean(saved?.slug) && saved.slug !== requestedSlug
      if (renamed && !menu) toast.success(`Menü oluşturuldu. Adres: ${saved.slug}`)
      else if (renamed) toast.success(`Menü kaydedildi. Adres: ${saved.slug}`)
      else toast.success(menu ? 'Menü kaydedildi.' : 'Menü oluşturuldu.')

      onSaved?.(saved)
      onClose?.()
    } catch (error) {
      // A rejected address comes back as 409 and belongs on the field itself,
      // not in a toast that disappears before it can be acted on.
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
            onChange={(event) => setName(event.target.value)}
          />
          <p className="help-text">2 ile 60 karakter arasında olmalıdır.</p>
        </div>

        {/* ------------------------------------------------------------ slug */}
        <div>
          <span className="label">Menü adresi</span>

          {/* Both segments, so the user sees the address a customer will type:
              the subdomain is the business, the path is this menu. */}
          <p className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 font-mono text-xs text-gray-700">
            {businessSlug}.{APP_DOMAIN}/<b className="text-gray-900">{displaySlug}</b>
          </p>

          <div className="mt-3">
            <ToggleField
              checked={customSlug}
              onChange={toggleCustomSlug}
              label="Adresi özelleştir"
              description="Kapalıyken adres menü adından oluşturulur."
            />
          </div>

          {customSlug ? (
            <input
              id="menu-slug"
              type="text"
              className="input mt-2"
              value={slug}
              placeholder="kahvalti"
              autoComplete="off"
              spellCheck={false}
              aria-label="Menü adresi"
              onChange={(event) => {
                setSlugError('')
                setSlug(cleanSlugInput(event.target.value))
              }}
            />
          ) : null}

          {slugError ? <p className="error-text">{slugError}</p> : null}

          <p className="help-text">
            Bu adres bu işletmede kullanılıyorsa sonuna otomatik olarak bir numara eklenir.
          </p>
          {menu ? (
            <p className="help-text">
              Adresi değiştirirseniz bu menüyü gösteren eski QR kodları çalışmaya devam ETMEZ.
            </p>
          ) : null}
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
        <ToggleField
          checked={isActive}
          onChange={setIsActive}
          label="Menü yayında"
          description="Kapatırsanız menü müşterilere gösterilmez, adresi de açılmaz."
        />
      </div>
    </Modal>
  )
}
