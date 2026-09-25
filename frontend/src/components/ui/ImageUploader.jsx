import { useRef, useState } from 'react'
import { ImagePlus, Loader2, Trash2 } from 'lucide-react'

import api from '../../lib/api'
import { IMAGE_ACCEPT, IMAGE_FORMATS_HINT, imageUploadProblem } from '../../lib/imageUpload'
import { useImageFallback } from '../../lib/useImageFallback'
import { useToast } from './Toast.jsx'

/**
 * Single image upload field. Reports the URL returned by the server.
 *
 * Every image field of the panel is one of these - the menu logo, the cover,
 * the background, the splash logo, the Yerli Üretim logo, the category and the
 * product image - so all of them take the same formats, SVG included, and
 * refuse the same files with the same message (lib/imageUpload.js). The
 * accepted formats and the size limit are always printed under the buttons;
 * `hint` adds the field's own advice above that line.
 *
 * @param {string|null} value    - Current image URL (/uploads/...)
 * @param {Function}    onChange - (url|null) => void
 * @param {string}      label
 * @param {string}      hint     - Optional advice specific to this field
 * @param {boolean}     round    - Circular preview, used for logos
 */
export default function ImageUploader({
  value,
  onChange,
  label = 'Görsel',
  hint = '',
  round = false,
}) {
  const [uploading, setUploading] = useState(false)
  const inputRef = useRef(null)
  const toast = useToast()

  // The preview of a URL that no longer loads — an upload whose file is gone —
  // falls back to the placeholder and says so, instead of an empty square the
  // owner has no reason to question. The failure resets when `value` changes.
  const preview = useImageFallback(value)

  async function onFileSelected(event) {
    const file = event.target.files?.[0]
    event.target.value = '' // allow picking the same file again
    if (!file) return

    // The server's own rules, checked before a byte is sent: a PDF or a 20 MB
    // photo is refused here with the reason, not after a wasted upload. An
    // SVG's contents are still checked by the server, whose message arrives
    // through the catch below.
    const problem = imageUploadProblem(file)
    if (problem) {
      toast.error(problem)
      return
    }

    setUploading(true)
    try {
      const result = await api.upload(file)
      onChange?.(result.url)
    } catch (error) {
      toast.error(error.message)
    } finally {
      setUploading(false)
    }
  }

  return (
    <div>
      <span className="label">{label}</span>

      <div className="flex items-center gap-3">
        <div
          className={`flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden border border-gray-200 bg-gray-50 ${
            round ? 'rounded-full' : 'rounded-lg'
          }`}
        >
          {preview.src ? (
            /* Never crop: wide (horizontal) logos are explicitly supported.

               The image fills the box and object-contain fits it inside. The
               box is sized, not the picture, because an SVG saved without
               width and height has no intrinsic size: with only max-w/max-h
               it can collapse to nothing in a flex box (Safari, Firefox),
               while a sized box always gives it room, and its viewBox keeps
               the proportions. A raster image is letterboxed the same way.

               An <img> never runs an SVG's scripts, so drawing an upload here
               is safe whatever the file holds. */
            <img
              src={preview.src}
              alt=""
              onError={preview.onError}
              className={`h-full w-full object-contain ${round ? 'p-1.5' : 'p-0.5'}`}
            />
          ) : (
            <ImagePlus className="h-5 w-5 text-gray-300" aria-hidden="true" />
          )}
        </div>

        <div className="flex flex-col gap-1.5">
          <div className="flex gap-2">
            <button
              type="button"
              className="btn-secondary btn-sm"
              onClick={() => inputRef.current?.click()}
              disabled={uploading}
            >
              {uploading ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin" /> Yükleniyor
                </>
              ) : (
                <>{value ? 'Değiştir' : 'Görsel seç'}</>
              )}
            </button>

            {value ? (
              <button
                type="button"
                className="btn-ghost btn-sm text-red-600 hover:bg-red-50"
                onClick={() => onChange?.(null)}
                disabled={uploading}
              >
                <Trash2 className="h-3.5 w-3.5" /> Kaldır
              </button>
            ) : null}
          </div>

          {preview.failed ? (
            <p className="text-xs text-red-600">Görsel yüklenemedi. Lütfen yeniden yükleyin.</p>
          ) : null}

          {hint ? <p className="text-xs text-gray-500">{hint}</p> : null}
          <p className="text-xs text-gray-400">{IMAGE_FORMATS_HINT}</p>
        </div>
      </div>

      <input
        ref={inputRef}
        type="file"
        accept={IMAGE_ACCEPT}
        className="hidden"
        onChange={onFileSelected}
      />
    </div>
  )
}
