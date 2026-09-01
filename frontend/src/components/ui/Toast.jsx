import { createContext, useCallback, useContext, useMemo, useState } from 'react'
import { AlertCircle, CheckCircle2, Info, X } from 'lucide-react'

const ToastContext = createContext(null)

let counter = 0

/**
 * Short-lived notifications shown in the bottom-right corner.
 *
 * Usage:
 *   const toast = useToast()
 *   toast.success('Kaydedildi.')
 *   toast.error(err.message)
 */
export function ToastProvider({ children }) {
  const [toasts, setToasts] = useState([])

  const remove = useCallback((id) => {
    setToasts((previous) => previous.filter((toast) => toast.id !== id))
  }, [])

  const add = useCallback(
    (kind, message, duration = 4000) => {
      if (!message) return
      counter += 1
      const id = counter
      setToasts((previous) => [...previous, { id, kind, message }])
      if (duration > 0) {
        setTimeout(() => remove(id), duration)
      }
    },
    [remove],
  )

  const value = useMemo(
    () => ({
      success: (message, duration) => add('success', message, duration),
      error: (message, duration) => add('error', message, duration ?? 6000),
      info: (message, duration) => add('info', message, duration),
      remove,
    }),
    [add, remove],
  )

  return (
    <ToastContext.Provider value={value}>
      {children}

      <div
        className="pointer-events-none fixed bottom-4 right-4 z-[100] flex w-full max-w-sm flex-col gap-2"
        role="status"
        aria-live="polite"
      >
        {toasts.map((toast) => (
          <ToastItem key={toast.id} toast={toast} onClose={() => remove(toast.id)} />
        ))}
      </div>
    </ToastContext.Provider>
  )
}

const STYLES = {
  success: { box: 'border-green-200 bg-green-50 text-green-800', Icon: CheckCircle2 },
  error: { box: 'border-red-200 bg-red-50 text-red-800', Icon: AlertCircle },
  info: { box: 'border-blue-200 bg-blue-50 text-blue-800', Icon: Info },
}

function ToastItem({ toast, onClose }) {
  const style = STYLES[toast.kind] || STYLES.info
  const { Icon } = style

  return (
    <div
      className={`pointer-events-auto flex items-start gap-3 rounded-lg border px-4 py-3 shadow-panel ${style.box}`}
    >
      <Icon className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
      <p className="flex-1 text-sm leading-snug">{toast.message}</p>
      <button
        type="button"
        onClick={onClose}
        className="shrink-0 opacity-60 hover:opacity-100"
        aria-label="Bildirimi kapat"
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  )
}

export function useToast() {
  const context = useContext(ToastContext)
  if (!context) {
    throw new Error('useToast can only be used inside <ToastProvider>.')
  }
  return context
}
