import { useEffect, useMemo, useRef, useState } from 'react'
import { BarChart3, EyeOff, Info, RefreshCw, Search, ShieldCheck, X } from 'lucide-react'

import api from '../../lib/api'
import { browserCookies, hasAnalyticsOptout } from '../../lib/analytics.js'
import { useActiveMenu } from '../../lib/menuContext.jsx'
import {
  DEFAULT_RANGE_PRESET,
  dayInZone,
  daysInRange,
  EVENT_TYPES,
  eventTypeLabel,
  formatCount,
  formatIp,
  formatPort,
  formatRange,
  IP_SOURCES,
  ipSourceInfo,
  presetRange,
  shortUserAgent,
} from '../../lib/analyticsFormat.js'
import {
  checkExclusion,
  excludableAddress,
  exclusionMatcher,
  readExclusionState,
  withAddedEntry,
  withoutEntry,
} from '../../lib/ipExclusion.js'
import { findLanguage } from '../../locales/index.js'
import EmptyState from '../../components/ui/EmptyState.jsx'
import Loading from '../../components/ui/Loading.jsx'
import { useToast } from '../../components/ui/Toast.jsx'
import DailyVisitsChart from '../../components/analytics/DailyVisitsChart.jsx'
import DateRangeFilter from '../../components/analytics/DateRangeFilter.jsx'
import DateTimeCell from '../../components/analytics/DateTimeCell.jsx'
import ErrorNote from '../../components/analytics/ErrorNote.jsx'
import ExcludedIpsSection from '../../components/analytics/ExcludedIpsSection.jsx'
import ExcludeIpDialog from '../../components/analytics/ExcludeIpDialog.jsx'
import Pagination from '../../components/analytics/Pagination.jsx'

/**
 * "Analitik" - how the customer menus are used: the headline counts, a daily
 * chart, the most viewed categories and products, and the visit log behind
 * them (public address, source port and where that address came from).
 *
 * Every number comes from the events the customer menu reports through
 * POST /api/public/events; this page only reads them (GET /api/analytics/...).
 *
 * One filter row at the top scopes EVERYTHING below it - the cards, the chart,
 * the two top lists and the visit log - so the numbers on the page always
 * agree with each other. The log adds two filters of its own (event type and
 * address), which narrow the log only.
 *
 * Unlike the editing pages it does not follow the menu chosen in the active
 * menu bar: "every menu" is a real answer here, and the page opens on it.
 *
 * At the bottom, "Hariç tutulan IP'ler" (ExcludedIpsSection) keeps the
 * owner's own visits out of all of the above: an IP list and a per-browser
 * switch, both enforced by the server before anything is stored. An address
 * can be put on the list from three places - the section's form, its
 * "Listeye ekle" for the current address, and the "Hariç tut" of a visit log
 * row - and all three open the same dialog (ExcludeIpDialog), which asks
 * what to do with the visits that address already made. That is why the list
 * and the dialog live here, on the page, rather than inside the section: the
 * visit log needs the list too, to mark the rows already covered.
 */

/** Rows per page of the visit log; the server allows up to 200. */
const EVENTS_PAGE_SIZE = 50

/**
 * The four headline counts, in the order the cards show them (C5 field names).
 * The summary also sends `menu_views`, but that is `total_visits` by contract -
 * every opening of the menu is one visit - so it has no card of its own: two
 * cards that can never differ would send the owner looking for a difference.
 */
const KPI_CARDS = [
  { key: 'total_visits', label: 'Toplam ziyaret', hint: 'Her menü açılışı bir ziyaret sayılır' },
  { key: 'unique_visitors', label: 'Tekil ziyaretçi', hint: 'Farklı ziyaretçi, yaklaşık' },
  { key: 'category_views', label: 'Kategori görüntüleme', hint: 'Açılan kategoriler' },
  { key: 'product_views', label: 'Ürün detayı açılışı', hint: 'Açılan ürün detayları' },
]

