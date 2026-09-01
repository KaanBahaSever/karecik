import { useEffect, useMemo, useState } from 'react'
import { ArrowLeft, Search, Star } from 'lucide-react'

import { formatPrice } from '../../lib/format'
import { themeVariables } from '../../themes/themes'
import { fontStack, loadFont } from '../../themes/fonts'
import { findAllergen, findLanguage, isRtl, t } from '../../locales/index.js'
import ProductDetailModal from './ProductDetailModal.jsx'
import MenuFooter from './MenuFooter.jsx'

/**
 * The main screen of the customer menu.
 *
 * Two components render it:
 *   1. pages/menu/CustomerMenu.jsx            — the real menu behind a QR code
 *   2. components/dashboard/LivePreview.jsx   — the dashboard live preview
 * The prop signature is therefore FIXED; do not change it.
 *
 * Colours come from theme-derived CSS custom properties rather than Tailwind
 * classes, so all six themes work with a single component tree.
 *
 * @param {object}   menu             - { business, categories, footer }
 * @param {string}   language         - Active language code
 * @param {Function} onLanguageChange - Called when the language changes
 * @param {boolean}  embedded         - Compact rendering for narrow containers
 */

/** Picks a readable text colour to sit on top of the accent colour. */
function readableTextColor(hex) {
  const clean = String(hex || '').trim().replace('#', '')
  let r
  let g
  let b

  if (clean.length === 3) {
    r = Number.parseInt(clean[0] + clean[0], 16)
    g = Number.parseInt(clean[1] + clean[1], 16)
    b = Number.parseInt(clean[2] + clean[2], 16)
  } else if (clean.length === 6) {
    r = Number.parseInt(clean.slice(0, 2), 16)
    g = Number.parseInt(clean.slice(2, 4), 16)
    b = Number.parseInt(clean.slice(4, 6), 16)
  } else {
    return '#ffffff'
  }

  if ([r, g, b].some(Number.isNaN)) return '#ffffff'

  const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255
  return luminance > 0.6 ? '#111827' : '#ffffff'
}

/** Turkish-aware lowercasing, used for search. */
function lower(value) {
  return String(value || '').toLocaleLowerCase('tr')
}

/** Clamp to two lines without needing the Tailwind line-clamp plugin. */
const TWO_LINES = {
  display: '-webkit-box',
  WebkitLineClamp: 2,
  WebkitBoxOrient: 'vertical',
  overflow: 'hidden',
}

