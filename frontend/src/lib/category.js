// Normalisation of the category records the customer menu renders.
//
// Everything a category card draws is optional: the icon, the image, the
// description, even the products. A payload can also be older than the rules the
// dashboard enforces today, or edited by hand, so the renderer must never assume
// that a field is present or of the right type. These helpers turn whatever
// arrived into one shape the components can read without a single guard.
//
// Pure functions only — no React, no side effects.

import { t } from '../locales/index.js'

/** The longest icon, in UTF-16 code units, still treated as a glyph. */
const MAX_EMOJI_LENGTH = 32

/*
  Any ASCII letter or digit marks an icon CODE rather than a glyph. The icon
  column once held codes such as "coffee", and printing one of those at emoji
  size is exactly the bug this guards against. Keycap emoji (1️⃣) start with a
  digit and are rejected too; the dashboard's icon picker offers none of them.

  Unicode property escapes (\p{...}) are deliberately NOT used. An engine that
  predates them rejects the regular expression while the script is being
  parsed, which takes the whole bundle down with it — and QR menus are opened in
  old in-app browsers all the time.
*/
const ASCII_ALPHANUMERIC = /[A-Za-z0-9]/

/**
 * The category icon when it is a glyph worth drawing, otherwise null.
 *
 *   '☕'        -> '☕'
 *   '  🍕 '     -> '🍕'
 *   'coffee'    -> null   (legacy icon code)
 *   '   '       -> null
 *   null / 42   -> null
 *
 * @param {unknown} icon
 * @returns {string|null}
 */
export function categoryEmoji(icon) {
  if (typeof icon !== 'string') return null

  const trimmed = icon.trim()
  if (trimmed === '' || trimmed.length > MAX_EMOJI_LENGTH) return null
  if (ASCII_ALPHANUMERIC.test(trimmed)) return null

  return trimmed
}

/**
 * The category image URL when there is one, otherwise null. Whether the image
 * actually LOADS is only known in the browser; CategoryThumb handles that part.
 *
 * @param {unknown} url
 * @returns {string|null}
 */
export function categoryImageUrl(url) {
  if (typeof url !== 'string') return null

  const trimmed = url.trim()
  return trimmed === '' ? null : trimmed
}

/** True for `{...}` records; false for null, arrays, strings, numbers and the like. */
function isPlainObject(value) {
  return Object.prototype.toString.call(value) === '[object Object]'
}

/** Whether a record carries an id that can identify it. */
function hasUsableId(value) {
  if (typeof value === 'string') return value !== ''
  return typeof value === 'number' && Number.isFinite(value)
}

/**
 * Turns `menu.categories` into a list the customer menu can render as-is.
 *
 * Always returns an array. Entries that are not plain objects are dropped; every
 * other entry keeps all of its original fields and gains:
 *
 *   key       the id when there is one, otherwise a position-based fallback.
 *             It exists for React's `key` only and identifies nothing else.
 *   name      the trimmed name, or the "untitled category" string when empty
 *   description  always a string
 *   emoji     categoryEmoji(icon)
 *   imageUrl  categoryImageUrl(image_url)
 *   products  always an array, holding plain objects only
 *
 * @param {unknown} list
 * @param {string}  language - Picks the wording of the untitled-category name
 * @returns {object[]}
 */
export function normalizeCategories(list, language = 'tr') {
  if (!Array.isArray(list)) return []

  const categories = []

  list.forEach((entry, index) => {
    if (!isPlainObject(entry)) return

    const name = typeof entry.name === 'string' ? entry.name.trim() : ''

    categories.push({
      ...entry,
      key: hasUsableId(entry.id) ? entry.id : `category-index-${index}`,
      name: name || t('untitledCategory', language),
      description: typeof entry.description === 'string' ? entry.description : '',
      emoji: categoryEmoji(entry.icon),
      imageUrl: categoryImageUrl(entry.image_url),
      products: Array.isArray(entry.products) ? entry.products.filter(isPlainObject) : [],
    })
  })

  return categories
}
