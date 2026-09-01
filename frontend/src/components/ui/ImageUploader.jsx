import { useRef, useState } from 'react'
import { ImagePlus, Loader2, Trash2 } from 'lucide-react'

import api from '../../lib/api'
import { useToast } from './Toast.jsx'

/**
 * Single image upload field. Reports the URL returned by the server.
 *
 * @param {string|null} value    - Current image URL (/uploads/...)
 * @param {Function}    onChange - (url|null) => void
 * @param {string}      label
 * @param {string}      hint
 * @param {boolean}     round    - Circular preview, used for logos
 */
export default function ImageUploader({
  value,
  onChange,
  label = 'Görsel',
  hint = 'JPG, PNG veya WEBP · en fazla 5 MB',
  round = false,
}) {
  const [uploading, setUploading] = useState(false)
  const inputRef = useRef(null)
  const toast = useToast()

  async function onFileSelected(event) {
    const file = event.target.files?.[0]
    event.target.value = '' // allow picking the same file again
    if (!file) return

    if (file.size > 5 * 1024 * 1024) {
      toast.error('Dosya çok büyük. En fazla 5 MB yükleyebilirsiniz.')
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
          {value ? (
            <img src={value} alt="" className="h-full w-full object-cover" />
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

          <p className="text-xs text-gray-400">{hint}</p>
        </div>
      </div>

      <input
        ref={inputRef}
        type="file"
        accept="image/jpeg,image/png,image/webp,image/gif"
        className="hidden"
        onChange={onFileSelected}
      />
    </div>
  )
}
