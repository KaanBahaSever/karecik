// Reordering a vertical dnd-kit list whose rows differ in height - the menu
// editor's categories, where an open category is many times taller than a
// closed one.
//
// dnd-kit's own pair for sortable lists, closestCenter (or closestCorners) with
// sortableKeyboardCoordinates, assumes rows of similar height. The keyboard
// getter moves the dragged row so that one edge lines up with the neighbour it
// picked, and the collision detection then compares centres or corners. For a
// row more than about three closed rows tall, the moved row's centre and
// corners stay closer to its own old place than to the neighbour's, so `over`
// never changes and ArrowDown does nothing; a short row moved past a tall one
// lands one place too far; and further presses make the getter pick between two
// rows at an equal corner distance, stepping back and forth.
//
// The two functions below measure positions instead of shapes. Every row
// defines a slot: the top the dragged row would have if it were dropped at that
// row's index, which is exactly where dnd-kit's verticalListSortingStrategy puts
// it (top-aligned with a row above its own place, bottom-aligned with a row
// below it). The collision detection picks the slot nearest to the dragged
// row's top, and the keyboard getter moves the dragged row to the next slot up
// or down. Slots strictly increase from the top of the list, so one arrow press
// is always one place, whatever the heights.
//
// Plain functions of dnd-kit's arguments - no React, no DOM, no import - so a
// Node script can check them.

const ARROW_KEYS = ['ArrowDown', 'ArrowUp', 'ArrowLeft', 'ArrowRight']

/* The collision detection receives the enabled containers as an array; the
   keyboard getter's context holds dnd-kit's container map instead. */
function enabledContainers(droppableContainers) {
  const list = Array.isArray(droppableContainers)
    ? droppableContainers
    : typeof droppableContainers?.getEnabled === 'function'
      ? droppableContainers.getEnabled()
      : []
  return list.filter((container) => container && !container.disabled)
}

/**
 * The slots of the list, top to bottom: { container, top }. Empty when the
 * dragged row is not one of the rows, which leaves nothing to reorder.
 *
 * The rects are dnd-kit's droppable measurements, which ignore transforms, so
 * they stay the rows' resting places while the other rows slide aside.
 */
function rowSlots(active, droppableRects, droppableContainers) {
  if (!active || !droppableRects || typeof droppableRects.get !== 'function') return []

  const rows = []
  for (const container of enabledContainers(droppableContainers)) {
    const rect = droppableRects.get(container.id)
    if (rect) rows.push({ container, rect })
  }
  rows.sort((a, b) => a.rect.top - b.rect.top)

  const activeIndex = rows.findIndex((row) => row.container.id === active.id)
  if (activeIndex === -1) return []

  const activeHeight = rows[activeIndex].rect.height
  return rows.map((row, index) => ({
    container: row.container,
    top: index > activeIndex ? row.rect.top + row.rect.height - activeHeight : row.rect.top,
  }))
}

/**
 * dnd-kit collision detection: every row, nearest slot first.
 *
 * @param {{ active: object, collisionRect: object, droppableRects: Map, droppableContainers: object[] }} args
 * @returns {{ id: string|number, data: { droppableContainer: object, value: number } }[]}
 */
export function closestRowSlot({ active, collisionRect, droppableRects, droppableContainers }) {
  if (!collisionRect) return []

  return rowSlots(active, droppableRects, droppableContainers)
    .map((slot) => ({
      id: slot.container.id,
      data: { droppableContainer: slot.container, value: Math.abs(collisionRect.top - slot.top) },
    }))
    .sort((a, b) => a.data.value - b.data.value)
}

/**
 * dnd-kit KeyboardSensor coordinate getter: ArrowDown and ArrowUp move the
 * dragged row to the next slot below or above the one it is nearest to, and do
 * nothing at either end of the list. ArrowLeft and ArrowRight are swallowed, as
 * dnd-kit's own getter swallows them, so they do not scroll the page mid-drag.
 *
 * @param {KeyboardEvent} event
 * @param {{ context: object }} args
 * @returns {{ x: number, y: number } | undefined}
 */
export function rowSlotKeyboardCoordinates(event, { context }) {
  if (!ARROW_KEYS.includes(event.code)) return undefined
  event.preventDefault()

  const { active, collisionRect, droppableRects, droppableContainers } = context || {}
  if (!collisionRect || (event.code !== 'ArrowDown' && event.code !== 'ArrowUp')) return undefined

  const slots = rowSlots(active, droppableRects, droppableContainers)
  if (slots.length === 0) return undefined

  let current = 0
  slots.forEach((slot, index) => {
    if (Math.abs(collisionRect.top - slot.top) < Math.abs(collisionRect.top - slots[current].top)) {
      current = index
    }
  })

  const target = current + (event.code === 'ArrowDown' ? 1 : -1)
  if (target < 0 || target >= slots.length) return undefined

  // Straight down or up: the row keeps its horizontal place.
  return { x: collisionRect.left, y: slots[target].top }
}
