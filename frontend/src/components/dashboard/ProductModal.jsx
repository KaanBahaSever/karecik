import { useEffect, useState } from 'react'
import { Ban, Loader2, Plus, X } from 'lucide-react'

import api from '../../lib/api'
import { currencySymbol, parsePrice, priceToInput } from '../../lib/format'
import { ALLERGENS, findLanguage } from '../../locales/index.js'
import {
  BADGE_COLOR_PRESETS,
  BADGE_ICONS,
  BadgeIcon,
  DEFAULT_BADGE,
  MAX_BADGES,
  MAX_BADGE_TEXT,
} from '../../themes/badges.js'
import Modal from '../ui/Modal.jsx'
import ImageUploader from '../ui/ImageUploader.jsx'
import { useToast } from '../ui/Toast.jsx'

const EMPTY_TRANSLATION = { name: '', description: '', ingredients: '' }

/* Badges have no id until the server assigns one, so the editor keeps a local
   `uid` purely as the React key. It is stripped before the payload is sent. */
let badgeRowCounter = 0

function newBadgeRow(values) {
  badgeRowCounter += 1
  return { uid: `badge-${badgeRowCounter}`, id: '', ...DEFAULT_BADGE, ...(values || {}) }
}

/** Turns the stored badge array into editable rows. */
function badgeRowsOf(product) {
  if (!Array.isArray(product?.badges)) return []
  return product.badges.map((badge) =>
    newBadgeRow({
      id: badge?.id || '',
      text: badge?.text || '',
      icon: badge?.icon || '',
      bg_color: badge?.bg_color || DEFAULT_BADGE.bg_color,
      text_color: badge?.text_color || DEFAULT_BADGE.text_color,
    }),
  )
}

/** Checks that an input such as "145,00" / "145.00" can be parsed as a number. */
function isValidPriceText(text) {
  const clean = String(text ?? '').trim()
  if (!clean) return false
  if (!/^[0-9.,\s]+$/.test(clean)) return false
  return /[0-9]/.test(clean)
}

/** Label shown for a category in the picker. */
function categoryName(category, defaultLanguage) {
  const translations = category?.translations || {}
  return (
    translations[defaultLanguage]?.name ||
    translations.tr?.name ||
    Object.values(translations)[0]?.name ||
    'Adsız kategori'
  )
}

/**
 * Create / edit dialog for a product.
 *
 * @param {boolean}     open
 * @param {Function}    onClose
 * @param {object|null} product            - null creates a new record
 * @param {Array}       categories
 * @param {string}      selectedCategoryId - preselected category for new products
 * @param {string[]}    languages
 * @param {string}      defaultLanguage
 * @param {string}      currency           - 'TRY', 'EUR' ...
 * @param {Function}    onSaved            - (product) => void
 */
