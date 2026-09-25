import { useId, useLayoutEffect, useMemo, useRef, useState } from 'react'

import {
  formatCount,
  formatDay,
  labelInterval,
  listDays,
  niceAxis,
} from '../../lib/analyticsFormat.js'

/*
  The daily chart of the analytics page: visits as columns, unique visitors as
  a line over them, one day per slot.

  Plain SVG, drawn at the real pixel width of its card - measured, not a
  stretched viewBox - so text never distorts and a phone gets the same crisp
  2px line as a desktop. No chart library: two series and an axis do not earn
  a dependency.

  The two measures share ONE axis on purpose. Both are counts of people-ish
  things over the same day, so their scales are comparable; a second axis
  would invent a relationship between them.

  Colours are the first two slots of the validated categorical palette
  (checked on the white card surface: CVD ΔE 24.7, both >= 3:1 contrast). The
  two series also differ in mark - column vs. line - so identity never rests
  on colour alone, and the legend above names both.
*/
const VISITS_COLOR = '#2a78d6'
const UNIQUE_COLOR = '#eb6834'
const SURFACE = '#ffffff'
const GRID_COLOR = '#e5e7eb' // gray-200: a hairline one step off the surface
const AXIS_TEXT = '#6b7280' // gray-500
const CROSSHAIR = '#9ca3af' // gray-400

const PLOT_HEIGHT = 180
const MARGIN = { top: 12, right: 12, bottom: 28 }
const MAX_BAR_WIDTH = 24
const CORNER = 4
const TOOLTIP_WIDTH = 188

/** Measures an element's content width and follows it as the layout changes. */
function useWidth(ref) {
  const [width, setWidth] = useState(0)

  useLayoutEffect(() => {
    const node = ref.current
    if (!node) return undefined

    const measure = () => setWidth(Math.floor(node.clientWidth))
    measure()

    if (typeof ResizeObserver === 'undefined') {
      window.addEventListener('resize', measure)
      return () => window.removeEventListener('resize', measure)
    }
    const observer = new ResizeObserver(measure)
    observer.observe(node)
    return () => observer.disconnect()
  }, [ref])

  return width
}

/**
 * A column with 4px rounded top corners and a square foot on the baseline.
 * Short columns get a smaller radius so the path never folds over itself.
 */
function columnPath(x, y, width, height) {
  if (height <= 0 || width <= 0) return ''
  const radius = Math.min(CORNER, width / 2, height)
  const bottom = y + height
  return [
    `M${x},${bottom}`,
    `V${y + radius}`,
    `Q${x},${y} ${x + radius},${y}`,
    `H${x + width - radius}`,
    `Q${x + width},${y} ${x + width},${y + radius}`,
    `V${bottom}`,
    'Z',
  ].join(' ')
}

/**
 * @param {Array}  daily - C5 `daily`: [{ date, visits, unique_visitors, category_views, product_views }]
 * @param {string} from  - first day of the range, 'YYYY-MM-DD'
 * @param {string} to    - last day of the range
 */