/** Badge colours of the address sources (ipSourceInfo tones). */
const TONE_CLASSES = {
  good: 'bg-emerald-50 text-emerald-700 ring-1 ring-inset ring-emerald-200',
  warn: 'bg-amber-50 text-amber-800 ring-1 ring-inset ring-amber-200',
  neutral: 'bg-gray-100 text-gray-600 ring-1 ring-inset ring-gray-200',
}

/* ------------------------------------------------------------ small pieces */

/** One headline number. Proportional figures: a big "121" should not look loose. */
function StatTile({ label, hint, value, loading }) {
  return (
    <div className="card p-4">
      <p className="text-xs font-medium text-gray-500">{label}</p>
      <p className={`mt-1.5 text-2xl font-semibold text-gray-900 ${loading ? 'opacity-50' : ''}`}>
        {value === null ? '—' : formatCount(value)}
      </p>
      <p className="mt-0.5 text-[11px] text-gray-400">{hint}</p>
    </div>
  )
}

/**
 * The ten most viewed categories or products. Each row carries a thin bar -
 * its share of the first row - so the ranking reads at a glance; the number
 * beside it is the value itself.
 */
function TopList({ title, rows, nameOf, detailOf, loading, emptyText }) {
  const list = Array.isArray(rows) ? rows : []
  const top = list.reduce((max, row) => Math.max(max, Number(row?.views) || 0), 0)

  return (
    <section className={`card p-5 ${loading ? 'opacity-60' : ''}`}>
      <h2 className="mb-3 text-base font-semibold text-gray-900">{title}</h2>

      {list.length === 0 ? (
        <p className="rounded-lg border border-dashed border-gray-300 px-3 py-6 text-center text-sm text-gray-500">
          {emptyText}
        </p>
      ) : (
        <ol className="space-y-2.5">
          {list.map((row, index) => {
            const views = Number(row?.views) || 0
            const detail = detailOf ? detailOf(row) : ''
            return (
              <li key={row?.id || index} className="flex items-start gap-3">
                <span className="w-5 shrink-0 pt-0.5 text-right text-xs text-gray-400 tabular-nums">
                  {index + 1}.
                </span>
                <div className="min-w-0 flex-1">
                  <div className="flex items-baseline justify-between gap-3">
                    <p className="min-w-0 truncate text-sm text-gray-900" title={nameOf(row)}>
                      {nameOf(row)}
                      {detail ? <span className="text-gray-400"> · {detail}</span> : null}
                    </p>
                    <span className="shrink-0 text-sm font-medium text-gray-900 tabular-nums">
                      {formatCount(views)}
                    </span>
                  </div>
                  <div className="mt-1 h-1.5 rounded-full bg-brand-50" aria-hidden="true">
                    <div
                      className="h-1.5 rounded-full bg-brand-500"
                      style={{ width: `${top > 0 ? Math.max(2, (views / top) * 100) : 0}%` }}
                    />
                  </div>
                </div>
              </li>
            )
          })}
        </ol>
      )}
    </section>
  )
}

/**
 * "Kaynak" cell: the source as a small badge with its qualifier ("doğrulanmış",
 * "doğrulanmamış") on a line of its own under it - two short lines keep the
 * log narrow enough for a laptop - and the full explanation on hover and for
 * screen readers.
 */
function SourceBadge({ source }) {
  const info = ipSourceInfo(source)
  return (
    <span className="inline-block" title={info.description}>
      <span
        className={`badge whitespace-nowrap ${TONE_CLASSES[info.tone] || TONE_CLASSES.neutral}`}
      >
        {info.label}
      </span>
      {info.detail ? (
        <span className="mt-0.5 block text-[11px] text-gray-500">{info.detail}</span>
      ) : null}
      <span className="sr-only"> ({info.description})</span>
    </span>
  )
}

/** "Kategori / Ürün" cell: what the visitor opened, product first. */
function TargetCell({ event }) {
  if (event?.type === 'product_view') {
    return (
      <>
        <span className="block max-w-[14rem] truncate text-gray-900" title={event.product_name || ''}>
          {event.product_name || 'Silinmiş ürün'}
        </span>
        {event.category_name ? (
          <span className="block max-w-[14rem] truncate text-[11px] text-gray-400">
            {event.category_name}
          </span>
        ) : null}
      </>
    )
  }
  if (event?.type === 'category_view') {
    return (
      <span className="block max-w-[14rem] truncate text-gray-900" title={event.category_name || ''}>
        {event.category_name || 'Silinmiş kategori'}
      </span>
    )
  }
  return <span className="text-gray-400">—</span>
}

