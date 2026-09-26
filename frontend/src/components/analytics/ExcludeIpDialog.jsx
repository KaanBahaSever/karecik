import { useEffect, useId, useState } from 'react'
import { History, Loader2, Trash2 } from 'lucide-react'

import api from '../../lib/api'
import { formatCount } from '../../lib/analyticsFormat.js'
import {
  addResultText,
  checkLabel,
  futureVisitsText,
  MAX_LABEL_LENGTH,
  pastRecordsText,
} from '../../lib/ipExclusion.js'
import Modal from '../ui/Modal.jsx'
import ErrorNote from './ErrorNote.jsx'

/**
 * The one dialog every "exclude this address" goes through - the add form,
 * the "Listeye ekle" of the current address, and the "Hariç tut" of a visit
 * log row - because the owner asked for the same question every time: what
 * happens to the visits this address ALREADY made?
 *
 * It opens on an entry the page has already checked (checkExclusion), asks
 * the server how many stored visits the entry covers
 * (GET /api/analytics/excluded-ips/match-count) and only then offers the
 * choice:
 *
 *   some visits   "Geçmiş kayıtlarını da sil" - delete_history: true, the
 *                 visits go in the same transaction as the insert - or
 *                 "Eskiler kalsın" - delete_history: false. Both stop future
 *                 visits; they differ only in the past.
 *   none          nothing to decide, so a plain "Ekle" (delete_history: false).
 *
 * There is deliberately no default: the API refuses an add without
 * delete_history, and this dialog never sends one before the owner chose.
 *
 * The note ("Ev", "Ofis") can still be written or corrected here - the two
 * one-click paths arrive without one.
 *
 * @param {{ cidr: string, display: string, label: string, single: boolean }} entry
 * @param {Function} onClose    - () => void; not called while an add runs
 * @param {Function} onAdded    - ({ item, deletedEvents, message }) => void
 * @param {Function} onConflict - () => void; the server already has the entry
 *                                (409), so the page's copy of the list is stale
 */
