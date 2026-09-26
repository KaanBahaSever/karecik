import { AlertCircle, RefreshCw } from 'lucide-react'

/**
 * A red box with the message and, when the failure is worth another try, a
 * retry button. Shared by every part of the analytics page that loads
 * something on its own - the summary, the visit log, the exclusion list and
 * the add dialog - so a failure looks the same wherever it happens.
 *
 * Without `onRetry` there is no button: a refusal the server will repeat
 * (a 422, a 409) must not offer a retry that can never succeed.
 *
 * @param {string}   message
 * @param {Function} onRetry - optional
 */
export default function ErrorNote({ message, onRetry }) {
  return (
    <div
      className="flex flex-wrap items-start gap-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2.5"
      role="alert"
    >
      <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-red-600" aria-hidden="true" />
      <p className="min-w-0 flex-1 text-sm text-red-800">{message}</p>
      {onRetry ? (
        <button type="button" className="btn-secondary btn-sm" onClick={onRetry}>
          <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
          Tekrar dene
        </button>
      ) : null}
    </div>
  )
}