export default function MenuContent({
  menu,
  language = 'tr',
  onLanguageChange,
  embedded = false,
}) {
  const business = menu?.business || {}
  const categories = useMemo(() => menu?.categories || [], [menu])

  const [selectedCategoryId, setSelectedCategoryId] = useState(null)
  const [search, setSearch] = useState('')
  const [selectedProduct, setSelectedProduct] = useState(null)

  // Load the selected font
  useEffect(() => {
    if (business.font_family) loadFont(business.font_family)
  }, [business.font_family])

  // Drop a stale selection when the menu changes (e.g. after a language switch)
  useEffect(() => {
    if (selectedCategoryId && !categories.some((category) => category.id === selectedCategoryId)) {
      setSelectedCategoryId(null)
    }
  }, [categories, selectedCategoryId])

  const style = themeVariables(
    business.theme,
    business.primary_color,
    fontStack(business.font_family),
  )

  const onAccentText = readableTextColor(business.primary_color)
  const languages = Array.isArray(business.languages) ? business.languages : []
  const searchTerm = search.trim()
  const searching = searchTerm.length > 0

  const productCountLabel = (count) => (language === 'tr' ? `${count} ürün` : `${count} items`)

  /* ----------------------------------------------------------------- search */

  const searchResults = useMemo(() => {
    if (!searching) return []
    const needle = lower(searchTerm)

    return categories.flatMap((category) =>
      (category.products || [])
        .filter((product) =>
          [product.name, product.description, product.ingredients].some((field) =>
            lower(field).includes(needle),
          ),
        )
        .map((product) => ({ product, categoryName: category.name })),
    )
  }, [searching, searchTerm, categories])

  const selectedCategory = categories.find((category) => category.id === selectedCategoryId) || null

  /* ------------------------------------------------------------- fragments */

  function ProductRow({ product, categoryName }) {
    const allergens = Array.isArray(product.allergens) ? product.allergens : []

    return (
      <button
        type="button"
        onClick={() => setSelectedProduct(product)}
        className="flex w-full items-start gap-3 p-3 text-left"
        style={{
          backgroundColor: 'var(--menu-surface)',
          borderRadius: 'var(--menu-radius)',
          border: '1px solid var(--menu-border)',
          boxShadow: 'var(--menu-shadow)',
          opacity: product.is_active === false ? 0.5 : 1,
        }}
      >
        {product.image_url ? (
          <img
            src={product.image_url}
            alt=""
            className="h-[72px] w-[72px] shrink-0 object-cover"
            style={{ borderRadius: 'calc(var(--menu-radius) * 0.7)' }}
          />
        ) : null}

        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="font-medium leading-snug" style={{ color: 'var(--menu-text)' }}>
                {product.name}
              </p>
              {categoryName ? (
                <p className="mt-0.5 text-[11px]" style={{ color: 'var(--menu-muted)' }}>
                  {categoryName}
                </p>
              ) : null}
            </div>

            <div className="shrink-0 text-right">
              {product.compare_price ? (
                <p className="text-[11px] line-through" style={{ color: 'var(--menu-muted)' }}>
                  {formatPrice(product.compare_price, business.currency)}
                </p>
              ) : null}
              <p className="font-semibold" style={{ color: 'var(--menu-primary)' }}>
                {formatPrice(product.price, business.currency)}
              </p>
            </div>
          </div>

          {product.description ? (
            <p
              className="mt-1 text-xs leading-relaxed"
              style={{ ...TWO_LINES, color: 'var(--menu-muted)' }}
            >
              {product.description}
            </p>
          ) : null}

          {(allergens.length > 0 || product.is_featured || product.is_active === false) && (
            <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
              {product.is_featured ? (
                <span
                  className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium"
                  style={{
                    backgroundColor: 'var(--menu-primary)',
                    color: onAccentText,
                  }}
                >
                  <Star className="h-2.5 w-2.5" aria-hidden="true" />
                  {t('featured', language)}
                </span>
              ) : null}

              {product.is_active === false ? (
                <span
                  className="rounded-full px-2 py-0.5 text-[10px] font-medium"
                  style={{ border: '1px solid var(--menu-border)', color: 'var(--menu-muted)' }}
                >
                  Gizli
                </span>
              ) : null}

              {allergens.map((code) => {
                const allergen = findAllergen(code)
                if (!allergen) return null
                return (
                  <span
                    key={code}
                    title={language === 'tr' ? allergen.tr : allergen.en}
                    className="text-xs"
                  >
                    {allergen.emoji}
                  </span>
                )
              })}
            </div>
          )}
        </div>
      </button>
    )
  }

  function CategoryStrip() {
    return (
      <div className="no-scrollbar -mx-4 mb-4 flex gap-2 overflow-x-auto px-4 pb-1">
        {categories.map((category) => {
          const isSelected = category.id === selectedCategoryId
          return (
            <button
              key={category.id}
              type="button"
              onClick={() => setSelectedCategoryId(category.id)}
              className="shrink-0 whitespace-nowrap rounded-full px-3.5 py-1.5 text-sm"
              style={
                isSelected
                  ? { backgroundColor: 'var(--menu-primary)', color: onAccentText }
                  : {
                      backgroundColor: 'var(--menu-surface)',
                      color: 'var(--menu-text)',
                      border: '1px solid var(--menu-border)',
                    }
              }
            >
              {category.icon ? `${category.icon} ` : ''}
              {category.name}
            </button>
          )
        })}
      </div>
    )
  }

  function EmptyLine({ text }) {
    return (
      <p className="py-10 text-center text-sm" style={{ color: 'var(--menu-muted)' }}>
        {text}
      </p>
    )
  }

  /* -------------------------------------------------------------- render */

  return (
    <div
      className={embedded ? 'min-h-full w-full' : 'min-h-screen w-full'}
      style={style}
      dir={isRtl(language) ? 'rtl' : 'ltr'}
    >
      <div className="mx-auto max-w-lg px-4 py-5">
        {/* ------------------------------------ header: logo in the TOP LEFT */}
        <header className="flex items-start justify-between gap-3">
          <div className="flex min-w-0 items-center gap-3">
            {business.logo_url ? (
              <img
                src={business.logo_url}
                alt=""
                className="h-12 w-12 shrink-0 object-contain"
                style={{ borderRadius: 'calc(var(--menu-radius) * 0.6)' }}
              />
            ) : (
              <div
                className="flex h-12 w-12 shrink-0 items-center justify-center text-lg font-semibold"
                style={{
                  backgroundColor: 'var(--menu-primary)',
                  color: onAccentText,
                  borderRadius: 'calc(var(--menu-radius) * 0.6)',
                }}
                aria-hidden="true"
              >
                {String(business.name || '•').charAt(0).toLocaleUpperCase('tr')}
              </div>
            )}

            <div className="min-w-0">
              <h1
                className="truncate text-lg font-semibold leading-tight"
                style={{ color: 'var(--menu-text)' }}
              >
                {business.name}
              </h1>
              {business.address ? (
                <p className="truncate text-xs" style={{ color: 'var(--menu-muted)' }}>
                  {business.address}
                </p>
              ) : null}
            </div>
          </div>

          {languages.length > 1 ? (
            <div className="flex shrink-0 items-center gap-1">
              {languages.map((code) => {
                const info = findLanguage(code)
                const isSelected = code === language
                return (
                  <button
                    key={code}
                    type="button"
                    onClick={() => onLanguageChange?.(code)}
                    aria-label={info.label}
                    title={info.label}
                    className="rounded-full px-2 py-1 text-xs font-semibold leading-none"
                    style={{
                      opacity: isSelected ? 1 : 0.45,
                      border: isSelected
                        ? '1px solid var(--menu-border)'
                        : '1px solid transparent',
                    }}
                  >
                    {/* Short code instead of a flag emoji: Windows cannot draw
                        flags and rendered English as "GB". */}
                    {info.short}
                  </button>
                )
              })}
            </div>
          ) : null}
        </header>

        {/* --------------------------------------------------------- search */}
        {categories.length > 0 ? (
          <div className="relative mt-4">
            <Search
              className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2"
              style={{ color: 'var(--menu-muted)' }}
              aria-hidden="true"
            />
            <input
              type="search"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={t('search', language)}
              className="w-full py-2.5 pl-9 pr-3 text-sm outline-none"
              style={{
                backgroundColor: 'var(--menu-surface)',
                color: 'var(--menu-text)',
                border: '1px solid var(--menu-border)',
                borderRadius: 'var(--menu-radius)',
              }}
            />
          </div>
        ) : null}

        <div className="mt-4">
          {/* --------------------------------------------- 1) search results */}
          {searching ? (
            searchResults.length === 0 ? (
              <EmptyLine text={t('noResults', language)} />
            ) : (
              <div className="flex flex-col gap-2.5">
                {searchResults.map(({ product, categoryName }) => (
                  <ProductRow key={product.id} product={product} categoryName={categoryName} />
                ))}
              </div>
            )
          ) : selectedCategory ? (
            /* ------------------------------------------- 2) product listing */
            <>
              <div className="mb-3 flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => setSelectedCategoryId(null)}
                  aria-label={language === 'tr' ? 'Kategorilere dön' : 'Back to categories'}
                  className="flex h-8 w-8 shrink-0 items-center justify-center"
                  style={{
                    backgroundColor: 'var(--menu-surface)',
                    border: '1px solid var(--menu-border)',
                    borderRadius: 'calc(var(--menu-radius) * 0.7)',
                    color: 'var(--menu-text)',
                  }}
                >
                  <ArrowLeft
                    className="h-4 w-4"
                    style={{ transform: isRtl(language) ? 'scaleX(-1)' : 'none' }}
                  />
                </button>

                <h2
                  className="truncate text-base font-semibold"
                  style={{ color: 'var(--menu-text)' }}
                >
                  {selectedCategory.icon ? `${selectedCategory.icon} ` : ''}
                  {selectedCategory.name}
                </h2>
              </div>

              <CategoryStrip />

              {selectedCategory.description ? (
                <p className="mb-3 text-xs" style={{ color: 'var(--menu-muted)' }}>
                  {selectedCategory.description}
                </p>
              ) : null}

              {(selectedCategory.products || []).length === 0 ? (
                <EmptyLine text={t('emptyCategory', language)} />
              ) : (
                <div className="flex flex-col gap-2.5">
                  {selectedCategory.products.map((product) => (
                    <ProductRow key={product.id} product={product} />
                  ))}
                </div>
              )}
            </>
          ) : categories.length === 0 ? (
            /* ------------------------------------------------ 3) empty menu */
            <EmptyLine text={t('emptyMenu', language)} />
          ) : (
            /* --------------------------------------------- 4) category grid */
            <div className="grid grid-cols-2 gap-3">
              {categories.map((category) => (
                <button
                  key={category.id}
                  type="button"
                  onClick={() => setSelectedCategoryId(category.id)}
                  className="flex flex-col overflow-hidden text-left"
                  style={{
                    backgroundColor: 'var(--menu-surface)',
                    border: '1px solid var(--menu-border)',
                    borderRadius: 'var(--menu-radius)',
                    boxShadow: 'var(--menu-shadow)',
                    opacity: category.is_active === false ? 0.5 : 1,
                  }}
                >
                  {category.image_url ? (
                    <img
                      src={category.image_url}
                      alt=""
                      className={embedded ? 'h-20 w-full object-cover' : 'h-24 w-full object-cover'}
                    />
                  ) : (
                    <div
                      className={`flex w-full items-center justify-center text-4xl ${
                        embedded ? 'h-20' : 'h-24'
                      }`}
                    >
                      {category.icon || '🍽️'}
                    </div>
                  )}

                  <div className="px-3 py-2.5">
                    <p
                      className="truncate text-sm font-medium"
                      style={{ color: 'var(--menu-text)' }}
                    >
                      {category.name}
                    </p>
                    <p className="mt-0.5 text-[11px]" style={{ color: 'var(--menu-muted)' }}>
                      {productCountLabel((category.products || []).length)}
                    </p>
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>

        {/*
          On the home view only "Karecik ile hazırlandı" is shown. The price
          date, the VAT notice and the contact details live on the screens that
          list products.
        */}
        <MenuFooter
          business={business}
          footer={menu?.footer}
          language={language}
          scope={searching || selectedCategory ? 'products' : 'home'}
        />
      </div>

      <ProductDetailModal
        product={selectedProduct}
        business={business}
        language={language}
        onClose={() => setSelectedProduct(null)}
      />
    </div>
  )
}
