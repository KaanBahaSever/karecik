// Normalisation of the category records the customer menu renders, and of the
// two free-text fields of the products inside them (see productTexts).
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

/**
 * Parses a space-separated list of hex code points and "first-last" ranges into
 * inclusive [first, last] pairs.
 */
function parseRanges(spec) {
  return spec
    .trim()
    .split(/\s+/)
    .map((part) => {
      const [first, last = first] = part.split('-')
      return [Number.parseInt(first, 16), Number.parseInt(last, 16)]
    })
}

/*
  WHAT COUNTS AS AN EMOJI

  An icon is drawn only when it is made of emoji and nothing else: at least one
  pictograph, plus the pieces emoji sequences are glued from — a zero-width
  joiner, a variation selector, the keycap mark, the tag characters of the
  England, Scotland and Wales flags — and a plain space between two emoji.

  Anything else is not a picture: a legacy icon code such as "coffee", a word in
  any script ("кофе", "咖啡", and "кофе ☕" with an emoji tacked on as well),
  punctuation, a Braille blank (U+2800), a string of invisible characters. A
  category is allowed to have no visual at all, so the honest answer for each of
  those is "no emoji", and the card shows its name alone.

  PICTOGRAPHS are the code points Unicode gives the Emoji property, minus the
  Emoji_Component ones and minus ASCII (whose digits, # and * are emoji only as
  the base of a keycap sequence, so keycaps are not accepted). The list is that
  set as of Unicode 17.0, generated with the \p{Emoji} and \p{Emoji_Component}
  property escapes in Node 26 and pasted in here as data. Regional indicators
  (flags), skin-tone swatches and hair components are Emoji_Component too, but
  they draw on their own, so they are added back at the end.

  It is data rather than a regular expression on purpose: an engine that
  predates property escapes rejects such a pattern while the script is being
  parsed, which takes the whole bundle down with it — and QR menus are opened in
  old in-app browsers all the time.
*/
const PICTOGRAPH_RANGES = parseRanges(`
  a9 ae 203c 2049 2122 2139 2194-2199 21a9-21aa 231a-231b 2328 23cf 23e9-23f3
  23f8-23fa 24c2 25aa-25ab 25b6 25c0 25fb-25fe 2600-2604 260e 2611 2614-2615
  2618 261d 2620 2622-2623 2626 262a 262e-262f 2638-263a 2640 2642 2648-2653
  265f-2660 2663 2665-2666 2668 267b 267e-267f 2692-2697 2699 269b-269c
  26a0-26a1 26a7 26aa-26ab 26b0-26b1 26bd-26be 26c4-26c5 26c8 26ce-26cf 26d1
  26d3-26d4 26e9-26ea 26f0-26f5 26f7-26fa 26fd 2702 2705 2708-270d 270f 2712
  2714 2716 271d 2721 2728 2733-2734 2744 2747 274c 274e 2753-2755 2757
  2763-2764 2795-2797 27a1 27b0 27bf 2934-2935 2b05-2b07 2b1b-2b1c 2b50 2b55
  3030 303d 3297 3299
  1f004 1f0cf 1f170-1f171 1f17e-1f17f 1f18e 1f191-1f19a 1f201-1f202 1f21a 1f22f
  1f232-1f23a 1f250-1f251 1f300-1f321 1f324-1f393 1f396-1f397 1f399-1f39b
  1f39e-1f3f0 1f3f3-1f3f5 1f3f7-1f3fa 1f400-1f4fd 1f4ff-1f53d 1f549-1f54e
  1f550-1f567 1f56f-1f570 1f573-1f57a 1f587 1f58a-1f58d 1f590 1f595-1f596
  1f5a4-1f5a5 1f5a8 1f5b1-1f5b2 1f5bc 1f5c2-1f5c4 1f5d1-1f5d3 1f5dc-1f5de
  1f5e1 1f5e3 1f5e8 1f5ef 1f5f3 1f5fa-1f64f 1f680-1f6c5 1f6cb-1f6d2 1f6d5-1f6d8
  1f6dc-1f6e5 1f6e9 1f6eb-1f6ec 1f6f0 1f6f3-1f6fc 1f7e0-1f7eb 1f7f0 1f90c-1f93a
  1f93c-1f945 1f947-1f9af 1f9b4-1f9ff 1fa70-1fa7c 1fa80-1fa8a 1fa8e-1fac6 1fac8
  1facd-1fadc 1fadf-1faea 1faef-1faf8
  1f1e6-1f1ff 1f3fb-1f3ff 1f9b0-1f9b3
`)

