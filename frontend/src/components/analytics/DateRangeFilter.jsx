import { useState } from 'react'
import { CalendarDays } from 'lucide-react'

import {
  checkCustomRange,
  DATE_RANGE_PRESETS,
  dayInZone,
  matchingPreset,
  presetRange,
} from '../../lib/analyticsFormat.js'

/**
 * The date part of the analytics filter row: the quick ranges first - nobody
 * fights two calendars for "the last 7 days" - and "Özel" behind them, which
 * opens a from/to pair for anything else.
 *
 * Every day here is a day in Europe/Istanbul, the zone the API counts in, so
 * "Bugün" is the same day for an owner abroad as for their venue.
 *
 * The custom pair is a draft until "Uygula": a half-typed date would otherwise
 * refetch the whole page on every keystroke. "Uygula" refuses a pair the
 * summary cannot answer - longer than a year (checkCustomRange) - here, with
 * the reason, instead of passing it on and emptying the page.
 *
 * @param {{from: string, to: string}} value
 * @param {Function} onChange - ({ from, to }) => void
 */
export default function DateRangeFilter({ value, onChange }) {
  const selected = matchingPreset(value)
  // Open while the owner is choosing a custom range, and whenever the current
  // range is one no preset matches (so its days are visible and editable).
  const [customOpen, setCustomOpen] = useState(selected === 'custom')
  const [draftFrom, setDraftFrom] = useState(value?.from || '')
  const [draftTo, setDraftTo] = useState(value?.to || '')
  const [problem, setProblem] = useState('')

  const today = dayInZone()
  const showCustom = customOpen || selected === 'custom'

  function choosePreset(id) {
    setCustomOpen(false)
    setProblem('')
    onChange(presetRange(id))
  }

  function openCustom() {
    setDraftFrom(value?.from || '')
    setDraftTo(value?.to || '')
    setProblem('')
    setCustomOpen(true)
  }

  function applyCustom(event) {
    event.preventDefault()
    const { range, problem: reason } = checkCustomRange(draftFrom, draftTo)
    if (!range) {
      setProblem(reason)
      return
    }
    setProblem('')
    setDraftFrom(range.from)
    setDraftTo(range.to)
    onChange(range)
  }

  return (
    <div className="min-w-0">
      <span className="label" id="analytics-range-label">
        Tarih aralığı
      </span>

      <div
        className="inline-flex flex-wrap rounded-lg border border-gray-200 bg-gray-50 p-1"
        role="group"
        aria-labelledby="analytics-range-label"
      >
        {DATE_RANGE_PRESETS.map((preset) => {
          const isSelected = !showCustom && selected === preset.id
          return (
            <button
              key={preset.id}
              type="button"
              onClick={() => choosePreset(preset.id)}
              aria-pressed={isSelected}
              className={`rounded-md px-3 py-1.5 text-sm ${
                isSelected
                  ? 'bg-white font-medium text-gray-900 shadow-card'
                  : 'text-gray-500 hover:text-gray-700'
              }`}
            >
              {preset.label}
            </button>
          )
        })}
        <button
          type="button"
          onClick={openCustom}
          aria-pressed={showCustom}
          aria-expanded={showCustom}
          aria-controls="analytics-custom-range"
          className={`inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm ${
            showCustom
              ? 'bg-white font-medium text-gray-900 shadow-card'
              : 'text-gray-500 hover:text-gray-700'
          }`}
        >
          <CalendarDays className="h-3.5 w-3.5" aria-hidden="true" />
          Özel
        </button>
      </div>

      {showCustom ? (
        <form
          id="analytics-custom-range"
          className="mt-3 flex flex-wrap items-end gap-2"
          onSubmit={applyCustom}
          noValidate
        >
          <div>
            <label className="mb-1 block text-xs font-medium text-gray-600" htmlFor="analytics-from">
              Başlangıç
            </label>
            <input
              id="analytics-from"
              type="date"
              className="input py-2"
              value={draftFrom}
              max={today}
              onChange={(event) => setDraftFrom(event.target.value)}
            />
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-gray-600" htmlFor="analytics-to">
              Bitiş
            </label>
            <input
              id="analytics-to"
              type="date"
              className="input py-2"
              value={draftTo}
              max={today}
              onChange={(event) => setDraftTo(event.target.value)}
            />
          </div>
          <button type="submit" className="btn-primary py-2">
            Uygula
          </button>
          {problem ? (
            <p className="error-text w-full" role="alert">
              {problem}
            </p>
          ) : null}
        </form>
      ) : null}
    </div>
  )
}