/**
 * "IP adresi" cell: the address, and under it the one-click way to stop
 * recording it - or, when the list already covers it, a quiet "Listede"
 * (the row itself stays: the owner chose to keep the past visits). An unknown
 * address ("-") gets neither: there is nothing to exclude.
 */
function IpCell({ ip, coveredBy, onExclude }) {
  const address = excludableAddress(ip)
  return (
    <>
      <span className="block">{formatIp(ip)}</span>
      {!address ? null : coveredBy ? (
        <span
          className="mt-0.5 block font-sans text-[11px] text-gray-400"
          title={`Hariç tutulanlar listesinde (${
            coveredBy.display || coveredBy.cidr
          }); bu adresten yeni ziyaretler kaydedilmiyor.`}
        >
          Listede
        </span>
      ) : (
        <button
          type="button"
          onClick={() => onExclude(address)}
          className="-mx-1 mt-0.5 inline-flex items-center gap-1 rounded px-1 py-0.5 font-sans text-[11px] font-medium text-brand-700 hover:bg-brand-50 hover:text-brand-800 focus:outline-none focus:ring-2 focus:ring-brand-100"
          aria-label={`${address} adresini hariç tut`}
        >
          <EyeOff className="h-3 w-3" aria-hidden="true" />
          Hariç tut
        </button>
      )}
    </>
  )
}

/* -------------------------------------------------------------------- page */

