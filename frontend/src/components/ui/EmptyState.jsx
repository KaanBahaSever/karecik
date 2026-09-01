/**
 * Placeholder shown when a list has no content.
 *
 * @param {Component} icon        - lucide-react component
 * @param {string}    title
 * @param {string}    description
 * @param {node}      action      - Button or similar call to action
 */
export default function EmptyState({ icon: Icon, title, description, action, className = '' }) {
  return (
    <div
      className={`flex flex-col items-center justify-center rounded-xl border border-dashed border-gray-300 bg-gray-50/60 px-6 py-12 text-center ${className}`}
    >
      {Icon ? (
        <div className="mb-3 flex h-11 w-11 items-center justify-center rounded-full bg-white shadow-card">
          <Icon className="h-5 w-5 text-gray-400" aria-hidden="true" />
        </div>
      ) : null}

      <h3 className="text-sm font-semibold text-gray-900">{title}</h3>
      {description ? <p className="mt-1 max-w-sm text-sm text-gray-500">{description}</p> : null}
      {action ? <div className="mt-4">{action}</div> : null}
    </div>
  )
}
