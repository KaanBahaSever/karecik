// Assertions for src/lib/sortableRows.js, run through a simulation of dnd-kit's
// keyboard reordering loop on vertical lists whose rows differ in height.
//
//   node frontend/tests/sortableRows.test.mjs
//
// Nothing imports this file, so it never reaches the app bundle.

import assert from 'node:assert/strict'

import { closestRowSlot, rowSlotKeyboardCoordinates } from '../src/lib/sortableRows.js'

let passed = 0
const failures = []

function check(name, run) {
  try {
    run()
    passed += 1
  } catch (error) {
    failures.push(`${name}\n    ${String(error.message).split('\n').join('\n    ')}`)
  }
}

const GAP = 12 // the menu editor's gap-3 between category rows
const CLOSED = 56

/** Row rects top to bottom, as dnd-kit's droppable measuring reports them. */
function layout(heights, top = 180, left = 24, width = 640) {
  let y = top
  return heights.map((height, index) => {
    const rect = { id: `row${index}`, top: y, left, width, height, bottom: y + height, right: left + width }
    y += height + GAP
    return rect
  })
}

/** The array the collision detection gets, and the map the keyboard getter's context holds. */
function containersOf(rects) {
  const list = rects.map((rect) => ({ id: rect.id, disabled: false }))
  const map = new Map(list.map((container) => [container.id, container]))
  map.getEnabled = () => list
  return { list, map }
}

const shifted = (rect, dx, dy) => ({
  ...rect,
  top: rect.top + dy,
  bottom: rect.bottom + dy,
  left: rect.left + dx,
  right: rect.right + dx,
})

/**
 * One keyboard drag, as dnd-kit runs it: for every key the KeyboardSensor asks
 * the coordinate getter for new coordinates, the drag translate becomes the
 * distance from the starting corner, and DndContext takes the first collision
 * as `over`. Returns the index `over` points at, at the start and after every key.
 */
function keyboardDrag(heights, activeIndex, keys) {
  const rects = layout(heights)
  const droppableRects = new Map(rects.map((rect) => [rect.id, rect]))
  const { list, map } = containersOf(rects)
  const start = rects[activeIndex]
  const active = { id: start.id, rect: { current: { initial: start } } }

  let collisionRect = { ...start }
  const overIndex = () => {
    const [first] = closestRowSlot({ active, collisionRect, droppableRects, droppableContainers: list })
    return rects.findIndex((rect) => rect.id === first.id)
  }

  const trail = [overIndex()]
  for (const code of keys) {
    const next = rowSlotKeyboardCoordinates(
      { code, preventDefault() {} },
      { active, context: { active, collisionRect, droppableRects, droppableContainers: map } },
    )
    if (next) collisionRect = shifted(start, next.x - start.left, next.y - start.top)
    trail.push(overIndex())
  }
  return trail
}

/** One place per arrow, stopping at either end. */
function expectedTrail(count, activeIndex, keys) {
  let index = activeIndex
  const trail = [index]
  for (const key of keys) {
    if (key === 'ArrowDown') index = Math.min(index + 1, count - 1)
    if (key === 'ArrowUp') index = Math.max(index - 1, 0)
    trail.push(index)
  }
  return trail
}

function assertWalk(heights, activeIndex, keys) {
  assert.deepEqual(
    keyboardDrag(heights, activeIndex, keys),
    expectedTrail(heights.length, activeIndex, keys),
    `heights ${heights.join(',')} from ${activeIndex}: ${keys.join(' ')}`,
  )
}

const down = (count) => Array(count).fill('ArrowDown')
const up = (count) => Array(count).fill('ArrowUp')

check('rows of one height move one place per key, both ways, stopping at the ends', () => {
  assertWalk([CLOSED, CLOSED, CLOSED, CLOSED, CLOSED], 1, [...down(5), ...up(6)])
})

check('an open category many closed rows tall moves one place per ArrowDown', () => {
  for (const height of [180, 200, 420, 900]) assertWalk([height, CLOSED, CLOSED, CLOSED, CLOSED], 0, down(5))
})

check('an open category at the bottom moves one place per ArrowUp', () => {
  assertWalk([CLOSED, CLOSED, CLOSED, CLOSED, 420], 4, up(5))
})

check('a closed row passes an open one without skipping a place', () => {
  assertWalk([CLOSED, 420, CLOSED, CLOSED], 0, down(3))
  assertWalk([CLOSED, CLOSED, 420, CLOSED], 3, up(3))
  assertWalk([CLOSED, 300, CLOSED, 500, CLOSED], 0, [...down(5), ...up(5)])
})

