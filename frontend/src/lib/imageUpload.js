// The rules a picked image must pass before the panel uploads it, shared by
// every image field of the panel through components/ui/ImageUploader.jsx: the
// menu logo, the cover, the background, the splash logo, the Yerli Üretim
// logo, and the category and product images.
//
// They mirror POST /api/uploads (backend/internal/handlers/upload.go) so a file
// the server would refuse is refused here first, with a message that says why,
// instead of after a wasted upload:
//
//   - the type: the browser's MIME type when it is one of IMAGE_MIME_TYPES,
//     otherwise the file's extension - the server does the same, because some
//     browsers send an SVG with an empty or generic type;
//   - the size: MAX_UPLOAD_BYTES, the server's default MAX_UPLOAD_BYTES (5 MB).
//     A deployment that raises it still gets this limit in the panel - the
//     stricter of the two wins, which is the safe direction.
//
// What is NOT checked here is the inside of an SVG. The server reads every SVG
// back and refuses one carrying a script, an event handler or a javascript:
// address; its 422 message reaches the owner through the uploader's toast.
// The panel and the customer menu only ever draw an upload through <img>,
// where an SVG cannot run anything anyway.
//
// Pure functions only - no React, no DOM - so frontend/tests/
// dashboardSettings.test.mjs runs them with plain node.
//
// NOTE: the messages are Turkish on purpose - they are shown to the owner.

/** The server's default upload limit (config MAX_UPLOAD_BYTES). */
export const MAX_UPLOAD_BYTES = 5 * 1024 * 1024

/**
 * The content types the server stores (allowedImageTypes); image/jpg is a
 * non-standard alias some systems send.
 */
export const IMAGE_MIME_TYPES = [
  'image/jpeg',
  'image/jpg',
  'image/png',
  'image/webp',
  'image/gif',
  'image/svg+xml',
]

/** The extensions accepted when the type says nothing useful. */
export const IMAGE_EXTENSIONS = ['.jpg', '.jpeg', '.png', '.webp', '.gif', '.svg']

/**
 * The file input's `accept`. The extensions are listed beside the types
 * because some system file pickers do not map .svg to image/svg+xml and would
 * grey the file out otherwise.
 */
export const IMAGE_ACCEPT = [
  ...IMAGE_MIME_TYPES.filter((type) => type !== 'image/jpg'),
  ...IMAGE_EXTENSIONS,
].join(',')

/** The line every image field prints under its buttons. */
export const IMAGE_FORMATS_HINT = 'JPG, PNG, WEBP, GIF veya SVG · en fazla 5 MB'

export const UPLOAD_MESSAGES = {
  type: 'Bu dosya türü desteklenmiyor. JPG, PNG, WEBP, GIF veya SVG formatında bir görsel seçin.',
  empty: 'Seçilen dosya boş görünüyor. Lütfen başka bir dosya seçin.',
  tooLarge: (size, limit) =>
    `Dosya çok büyük (${formatMegabytes(size)}). En fazla ${formatMegabytes(limit)} yükleyebilirsiniz.`,
}

/** 5242880 -> "5 MB", 7549747 -> "7,2 MB": one decimal only when it says something. */
export function formatMegabytes(bytes) {
  const megabytes = Math.max(0, Number(bytes) || 0) / (1024 * 1024)
  const rounded = Math.round(megabytes * 10) / 10
  const text = Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(1).replace('.', ',')
  return `${text} MB`
}

/** "Logo.SVG" -> ".svg"; '' when there is no extension. */
export function fileExtension(name) {
  const text = typeof name === 'string' ? name.trim().toLowerCase() : ''
  const dot = text.lastIndexOf('.')
  return dot > 0 && dot < text.length - 1 ? text.slice(dot) : ''
}

/** "image/SVG+XML; charset=utf-8" -> "image/svg+xml". */
function baseType(type) {
  return typeof type === 'string' ? type.split(';')[0].trim().toLowerCase() : ''
}

/** Whether a picked file is one of the image formats the server stores. */
export function isAcceptedImage(file) {
  if (!file || typeof file !== 'object') return false
  return (
    IMAGE_MIME_TYPES.includes(baseType(file.type)) ||
    IMAGE_EXTENSIONS.includes(fileExtension(file.name))
  )
}

/**
 * Why a picked file cannot be uploaded, or '' when it can.
 *
 * @param {{ name?: string, type?: string, size?: number }} file - a File, or anything shaped like one
 * @param {number} maxBytes
 * @returns {string}
 */
export function imageUploadProblem(file, maxBytes = MAX_UPLOAD_BYTES) {
  if (!file || typeof file !== 'object') return UPLOAD_MESSAGES.type
  if (!isAcceptedImage(file)) return UPLOAD_MESSAGES.type

  const size = Number(file.size)
  if (Number.isFinite(size) && size === 0) return UPLOAD_MESSAGES.empty
  if (Number.isFinite(size) && size > maxBytes) return UPLOAD_MESSAGES.tooLarge(size, maxBytes)
  return ''
}
