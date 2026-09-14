import { createContext, useCallback, useContext, useMemo, useRef, useState } from 'react'
import { AlertCircle, CheckCircle2, Info, X } from 'lucide-react'

const ToastContext = createContext(null)

let counter = 0

/**
 * How many toasts are on screen at once. A new one pushes the oldest out: a
 * third stacked toast would reach down over the menu editor's "Kategori Ekle"
 * and "Toplu Fiyat Güncelle" buttons at 1440x900.
 */
const MAX_VISIBLE_TOASTS = 2

/**
 * Short-lived notifications, stacked at the top of the screen: top right, just
 * below the dashboard's 64 px top bar, and across the full width inside side
 * gutters on a narrow screen.
 *
 * Not at the bottom: there a toast would lie on top of the very things it is
 * usually about - the settings page's sticky "Kaydet" bar and the footer
 * buttons of every modal. At the top it still lies over something: a modal's
 * title and its close button on a phone, the page's own buttons on a desktop.
 * The stack stays above the modals (z-[100] against their z-50) so it can still
 * be read while one is open.
 *
 * So nothing done on the stack reaches what lies beneath it, and nothing done
 * on it takes focus away from where the owner is typing:
 *
 *   - While a toast is on screen the stack's whole box takes pointer events. A
 *     click on a card dismisses that card; a click in the gap between two
 *     cards, or in a card's rounded corner, lands on the stack itself and does
 *     nothing. Under an open dialog such a click would otherwise reach the
 *     backdrop, which closes the dialog on a press and throws away what was
 *     typed into it. With no toast on screen the stack takes no pointer events.
 *   - A mousedown anywhere on the stack - a card, its close button, a gap - is
 *     cancelled, so it moves no focus: the focused field keeps focus, runs no
 *     onBlur and keeps the phone keyboard open.
 *   - The close button is how the keyboard reaches a toast. Dismissing a toast
 *     whose button has focus hands focus back to the element that had it before
 *     focus entered the stack, when that element is still in the document.
 *
 * Usage:
 *   const toast = useToast()
 *   toast.success('Kaydedildi.')
 *   toast.error(err.message)
 */
export function ToastProvider({ children }) {
  const [toasts, setToasts] = useState([])
  const stackRef = useRef(null)
  // Where focus came from when it last entered the stack; see restoreFocus.
  const returnFocusRef = useRef(null)

  const remove = useCallback((id) => {
    setToasts((previous) => previous.filter((toast) => toast.id !== id))
  }, [])

  const add = useCallback(
    (kind, message, duration = 4000) => {
      if (!message) return
      counter += 1
      const id = counter
      setToasts((previous) => [...previous, { id, kind, message }].slice(-MAX_VISIBLE_TOASTS))
      // A toast pushed out early still has its timer; removing an id that is
      // no longer in the list changes nothing.
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

  /* A focus event inside the stack carries the element focus left in
     relatedTarget. Only focus arriving from outside the stack is remembered, so
     tabbing from one toast's button to the next keeps the element focus had
     before either of them. */
  const rememberFocusOrigin = useCallback((event) => {
    const origin = event.relatedTarget
    if (origin && stackRef.current && stackRef.current.contains(origin)) return
    returnFocusRef.current = origin || null
  }, [])

  /* Called just before a toast whose button has focus is removed. The button
     leaves the document with the toast, which would drop focus onto <body>. */
  const restoreFocus = useCallback(() => {
    const target = returnFocusRef.current
    returnFocusRef.current = null
    if (target && document.contains(target) && typeof target.focus === 'function') {
      target.focus()
    }
  }, [])

  return (
    <ToastContext.Provider value={value}>
      {children}

      {/* top-20 is the h-16 top bar plus a 1rem gap. Below `sm` the stack spans
          the screen between 1rem gutters; from `sm` up it is a 24rem column
          on the right, in line with the main area's sm:p-6 padding. */}
      <div
        ref={stackRef}
        className={`fixed left-4 right-4 top-20 z-[100] flex flex-col gap-2 sm:left-auto sm:right-6 sm:w-full sm:max-w-sm ${
          toasts.length > 0 ? 'pointer-events-auto' : 'pointer-events-none'
        }`}
        role="status"
        aria-live="polite"
        onMouseDown={(event) => event.preventDefault()}
        onFocus={rememberFocusOrigin}
      >
        {toasts.map((toast) => (
          <ToastItem
            key={toast.id}
            toast={toast}
            onClose={() => remove(toast.id)}
            onRestoreFocus={restoreFocus}
          />
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

function ToastItem({ toast, onClose, onRestoreFocus }) {
  const style = STYLES[toast.kind] || STYLES.info
  const { Icon } = style
  const cardRef = useRef(null)

  /* Every dismissal comes through here: a pointer click anywhere on the card,
     and Enter or Space on the close button, whose click bubbles up to the card.
     The button has focus only when the keyboard put it there - a mousedown on
     the stack never focuses anything - so a card holding focus is dismissed
     from the keyboard, and focus goes back to where it came from. */
  function dismiss() {
    if (cardRef.current && cardRef.current.contains(document.activeElement)) onRestoreFocus()
    onClose()
  }

  return (
    <div
      ref={cardRef}
      onClick={dismiss}
      className={`flex cursor-pointer items-start gap-2 rounded-lg border py-3 pl-2 pr-4 shadow-panel ${style.box}`}
    >
      <Icon className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
      <p className="min-w-0 flex-1 text-sm leading-snug">{toast.message}</p>
      {/* After the message in the DOM, drawn first by `order-first`. The stack
          is a role="status" region, which a screen reader reads out whole and
          in DOM order, so the message is announced before "Bildirimi kapat".
          On screen the button sits at the start of the card, away from the
          top-right corner where a modal keeps its own close button.

          It has no click handler of its own: its click bubbles to the card. */}
      <button
        type="button"
        className="order-first -my-1 shrink-0 rounded-md p-1 opacity-60 hover:opacity-100"
        aria-label="Bildirimi kapat"
      >
        <X className="h-4 w-4" aria-hidden="true" />
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