/** What emoji sequences are glued from: space, ZWJ, keycap mark, VS15/VS16, tags. */
const GLUE_RANGES = parseRanges('20 200d 20e3 fe0e fe0f e0020-e007f')

/** Invisible characters that can ride along with a pasted emoji; dropped, not refused. */
const IGNORED_RANGES = parseRanges('200b-200c 2060 feff')

function inRanges(ranges, codePoint) {
  for (let i = 0; i < ranges.length; i += 1) {
    if (codePoint >= ranges[i][0] && codePoint <= ranges[i][1]) return true
  }
  return false
}

/**
 * The category icon when it is an emoji worth drawing, otherwise null — and null
 * is a perfectly good answer: the card then has no visual and shows its name.
 *
 *   '☕'         -> '☕'
 *   '  🍕 '      -> '🍕'
 *   '🇹🇷'        -> '🇹🇷'
 *   'coffee'     -> null   (legacy icon code)
 *   'кофе ☕'    -> null   (a word, emoji or not)
 *   '\u2800'    -> null   (Braille blank: invisible)
 *   '\u200b'    -> null   (nothing but an invisible character)
 *   '   '        -> null
 *   null / 42    -> null
 *
 * @param {unknown} icon
 * @returns {string|null}
 */
export function categoryEmoji(icon) {
  if (typeof icon !== 'string') return null

  const trimmed = icon.trim()
  if (trimmed === '' || trimmed.length > MAX_EMOJI_LENGTH) return null

  let glyph = ''
  let pictographs = 0

  for (let i = 0; i < trimmed.length; i += 1) {
    let codePoint = trimmed.charCodeAt(i)
    let width = 1

    if (codePoint >= 0xd800 && codePoint <= 0xdbff) {
      const low = trimmed.charCodeAt(i + 1)
      // A high surrogate without its low half is not a character at all.
      if (!(low >= 0xdc00 && low <= 0xdfff)) return null
      codePoint = (codePoint - 0xd800) * 0x400 + (low - 0xdc00) + 0x10000
      width = 2
    } else if (codePoint >= 0xdc00 && codePoint <= 0xdfff) {
      return null
    }

    const character = trimmed.slice(i, i + width)
    i += width - 1

    if (inRanges(IGNORED_RANGES, codePoint)) continue
    if (inRanges(PICTOGRAPH_RANGES, codePoint)) {
      pictographs += 1
    } else if (!inRanges(GLUE_RANGES, codePoint)) {
      return null
    }
    glyph += character
  }

  return pictographs > 0 ? glyph.trim() : null
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

/*
  SVG images.

  Logos, category pictures and product photos may be SVG files (image/svg+xml,
  stored under /uploads with a .svg name). The customer menu always draws them
  through <img>, never inlined, put in an <object> or set as innerHTML: an
  <img> renders an SVG as a plain picture with its scripts, event handlers and
  external loads disabled, which is the one mode in which markup an owner
  uploaded is harmless.

  What differs is sizing. A photo has intrinsic pixels; an SVG may have no
  width or height at all — only a viewBox, or not even that — and then the
  browser invents 300 x 150 or collapses it, depending on the engine and the
  layout around it. So an SVG gets an explicit box from its caller, and is
  CONTAINED in that box rather than covering it: a logo or an illustration is
  artwork whose edges matter, not a photograph that can lose a margin.
*/

/**
 * Whether a URL points at an SVG image: a path ending in .svg or .svgz (before
 * any query or fragment), or an image/svg+xml data URL.
 *
 *   '/uploads/1726-ab12cd34.svg'          -> true
 *   'https://cdn.example.com/logo.SVG?v=2' -> true
 *   'data:image/svg+xml;base64,PHN2Zz4='   -> true
 *   '/uploads/photo.png', '/svg/photo.jpg' -> false
 *
 * @param {unknown} url
 * @returns {boolean}
 */
export function isSvgUrl(url) {
  if (typeof url !== 'string') return false
  const trimmed = url.trim()
  if (/^data:image\/svg\+xml[;,]/i.test(trimmed)) return true
  return /\.svgz?$/i.test(trimmed.split(/[?#]/)[0])
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

/* ------------------------------------------------------ product texts */

/*
  A product has two free-text fields that are easy to fill with the same words:
  `description`, the owner's sentence about the dish, and `ingredients`, what
  is in it. Plenty of menus were typed with the ingredient list pasted into
  both, and the detail sheet then printed it twice — once as the paragraph,
  once under "Ingredients".

  "The same" is decided loosely, the way a reader would: surrounding and
  repeated white space, letter case and a closing full stop or comma make no
  difference. Anything more than that — a reworded sentence, a different
  order — is a different text and both are shown.

  The product dialog of the dashboard warns the owner about the same overlap
  with the same rule (lib/productText.js), so a product the dialog calls a
  repeat is exactly one the menu collapses.
*/

/** Punctuation a text may end with without becoming a different text. */
const TRAILING_PUNCTUATION = /[\s.,;:!?…。·،؛؟۔]+$/

/**
 * The form two product texts are compared in: Unicode-normalized, lowercased
 * the Turkish way (so "İ" and "i" match), white space collapsed to single
 * spaces and trimmed, trailing punctuation dropped. Anything but a string
 * gives ''.
 *
 *   ' Espresso,  SÜT.  '  -> 'espresso, süt'
 *
 * @param {unknown} value
 * @returns {string}
 */
export function comparableText(value) {
  if (typeof value !== 'string') return ''

  let text = typeof value.normalize === 'function' ? value.normalize('NFC') : value
  try {
    text = text.toLocaleLowerCase('tr')
  } catch {
    text = text.toLowerCase()
  }
  return text.replace(/\s+/g, ' ').trim().replace(TRAILING_PUNCTUATION, '')
}

/**
 * The form the menu's search compares text in: lowercased the Turkish way, so
 * "İ" is "i", and then with the dotless "ı" folded into "i" as well.
 *
 * Turkish lowercasing alone turns the capital I of "Ice Latte" into "ı", and a
 * visitor who types "ice" — in any of the six languages — finds none of the
 * Ice drinks; lowercasing without Turkish rules breaks "İ" apart instead. With
 * both folded into one letter "ICE", "Ice", "ıce" and "ice" all meet, at the
 * price of "ırmak" also finding "irmak", which no menu search is poorer for.
 * Numbers are searched as their digits; anything else that is not a string
 * gives ''.
 *
 *   'Ice Latte'  -> 'ice latte'
 *   'İÇECEK'     -> 'içecek'
 *   'Irmak'      -> 'irmak'
 *
 * @param {unknown} value
 * @returns {string}
 */
export function searchFold(value) {
  if (typeof value === 'number') return Number.isFinite(value) ? String(value) : ''
  if (typeof value !== 'string') return ''

  let text = typeof value.normalize === 'function' ? value.normalize('NFC') : value
  try {
    text = text.toLocaleLowerCase('tr')
  } catch {
    // Without locale support "İ" lowercases to "i" plus a combining dot.
    text = text.toLowerCase().replace(/i̇/g, 'i')
  }
  return text.replace(/ı/g, 'i')
}

/**
 * What the customer menu prints for a product's two texts.
 *
 *   subtitle     the card's grey line: the description, or the ingredients
 *                when there is no description
 *   description  the detail sheet's paragraph — '' when it only repeats the
 *                ingredients, which the sheet shows under their own heading
 *   ingredients  the detail sheet's "Ingredients" section
 *
 * All three are trimmed strings, '' when absent. Search reads the raw fields,
 * so a word in either one still finds the product.
 *
 * @param {unknown} product
 * @returns {{subtitle: string, description: string, ingredients: string}}
 */
export function productTexts(product) {
  const record = isPlainObject(product) ? product : {}
  const description = typeof record.description === 'string' ? record.description.trim() : ''
  const ingredients = typeof record.ingredients === 'string' ? record.ingredients.trim() : ''

  const repeats =
    description !== '' &&
    ingredients !== '' &&
    comparableText(description) === comparableText(ingredients)

  return {
    subtitle: description || ingredients,
    description: repeats ? '' : description,
    ingredients,
  }
}
