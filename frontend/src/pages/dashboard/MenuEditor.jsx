import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
} from '@dnd-kit/core'
import {
  SortableContext,
  arrayMove,
  sortableKeyboardCoordinates,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { LayoutList, Percent, Plus } from 'lucide-react'

import api from '../../lib/api'
import { useAuth } from '../../lib/auth.jsx'
import { findLanguage } from '../../locales/index.js'

import { useToast } from '../../components/ui/Toast.jsx'
import EmptyState from '../../components/ui/EmptyState.jsx'
import ConfirmModal from '../../components/ui/ConfirmModal.jsx'
import Loading from '../../components/ui/Loading.jsx'

import CategoryRow from '../../components/dashboard/CategoryRow.jsx'
import ProductRow from '../../components/dashboard/ProductRow.jsx'
import CategoryModal from '../../components/dashboard/CategoryModal.jsx'
import ProductModal from '../../components/dashboard/ProductModal.jsx'
import BulkPriceModal from '../../components/dashboard/BulkPriceModal.jsx'
import LivePreview from '../../components/dashboard/LivePreview.jsx'

/** Groups products by category_id and sorts each group by position. */
function groupProducts(products) {
  const map = {}
  const list = Array.isArray(products) ? products : []

  list.forEach((product) => {
    const key = String(product.category_id)
    if (!map[key]) map[key] = []
    map[key].push(product)
  })

  Object.keys(map).forEach((key) => {
    map[key].sort((a, b) => (a.position ?? 0) - (b.position ?? 0))
  })

  return map
}

/** Category name in the selected language, falling back to Turkish. */
function getCategoryName(category, language) {
  if (!category) return ''
  return (
    category.translations?.[language]?.name ||
    category.translations?.tr?.name ||
    'İsimsiz kategori'
  )
}

/** Product name in the selected language, falling back to Turkish. */
function getProductName(product, language) {
  if (!product) return ''
  return product.translations?.[language]?.name || product.translations?.tr?.name || 'İsimsiz ürün'
}

export default function MenuEditor() {
  const { business } = useAuth()
  const toast = useToast()

  /* ------------------------------------------------------------ business */

  const languages = useMemo(() => {
    const list = business?.languages
    return Array.isArray(list) && list.length > 0 ? list : ['tr']
  }, [business])

  const defaultLanguage = business?.default_language || languages[0] || 'tr'
  const currency = business?.currency || 'TRY'

  const [language, setLanguage] = useState(defaultLanguage)

  useEffect(() => {
    if (!languages.includes(language)) setLanguage(defaultLanguage)
  }, [languages, language, defaultLanguage])

  /* ---------------------------------------------------------------- data */

  const [loading, setLoading] = useState(true)
  const [categories, setCategories] = useState([])
  const [productsByCategory, setProductsByCategory] = useState({})
  const [expandedIds, setExpandedIds] = useState([])
  const [previewCounter, setPreviewCounter] = useState(0)

  /* -------------------------------------------------------------- modals */

  const [categoryModalOpen, setCategoryModalOpen] = useState(false)
  const [editingCategory, setEditingCategory] = useState(null)

  const [productModalOpen, setProductModalOpen] = useState(false)
  const [editingProduct, setEditingProduct] = useState(null)
  const [targetCategoryId, setTargetCategoryId] = useState(null)

  const [bulkPriceOpen, setBulkPriceOpen] = useState(false)

  const [categoryToDelete, setCategoryToDelete] = useState(null)
  const [productToDelete, setProductToDelete] = useState(null)
  const [deleting, setDeleting] = useState(false)

  const refreshPreview = useCallback(() => {
    setPreviewCounter((previous) => previous + 1)
  }, [])

  /* ------------------------------------------------------------- loading */

  const loadData = useCallback(async () => {
    setLoading(true)
    try {
      const [incomingCategories, incomingProducts] = await Promise.all([
        api.listCategories(),
        api.listProducts(),
      ])

      const list = Array.isArray(incomingCategories) ? incomingCategories : []
      setCategories(list)
      setProductsByCategory(groupProducts(incomingProducts))
      setExpandedIds((previous) =>
        previous.length > 0 ? previous : list.slice(0, 1).map((category) => category.id),
      )
    } catch (err) {
      toast.error(err.message)
    } finally {
      setLoading(false)
    }
  }, [toast])

  useEffect(() => {
    loadData()
  }, [loadData])

  const getProducts = useCallback(
    (categoryId) => productsByCategory[String(categoryId)] || [],
    [productsByCategory],
  )

  const findProduct = useCallback(
    (productId) => {
      const groups = Object.values(productsByCategory)
      for (let i = 0; i < groups.length; i += 1) {
        const found = groups[i].find((product) => product.id === productId)
        if (found) return found
      }
      return null
    },
    [productsByCategory],
  )

  /** Patches a single product locally (optimistic update). */
  const patchProductLocally = useCallback((productId, changes) => {
    setProductsByCategory((previous) => {
      const next = {}
      Object.keys(previous).forEach((key) => {
        next[key] = previous[key].map((product) =>
          product.id === productId ? { ...product, ...changes } : product,
        )
      })
      return next
    })
  }, [])

  /* ------------------------------------------------------ drag and drop */

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  /** Category order changed: local state first, then the server. */
  async function onCategoryDragEnd(event) {
    const { active, over } = event
    if (!over || active.id === over.id) return

    const previousOrder = categories
    const fromIndex = categories.findIndex((category) => category.id === active.id)
    const toIndex = categories.findIndex((category) => category.id === over.id)
    if (fromIndex < 0 || toIndex < 0) return

    const nextOrder = arrayMove(categories, fromIndex, toIndex)
    setCategories(nextOrder)
    refreshPreview()

    try {
      await api.reorderCategories(nextOrder.map((category) => category.id))
    } catch (err) {
      setCategories(previousOrder)
      refreshPreview()
      toast.error(err.message)
    }
  }

  /** Product order changed inside one category. */
  async function onProductDragEnd(categoryId, event) {
    const { active, over } = event
    if (!over || active.id === over.id) return

    const key = String(categoryId)
    const previousList = productsByCategory[key] || []
    const fromIndex = previousList.findIndex((product) => product.id === active.id)
    const toIndex = previousList.findIndex((product) => product.id === over.id)
    if (fromIndex < 0 || toIndex < 0) return

    const nextList = arrayMove(previousList, fromIndex, toIndex)
    setProductsByCategory((previous) => ({ ...previous, [key]: nextList }))
    refreshPreview()

    try {
      await api.reorderProducts(
        categoryId,
        nextList.map((product) => product.id),
      )
    } catch (err) {
      setProductsByCategory((previous) => ({ ...previous, [key]: previousList }))
      refreshPreview()
      toast.error(err.message)
    }
  }

  /* ----------------------------------------------------------- row actions */

  function toggleCategory(categoryId) {
    setExpandedIds((previous) =>
      previous.includes(categoryId)
        ? previous.filter((id) => id !== categoryId)
        : [...previous, categoryId],
    )
  }

  function openCreateCategory() {
    setEditingCategory(null)
    setCategoryModalOpen(true)
  }

  function openEditCategory(category) {
    setEditingCategory(category)
    setCategoryModalOpen(true)
  }

  function openCreateProduct(categoryId) {
    setEditingProduct(null)
    setTargetCategoryId(categoryId)
    setProductModalOpen(true)
  }

  function openEditProduct(product) {
    setEditingProduct(product)
    setTargetCategoryId(product.category_id)
    setProductModalOpen(true)
  }

  function onCategorySaved(category) {
    setCategoryModalOpen(false)
    setEditingCategory(null)

    if (!category || !category.id) {
      loadData()
      refreshPreview()
      return
    }

    setCategories((previous) => {
      const exists = previous.some((item) => item.id === category.id)
      return exists
        ? previous.map((item) => (item.id === category.id ? category : item))
        : [...previous, category]
    })
    setExpandedIds((previous) =>
      previous.includes(category.id) ? previous : [...previous, category.id],
    )
    refreshPreview()
  }

  function onProductSaved(product) {
    setProductModalOpen(false)
    setEditingProduct(null)

    if (!product || !product.id) {
      loadData()
      refreshPreview()
      return
    }

    setProductsByCategory((previous) => {
      const next = {}
      // The product may have moved to another category: remove it everywhere first.
      Object.keys(previous).forEach((key) => {
        next[key] = previous[key].filter((item) => item.id !== product.id)
      })

      const key = String(product.category_id)
      const list = [...(next[key] || []), product]
      list.sort((a, b) => (a.position ?? 0) - (b.position ?? 0))
      next[key] = list
      return next
    })

    setExpandedIds((previous) =>
      previous.includes(product.category_id) ? previous : [...previous, product.category_id],
    )
    refreshPreview()
  }

  /** Inline quick price editing. */
  async function onProductPriceChange(productId, nextPrice) {
    const current = findProduct(productId)
    const previousPrice = current ? current.price : 0

    patchProductLocally(productId, { price: nextPrice })
    refreshPreview()

    try {
      const updated = await api.updateProductPrice(productId, nextPrice)
      if (updated && updated.id) patchProductLocally(productId, updated)
      toast.success('Fiyat güncellendi.')
    } catch (err) {
      patchProductLocally(productId, { price: previousPrice })
      refreshPreview()
      toast.error(err.message)
    }
  }

  /** Show / hide a product from the menu via the eye icon. */
  async function onProductActiveChange(productId, nextState) {
    patchProductLocally(productId, { is_active: nextState })
    refreshPreview()

    try {
      const updated = await api.updateProduct(productId, { is_active: nextState })
      if (updated && updated.id) patchProductLocally(productId, updated)
    } catch (err) {
      patchProductLocally(productId, { is_active: !nextState })
      refreshPreview()
      toast.error(err.message)
    }
  }

  async function confirmCategoryDelete() {
    if (!categoryToDelete) return
    const target = categoryToDelete
    setDeleting(true)

    try {
      await api.deleteCategory(target.id)

      setCategories((previous) => previous.filter((category) => category.id !== target.id))
      setProductsByCategory((previous) => {
        const next = { ...previous }
        delete next[String(target.id)]
        return next
      })
      setExpandedIds((previous) => previous.filter((id) => id !== target.id))

      setCategoryToDelete(null)
      refreshPreview()
      toast.success('Kategori silindi.')
    } catch (err) {
      toast.error(err.message)
    } finally {
      setDeleting(false)
    }
  }

  async function confirmProductDelete() {
    if (!productToDelete) return
    const target = productToDelete
    setDeleting(true)

    try {
      await api.deleteProduct(target.id)

      setProductsByCategory((previous) => {
        const next = {}
        Object.keys(previous).forEach((key) => {
          next[key] = previous[key].filter((product) => product.id !== target.id)
        })
        return next
      })

      setProductToDelete(null)
      refreshPreview()
      toast.success('Ürün silindi.')
    } catch (err) {
      toast.error(err.message)
    } finally {
      setDeleting(false)
    }
  }

  function onBulkPriceApplied() {
    setBulkPriceOpen(false)
    loadData()
    refreshPreview()
  }

  /* --------------------------------------------------------------- render */

  return (
    <div>
      {/* --------------------------------------------------------- header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <h1 className="text-xl font-semibold text-gray-900">Menü Yönetimi</h1>
          <p className="mt-1 text-sm text-gray-500">
            Kategorileri ve ürünleri sürükleyerek sıralayabilirsiniz.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <button type="button" className="btn-secondary" onClick={() => setBulkPriceOpen(true)}>
            <Percent className="h-4 w-4" aria-hidden="true" />
            Toplu Fiyat Güncelle
          </button>

          <button type="button" className="btn-primary" onClick={openCreateCategory}>
            <Plus className="h-4 w-4" aria-hidden="true" />
            Kategori Ekle
          </button>
        </div>
      </div>

      {/* ------------------------------------------------- language picker */}
      {languages.length > 1 ? (
        <div className="mt-4 inline-flex flex-wrap gap-1 rounded-lg border border-gray-200 bg-gray-50 p-1">
          {languages.map((code) => {
            const info = findLanguage(code)
            const isSelected = code === language
            return (
              <button
                key={code}
                type="button"
                onClick={() => setLanguage(code)}
                aria-pressed={isSelected}
                className={`rounded-md px-3 py-1.5 text-xs font-medium ${
                  isSelected
                    ? 'bg-white text-gray-900 shadow-card'
                    : 'text-gray-500 hover:text-gray-700'
                }`}
              >
                <span className="mr-1" aria-hidden="true">
                  {info.short}
                </span>
                {info.label}
              </button>
            )
          })}
        </div>
      ) : null}

      {/* ------------------------------------------------------ main layout */}
      <div className="mt-6 grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
        {/* ------------------------------------------------------- editor */}
        <div className="min-w-0">
          {loading ? (
            <Loading text="Menü yükleniyor..." />
          ) : categories.length === 0 ? (
            <EmptyState
              icon={LayoutList}
              title="Henüz kategori eklemediniz"
              description="Menünüzü oluşturmak için ilk kategoriyi ekleyin. Örn: Sıcak İçecekler, Tatlılar"
              action={
                <button type="button" className="btn-primary" onClick={openCreateCategory}>
                  <Plus className="h-4 w-4" aria-hidden="true" />
                  Kategori Ekle
                </button>
              }
            />
          ) : (
            /* 1st DndContext — category ordering only */
            <DndContext
              sensors={sensors}
              collisionDetection={closestCenter}
              onDragEnd={onCategoryDragEnd}
            >
              <SortableContext
                items={categories.map((category) => category.id)}
                strategy={verticalListSortingStrategy}
              >
                <div className="flex flex-col gap-3">
                  {categories.map((category) => {
                    const products = getProducts(category.id)
                    const expanded = expandedIds.includes(category.id)

                    return (
                      <CategoryRow
                        key={category.id}
                        category={category}
                        products={products}
                        open={expanded}
                        onToggle={() => toggleCategory(category.id)}
                        language={language}
                        onEdit={() => openEditCategory(category)}
                        onDelete={() => setCategoryToDelete(category)}
                        onAddProduct={() => openCreateProduct(category.id)}
                      >
                        <div className="border-t border-gray-100 bg-gray-50/70 px-2 py-3 sm:px-3">
                          {products.length === 0 ? (
                            <p className="py-3 text-center text-sm text-gray-500">
                              Bu kategoride henüz ürün yok.
                            </p>
                          ) : (
                            /* 2nd DndContext — products of this category only */
                            <DndContext
                              sensors={sensors}
                              collisionDetection={closestCenter}
                              onDragEnd={(event) => onProductDragEnd(category.id, event)}
                            >
                              <SortableContext
                                items={products.map((product) => product.id)}
                                strategy={verticalListSortingStrategy}
                              >
                                <div className="flex flex-col gap-2">
                                  {products.map((product) => (
                                    <ProductRow
                                      key={product.id}
                                      product={product}
                                      language={language}
                                      currency={currency}
                                      onEdit={() => openEditProduct(product)}
                                      onDelete={() => setProductToDelete(product)}
                                      onPriceChange={onProductPriceChange}
                                      onActiveChange={onProductActiveChange}
                                    />
                                  ))}
                                </div>
                              </SortableContext>
                            </DndContext>
                          )}

                          <button
                            type="button"
                            className="btn-secondary btn-sm mt-3 w-full"
                            onClick={() => openCreateProduct(category.id)}
                          >
                            <Plus className="h-3.5 w-3.5" aria-hidden="true" />
                            Ürün Ekle
                          </button>
                        </div>
                      </CategoryRow>
                    )
                  })}
                </div>
              </SortableContext>
            </DndContext>
          )}
        </div>

        {/* ------------------------------------------------------ live preview */}
        <div className="hidden xl:block">
          <div className="sticky top-6">
            <LivePreview business={business} refresh={previewCounter} />
          </div>
        </div>
      </div>

      {/* ---------------------------------------------------------- modals */}
      <CategoryModal
        open={categoryModalOpen}
        onClose={() => {
          setCategoryModalOpen(false)
          setEditingCategory(null)
        }}
        category={editingCategory}
        languages={languages}
        defaultLanguage={defaultLanguage}
        onSaved={onCategorySaved}
      />

      <ProductModal
        open={productModalOpen}
        onClose={() => {
          setProductModalOpen(false)
          setEditingProduct(null)
        }}
        product={editingProduct}
        categories={categories}
        selectedCategoryId={targetCategoryId}
        languages={languages}
        defaultLanguage={defaultLanguage}
        currency={currency}
        onSaved={onProductSaved}
      />

      <BulkPriceModal
        open={bulkPriceOpen}
        onClose={() => setBulkPriceOpen(false)}
        categories={categories}
        currency={currency}
        onApplied={onBulkPriceApplied}
      />

      <ConfirmModal
        open={Boolean(categoryToDelete)}
        onClose={() => setCategoryToDelete(null)}
        onConfirm={confirmCategoryDelete}
        title="Kategoriyi sil"
        message={
          categoryToDelete
            ? `"${getCategoryName(categoryToDelete, language)}" kategorisi ve içindeki ${
                getProducts(categoryToDelete.id).length
              } ürün kalıcı olarak silinecek.`
            : ''
        }
        confirmText="Sil"
        busy={deleting}
      />

      <ConfirmModal
        open={Boolean(productToDelete)}
        onClose={() => setProductToDelete(null)}
        onConfirm={confirmProductDelete}
        title="Ürünü sil"
        message={
          productToDelete
            ? `"${getProductName(productToDelete, language)}" ürünü kalıcı olarak silinecek.`
            : ''
        }
        confirmText="Sil"
        busy={deleting}
      />
    </div>
  )
}
