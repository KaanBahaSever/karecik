import { formatDateTime } from '../../lib/analyticsFormat.js'

/**
 * A timestamp as a table cell: the day, and the time under it in grey - both
 * as the wall clock in Europe/Istanbul (formatDateTime). Two short lines keep
 * the visit log and the change history narrow enough for a laptop screen.
 *
 * @param {string} value - RFC 3339 timestamp from the API
 */
export default function DateTimeCell({ value }) {
  const [day, time] = formatDateTime(value).split(' ')
  return (
    <>
      <span className="block">{day}</span>
      {time ? <span className="block text-[11px] text-gray-500">{time}</span> : null}
    </>
  )
}