check('random heights and key sequences', () => {
  let seed = 11
  const random = () => {
    seed = (seed * 16807) % 2147483647
    return seed / 2147483647
  }
  for (let run = 0; run < 2000; run += 1) {
    const count = 2 + Math.floor(random() * 9)
    const heights = Array.from({ length: count }, () => (random() < 0.3 ? 80 + Math.floor(random() * 900) : CLOSED))
    const keys = Array.from({ length: 1 + Math.floor(random() * 14) }, () => (random() < 0.5 ? 'ArrowDown' : 'ArrowUp'))
    assertWalk(heights, Math.floor(random() * count), keys)
  }
})

check('a slot below the dragged row is bottom-aligned with that row, one above is top-aligned', () => {
  const rects = layout([420, CLOSED, CLOSED])
  const droppableRects = new Map(rects.map((rect) => [rect.id, rect]))
  const { map } = containersOf(rects)
  const active = { id: rects[1].id }
  const context = (collisionRect) => ({ active, collisionRect, droppableRects, droppableContainers: map })
  const event = (code) => ({ code, preventDefault() {} })

  const downTo = rowSlotKeyboardCoordinates(event('ArrowDown'), { context: context({ ...rects[1] }) })
  assert.deepEqual(downTo, { x: rects[1].left, y: rects[2].bottom - CLOSED })
  const upTo = rowSlotKeyboardCoordinates(event('ArrowUp'), { context: context({ ...rects[1] }) })
  assert.deepEqual(upTo, { x: rects[1].left, y: rects[0].top })
})

check('the keyboard getter ignores other keys, swallows ArrowLeft and ArrowRight, and stops at the ends', () => {
  const rects = layout([CLOSED, CLOSED])
  const droppableRects = new Map(rects.map((rect) => [rect.id, rect]))
  const { map } = containersOf(rects)
  const context = { active: { id: rects[0].id }, collisionRect: { ...rects[0] }, droppableRects, droppableContainers: map }

  let prevented = 0
  const event = (code) => ({ code, preventDefault: () => (prevented += 1) })

  assert.equal(rowSlotKeyboardCoordinates(event('Space'), { context }), undefined)
  assert.equal(prevented, 0)
  assert.equal(rowSlotKeyboardCoordinates(event('ArrowLeft'), { context }), undefined)
  assert.equal(rowSlotKeyboardCoordinates(event('ArrowRight'), { context }), undefined)
  assert.equal(prevented, 2)
  assert.equal(rowSlotKeyboardCoordinates(event('ArrowUp'), { context }), undefined)
  assert.equal(rowSlotKeyboardCoordinates(event('ArrowDown'), { context: { ...context, collisionRect: null } }), undefined)
  assert.equal(rowSlotKeyboardCoordinates(event('ArrowDown'), { context: { ...context, active: { id: 'elsewhere' } } }), undefined)
})

check('pointer drags: the nearest slot wins, so an open row trades places halfway to the next slot', () => {
  const rects = layout([420, CLOSED, CLOSED])
  const droppableRects = new Map(rects.map((rect) => [rect.id, rect]))
  const { list } = containersOf(rects)
  const active = { id: rects[0].id }
  const overAt = (dy) =>
    closestRowSlot({ active, collisionRect: shifted(rects[0], 0, dy), droppableRects, droppableContainers: list })[0].id

  // The next slot is CLOSED + GAP = 68 px below: the open row's bottom on the next row's bottom.
  assert.equal(overAt(0), 'row0')
  assert.equal(overAt(30), 'row0')
  assert.equal(overAt(40), 'row1')
  assert.equal(overAt(68), 'row1')
  assert.equal(overAt(110), 'row2')
  assert.equal(overAt(-500), 'row0')
  assert.deepEqual(closestRowSlot({ active, collisionRect: null, droppableRects, droppableContainers: list }), [])
})

check('pointer drags on rows of one height switch at the midpoint, as centre distance does', () => {
  const rects = layout([CLOSED, CLOSED, CLOSED, CLOSED])
  const droppableRects = new Map(rects.map((rect) => [rect.id, rect]))
  const { list } = containersOf(rects)
  const active = { id: rects[1].id }
  const overAt = (dy) =>
    closestRowSlot({ active, collisionRect: shifted(rects[1], 0, dy), droppableRects, droppableContainers: list })[0].id

  const pitch = CLOSED + GAP
  assert.equal(overAt(pitch / 2 - 1), 'row1')
  assert.equal(overAt(pitch / 2 + 1), 'row2')
  assert.equal(overAt(-pitch / 2 - 1), 'row0')
  assert.equal(overAt(pitch + pitch / 2 + 1), 'row3')
})

if (failures.length > 0) {
  console.error(`${failures.length} failed, ${passed} passed\n\n${failures.join('\n\n')}`)
  process.exit(1)
}
console.log(`sortableRows.test.mjs: all ${passed} checks passed`)
