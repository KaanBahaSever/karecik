import { useEffect, useRef, useState } from 'react'
import { AlertCircle, History, Lock, RefreshCw } from 'lucide-react'

import api from '../../lib/api'
import { formatIp, formatPort, ipSourceInfo } from '../../lib/analyticsFormat.js'
import {
  AUDIT_ENTITY_TYPES,
  auditActionLabel,
  auditActionTone,
  auditChangeRows,
  auditEntityLabel,
  auditNameLookup,
  truncateText,
} from '../../lib/auditFormat.js'
import EmptyState from '../../components/ui/EmptyState.jsx'
import Loading from '../../components/ui/Loading.jsx'
import DateTimeCell from '../../components/analytics/DateTimeCell.jsx'
import Pagination from '../../components/analytics/Pagination.jsx'

/**
 * "Değişiklik Geçmişi" - who changed what in the panel, and when: products and
 * prices, categories, menu settings (contact details, logos, footer notices),
 * the business record and the account password. Read from GET /api/audit-logs,
 * which the server writes on every such change; nothing here can edit it.
 *
 * It is a page of its own rather than a tab of "Analitik". The two answer
 * different questions with different filters - visitor traffic is scoped to a
 * menu and a date range, the history to a kind of record - and the history is
 * business-wide by nature: a password change or a business rename belongs to
 * no menu, so a page that shows a menu filter above it would misdescribe it.
 * Its own address also lets the owner come straight back to it.
 */

/** Rows per page; the server allows up to 200. */
const PAGE_SIZE = 50

/** Badge colours of the action tones (auditActionTone). */
const ACTION_CLASSES = {
  create: 'bg-emerald-50 text-emerald-700 ring-1 ring-inset ring-emerald-200',
  delete: 'bg-red-50 text-red-700 ring-1 ring-inset ring-red-200',
  price: 'bg-amber-50 text-amber-800 ring-1 ring-inset ring-amber-200',
  neutral: 'bg-gray-100 text-gray-700 ring-1 ring-inset ring-gray-200',
}

/**
 * One value of a change: cut to the preview length unless the entry is
 * expanded. A value that was cut carries its full text in its title too.
 */
function ChangeValue({ text, expanded, muted = false }) {
  const shown = expanded ? { text, truncated: false } : truncateText(text)
  return (
    <span
      className={`break-anywhere ${muted ? 'text-gray-500' : 'text-gray-900'}`}
      title={shown.truncated ? text : undefined}
    >
      {shown.text}
    </span>
  )
}

/**
 * The "Değişiklikler" cell: one line per field, "alan: eski → yeni".
 * A created record has no old value and a deleted one no new value, so those
 * lines print the one side they have. A category or menu id prints as that
 * record's name, from `names` (auditNameLookup).
 */
function Changes({ entry, names, expanded, onToggle }) {
  const rows = auditChangeRows(entry?.changes, { names })
  // A password change records only how it was made. Saying so outright
  // answers the question an owner reading the history is bound to ask.
  const passwordNote =
    entry?.action === 'account.password_change' ? (
      <p className="mt-1 text-[11px] text-gray-400">Şifreler hiçbir biçimde kaydedilmez.</p>
    ) : null

  if (rows.length === 0) {
    return passwordNote || <span className="text-xs text-gray-400">Ayrıntı yok</span>
  }

  const hasLong = rows.some((row) => row.long)

  return (
    <div>
      <ul className="space-y-1 text-xs leading-relaxed">
        {rows.map((row) => (
          <li key={row.field}>
            <span className="font-medium text-gray-600">{row.label}:</span>{' '}
            {row.hasBefore && row.hasAfter ? (
              <>
                <ChangeValue text={row.before} expanded={expanded} muted />
                <span className="mx-1 text-gray-400" aria-hidden="true">
                  →
                </span>
                <span className="sr-only">, yeni değer: </span>
                <ChangeValue text={row.after} expanded={expanded} />
              </>
            ) : row.hasAfter ? (
              <ChangeValue text={row.after} expanded={expanded} />
            ) : (
              <>
                <ChangeValue text={row.before} expanded={expanded} muted />
                <span className="ml-1 text-gray-400">(kaldırıldı)</span>
              </>
            )}
          </li>
        ))}
      </ul>

      {hasLong ? (
        <button
          type="button"
          onClick={onToggle}
          aria-expanded={expanded}
          className="mt-1 text-xs font-medium text-brand-700 hover:text-brand-800"
        >
          {expanded ? 'Daha az göster' : 'Tümünü göster'}
        </button>
      ) : null}

      {passwordNote}
    </div>
  )
}

