import { forwardRef, useId, useState } from 'react'
import { AlertTriangle, CheckCircle2, EyeOff, Info, Loader2, Plus, Trash2 } from 'lucide-react'

import api from '../../lib/api'
import { formatDateTime, ipSourceInfo } from '../../lib/analyticsFormat.js'
import {
  alreadyRemovedText,
  cidrContains,
  currentIpCovered,
  entryName,
  isSingleEntry,
  MAX_EXCLUDED_IPS,
  MAX_LABEL_LENGTH,
  removeEntryLabel,
  removeEntryText,
} from '../../lib/ipExclusion.js'
import ConfirmModal from '../ui/ConfirmModal.jsx'
import Loading from '../ui/Loading.jsx'
import { useToast } from '../ui/Toast.jsx'
import ErrorNote from './ErrorNote.jsx'

/**
 * "Hariç tutulan IP'ler" - the visits that are never recorded at all: the
 * owner's own, and whoever else they choose. Two tools, because neither one
 * covers everything:
 *
 *   THIS BROWSER  a switch that sets a cookie (POST/DELETE
 *                 /api/analytics/optout). It follows the device wherever it
 *                 goes and whatever address it has, but only this browser.
 *   AN ADDRESS    an IP or a range on the business's list. It covers every
 *                 device behind it - the venue's Wi-Fi, the office - but a
 *                 home or office address can change, and a mobile operator's
 *                 address is shared by thousands of strangers.
 *
 * The server enforces both before anything is stored (TrackEvent), so an
 * excluded visit spends none of the per-visitor limits either.
 *
 * This component draws the section and owns the switch and the removals. The
 * adds are the page's: the visit log adds from its rows too, and every add
 * goes through the same dialog (ExcludeIpDialog), which the page renders.
 *
 * @param {object|null} state        - readExclusionState(GET /excluded-ips), null until loaded
 * @param {boolean}     loading
 * @param {string}      error
 * @param {Function}    onRetry
 * @param {boolean}     optout       - whether this browser is opted out
 * @param {Function}    onOptoutChange - (on) => void, after the server agreed
 * @param {Function}    onRequestAdd - ({ cidr, label }, afterAdd?) => { problem, field };
 *                                     opens the dialog when the entry passes
 * @param {Function}    onRemoved    - (item) => void, after the server removed it
 */