export default function ExcludeIpDialog({ entry, onClose, onAdded, onConflict }) {
  const labelId = useId()
  const [label, setLabel] = useState(entry.label || '')
  const [labelProblem, setLabelProblem] = useState('')

  // The count of past visits: null until the server answered.
  const [count, setCount] = useState(null)
  const [counting, setCounting] = useState(true)
  const [countError, setCountError] = useState('')
  // A refusal (422) will be repeated by any retry; a network failure may not.
  const [countRetryable, setCountRetryable] = useState(true)
  const [countAttempt, setCountAttempt] = useState(0)

  // '' while idle, else which choice is being saved: 'delete' | 'keep'.
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  // After a 409 the entry cannot be added from here at all: only "Kapat" stays.
  const [settled, setSettled] = useState(false)

  useEffect(() => {
    let current = true
    setCounting(true)
    setCountError('')

    api
      .excludedIpMatchCount(entry.cidr)
      .then((data) => {
        if (!current) return
        const total = Math.trunc(Number(data?.count))
        setCount(Number.isFinite(total) && total > 0 ? total : 0)
      })
      .catch((err) => {
        if (!current) return
        setCount(null)
        setCountRetryable(err?.status !== 422 && err?.status !== 400)
        setCountError(err?.message || 'Geçmiş ziyaret kayıtları sayılamadı.')
      })
      .finally(() => {
        if (current) setCounting(false)
      })

    return () => {
      current = false
    }
  }, [entry.cidr, countAttempt])

  function close() {
    if (!busy) onClose()
  }

  async function add(deleteHistory) {
    if (busy || settled) return
    const checked = checkLabel(label)
    if (checked.problem) {
      setLabelProblem(checked.problem)
      return
    }
    setLabelProblem('')
    setError('')
    setBusy(deleteHistory ? 'delete' : 'keep')

    try {
      const data = await api.addExcludedIp({
        cidr: entry.cidr,
        label: checked.label,
        delete_history: deleteHistory,
      })
      const item = data?.item && typeof data.item === 'object' ? data.item : null
      const deletedEvents = Math.max(0, Math.trunc(Number(data?.deleted_events)) || 0)
      onAdded({
        item,
        deletedEvents,
        message: addResultText({
          display: item?.display || entry.display,
          deleteHistory,
          deletedEvents,
          pastCount: count,
        }),
      })
    } catch (err) {
      setBusy('')
      if (err?.status === 409) {
        setSettled(true)
        setError(err.message || 'Bu IP zaten listede.')
        onConflict?.()
        return
      }
      setError(err?.message || 'IP listeye eklenemedi.')
    }
  }

  const ready = !counting && count !== null && !settled
  const labelLength = Array.from(label.trim()).length

  let footer
  if (settled) {
    footer = (
      <button type="button" className="btn-secondary" onClick={close}>
        Kapat
      </button>
    )
  } else {
    footer = (
      <>
        <button type="button" className="btn-secondary" onClick={close} disabled={Boolean(busy)}>
          Vazgeç
        </button>
        {ready && count === 0 ? (
          <button
            type="button"
            className="btn-primary"
            onClick={() => add(false)}
            disabled={Boolean(busy)}
            autoFocus
          >
            {busy ? (
              <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
            ) : null}
            {busy ? 'Ekleniyor...' : 'Ekle'}
          </button>
        ) : null}
      </>
    )
  }

  return (
    <Modal
      open
      onClose={close}
      title={entry.single ? "IP'yi hariç tut" : 'IP aralığını hariç tut'}
      description={futureVisitsText(entry)}
      width="max-w-md"
      footer={footer}
    >
      <div className="space-y-4">
        <div className="rounded-lg bg-gray-50 px-3 py-2.5">
          <p className="text-xs text-gray-500">{entry.single ? 'IP adresi' : 'IP aralığı'}</p>
          <p className="break-anywhere font-mono text-sm font-medium text-gray-900">
            {entry.display}
          </p>
        </div>

        <div>
          <label className="label" htmlFor={labelId}>
            Not <span className="font-normal text-gray-400">(isteğe bağlı)</span>
          </label>
          <input
            id={labelId}
            className="input"
            value={label}
            placeholder="örn. Ev, Ofis"
            autoComplete="off"
            disabled={Boolean(busy) || settled}
            aria-invalid={labelProblem ? 'true' : undefined}
            aria-describedby={`${labelId}-hint`}
            onChange={(event) => {
              setLabel(event.target.value)
              if (labelProblem) setLabelProblem('')
            }}
          />
          <p
            id={`${labelId}-hint`}
            className={labelProblem ? 'error-text' : 'help-text tabular-nums'}
          >
            {labelProblem || `${labelLength} / ${MAX_LABEL_LENGTH}`}
          </p>
        </div>

        {/* The count arrives after the dialog opens; aria-live reads it out. */}
        <div aria-live="polite">
          {counting ? (
            <p className="flex items-center gap-2 text-sm text-gray-500">
              <Loader2 className="h-4 w-4 animate-spin text-brand-600" aria-hidden="true" />
              Geçmiş ziyaret kayıtları sayılıyor...
            </p>
          ) : countError ? (
            <ErrorNote
              message={countError}
              onRetry={countRetryable ? () => setCountAttempt((value) => value + 1) : undefined}
            />
          ) : count !== null ? (
            <p className="text-sm font-medium text-gray-900">
              {pastRecordsText(count, { single: entry.single })}
            </p>
          ) : null}
        </div>

        {ready && count > 0 ? (
          <div className="space-y-2" role="group" aria-label="Geçmiş kayıtlar ne olsun?">
            <p className="text-sm text-gray-600">
              Bu kayıtlar ne olsun? İkisinde de bundan sonraki ziyaretler kaydedilmez.
            </p>
            <Choice
              icon={Trash2}
              danger
              title="Geçmiş kayıtlarını da sil"
              description={`${formatCount(count)} ziyaret kaydı kalıcı olarak silinir; kartlar, grafik ve listeler bu ziyaretler olmadan yeniden hesaplanır.`}
              busy={busy === 'delete'}
              disabled={Boolean(busy)}
              onClick={() => add(true)}
            />
            {/* The safe choice takes the focus: Enter never deletes anything. */}
            <Choice
              icon={History}
              title="Eskiler kalsın"
              description="Geçmiş kayıtlar ve istatistikler olduğu gibi kalır."
              busy={busy === 'keep'}
              disabled={Boolean(busy)}
              onClick={() => add(false)}
              autoFocus
            />
          </div>
        ) : null}

        {error ? <ErrorNote message={error} /> : null}
      </div>
    </Modal>
  )
}

/**
 * One of the two answers about the past visits: a full-width button with its
 * consequence written under it, so nobody has to guess what "sil" deletes.
 * The title names the button (aria-label, which follows it into the busy
 * state); the consequence describes it.
 */
function Choice({ icon: Icon, title, description, onClick, busy, disabled, danger = false, autoFocus }) {
  const id = useId()
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      autoFocus={autoFocus}
      aria-label={busy ? 'Ekleniyor...' : title}
      aria-describedby={`${id}-description`}
      className={`flex w-full items-start gap-3 rounded-lg border px-3 py-3 text-left focus:outline-none focus:ring-2 disabled:cursor-not-allowed disabled:opacity-60 ${
        danger
          ? 'border-red-200 hover:bg-red-50 focus:ring-red-100'
          : 'border-gray-300 hover:bg-gray-50 focus:ring-brand-100'
      }`}
    >
      {busy ? (
        <Loader2 className="mt-0.5 h-4 w-4 shrink-0 animate-spin text-gray-500" aria-hidden="true" />
      ) : (
        <Icon
          className={`mt-0.5 h-4 w-4 shrink-0 ${danger ? 'text-red-600' : 'text-gray-500'}`}
          aria-hidden="true"
        />
      )}
      <span className="min-w-0">
        <span className={`block text-sm font-medium ${danger ? 'text-red-700' : 'text-gray-900'}`}>
          {busy ? 'Ekleniyor...' : title}
        </span>
        <span id={`${id}-description`} className="mt-0.5 block text-xs leading-relaxed text-gray-500">
          {description}
        </span>
      </span>
    </button>
  )
}