export default function ProductModal({
  open,
  onClose,
  product,
  categories,
  selectedCategoryId,
  languages,
  defaultLanguage,
  currency,
  onSaved,
}) {
  const toast = useToast()

  const categoryList = Array.isArray(categories) ? categories : []
  const languageList = Array.isArray(languages) && languages.length > 0 ? languages : ['tr']
  const primaryLanguage = languageList.includes(defaultLanguage)
    ? defaultLanguage
    : languageList[0]
  const languageKey = languageList.join(',')
  const symbol = currencySymbol(currency)

  const [categoryId, setCategoryId] = useState('')
  const [activeLanguage, setActiveLanguage] = useState(primaryLanguage)
  const [translations, setTranslations] = useState({})
  const [price, setPrice] = useState('')
  const [comparePrice, setComparePrice] = useState('')
  const [calories, setCalories] = useState('')
  const [imageUrl, setImageUrl] = useState(null)
  const [allergens, setAllergens] = useState([])
  const [badges, setBadges] = useState([])
  const [visible, setVisible] = useState(true)
  const [featured, setFeatured] = useState(false)
  const [errors, setErrors] = useState({})
  const [saving, setSaving] = useState(false)

  /* Fill the form when the dialog opens. */
  useEffect(() => {
    if (!open) return

    const initial = {}
    languageList.forEach((code) => {
      initial[code] = {
        name: product?.translations?.[code]?.name || '',
        description: product?.translations?.[code]?.description || '',
        ingredients: product?.translations?.[code]?.ingredients || '',
      }
    })

    setTranslations(initial)
    setCategoryId(product?.category_id || selectedCategoryId || categoryList[0]?.id || '')
    setPrice(product ? priceToInput(product.price) : '')
    setComparePrice(
      product?.compare_price !== null && product?.compare_price !== undefined
        ? priceToInput(product.compare_price)
        : '',
    )
    setCalories(
      product?.calories === null || product?.calories === undefined
        ? ''
        : String(product.calories),
    )
    setImageUrl(product?.image_url || null)
    setAllergens(Array.isArray(product?.allergens) ? [...product.allergens] : [])
    setBadges(badgeRowsOf(product))
    setVisible(product?.is_active !== false)
    setFeatured(Boolean(product?.is_featured))
    setActiveLanguage(primaryLanguage)
    setErrors({})
    setSaving(false)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, product, selectedCategoryId, primaryLanguage, languageKey])

  function updateField(languageCode, field, value) {
    setTranslations((previous) => ({
      ...previous,
      [languageCode]: { ...(previous[languageCode] || EMPTY_TRANSLATION), [field]: value },
    }))
    if (field === 'name' && languageCode === primaryLanguage && value.trim()) {
      setErrors((previous) => ({ ...previous, name: '' }))
    }
  }

  function toggleAllergen(code) {
    setAllergens((previous) =>
      previous.includes(code) ? previous.filter((item) => item !== code) : [...previous, code],
    )
  }

  /* ------------------------------------------------------------- badges */

  function addBadge() {
    setBadges((previous) =>
      previous.length >= MAX_BADGES ? previous : [...previous, newBadgeRow()],
    )
  }

  function updateBadge(uid, patch) {
    setBadges((previous) =>
      previous.map((badge) => (badge.uid === uid ? { ...badge, ...patch } : badge)),
    )
  }

  function removeBadge(uid) {
    setBadges((previous) => previous.filter((badge) => badge.uid !== uid))
  }

  async function save() {
    const found = {}

    if (!categoryId) found.category = 'Ürünün ekleneceği kategoriyi seçin.'

    const primaryName = (translations[primaryLanguage]?.name || '').trim()
    if (!primaryName) found.name = 'Ürün adı zorunludur.'

    if (!isValidPriceText(price)) {
      found.price = 'Geçerli bir fiyat girin. Örn. 145,00'
    } else if (parsePrice(price) < 0) {
      found.price = 'Fiyat sıfırdan küçük olamaz.'
    }

    const hasComparePrice = String(comparePrice).trim() !== ''
    if (hasComparePrice) {
      if (!isValidPriceText(comparePrice)) {
        found.comparePrice = 'Geçerli bir fiyat girin. Örn. 180,00'
      } else if (parsePrice(comparePrice) < 0) {
        found.comparePrice = 'Fiyat sıfırdan küçük olamaz.'
      }
    }

    // Empty means "unknown", which is stored as NULL.
    const caloriesText = String(calories).trim()
    let caloriesValue = null
    if (caloriesText !== '') {
      const parsed = Number(caloriesText)
      if (!Number.isInteger(parsed) || parsed < 0 || parsed > 20000) {
        found.calories = 'Kalori 0 ile 20000 arasında bir tam sayı olmalıdır.'
      } else {
        caloriesValue = parsed
      }
    }

    if (Object.keys(found).length > 0) {
      setErrors(found)
      if (found.name) setActiveLanguage(primaryLanguage)
      return
    }

    // Languages left without a name are not sent.
    const payloadTranslations = {}
    languageList.forEach((code) => {
      const translation = translations[code]
      if (!translation) return
      const name = (translation.name || '').trim()
      if (!name) return
      payloadTranslations[code] = {
        name,
        description: (translation.description || '').trim(),
        ingredients: (translation.ingredients || '').trim(),
      }
    })

    // Rows left without text are dropped; `id` is omitted so the server mints one.
    const payloadBadges = badges
      .map((badge) => {
        const entry = {
          text: (badge.text || '').trim(),
          icon: badge.icon || '',
          bg_color: badge.bg_color,
          text_color: badge.text_color,
        }
        if (badge.id) entry.id = badge.id
        return entry
      })
      .filter((badge) => badge.text !== '')

    const payload = {
      category_id: categoryId,
      translations: payloadTranslations,
      price: parsePrice(price),
      compare_price: hasComparePrice ? parsePrice(comparePrice) : null,
      calories: caloriesValue,
      image_url: imageUrl || null,
      allergens,
      badges: payloadBadges,
      is_active: visible,
      is_featured: featured,
    }

    setSaving(true)
    try {
      const result = product
        ? await api.updateProduct(product.id, payload)
        : await api.createProduct(payload)

      onSaved?.(result)
      toast.success('Ürün kaydedildi.')
      onClose?.()
    } catch (error) {
      toast.error(error.message)
    } finally {
      setSaving(false)
    }
  }

  const activeTranslation = translations[activeLanguage] || EMPTY_TRANSLATION
  const multiLanguage = languageList.length > 1

  return (
    <Modal
      open={open}
      onClose={saving ? () => {} : onClose}
      title={product ? 'Ürünü Düzenle' : 'Yeni Ürün'}
      description="Ürün bilgileri, fiyatı ve alerjen uyarıları."
      width="max-w-2xl"
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
        {/* --------------------------------------------------------- category */}
        <div>
          <label className="label" htmlFor="product-category">
            Kategori <span className="text-red-500">*</span>
          </label>
          <select
            id="product-category"
            className="input"
            value={categoryId}
            onChange={(event) => {
              setCategoryId(event.target.value)
              setErrors((previous) => ({ ...previous, category: '' }))
            }}
          >
            <option value="">Kategori seçin</option>
            {categoryList.map((category) => (
              <option key={category.id} value={category.id}>
                {category.icon ? `${category.icon} ` : ''}
                {categoryName(category, defaultLanguage)}
              </option>
            ))}
          </select>
          {errors.category ? <p className="error-text">{errors.category}</p> : null}
        </div>

        {/* ---------------------------------------------------- language tabs */}
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

        {/* ------------------------------------------- per-language text fields */}
        <div className="space-y-4">
          <div>
            <label className="label" htmlFor={`product-name-${activeLanguage}`}>
              Ürün adı
              {activeLanguage === primaryLanguage ? <span className="text-red-500"> *</span> : null}
            </label>
            <input
              id={`product-name-${activeLanguage}`}
              type="text"
              className="input"
              value={activeTranslation.name}
              onChange={(event) => updateField(activeLanguage, 'name', event.target.value)}
              placeholder={activeLanguage === 'tr' ? 'Örn. Filtre Kahve' : 'Örn. Filter Coffee'}
              maxLength={120}
              autoComplete="off"
            />
            {activeLanguage === primaryLanguage && errors.name ? (
              <p className="error-text">{errors.name}</p>
            ) : activeLanguage !== primaryLanguage ? (
              <p className="help-text">
                {findLanguage(activeLanguage).label} çevirisi boş bırakılırsa bu dil gönderilmez.
              </p>
            ) : null}
          </div>

          <div>
            <label className="label" htmlFor={`product-description-${activeLanguage}`}>
              Açıklama (opsiyonel)
            </label>
            <textarea
              id={`product-description-${activeLanguage}`}
              className="input resize-none"
              rows={2}
              value={activeTranslation.description}
              onChange={(event) => updateField(activeLanguage, 'description', event.target.value)}
              placeholder="Ürünü kısaca tanıtın"
              maxLength={400}
            />
          </div>

          <div>
            <label className="label" htmlFor={`product-ingredients-${activeLanguage}`}>
              İçindekiler (opsiyonel)
            </label>
            <textarea
              id={`product-ingredients-${activeLanguage}`}
              className="input resize-none"
              rows={2}
              value={activeTranslation.ingredients}
              onChange={(event) => updateField(activeLanguage, 'ingredients', event.target.value)}
              placeholder="Örn. espresso, süt, kakao"
              maxLength={400}
            />
          </div>
        </div>

        {/* ------------------------------------------------------------ price */}
        <div className="grid gap-4 sm:grid-cols-2">
          <div>
            <label className="label" htmlFor="product-price">
              Fiyat <span className="text-red-500">*</span>
            </label>
            <div className="relative">
              <input
                id="product-price"
                type="text"
                inputMode="decimal"
                className="input pr-9"
                value={price}
                onChange={(event) => {
                  setPrice(event.target.value)
                  setErrors((previous) => ({ ...previous, price: '' }))
                }}
                onBlur={() => {
                  if (isValidPriceText(price)) setPrice(priceToInput(parsePrice(price)))
                }}
                placeholder="145,00"
                autoComplete="off"
              />
              <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-sm text-gray-500">
                {symbol}
              </span>
            </div>
            {errors.price ? <p className="error-text">{errors.price}</p> : null}
          </div>

          <div>
            <label className="label" htmlFor="product-compare-price">
              Karşılaştırma fiyatı (opsiyonel)
            </label>
            <div className="relative">
              <input
                id="product-compare-price"
                type="text"
                inputMode="decimal"
                className="input pr-9"
                value={comparePrice}
                onChange={(event) => {
                  setComparePrice(event.target.value)
                  setErrors((previous) => ({ ...previous, comparePrice: '' }))
                }}
                onBlur={() => {
                  if (isValidPriceText(comparePrice)) {
                    setComparePrice(priceToInput(parsePrice(comparePrice)))
                  }
                }}
                placeholder="180,00"
                autoComplete="off"
              />
              <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-sm text-gray-500">
                {symbol}
              </span>
            </div>
            {errors.comparePrice ? (
              <p className="error-text">{errors.comparePrice}</p>
            ) : (
              <p className="help-text">Üstü çizili eski fiyat. Boş bırakabilirsiniz.</p>
            )}
          </div>

          {/* Calories share the price grid and fall into the second row. */}
          <div>
            <label className="label" htmlFor="product-calories">
              Kalori (opsiyonel)
            </label>
            <div className="relative">
              <input
                id="product-calories"
                type="number"
                inputMode="numeric"
                min={0}
                max={20000}
                step={1}
                className="input pr-12"
                value={calories}
                onChange={(event) => {
                  setCalories(event.target.value)
                  setErrors((previous) => ({ ...previous, calories: '' }))
                }}
                placeholder="220"
                autoComplete="off"
              />
              <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-sm text-gray-500">
                kcal
              </span>
            </div>
            {errors.calories ? (
              <p className="error-text">{errors.calories}</p>
            ) : (
              <p className="help-text">Boş bırakırsanız menüde gösterilmez.</p>
            )}
          </div>
        </div>

        {/* ----------------------------------------------------------- image */}
        <ImageUploader value={imageUrl} onChange={setImageUrl} label="Ürün görseli (opsiyonel)" />

        {/* -------------------------------------------------------- allergens */}
        <div>
          <span className="label">Alerjen / uyarı etiketleri (opsiyonel)</span>
          <div className="flex flex-wrap gap-2">
            {ALLERGENS.map((allergen) => {
              const isSelected = allergens.includes(allergen.code)
              return (
                <button
                  key={allergen.code}
                  type="button"
                  onClick={() => toggleAllergen(allergen.code)}
                  aria-pressed={isSelected}
                  className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-xs font-medium ${
                    isSelected
                      ? 'border-brand-600 bg-brand-50 text-brand-700'
                      : 'border-gray-300 text-gray-600 hover:bg-gray-50'
                  }`}
                >
                  <span aria-hidden="true">{allergen.emoji}</span>
                  {allergen.tr}
                </button>
              )
            })}
          </div>
          <p className="help-text">Seçtikleriniz ürün kartında küçük rozetler olarak görünür.</p>
        </div>

        {/* ----------------------------------------------------- custom badges */}
        <div className="rounded-xl border border-gray-200 p-4">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="min-w-0">
              <span className="label mb-0.5">Özel rozetler (opsiyonel)</span>
              <p className="text-xs text-gray-500">
                Ürün kartında alerjen simgelerinin yanında görünür. En fazla {MAX_BADGES} rozet
                ekleyebilirsiniz.
              </p>
            </div>

            <button
              type="button"
              onClick={addBadge}
              disabled={badges.length >= MAX_BADGES}
              className="btn-secondary btn-sm shrink-0"
            >
              <Plus className="h-3.5 w-3.5" aria-hidden="true" />
              Rozet ekle
            </button>
          </div>

          {badges.length === 0 ? (
            <p className="mt-3 rounded-lg border border-dashed border-gray-300 px-3 py-4 text-center text-xs text-gray-500">
              Henüz rozet yok. Örn. “Şefin Önerisi”, “Yeni”, “2 Kişilik”.
            </p>
          ) : (
            <div className="mt-4 space-y-3">
              {badges.map((badge, index) => (
                <div key={badge.uid} className="rounded-lg border border-gray-200 bg-gray-50 p-3">
                  {/* text + remove */}
                  <div className="flex items-start gap-2">
                    <div className="min-w-0 flex-1">
                      <label className="sr-only" htmlFor={`badge-text-${badge.uid}`}>
                        {index + 1}. rozet metni
                      </label>
                      <input
                        id={`badge-text-${badge.uid}`}
                        type="text"
                        className="input bg-white py-2"
                        value={badge.text}
                        onChange={(event) => updateBadge(badge.uid, { text: event.target.value })}
                        placeholder="Örn. Şefin Önerisi"
                        maxLength={MAX_BADGE_TEXT}
                        autoComplete="off"
                      />
                    </div>

                    <button
                      type="button"
                      onClick={() => removeBadge(badge.uid)}
                      className="shrink-0 rounded-md p-2 text-gray-400 hover:bg-red-50 hover:text-red-600"
                      aria-label="Rozeti kaldır"
                      title="Rozeti kaldır"
                    >
                      <X className="h-4 w-4" aria-hidden="true" />
                    </button>
                  </div>

                  {/* icon picker */}
                  <div className="mt-3">
                    <span className="mb-1.5 block text-xs font-medium text-gray-600">Simge</span>
                    <div className="grid max-h-28 grid-cols-8 gap-1 overflow-y-auto rounded-lg border border-gray-200 bg-white p-1.5">
                      <button
                        type="button"
                        onClick={() => updateBadge(badge.uid, { icon: '' })}
                        aria-pressed={!badge.icon}
                        aria-label="Simgesiz"
                        title="Simgesiz"
                        className={`flex h-7 w-full items-center justify-center rounded-md ${
                          badge.icon
                            ? 'text-gray-400 hover:bg-gray-100 hover:text-gray-600'
                            : 'bg-brand-50 text-brand-700 ring-2 ring-brand-600'
                        }`}
                      >
                        <Ban className="h-4 w-4" aria-hidden="true" />
                      </button>

                      {BADGE_ICONS.map(({ id, label, Icon }) => {
                        const isSelected = badge.icon === id
                        return (
                          <button
                            key={id}
                            type="button"
                            onClick={() => updateBadge(badge.uid, { icon: id })}
                            aria-pressed={isSelected}
                            aria-label={label}
                            title={label}
                            className={`flex h-7 w-full items-center justify-center rounded-md ${
                              isSelected
                                ? 'bg-brand-50 text-brand-700 ring-2 ring-brand-600'
                                : 'text-gray-500 hover:bg-gray-100 hover:text-gray-700'
                            }`}
                          >
                            <Icon className="h-4 w-4" aria-hidden="true" />
                          </button>
                        )
                      })}
                    </div>
                  </div>

                  {/* colours */}
                  <div className="mt-3">
                    <span className="mb-1.5 block text-xs font-medium text-gray-600">Renk</span>
                    <div className="flex flex-wrap items-center gap-1.5">
                      {BADGE_COLOR_PRESETS.map((preset) => {
                        const isSelected =
                          String(badge.bg_color).toLowerCase() === preset.bg &&
                          String(badge.text_color).toLowerCase() === preset.fg
                        return (
                          <button
                            key={preset.bg}
                            type="button"
                            onClick={() =>
                              updateBadge(badge.uid, {
                                bg_color: preset.bg,
                                text_color: preset.fg,
                              })
                            }
                            aria-pressed={isSelected}
                            aria-label={preset.label}
                            title={preset.label}
                            className={`flex h-7 w-7 items-center justify-center rounded-full border border-black/10 text-[10px] font-semibold ${
                              isSelected ? 'ring-2 ring-brand-600 ring-offset-1' : ''
                            }`}
                            style={{ backgroundColor: preset.bg, color: preset.fg }}
                          >
                            Aa
                          </button>
                        )
                      })}

                      <span className="mx-1 h-5 w-px shrink-0 bg-gray-300" aria-hidden="true" />

                      <label className="inline-flex items-center gap-1 text-[11px] text-gray-600">
                        <input
                          type="color"
                          value={badge.bg_color}
                          onChange={(event) =>
                            updateBadge(badge.uid, { bg_color: event.target.value })
                          }
                          className="h-7 w-7 cursor-pointer rounded border border-gray-300 bg-white p-0.5"
                          aria-label="Rozet zemin rengi"
                        />
                        Zemin
                      </label>

                      <label className="inline-flex items-center gap-1 text-[11px] text-gray-600">
                        <input
                          type="color"
                          value={badge.text_color}
                          onChange={(event) =>
                            updateBadge(badge.uid, { text_color: event.target.value })
                          }
                          className="h-7 w-7 cursor-pointer rounded border border-gray-300 bg-white p-0.5"
                          aria-label="Rozet yazı rengi"
                        />
                        Yazı
                      </label>
                    </div>
                  </div>

                  {/* live preview — same pill the customer card renders */}
                  <div className="mt-3 flex items-center gap-2">
                    <span className="text-[11px] text-gray-500">Önizleme</span>
                    <span
                      className="inline-flex max-w-[9rem] items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium"
                      style={{ backgroundColor: badge.bg_color, color: badge.text_color }}
                    >
                      <BadgeIcon id={badge.icon} className="h-2.5 w-2.5 shrink-0" />
                      <span className="truncate">{badge.text.trim() || 'Rozet'}</span>
                    </span>
                  </div>
                </div>
              ))}

              <p className="help-text">Metni boş bırakılan rozetler kaydedilmez.</p>
            </div>
          )}
        </div>

        {/* ---------------------------------------------------------- toggles */}
        <div className="grid gap-3 sm:grid-cols-2">
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
                Kapatırsanız ürün müşteri menüsünde görünmez.
              </span>
            </span>
          </label>

          <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-gray-200 p-3">
            <input
              type="checkbox"
              className="mt-0.5 h-4 w-4 shrink-0 rounded border-gray-300 text-brand-600 focus:ring-brand-500"
              checked={featured}
              onChange={(event) => setFeatured(event.target.checked)}
            />
            <span className="min-w-0">
              <span className="block text-sm font-medium text-gray-700">Öne çıkar</span>
              <span className="block text-xs text-gray-500">
                Ürün kartında “Öne çıkan” rozeti gösterilir.
              </span>
            </span>
          </label>
        </div>
      </div>
    </Modal>
  )
}