const ExcludedIpsSection = forwardRef(function ExcludedIpsSection(
  { state, loading, error, onRetry, optout, onOptoutChange, onRequestAdd, onRemoved },
  ref,
) {
  const toast = useToast()

  /* ------------------------------------------------------------ add form */

  const [cidrInput, setCidrInput] = useState('')
  const [labelInput, setLabelInput] = useState('')
  const [formProblem, setFormProblem] = useState({ field: '', message: '' })

  function submitForm(event) {
    event.preventDefault()
    const { problem, field } = onRequestAdd({ cidr: cidrInput, label: labelInput }, () => {
      // Only once it is really on the list: a "Vazgeç" keeps what was typed.
      setCidrInput('')
      setLabelInput('')
    })
    setFormProblem({ field: problem ? field || 'form' : '', message: problem || '' })
  }

  /* ------------------------------------------------------------- removal */

  const [removing, setRemoving] = useState(null)
  const [removeBusy, setRemoveBusy] = useState(false)

  async function confirmRemove() {
    const item = removing
    if (!item || removeBusy) return
    setRemoveBusy(true)
    try {
      await api.removeExcludedIp(item.id)
      onRemoved(item)
      toast.success(`${entryName(item)} listeden çıkarıldı.`)
      setRemoving(null)
    } catch (err) {
      if (err?.status === 404) {
        // Removed elsewhere (another tab, another user of the business).
        onRemoved(item)
        toast.info(alreadyRemovedText(item))
        setRemoving(null)
      } else {
        toast.error(err?.message || 'IP listeden çıkarılamadı.')
      }
    } finally {
      setRemoveBusy(false)
    }
  }

  /* ------------------------------------------------------------- opt-out */

  const [optoutBusy, setOptoutBusy] = useState(false)

  async function changeOptout(next) {
    if (optoutBusy) return
    setOptoutBusy(true)
    try {
      await api.setAnalyticsOptout(next)
      onOptoutChange(next)
      toast.success(
        next
          ? 'Bu tarayıcıdan yapılan ziyaretler artık sayılmayacak.'
          : 'Bu tarayıcıdan yapılan ziyaretler yeniden sayılacak.',
      )
    } catch (err) {
      toast.error(err?.message || 'Ayar kaydedilemedi.')
    } finally {
      setOptoutBusy(false)
    }
  }

  /* -------------------------------------------------------------- render */

  const items = state?.items || []
  const max = state?.max || MAX_EXCLUDED_IPS
  const full = Boolean(state) && items.length >= max

  return (
    <section
      ref={ref}
      id="haric-tutulan-ipler"
      className="card mt-6 scroll-mt-20"
      aria-labelledby="excluded-ips-title"
    >
      <div className="border-b border-gray-100 p-4">
        <div className="flex items-center gap-2">
          <EyeOff className="h-4 w-4 shrink-0 text-brand-600" aria-hidden="true" />
          <h2 id="excluded-ips-title" className="text-base font-semibold text-gray-900">
            Hariç tutulan IP'ler
          </h2>
        </div>
        <p className="help-text">
          Kendi ziyaretleriniz istatistiklerinizi şişirmesin: buradaki adreslerden ve bu anahtarı
          açtığınız tarayıcılardan gelen ziyaretler hiç kaydedilmez.
        </p>
      </div>

      <div className="space-y-6 p-4">
        {error ? <ErrorNote message={error} onRetry={onRetry} /> : null}

        {/* ------------------------------------------------ this device */}
        <div className="divide-y divide-gray-200 rounded-lg border border-gray-200 bg-gray-50">
          <CurrentIp state={state} loading={loading} full={full} onRequestAdd={onRequestAdd} />
          <OptoutSwitch checked={optout} busy={optoutBusy} onChange={changeOptout} />
        </div>

        {/* --------------------------------------------------- add form */}
        <form onSubmit={submitForm} noValidate aria-labelledby="exclude-form-title">
          <h3 id="exclude-form-title" className="mb-2 text-sm font-semibold text-gray-900">
            IP ya da aralık ekle
          </h3>
          <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] sm:items-start">
            <div className="min-w-0">
              <label className="label" htmlFor="exclude-ip-cidr">
                IP adresi ya da aralık
              </label>
              <input
                id="exclude-ip-cidr"
                className="input font-mono"
                value={cidrInput}
                placeholder="örn. 203.0.113.7"
                inputMode="text"
                autoComplete="off"
                autoCapitalize="off"
                spellCheck={false}
                maxLength={64}
                disabled={full}
                aria-invalid={formProblem.field === 'cidr' ? 'true' : undefined}
                aria-describedby="exclude-ip-cidr-hint"
                onChange={(event) => {
                  setCidrInput(event.target.value)
                  if (formProblem.field === 'cidr') setFormProblem({ field: '', message: '' })
                }}
              />
              <p
                id="exclude-ip-cidr-hint"
                className={formProblem.field === 'cidr' ? 'error-text' : 'help-text'}
              >
                {formProblem.field === 'cidr'
                  ? formProblem.message
                  : 'Tek bir adres ya da en geniş /16 (IPv6 için /48) bir aralık.'}
              </p>
            </div>

            <div className="min-w-0">
              <label className="label" htmlFor="exclude-ip-label">
                Not <span className="font-normal text-gray-400">(isteğe bağlı)</span>
              </label>
              <input
                id="exclude-ip-label"
                className="input"
                value={labelInput}
                placeholder="örn. Ev, Ofis"
                autoComplete="off"
                disabled={full}
                aria-invalid={formProblem.field === 'label' ? 'true' : undefined}
                aria-describedby="exclude-ip-label-hint"
                onChange={(event) => {
                  setLabelInput(event.target.value)
                  if (formProblem.field === 'label') setFormProblem({ field: '', message: '' })
                }}
              />
              <p
                id="exclude-ip-label-hint"
                className={formProblem.field === 'label' ? 'error-text' : 'help-text'}
              >
                {formProblem.field === 'label'
                  ? formProblem.message
                  : `En fazla ${MAX_LABEL_LENGTH} karakter.`}
              </p>
            </div>

            {/* Level with the inputs beside it: .label is 20px of text plus a
                6px gap above them, and an .input is 42px tall. */}
            <div className="sm:pt-[26px]">
              <button
                type="submit"
                className="btn-primary w-full sm:h-[42px] sm:w-auto"
                disabled={full}
              >
                <Plus className="h-4 w-4" aria-hidden="true" />
                Ekle
              </button>
            </div>
          </div>
          {formProblem.field === 'form' || full ? (
            <p className="mt-2 text-xs text-amber-800" role="status">
              {formProblem.field === 'form'
                ? formProblem.message
                : `Liste dolu (${max} / ${max}). Yeni bir IP eklemek için önce listeden birini çıkarın.`}
            </p>
          ) : null}
        </form>

        {/* ------------------------------------------------------- list */}
        <div>
          <div className="mb-2 flex items-baseline justify-between gap-3">
            <h3 className="text-sm font-semibold text-gray-900">Liste</h3>
            {state ? (
              <span className="text-xs text-gray-500 tabular-nums">
                {items.length} / {max}
              </span>
            ) : null}
          </div>

          {!state && loading ? (
            <Loading text="Liste yükleniyor..." />
          ) : !state ? (
            <p className="rounded-lg border border-dashed border-gray-300 px-3 py-6 text-center text-sm text-gray-500">
              Liste yüklenemedi.
            </p>
          ) : items.length === 0 ? (
            <p className="rounded-lg border border-dashed border-gray-300 px-3 py-6 text-center text-sm text-gray-500">
              Henüz hariç tutulan bir IP yok.
            </p>
          ) : (
            <ul
              className={`divide-y divide-gray-100 rounded-lg border border-gray-200 ${
                loading ? 'opacity-60' : ''
              }`}
            >
              {items.map((item) => (
                <ExclusionRow
                  key={item.id}
                  item={item}
                  isCurrent={Boolean(state.currentIp) && cidrContains(item.cidr, state.currentIp)}
                  onRemove={() => setRemoving(item)}
                />
              ))}
            </ul>
          )}
        </div>

        {/* ------------------------------------------------------- note */}
        <div className="flex items-start gap-2 rounded-lg bg-blue-50 px-3 py-3 text-xs leading-relaxed text-blue-900">
          <Info className="mt-0.5 h-4 w-4 shrink-0 text-blue-500" aria-hidden="true" />
          <ul className="list-disc space-y-1 pl-4">
            <li>
              Listedeki adreslerden gelen ziyaretler hiç kaydedilmez; ne istatistiklere ne de
              ziyaret kayıtlarına girer.
            </li>
            <li>
              Ev ve iş yeri internetinin IP adresi zamanla değişebilir. Kendi telefonunuz ve
              bilgisayarınız için yukarıdaki “Bu tarayıcıdan yapılan ziyaretleri
              sayma” anahtarı daha güvenilirdir.
            </li>
            <li>
              Mobil hatların (4.5G/5G) IP adresleri aynı anda çok sayıda kişi tarafından
              paylaşılır. Böyle bir adresi eklerseniz gerçek müşterilerinizin ziyaretleri de
              kaydedilmez.
            </li>
          </ul>
        </div>
      </div>

      <ConfirmModal
        open={Boolean(removing)}
        onClose={() => {
          if (!removeBusy) setRemoving(null)
        }}
        onConfirm={confirmRemove}
        busy={removeBusy}
        title="Listeden çıkarılsın mı?"
        message={removing ? removeEntryText(removing) : ''}
        confirmText="Listeden çıkar"
      />
    </section>
  )
})