export default function AuditLog() {
  const [entityType, setEntityType] = useState('')
  const [offset, setOffset] = useState(0)
  const [reloadKey, setReloadKey] = useState(0)

  // `names` names the category and menu ids the entries hold (auditNameLookup).
  const [data, setData] = useState({ items: [], total: 0, names: null })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  // Entry ids whose long values are shown in full.
  const [expanded, setExpanded] = useState(() => new Set())
  const requestRef = useRef(0)

  useEffect(() => {
    // Numbered, so a slow answer for an older filter never overwrites a newer one.
    const request = requestRef.current + 1
    requestRef.current = request
    setLoading(true)
    setError('')

    Promise.all([
      api.auditLogs({ entity_type: entityType, limit: PAGE_SIZE, offset }),
      // The menus and categories the entries' ids are named from, fetched with
      // every page of entries so a record created since the last one is never
      // mistaken for a deleted one. Straight from the API rather than from the
      // active-menu bar's list, which another tab can leave behind. A list that
      // fails costs only its names ("Bilinmeyen …"), never the history.
      api.listMenus().catch(() => null),
      api.listCategories().catch(() => null),
    ])
      .then(([result, menus, categories]) => {
        if (request !== requestRef.current) return
        setData({
          items: Array.isArray(result?.items) ? result.items : [],
          total: Number(result?.total) || 0,
          names: auditNameLookup({ menus, categories }),
        })
      })
      .catch((err) => {
        if (request !== requestRef.current) return
        setData({ items: [], total: 0, names: null })
        setError(err.message || 'Değişiklik geçmişi yüklenemedi.')
      })
      .finally(() => {
        if (request === requestRef.current) setLoading(false)
      })
  }, [entityType, offset, reloadKey])

  function toggle(id) {
    setExpanded((previous) => {
      const next = new Set(previous)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const { items, total, names } = data

  return (
    <div className="pb-10">
      <div className="mb-6">
        <h1 className="text-2xl font-semibold text-gray-900">Değişiklik Geçmişi</h1>
        <p className="mt-1 text-sm text-gray-500">
          Panelde yapılan önemli değişiklikler: ürün ve fiyat düzenlemeleri, kategoriler, menü
          ayarları, iletişim bilgileri, logolar ve hesap işlemleri. Kim, ne zaman, neyi değiştirdi
          burada görünür.
        </p>
      </div>

      <section className="card" aria-labelledby="audit-title">
        <div className="flex flex-wrap items-end justify-between gap-4 border-b border-gray-100 p-4">
          <h2 id="audit-title" className="text-base font-semibold text-gray-900">
            Kayıtlar
          </h2>

          <div className="flex flex-wrap items-end gap-3">
            <div>
              <label className="mb-1 block text-xs font-medium text-gray-600" htmlFor="audit-entity">
                Kayıt türü
              </label>
              <select
                id="audit-entity"
                className="input py-2"
                value={entityType}
                onChange={(event) => {
                  setEntityType(event.target.value)
                  setOffset(0)
                }}
              >
                <option value="">Tümü</option>
                {AUDIT_ENTITY_TYPES.map((entity) => (
                  <option key={entity.id} value={entity.id}>
                    {entity.label}
                  </option>
                ))}
              </select>
            </div>

            <button
              type="button"
              className="btn-secondary py-2"
              onClick={() => setReloadKey((key) => key + 1)}
              disabled={loading}
            >
              <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} aria-hidden="true" />
              Yenile
            </button>
          </div>
        </div>

        {error ? (
          <div className="p-4">
            <div
              className="flex flex-wrap items-start gap-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2.5"
              role="alert"
            >
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-red-600" aria-hidden="true" />
              <p className="min-w-0 flex-1 text-sm text-red-800">{error}</p>
              <button
                type="button"
                className="btn-secondary btn-sm"
                onClick={() => setReloadKey((key) => key + 1)}
              >
                <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
                Tekrar dene
              </button>
            </div>
          </div>
        ) : null}

        {loading && items.length === 0 && !error ? (
          <Loading text="Değişiklik geçmişi yükleniyor..." />
        ) : items.length === 0 && !error ? (
          <div className="p-4">
            <EmptyState
              icon={History}
              title={entityType ? 'Bu türde kayıt yok' : 'Henüz kayıtlı bir değişiklik yok'}
              description={
                entityType
                  ? 'Başka bir kayıt türü seçmeyi deneyin.'
                  : 'Ürün, fiyat, menü ayarı ya da hesap değişikliği yaptığınızda burada listelenir.'
              }
            />
          </div>
        ) : items.length > 0 ? (
          /* A refetch keeps the current page on screen, dimmed. The table
             scrolls sideways inside its card on a narrow screen; `relative`
             keeps its sr-only texts (absolutely positioned) inside that
             scroller, or they would widen the whole page. */
          <div className={`relative overflow-x-auto ${loading ? 'opacity-60' : ''}`}>
            <table className="w-full min-w-[860px] text-left text-sm">
              <thead className="bg-gray-50 text-xs text-gray-500">
                <tr>
                  <th scope="col" className="px-4 py-2.5 font-medium">
                    Tarih
                  </th>
                  <th scope="col" className="px-3 py-2.5 font-medium">
                    Kullanıcı
                  </th>
                  <th scope="col" className="px-3 py-2.5 font-medium">
                    İşlem
                  </th>
                  <th scope="col" className="px-3 py-2.5 font-medium">
                    Kayıt
                  </th>
                  <th scope="col" className="w-[40%] px-3 py-2.5 font-medium">
                    Değişiklikler
                  </th>
                  <th scope="col" className="px-4 py-2.5 font-medium">
                    IP / Port
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {items.map((entry, index) => {
                  const id = entry?.id || `row-${offset + index}`
                  const tone = auditActionTone(entry?.action)
                  const source = ipSourceInfo(entry?.ip_source)
                  const port = formatPort(entry?.port)
                  const entityLabel = auditEntityLabel(entry?.entity_type)
                  return (
                    <tr key={id} className="align-top">
                      <td className="whitespace-nowrap px-4 py-3 text-gray-700 tabular-nums">
                        <DateTimeCell value={entry?.created_at} />
                      </td>
                      <td className="px-3 py-3">
                        <span
                          className="block max-w-[12rem] truncate text-gray-900"
                          title={entry?.user_email || entry?.user_id || ''}
                        >
                          {entry?.user_email || '—'}
                        </span>
                      </td>
                      <td className="px-3 py-3">
                        <span className={`badge whitespace-nowrap ${ACTION_CLASSES[tone]}`}>
                          {auditActionLabel(entry?.action)}
                        </span>
                      </td>
                      <td className="px-3 py-3">
                        <span
                          className="block max-w-[14rem] truncate font-medium text-gray-900"
                          title={entry?.entity_label || ''}
                        >
                          {entry?.entity_label || '—'}
                        </span>
                        {entityLabel ? (
                          <span className="text-[11px] text-gray-400">{entityLabel}</span>
                        ) : null}
                      </td>
                      <td className="px-3 py-3">
                        <Changes
                          entry={entry}
                          names={names}
                          expanded={expanded.has(id)}
                          onToggle={() => toggle(id)}
                        />
                      </td>
                      <td className="whitespace-nowrap px-4 py-3 font-mono text-xs text-gray-700">
                        <span className="block" title={source.description}>
                          {formatIp(entry?.ip)}
                        </span>
                        <span className="block text-gray-400">
                          {port === '—' ? 'port —' : `port ${port}`}
                        </span>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        ) : null}

        <Pagination
          total={total}
          limit={PAGE_SIZE}
          offset={offset}
          onChange={setOffset}
          disabled={loading}
        />
      </section>

      <p className="help-text mt-3 flex items-start gap-1.5">
        <Lock className="mt-0.5 h-3.5 w-3.5 shrink-0 text-gray-400" aria-hidden="true" />
        <span>
          Wi-Fi şifreleri geçmişte maskelenmiş olarak (••••) görünür; hesap şifreleri hiçbir
          biçimde kaydedilmez. IP adresleri yalnızca güvenlik amacıyla, sınırlı bir süre saklanır.
        </span>
      </p>
    </div>
  )
}
