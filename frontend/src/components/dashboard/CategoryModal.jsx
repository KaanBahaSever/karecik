import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'

import api from '../../lib/api'
import { findLanguage } from '../../locales/index.js'
import Modal from '../ui/Modal.jsx'
import ImageUploader from '../ui/ImageUploader.jsx'
import { useToast } from '../ui/Toast.jsx'

/** Emoji offered in the icon grid (the backend column is plain TEXT). */
const ICON_OPTIONS = [
  '☕', '🥤', '🍽️', '🍰', '🍕', '🍔', '🥗', '🍺', '🍷',
  '🧃', '🥐', '🍳', '🍜', '🌮', '🍦', '🫖', '🧊', '⭐',
]

/**
 * Create / edit dialog for a category.
 *
 * @param {boolean}     open
 * @param {Function}    onClose
 * @param {object|null} category        - null creates a new record
 * @param {string[]}    languages       - the business' menu languages, e.g. ['tr','en']
 * @param {string}      defaultLanguage - the language in which the name is required
 * @param {string}      menuId          - the menu a NEW category is created in;
 *                                        omitted, the API falls back to the
 *                                        business' default menu
 * @param {Function}    onSaved         - (category) => void
 */
export default function CategoryModal({
  open,
  onClose,
  category,
  languages,
  defaultLanguage,
  menuId,
  onSaved,
}) {
  const toast = useToast()

  const languageList = Array.isArray(languages) && languages.length > 0 ? languages : ['tr']
  const primaryLanguage = languageList.includes(defaultLanguage)
    ? defaultLanguage
    : languageList[0]
  const languageKey = languageList.join(',')

  const [activeLanguage, setActiveLanguage] = useState(primaryLanguage)
  const [translations, setTranslations] = useState({})
  const [icon, setIcon] = useState('')
  const [imageUrl, setImageUrl] = useState(null)
  const [visible, setVisible] = useState(true)
  const [nameError, setNameError] = useState('')
  const [saving, setSaving] = useState(false)

  /* Refill the form from the incoming category every time the dialog opens. */
  useEffect(() => {
    if (!open) return

    const initial = {}
    languageList.forEach((code) => {
      initial[code] = {
        name: category?.translations?.[code]?.name || '',
        description: category?.translations?.[code]?.description || '',
      }
    })

    setTranslations(initial)
    setIcon(category?.icon || '')
    setImageUrl(category?.image_url || null)
    setVisible(category?.is_active !== false)
    setActiveLanguage(primaryLanguage)
    setNameError('')
    setSaving(false)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, category, primaryLanguage, languageKey])

  function updateField(languageCode, field, value) {
    setTranslations((previous) => ({
      ...previous,
      [languageCode]: {
        ...(previous[languageCode] || { name: '', description: '' }),
        [field]: value,
      },
    }))
    if (field === 'name' && languageCode === primaryLanguage && value.trim()) setNameError('')
  }

  async function save() {
    const primaryName = (translations[primaryLanguage]?.name || '').trim()
    if (!primaryName) {
      setActiveLanguage(primaryLanguage)
      setNameError('Kategori adı zorunludur.')
      return
    }

    // Languages left without a name are not sent at all.
    const payloadTranslations = {}
    languageList.forEach((code) => {
      const translation = translations[code]
      if (!translation) return
      const name = (translation.name || '').trim()
      if (!name) return
      payloadTranslations[code] = {
        name,
        description: (translation.description || '').trim(),
      }
    })

    const payload = {
      translations: payloadTranslations,
      icon: icon || null,
      image_url: imageUrl || null,
      is_active: visible,
    }

    // A new category belongs to the menu currently being edited. An existing one
    // is never moved implicitly — that is the branch/menu assignment's job.
    if (!category && menuId) payload.menu_id = menuId

    setSaving(true)
    try {
      const result = category
        ? await api.updateCategory(category.id, payload)
        : await api.createCategory(payload)

      onSaved?.(result)
      toast.success('Kategori kaydedildi.')
      onClose?.()
    } catch (error) {
      toast.error(error.message)
    } finally {
      setSaving(false)
    }
  }

  const activeTranslation = translations[activeLanguage] || { name: '', description: '' }
  const multiLanguage = languageList.length > 1

  return (
    <Modal
      open={open}
      onClose={saving ? () => {} : onClose}
      title={category ? 'Kategoriyi Düzenle' : 'Yeni Kategori'}
      description="Kategori adı, görseli ve menüde görünürlüğü."
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
        {/* ------------------------------------------------- language tabs */}
        {multiLanguage ? (
          <div className="flex flex-wrap gap-1 rounded-lg bg-gray-100 p-1">
            {languageList.map((code) => {
              const language = findLanguage(code)
              const isSelected = code === activeLanguage
              return (
                <button
                  key={code}
                  type="button"
                  onClick={() => setActiveLanguage(code)}
                  className={`flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium ${
                    isSelected
                      ? 'bg-white text-gray-900 shadow-card'
                      : 'text-gray-500 hover:text-gray-700'
                  }`}
                >
                  <span aria-hidden="true">{language.short}</span>
                  {language.label}
                  {code === primaryLanguage ? (
                    <span className="text-xs text-brand-600">•</span>
                  ) : null}
                </button>
              )
            })}
          </div>
        ) : null}

        {/* --------------------------------------------- name / description */}
        <div>
          <label className="label" htmlFor={`category-name-${activeLanguage}`}>
            Kategori adı
            {activeLanguage === primaryLanguage ? <span className="text-red-500"> *</span> : null}
          </label>
          <input
            id={`category-name-${activeLanguage}`}
            type="text"
            className="input"
            value={activeTranslation.name}
            onChange={(event) => updateField(activeLanguage, 'name', event.target.value)}
            placeholder={activeLanguage === 'tr' ? 'Örn. Sıcak İçecekler' : 'Örn. Hot Drinks'}
            maxLength={80}
            autoComplete="off"
          />
          {activeLanguage === primaryLanguage && nameError ? (
            <p className="error-text">{nameError}</p>
          ) : (
            <p className="help-text">
              {activeLanguage === primaryLanguage
                ? 'Menüde bu ad görünür.'
                : `${findLanguage(activeLanguage).label} çevirisi boş bırakılırsa bu dil gönderilmez.`}
            </p>
          )}
        </div>

        <div>
          <label className="label" htmlFor={`category-description-${activeLanguage}`}>
            Açıklama (opsiyonel)
          </label>
          <textarea
            id={`category-description-${activeLanguage}`}
            className="input resize-none"
            rows={2}
            value={activeTranslation.description}
            onChange={(event) => updateField(activeLanguage, 'description', event.target.value)}
            placeholder="Kategori hakkında kısa bir not"
            maxLength={240}
          />
        </div>

        {/* ------------------------------------------------------------ icon */}
        <div>
          <span className="label">Kategori ikonu (opsiyonel)</span>
          <div className="grid grid-cols-9 gap-1.5">
            {ICON_OPTIONS.map((emoji) => {
              const isSelected = icon === emoji
              return (
                <button
                  key={emoji}
                  type="button"
                  onClick={() => setIcon(isSelected ? '' : emoji)}
                  aria-pressed={isSelected}
                  className={`flex h-9 items-center justify-center rounded-lg border text-lg ${
                    isSelected
                      ? 'border-brand-600 bg-brand-50'
                      : 'border-gray-200 bg-white hover:bg-gray-50'
                  }`}
                >
                  <span aria-hidden="true">{emoji}</span>
                </button>
              )
            })}
          </div>
          <p className="help-text">
            {icon ? 'Seçimi kaldırmak için ikona tekrar tıklayın.' : 'İsteğe bağlı.'}
          </p>
        </div>

        {/* ----------------------------------------------------------- image */}
        <ImageUploader
          value={imageUrl}
          onChange={setImageUrl}
          label="Kategori görseli (opsiyonel)"
        />

        {/* ---------------------------------------------------------- toggle */}
        <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-gray-200 p-3">
          <input
            type="checkbox"
            className="mt-0.5 h-4 w-4 shrink-0 rounded border-gray-300 text-brand-600 focus:ring-brand-500"
            checked={visible}
            onChange={(event) => setVisible(event.target.checked)}
          />
          <span className="min-w-0">
            <span className="block text-sm font-medium text-gray-700">Menüde göster</span>
            <span className="block text-xs text-gray-500">
              Kapatırsanız kategori ve ürünleri müşteri menüsünde görünmez.
            </span>
          </span>
        </label>
      </div>
    </Modal>
  )
}