export default function Analytics() {
  const { menus, hasMenus, loading: menusLoading, error: menusError, reload: reloadMenus } =
    useActiveMenu()
  const toast = useToast()

  /* ------------------------------------------------------------ filters */

  const [menuId, setMenuId] = useState('') // '' = every menu of the business
  const [range, setRange] = useState(() => presetRange(DEFAULT_RANGE_PRESET))
  const [eventType, setEventType] = useState('')
  const [ipInput, setIpInput] = useState('')
  const [ipQuery, setIpQuery] = useState('')
  const [offset, setOffset] = useState(0)
  // Bumped by "Yenile" and the retry buttons: refetches with the same filters.
  const [reloadKey, setReloadKey] = useState(0)

  // A menu deleted in another tab drops out of the list; the filter falls back
  // to every menu instead of asking the server about a menu that is gone.
  const scopedMenuId = menuId && menus.some((menu) => menu.id === menuId) ? menuId : ''

  /* -------------------------------------------------------------- summary */

  const [summary, setSummary] = useState(null)
  const [summaryLoading, setSummaryLoading] = useState(true)
  const [summaryError, setSummaryError] = useState('')
  const summaryRequest = useRef(0)

  useEffect(() => {
    // Nothing to measure yet: the menu list is still loading, or there is none.
    if (!hasMenus) return

    // Numbered, so a slow answer for an older filter never overwrites a newer one.
    const request = summaryRequest.current + 1
    summaryRequest.current = request
    setSummaryLoading(true)
    setSummaryError('')

    api
      .analyticsSummary({ menu_id: scopedMenuId, from: range.from, to: range.to })
      .then((data) => {
        if (request !== summaryRequest.current) return
        setSummary(data && typeof data === 'object' ? data : null)
      })
      .catch((err) => {
        if (request !== summaryRequest.current) return
        // The last answer belongs to another filter; showing it under this one would lie.
        setSummary(null)
        setSummaryError(err.message || 'İstatistikler yüklenemedi.')
      })
      .finally(() => {
        if (request === summaryRequest.current) setSummaryLoading(false)
      })
  }, [hasMenus, scopedMenuId, range.from, range.to, reloadKey])

  /* ------------------------------------------------------------ visit log */

  const [events, setEvents] = useState({ items: [], total: 0 })
  const [eventsLoading, setEventsLoading] = useState(true)
  const [eventsError, setEventsError] = useState('')
  const eventsRequest = useRef(0)

  useEffect(() => {
    if (!hasMenus) return

    const request = eventsRequest.current + 1
    eventsRequest.current = request
    setEventsLoading(true)
    setEventsError('')

    api
      .analyticsEvents({
        menu_id: scopedMenuId,
        type: eventType,
        from: range.from,
        to: range.to,
        ip: ipQuery,
        limit: EVENTS_PAGE_SIZE,
        offset,
      })
      .then((data) => {
        if (request !== eventsRequest.current) return
        setEvents({
          items: Array.isArray(data?.items) ? data.items : [],
          total: Number(data?.total) || 0,
        })
      })
      .catch((err) => {
        if (request !== eventsRequest.current) return
        setEvents({ items: [], total: 0 })
        setEventsError(err.message || 'Ziyaret kayıtları yüklenemedi.')
      })
      .finally(() => {
        if (request === eventsRequest.current) setEventsLoading(false)
      })
  }, [hasMenus, scopedMenuId, eventType, range.from, range.to, ipQuery, offset, reloadKey])

  /* ----------------------------------------------------- exclusion list */

  // readExclusionState of GET /api/analytics/excluded-ips; null until loaded.
  const [exclusions, setExclusions] = useState(null)
  const [exclusionsLoading, setExclusionsLoading] = useState(true)
  const [exclusionsError, setExclusionsError] = useState('')
  // Bumped after a removal or a switch change: refetches the list alone.
  const [exclusionsKey, setExclusionsKey] = useState(0)
  const exclusionsRequest = useRef(0)
  // Whether this browser is opted out. The cookie answers at once; the server
  // (which sees the cookie the request carried) answers once the list loads.
  const [optout, setOptout] = useState(() => hasAnalyticsOptout(browserCookies()))
  // The entry the add dialog is open for, or null.
  const [pendingExclusion, setPendingExclusion] = useState(null)
  const exclusionsSectionRef = useRef(null)

  useEffect(() => {
    if (!hasMenus) return

    const request = exclusionsRequest.current + 1
    exclusionsRequest.current = request
    setExclusionsLoading(true)
    setExclusionsError('')

    api
      .excludedIps()
      .then((data) => {
        if (request !== exclusionsRequest.current) return
        const state = readExclusionState(data)
        setExclusions(state)
        setOptout(state.optout)
      })
      .catch((err) => {
        if (request !== exclusionsRequest.current) return
        // The last list stays on screen: it was true a moment ago, and the
        // server checks every add and removal again anyway.
        setExclusionsError(err.message || "Hariç tutulan IP'ler yüklenemedi.")
      })
      .finally(() => {
        if (request === exclusionsRequest.current) setExclusionsLoading(false)
      })
  }, [hasMenus, reloadKey, exclusionsKey])

  // Which entry covers each address of the visit log; the list is read once.
  const matchExclusion = useMemo(() => exclusionMatcher(exclusions?.items), [exclusions])

  /**
   * Opens the add dialog for `input` ({ cidr, label }) when it passes every
   * rule the page can check, and says why not otherwise. `afterAdd` runs once
   * the entry is really on the list - the form clears itself then.
   */
  function requestExclusion(input, afterAdd) {
    const { value, problem, field } = checkExclusion(input, {
      items: exclusions ? exclusions.items : null,
      max: exclusions?.max,
    })
    if (!value) return { problem, field }
    setPendingExclusion({ ...value, afterAdd })
    return { problem: '', field: '' }
  }

  /** "Hariç tut" of a visit log row: the same dialog, the row's address in it. */
  function excludeFromLog(address) {
    const { problem } = requestExclusion({ cidr: address, label: '' })
    if (problem) toast.error(problem)
  }

  function exclusionAdded({ item, message }) {
    const afterAdd = pendingExclusion?.afterAdd
    setPendingExclusion(null)
    // Last, where the server's oldest-first order puts it, so the reload
    // below confirms the row in place instead of moving it.
    setExclusions((previous) => withAddedEntry(previous, item))
    if (afterAdd) afterAdd()
    toast.success(message)
    // Everything on the page may have changed - the past visits may be gone -
    // so the cards, the chart, the top lists, the log and the list all reload.
    setReloadKey((key) => key + 1)
  }

  function exclusionRemoved(item) {
    setExclusions((previous) => withoutEntry(previous, item?.id))
    setExclusionsKey((key) => key + 1)
  }

  function optoutChanged(next) {
    setOptout(next)
    // A list request already on its way carried the old cookie; the refetch
    // supersedes it, so its stale `optout` never flips the switch back.
    setExclusionsKey((key) => key + 1)
  }

  function scrollToExclusions() {
    exclusionsSectionRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  /* A filter change starts the log at its first page again: page 4 of the old
     filter means nothing under the new one. */
  function changeMenu(value) {
    setMenuId(value)
    setOffset(0)
  }
  function changeRange(value) {
    setRange(value)
    setOffset(0)
  }
  function changeEventType(value) {
    setEventType(value)
    setOffset(0)
  }
  function searchIp(event) {
    event.preventDefault()
    setIpQuery(ipInput.trim())
    setOffset(0)
  }
  function clearIp() {
    setIpInput('')
    setIpQuery('')
    setOffset(0)
  }

  const menuNames = useMemo(() => new Map(menus.map((menu) => [menu.id, menu.name])), [menus])

  /* ------------------------------------------------------------- render */

  if (menusLoading && !hasMenus) {
    return <Loading text="Analitik yükleniyor..." />
  }

  if (!hasMenus) {
    return (
      <div className="pb-10">
        <PageHeader range={range} />
        <EmptyState
          icon={BarChart3}
          title={menusError ? 'Menüler yüklenemedi' : 'Henüz ölçülecek bir menü yok'}
          description={
            menusError ||
            'Bir menü oluşturup yayınladığınızda müşterilerinizin ziyaretleri burada görünür.'
          }
          action={
            menusError ? (
              <button type="button" className="btn-secondary btn-sm" onClick={reloadMenus}>
                Tekrar dene
              </button>
            ) : null
          }
        />
      </div>
    )
  }

  const dayCount = daysInRange(range.from, range.to)
  const hasSummary = summary !== null

  return (
    <div className="pb-10">
      <PageHeader range={range} />

      {/* ------------------------------------------------------ filter row */}
      <div className="card mb-6 flex flex-wrap items-start gap-x-6 gap-y-4 p-4">
        <div className="w-full sm:w-60">
          <label className="label" htmlFor="analytics-menu">
            Menü
          </label>
          <select
            id="analytics-menu"
            className="input"
            value={scopedMenuId}
            onChange={(event) => changeMenu(event.target.value)}
          >
            <option value="">Tüm menüler</option>
            {menus.map((menu) => (
              <option key={menu.id} value={menu.id}>
                {menu.name}
                {menu.is_active === false ? ' (yayında değil)' : ''}
              </option>
            ))}
          </select>
        </div>

        <DateRangeFilter value={range} onChange={changeRange} />

        <div className="sm:ml-auto sm:self-end">
          <button
            type="button"
            className="btn-secondary"
            onClick={() => setReloadKey((key) => key + 1)}
            disabled={summaryLoading && eventsLoading}
          >
            <RefreshCw
              className={`h-4 w-4 ${summaryLoading || eventsLoading ? 'animate-spin' : ''}`}
              aria-hidden="true"
            />
            Yenile
          </button>
        </div>
      </div>

      {summaryError ? (
        <div className="mb-6">
          <ErrorNote message={summaryError} onRetry={() => setReloadKey((key) => key + 1)} />
        </div>
      ) : null}

      {/* ---------------------------------------------------------- KPI row
          Two by two up to a laptop, one row of four from there: four cards
          never leave an orphan in either grid. */}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {KPI_CARDS.map((card) => (
          <StatTile
            key={card.key}
            label={card.label}
            hint={card.hint}
            value={hasSummary ? Number(summary[card.key]) || 0 : null}
            loading={summaryLoading}
          />
        ))}
      </div>
      <p className="help-text mt-2 flex items-start gap-1.5">
        <Info className="mt-0.5 h-3.5 w-3.5 shrink-0 text-gray-400" aria-hidden="true" />
        <span>
          Tekil ziyaretçi, tarayıcıya özgü anonim bir kimlikle (o yoksa IP adresi ve tarayıcı
          bilgisiyle) yaklaşık olarak hesaplanır. Günler Türkiye saatine göre sayılır. Kendi
          ziyaretlerinizin sayılmaması için{' '}
          <button
            type="button"
            onClick={scrollToExclusions}
            className="font-medium text-brand-700 underline decoration-brand-200 underline-offset-2 hover:text-brand-800"
          >
            hariç tutulan IP'ler
          </button>{' '}
          bölümünü kullanın.
        </span>
      </p>

      {/* ------------------------------------------------------------ chart */}
      <section className="card mt-6 p-5">
        <div className="mb-4 flex flex-wrap items-baseline justify-between gap-2">
          <h2 className="text-base font-semibold text-gray-900">Günlük ziyaretler</h2>
          <span className="text-xs text-gray-500">{formatRange(range.from, range.to)}</span>
        </div>

        {!hasSummary && summaryLoading ? (
          <Loading text="Grafik hazırlanıyor..." />
        ) : !hasSummary ? (
          <p className="py-10 text-center text-sm text-gray-500">Grafik yüklenemedi.</p>
        ) : dayCount < 2 ? (
          /* One day is one column - the cards above already say it better.
             The day is only "today" when it is; a custom single day is not. */
          <p className="rounded-lg bg-gray-50 px-3 py-6 text-center text-sm text-gray-500">
            Günlük grafik en az iki günlük bir aralıkta gösterilir.{' '}
            {range.from === dayInZone() ? 'Bugünün' : 'Seçilen günün'} sayıları yukarıdaki
            kartlarda.
          </p>
        ) : (
          /* A refetch keeps the last chart on screen, dimmed - no flash, no jump. */
          <div className={summaryLoading ? 'opacity-50' : ''}>
            <DailyVisitsChart daily={summary.daily} from={range.from} to={range.to} />
          </div>
        )}
      </section>

      {/* -------------------------------------------------------- top lists */}
      <div className="mt-6 grid gap-6 lg:grid-cols-2">
        <TopList
          title="En çok görüntülenen kategoriler"
          rows={summary?.top_categories}
          nameOf={(row) => row?.name || 'Silinmiş kategori'}
          loading={summaryLoading}
          emptyText="Bu aralıkta kategori görüntülemesi yok."
        />
        <TopList
          title="En çok açılan ürünler"
          rows={summary?.top_products}
          nameOf={(row) => row?.name || 'Silinmiş ürün'}
          detailOf={(row) => row?.category_name || ''}
          loading={summaryLoading}
          emptyText="Bu aralıkta ürün detayı açılmadı."
        />
      </div>

      {/* -------------------------------------------------------- visit log */}
      <section className="card mt-6" aria-labelledby="visit-log-title">
        <div className="flex flex-wrap items-end justify-between gap-4 border-b border-gray-100 p-4">
          <div className="min-w-0">
            <h2 id="visit-log-title" className="text-base font-semibold text-gray-900">
              Ziyaret kayıtları
            </h2>
            <p className="help-text">
              Menülerinizdeki her görüntüleme, en yenisi en üstte. Tarih ve saatler Türkiye
              saatidir.
            </p>
          </div>

          <div className="flex flex-wrap items-end gap-3">
            <div>
              <label className="mb-1 block text-xs font-medium text-gray-600" htmlFor="event-type">
                Olay
              </label>
              <select
                id="event-type"
                className="input py-2"
                value={eventType}
                onChange={(event) => changeEventType(event.target.value)}
              >
                <option value="">Tümü</option>
                {EVENT_TYPES.map((type) => (
                  <option key={type.id} value={type.id}>
                    {type.label}
                  </option>
                ))}
              </select>
            </div>

            <form className="flex items-end gap-2" onSubmit={searchIp} role="search">
              <div>
                <label className="mb-1 block text-xs font-medium text-gray-600" htmlFor="event-ip">
                  IP adresi
                </label>
                <div className="relative">
                  <input
                    id="event-ip"
                    type="search"
                    inputMode="text"
                    className="input w-48 py-2 pr-8 font-mono text-xs"
                    value={ipInput}
                    maxLength={45}
                    placeholder="örn. 203.0.113.7"
                    autoComplete="off"
                    spellCheck={false}
                    onChange={(event) => setIpInput(event.target.value)}
                  />
                  {ipInput || ipQuery ? (
                    <button
                      type="button"
                      onClick={clearIp}
                      className="absolute inset-y-0 right-0 flex w-8 items-center justify-center text-gray-400 hover:text-gray-600"
                      aria-label="IP aramasını temizle"
                    >
                      <X className="h-3.5 w-3.5" aria-hidden="true" />
                    </button>
                  ) : null}
                </div>
              </div>
              <button type="submit" className="btn-secondary py-2">
                <Search className="h-4 w-4" aria-hidden="true" />
                Ara
              </button>
            </form>
          </div>
        </div>

        {eventsError ? (
          <div className="p-4">
            <ErrorNote message={eventsError} onRetry={() => setReloadKey((key) => key + 1)} />
          </div>
        ) : null}

        {eventsLoading && events.items.length === 0 && !eventsError ? (
          <Loading text="Kayıtlar yükleniyor..." />
        ) : events.items.length === 0 && !eventsError ? (
          <div className="p-4">
            <EmptyState
              icon={BarChart3}
              title="Bu filtrelerle kayıt yok"
              description={
                ipQuery || eventType
                  ? 'Olay türü ya da IP filtresini değiştirmeyi deneyin.'
                  : 'Seçtiğiniz tarih aralığında menülerinize bir ziyaret kaydedilmedi.'
              }
            />
          </div>
        ) : events.items.length > 0 ? (
          /* The table scrolls sideways inside its card on a narrow screen.
             `relative` keeps the sr-only texts in it (absolutely positioned)
             inside that scroller - without it they would widen the page. */
          <div className={`relative overflow-x-auto ${eventsLoading ? 'opacity-60' : ''}`}>
            <table className="w-full min-w-[880px] text-left text-sm">
              <thead className="bg-gray-50 text-xs text-gray-500">
                <tr>
                  <th scope="col" className="px-4 py-2.5 font-medium">
                    Tarih / Saat
                  </th>
                  <th scope="col" className="px-3 py-2.5 font-medium">
                    Olay
                  </th>
                  <th scope="col" className="px-3 py-2.5 font-medium">
                    Menü
                  </th>
                  <th scope="col" className="px-3 py-2.5 font-medium">
                    Kategori / Ürün
                  </th>
                  <th scope="col" className="px-3 py-2.5 font-medium">
                    IP adresi
                  </th>
                  <th scope="col" className="px-3 py-2.5 font-medium">
                    Port
                  </th>
                  <th scope="col" className="px-3 py-2.5 font-medium">
                    Kaynak
                  </th>
                  <th scope="col" className="px-3 py-2.5 font-medium">
                    Dil
                  </th>
                  <th scope="col" className="px-4 py-2.5 font-medium">
                    Tarayıcı
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {events.items.map((event, index) => {
                  const language = typeof event?.language === 'string' ? event.language : ''
                  const menuName =
                    event?.menu_name || menuNames.get(event?.menu_id) || 'Silinmiş menü'
                  return (
                    <tr key={event?.id || index} className="align-top">
                      <td className="whitespace-nowrap px-4 py-2.5 text-gray-700 tabular-nums">
                        <DateTimeCell value={event?.created_at} />
                      </td>
                      <td className="min-w-[7rem] px-3 py-2.5 text-gray-900">
                        {eventTypeLabel(event?.type)}
                      </td>
                      <td className="px-3 py-2.5 text-gray-700">
                        <span className="block max-w-[10rem] truncate" title={menuName}>
                          {menuName}
                        </span>
                      </td>
                      <td className="px-3 py-2.5">
                        <TargetCell event={event} />
                      </td>
                      <td className="whitespace-nowrap px-3 py-2.5 font-mono text-xs text-gray-900">
                        <IpCell
                          ip={event?.ip}
                          coveredBy={matchExclusion(event?.ip)}
                          onExclude={excludeFromLog}
                        />
                      </td>
                      <td className="whitespace-nowrap px-3 py-2.5 font-mono text-xs text-gray-700">
                        {formatPort(event?.port)}
                      </td>
                      <td className="px-3 py-2.5">
                        <SourceBadge source={event?.ip_source} />
                      </td>
                      <td
                        className="whitespace-nowrap px-3 py-2.5 text-xs text-gray-700"
                        title={language ? findLanguage(language).label : ''}
                      >
                        {language ? language.toUpperCase() : '—'}
                      </td>
                      <td
                        className="min-w-[8rem] px-4 py-2.5 text-xs text-gray-700"
                        title={event?.user_agent || ''}
                      >
                        {shortUserAgent(event?.user_agent)}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        ) : null}

        <Pagination
          total={events.total}
          limit={EVENTS_PAGE_SIZE}
          offset={offset}
          onChange={setOffset}
          disabled={eventsLoading}
        />

        {/* What each "Kaynak" means, for everyone who cannot hover. */}
        <details className="border-t border-gray-100 px-4 py-3 text-sm">
          <summary className="cursor-pointer select-none text-xs font-medium text-brand-700 hover:text-brand-800">
            “Kaynak” sütunu ne anlatıyor?
          </summary>
          <dl className="mt-2 space-y-2">
            {Object.keys(IP_SOURCES).map((id) => (
              <div key={id} className="flex flex-col gap-1 sm:flex-row sm:gap-3">
                <dt className="shrink-0 sm:w-56">
                  <SourceBadge source={id} />
                </dt>
                <dd className="text-xs leading-relaxed text-gray-600">
                  {ipSourceInfo(id).description}
                </dd>
              </div>
            ))}
          </dl>
          <p className="mt-2 text-xs leading-relaxed text-gray-600">
            Port yalnızca doğrulanmış Cloudflare kaynağında ve doğrudan bağlantıda kaydedilebilir;
            diğer kaynaklarda “—” görünür.
          </p>
        </details>

        {/* KVKK */}
        <p className="flex items-start gap-2 rounded-b-xl border-t border-gray-100 bg-gray-50 px-4 py-3 text-xs leading-relaxed text-gray-600">
          <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-gray-400" aria-hidden="true" />
          <span>
            <b className="font-medium text-gray-700">KVKK bilgilendirmesi:</b> IP adresi ve port
            bilgileri kişisel veri sayılabilir. Yalnızca güvenlik, kötüye kullanımın önlenmesi ve
            istatistik amacıyla sınırlı bir süre saklanır. Bu kayıtları bu amaçlar dışında
            kullanmayın ve üçüncü kişilerle paylaşmayın.
          </span>
        </p>
      </section>

      {/* ------------------------------------------------ excluded visits */}
      <ExcludedIpsSection
        ref={exclusionsSectionRef}
        state={exclusions}
        loading={exclusionsLoading}
        error={exclusionsError}
        onRetry={() => setExclusionsKey((key) => key + 1)}
        optout={optout}
        onOptoutChange={optoutChanged}
        onRequestAdd={requestExclusion}
        onRemoved={exclusionRemoved}
      />

      {pendingExclusion ? (
        <ExcludeIpDialog
          entry={pendingExclusion}
          onClose={() => setPendingExclusion(null)}
          onAdded={exclusionAdded}
          onConflict={() => setExclusionsKey((key) => key + 1)}
        />
      ) : null}
    </div>
  )
}

/** Title, one line of explanation and the range being shown. */
function PageHeader({ range }) {
  return (
    <div className="mb-6">
      <h1 className="text-2xl font-semibold text-gray-900">Analitik</h1>
      <p className="mt-1 text-sm text-gray-500">
        Müşterilerinizin menülerinizi ne sıklıkla açtığını, hangi kategori ve ürünlere baktığını
        görün
        {range ? (
          <>
            {' · '}
            <span className="whitespace-nowrap text-gray-700">
              {formatRange(range.from, range.to)}
            </span>
          </>
        ) : null}
        .
      </p>
    </div>
  )
}
