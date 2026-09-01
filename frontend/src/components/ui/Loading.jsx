import { Loader2 } from 'lucide-react'

/**
 * Loading indicator.
 *
 * @param {boolean} fullScreen - Render a full-viewport variant
 * @param {string}  text       - Optional caption below the spinner
 * @param {string}  className
 */
export default function Loading({ fullScreen = false, text = '', className = '' }) {
  const content = (
    <div className={`flex flex-col items-center justify-center gap-3 ${className}`}>
      <Loader2 className="h-6 w-6 animate-spin text-brand-600" aria-hidden="true" />
      {text ? <p className="text-sm text-gray-500">{text}</p> : null}
      <span className="sr-only">Yükleniyor</span>
    </div>
  )

  if (fullScreen) {
    return <div className="flex min-h-screen items-center justify-center bg-white">{content}</div>
  }
  return <div className="py-12">{content}</div>
}