export default ExcludedIpsSection

/* ---------------------------------------------------------- small pieces */

/**
 * "Şu anki IP adresiniz: …" - the address a menu opened from here right now
 * would be logged under (the same resolution as the visit log), and the
 * one-click way to put it on the list.
 */
function CurrentIp({ state, loading, full, onRequestAdd }) {
  const toast = useToast()

  if (!state) {
    return (
      <div className="flex items-center gap-2 px-3 py-3 text-sm text-gray-500">
        {loading ? <Loader2 className="h-4 w-4 animate-spin text-brand-600" aria-hidden="true" /> : null}
        {loading ? 'IP adresiniz alınıyor...' : 'IP adresiniz alınamadı.'}
      </div>
    )
  }

  if (!state.currentIp) {
    return (
      <p className="px-3 py-3 text-sm text-gray-600">
        Şu anki IP adresiniz belirlenemedi. Kendi cihazlarınız için aşağıdaki tarayıcı
        anahtarını kullanın.
      </p>
    )
  }

  const covered = currentIpCovered(state)
  const source = ipSourceInfo(state.currentIpSource)
  // An address the server took from a connection or a hosting edge can belong
  // to a proxy that every visitor shares; the dialog's count then gives it away.
  const shared = state.currentIpSource === 'peer' || state.currentIpSource === 'edge'

  function addCurrent() {
    const { problem } = onRequestAdd({ cidr: state.currentIp, label: '' })
    if (problem) toast.error(problem)
  }

  return (
    <div className="px-3 py-3">
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <div className="min-w-0">
          <p className="text-sm text-gray-700">
            Şu anki IP adresiniz:{' '}
            <span className="break-anywhere font-mono font-medium text-gray-900">
              {state.currentIp}
            </span>
          </p>
          {state.currentIpSource ? (
            <p className="mt-0.5 text-[11px] text-gray-500" title={source.description}>
              Kaynak: {source.text}
            </p>
          ) : null}
        </div>

        {covered ? (
          <span className="badge bg-emerald-50 text-emerald-700 ring-1 ring-inset ring-emerald-200">
            <CheckCircle2 className="h-3.5 w-3.5" aria-hidden="true" />
            Listede
          </span>
        ) : (
          <button
            type="button"
            className="btn-secondary btn-sm"
            onClick={addCurrent}
            disabled={full}
            aria-describedby={full ? 'current-ip-full' : undefined}
          >
            <Plus className="h-3.5 w-3.5" aria-hidden="true" />
            Listeye ekle
          </button>
        )}
      </div>

      {covered ? (
        <p className="mt-2 text-xs text-gray-600">
          Bu adresten açılan menü ziyaretleri kaydedilmiyor.
        </p>
      ) : full ? (
        <p id="current-ip-full" className="mt-2 text-xs text-gray-600">
          Liste dolu; eklemek için önce listeden bir IP çıkarın.
        </p>
      ) : null}

      {shared && !covered ? (
        <p className="mt-2 flex items-start gap-1.5 text-xs text-amber-800">
          <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden="true" />
          <span>
            Bu adres bir ara sunucuya ait olabilir ve başka ziyaretçilerle ortak olabilir.
            Eklemeden önce kaç ziyaret kaydını kapsadığına bakın.
          </span>
        </p>
      ) : null}
    </div>
  )
}