export default function DailyVisitsChart({ daily, from, to }) {
  const wrapperRef = useRef(null)
  const width = useWidth(wrapperRef)
  const [active, setActive] = useState(null) // index of the day under the pointer or the keyboard
  const hintId = useId()

  /* Every day of the range, zeros included. The API promises the same, but a
     day it left out would otherwise shift every later column one slot left. */
  const rows = useMemo(() => {
    const byDay = new Map(
      (Array.isArray(daily) ? daily : [])
        .filter((row) => row && typeof row.date === 'string')
        .map((row) => [row.date, row]),
    )
    return listDays(from, to).map((date) => {
      const row = byDay.get(date) || {}
      return {
        date,
        visits: Number(row.visits) || 0,
        unique: Number(row.unique_visitors) || 0,
        categoryViews: Number(row.category_views) || 0,
        productViews: Number(row.product_views) || 0,
      }
    })
  }, [daily, from, to])

  const count = rows.length
  const peak = rows.reduce((top, row) => Math.max(top, row.visits, row.unique), 0)
  const axis = niceAxis(peak)
  const ticks = []
  for (let value = 0; value <= axis.max; value += axis.step) ticks.push(value)

  // Room for the widest tick label on the left, measured in characters.
  const left = 12 + formatCount(axis.max).length * 7
  const plotWidth = Math.max(0, width - left - MARGIN.right)
  const band = count > 0 ? plotWidth / count : 0
  // Capped at 24px and never wider than 60% of the slot, so neighbouring
  // columns always keep a clear surface gap between them.
  const barWidth = Math.max(1, Math.min(MAX_BAR_WIDTH, band * 0.6))
  const every = labelInterval(count, plotWidth)

  const yOf = (value) => MARGIN.top + PLOT_HEIGHT - (value / axis.max) * PLOT_HEIGHT
  const xCenter = (index) => left + band * index + band / 2
  const height = MARGIN.top + PLOT_HEIGHT + MARGIN.bottom

  const linePoints = rows.map((row, index) => `${xCenter(index)},${yOf(row.unique)}`).join(' ')

  const busiest = rows.reduce(
    (best, row) => (!best || row.visits > best.visits ? row : best),
    null,
  )
  const totalVisits = rows.reduce((sum, row) => sum + row.visits, 0)
  const summary = busiest
    ? `Günlük ziyaret grafiği, ${formatDay(from, 'numeric')} ile ${formatDay(to, 'numeric')} arası: toplam ${formatCount(totalVisits)} ziyaret; en yoğun gün ${formatDay(busiest.date, 'long')} (${formatCount(busiest.visits)} ziyaret).`
    : 'Günlük ziyaret grafiği: gösterilecek gün yok.'

  const activeRow = active !== null && rows[active] ? rows[active] : null

  /** The day slot under a pointer x, relative to the wrapper. */
  function indexAt(clientX) {
    const box = wrapperRef.current?.getBoundingClientRect()
    if (!box || band <= 0) return null
    const index = Math.floor((clientX - box.left - left) / band)
    return Math.min(count - 1, Math.max(0, index))
  }

  function onKeyDown(event) {
    if (count === 0) return
    const current = active ?? count - 1
    const moves = {
      ArrowLeft: current - 1,
      ArrowRight: current + 1,
      Home: 0,
      End: count - 1,
    }
    if (!(event.key in moves)) return
    event.preventDefault()
    setActive(Math.min(count - 1, Math.max(0, moves[event.key])))
  }

  // The readout sits BESIDE the crosshair - right of it, or left of it near
  // the right edge - so it never covers the column it describes, and it stays
  // inside the card.
  let tooltipLeft = 0
  if (activeRow && width > 0) {
    const x = xCenter(active)
    tooltipLeft =
      x + 12 + TOOLTIP_WIDTH <= width
        ? x + 12
        : Math.max(0, Math.min(x - 12 - TOOLTIP_WIDTH, width - TOOLTIP_WIDTH))
  }

  return (
    <div>
      {/* Legend: always present for two series. The keys mirror the marks - a
          block for the columns, a stroke for the line. */}
      <div className="mb-3 flex flex-wrap items-center gap-x-5 gap-y-1 text-xs text-gray-600">
        <span className="inline-flex items-center gap-1.5">
          <span
            className="inline-block h-3 w-3 rounded-sm"
            style={{ backgroundColor: VISITS_COLOR }}
            aria-hidden="true"
          />
          Ziyaret
        </span>
        <span className="inline-flex items-center gap-1.5">
          <span
            className="inline-block h-0.5 w-4 rounded-full"
            style={{ backgroundColor: UNIQUE_COLOR }}
            aria-hidden="true"
          />
          Tekil ziyaretçi
        </span>
      </div>

      <div
        ref={wrapperRef}
        className="relative rounded-lg outline-none focus-visible:ring-2 focus-visible:ring-brand-100"
        style={{ height }}
        tabIndex={count > 0 ? 0 : -1}
        role="group"
        aria-label={summary}
        aria-describedby={hintId}
        onPointerMove={(event) => setActive(indexAt(event.clientX))}
        onPointerDown={(event) => setActive(indexAt(event.clientX))}
        onPointerLeave={(event) => {
          // A tap on a phone ends with a leave; keep that day's readout on screen.
          if (event.pointerType === 'mouse') setActive(null)
        }}
        onFocus={() => setActive((current) => current ?? (count > 0 ? count - 1 : null))}
        onBlur={() => setActive(null)}
        onKeyDown={onKeyDown}
      >
        {width > 0 ? (
          <svg width={width} height={height} className="block select-none" aria-hidden="true">
            {/* horizontal gridlines and their values */}
            {ticks.map((value) => (
              <g key={value}>
                <line
                  x1={left}
                  x2={width - MARGIN.right}
                  y1={yOf(value)}
                  y2={yOf(value)}
                  stroke={GRID_COLOR}
                  strokeWidth={1}
                  shapeRendering="crispEdges"
                />
                <text
                  x={left - 8}
                  y={yOf(value)}
                  textAnchor="end"
                  dominantBaseline="middle"
                  fontSize={11}
                  fill={AXIS_TEXT}
                  style={{ fontVariantNumeric: 'tabular-nums' }}
                >
                  {formatCount(value)}
                </text>
              </g>
            ))}

            {/* crosshair: behind the marks, so it never covers a value */}
            {activeRow ? (
              <line
                x1={xCenter(active)}
                x2={xCenter(active)}
                y1={MARGIN.top}
                y2={MARGIN.top + PLOT_HEIGHT}
                stroke={CROSSHAIR}
                strokeWidth={1}
                shapeRendering="crispEdges"
              />
            ) : null}

            {/* visits */}
            {rows.map((row, index) => {
              const top = yOf(row.visits)
              const path = columnPath(
                xCenter(index) - barWidth / 2,
                top,
                barWidth,
                MARGIN.top + PLOT_HEIGHT - top,
              )
              return path ? (
                <path
                  key={row.date}
                  d={path}
                  fill={VISITS_COLOR}
                  opacity={activeRow && active !== index ? 0.55 : 1}
                />
              ) : null
            })}

            {/* unique visitors: 2px, round joins, over the columns */}
            {count > 1 ? (
              <polyline
                points={linePoints}
                fill="none"
                stroke={UNIQUE_COLOR}
                strokeWidth={2}
                strokeLinejoin="round"
                strokeLinecap="round"
              />
            ) : null}

            {/* Markers only where they say something: the active day and the
                last one. A dot on each of 90 days would be noise. */}
            {rows.map((row, index) =>
              index === active || (active === null && index === count - 1) ? (
                <circle
                  key={row.date}
                  cx={xCenter(index)}
                  cy={yOf(row.unique)}
                  r={4}
                  fill={UNIQUE_COLOR}
                  stroke={SURFACE}
                  strokeWidth={2}
                />
              ) : null,
            )}

            {/* day labels, thinned to fit; the last day is always labelled */}
            {rows.map((row, index) =>
              (count - 1 - index) % every === 0 ? (
                <text
                  key={row.date}
                  x={xCenter(index)}
                  y={MARGIN.top + PLOT_HEIGHT + 18}
                  textAnchor={index === count - 1 && count > 1 ? 'end' : 'middle'}
                  dx={index === count - 1 && count > 1 ? Math.min(band / 2, 12) : 0}
                  fontSize={11}
                  fill={AXIS_TEXT}
                >
                  {formatDay(row.date)}
                </text>
              ) : null,
            )}
          </svg>
        ) : null}

        {/* The readout: values lead, the date follows. Tooltips enhance, they
            never gate - the table below holds every value too. */}
        {activeRow ? (
          <div
            className="pointer-events-none absolute top-0 z-10 rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs shadow-panel"
            style={{ left: tooltipLeft, width: TOOLTIP_WIDTH }}
          >
            <p className="mb-1.5 text-gray-500">{formatDay(activeRow.date, 'long')}</p>
            <p className="flex items-center justify-between gap-3">
              <span className="inline-flex items-center gap-1.5 text-gray-600">
                <span
                  className="inline-block h-0.5 w-3 rounded-full"
                  style={{ backgroundColor: VISITS_COLOR }}
                  aria-hidden="true"
                />
                Ziyaret
              </span>
              <b className="font-semibold text-gray-900 tabular-nums">{formatCount(activeRow.visits)}</b>
            </p>
            <p className="mt-0.5 flex items-center justify-between gap-3">
              <span className="inline-flex items-center gap-1.5 text-gray-600">
                <span
                  className="inline-block h-0.5 w-3 rounded-full"
                  style={{ backgroundColor: UNIQUE_COLOR }}
                  aria-hidden="true"
                />
                Tekil ziyaretçi
              </span>
              <b className="font-semibold text-gray-900 tabular-nums">{formatCount(activeRow.unique)}</b>
            </p>
          </div>
        ) : null}

        {/* What a screen reader hears as the keyboard moves from day to day. */}
        <p className="sr-only" aria-live="polite">
          {activeRow
            ? `${formatDay(activeRow.date, 'long')}: ${formatCount(activeRow.visits)} ziyaret, ${formatCount(activeRow.unique)} tekil ziyaretçi`
            : ''}
        </p>
      </div>

      <p id={hintId} className="mt-1 text-[11px] text-gray-400">
        Bir güne dokunun ya da grafiği seçip ok tuşlarıyla günler arasında gezinin.
      </p>

      {/* The table twin: every value of the chart without hovering. */}
      <details className="mt-3 text-sm">
        <summary className="cursor-pointer select-none text-xs font-medium text-brand-700 hover:text-brand-800">
          Günlük değerleri tablo olarak göster
        </summary>
        <div className="mt-2 max-h-72 overflow-auto rounded-lg border border-gray-200">
          <table className="w-full text-left text-xs">
            <thead className="sticky top-0 bg-gray-50 text-gray-500">
              <tr>
                <th scope="col" className="px-3 py-2 font-medium">
                  Gün
                </th>
                <th scope="col" className="px-3 py-2 text-right font-medium">
                  Ziyaret
                </th>
                <th scope="col" className="px-3 py-2 text-right font-medium">
                  Tekil ziyaretçi
                </th>
                <th scope="col" className="px-3 py-2 text-right font-medium">
                  Kategori görüntüleme
                </th>
                <th scope="col" className="px-3 py-2 text-right font-medium">
                  Ürün detayı
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 tabular-nums text-gray-700">
              {rows
                .slice()
                .reverse()
                .map((row) => (
                  <tr key={row.date}>
                    <th scope="row" className="whitespace-nowrap px-3 py-1.5 font-normal">
                      {formatDay(row.date, 'numeric')}
                    </th>
                    <td className="px-3 py-1.5 text-right">{formatCount(row.visits)}</td>
                    <td className="px-3 py-1.5 text-right">{formatCount(row.unique)}</td>
                    <td className="px-3 py-1.5 text-right">{formatCount(row.categoryViews)}</td>
                    <td className="px-3 py-1.5 text-right">{formatCount(row.productViews)}</td>
                  </tr>
                ))}
            </tbody>
          </table>
        </div>
      </details>
    </div>
  )
}