/** "Bu tarayıcıdan yapılan ziyaretleri sayma": the opt-out cookie as a switch. */
const OPTOUT_LABEL = 'Bu tarayıcıdan yapılan ziyaretleri sayma'

function OptoutSwitch({ checked, busy, onChange }) {
  const id = useId()
  return (
    <div className="flex items-start justify-between gap-4 px-3 py-3">
      <div className="min-w-0">
        <p className="text-sm font-medium text-gray-900">{OPTOUT_LABEL}</p>
        <p id={`${id}-description`} className="help-text leading-relaxed">
          Yalnızca bu tarayıcı için geçerlidir ve IP adresiniz değişse de çalışır. Telefonunuzda
          da saymamak için o telefonun tarayıcısından panele girip anahtarı orada da açın.
          Tarayıcı çerezlerini silerseniz yeniden açmanız gerekir.
        </p>
        {checked ? (
          <p className="mt-1 text-xs font-medium text-emerald-700">
            Açık: bu tarayıcıda açtığınız menüler sayılmıyor.
          </p>
        ) : null}
      </div>

      <button
        type="button"
        role="switch"
        aria-checked={checked}
        aria-label={OPTOUT_LABEL}
        aria-describedby={`${id}-description`}
        aria-busy={busy || undefined}
        disabled={busy}
        onClick={() => onChange(!checked)}
        className={`relative mt-0.5 inline-flex h-6 w-11 shrink-0 items-center rounded-full focus:outline-none focus:ring-2 focus:ring-brand-100 disabled:cursor-wait disabled:opacity-70 ${
          checked ? 'bg-brand-600' : 'bg-gray-300'
        }`}
      >
        <span
          className={`inline-block h-5 w-5 rounded-full bg-white shadow ${
            checked ? 'translate-x-[22px]' : 'translate-x-0.5'
          }`}
        />
      </button>
    </div>
  )
}

/** One entry of the list: the address, its note, when and by whom, and removal. */
function ExclusionRow({ item, isCurrent, onRemove }) {
  const text = entryName(item)
  const single = isSingleEntry(item)
  const label = typeof item?.label === 'string' ? item.label.trim() : ''
  const addedBy = typeof item?.created_by_email === 'string' ? item.created_by_email.trim() : ''
  const addedAt = formatDateTime(item?.created_at)
  // The adding account may have been deleted since (created_by is then null).
  const meta = [addedAt !== '—' ? addedAt : '', addedBy ? `Ekleyen: ${addedBy}` : '']
    .filter(Boolean)
    .join(' · ')

  return (
    <li className="flex items-start gap-3 px-3 py-2.5">
      <div className="min-w-0 flex-1">
        <p className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
          <span className="break-anywhere font-mono text-sm text-gray-900">{text}</span>
          {label ? <span className="break-anywhere text-sm text-gray-600">{label}</span> : null}
          {isCurrent ? (
            <span className="badge bg-brand-50 text-brand-700 ring-1 ring-inset ring-brand-100">
              {single ? 'Şu anki adresiniz' : 'Şu anki adresinizi kapsıyor'}
            </span>
          ) : null}
        </p>
        {meta ? <p className="mt-0.5 break-anywhere text-[11px] text-gray-500">{meta}</p> : null}
      </div>
      <button
        type="button"
        className="btn-ghost btn-sm shrink-0 text-red-600 hover:bg-red-50 hover:text-red-700"
        onClick={onRemove}
        aria-label={removeEntryLabel(item)}
      >
        <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
        <span className="hidden sm:inline">Çıkar</span>
      </button>
    </li>
  )
}
