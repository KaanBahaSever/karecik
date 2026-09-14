import { Component } from 'react'

/**
 * Whether `resetKeys` changed between two renders, compared element by element.
 *
 * Callers write the array inline — `resetKeys={[menu, refresh]}` — so it is a
 * new array on every render. Comparing the arrays themselves would read every
 * render of the parent as a change, throw the fallback away and re-run the very
 * render that just failed. Only a change in one of the values counts.
 */
function resetKeysChanged(previous, next) {
  const before = Array.isArray(previous) ? previous : []
  const after = Array.isArray(next) ? next : []

  if (before.length !== after.length) return true
  return before.some((value, index) => !Object.is(value, after[index]))
}

/**
 * Catches an exception thrown while rendering its children and draws `fallback`
 * in their place.
 *
 * With no boundary React unmounts the entire tree when a render throws, and the
 * visitor is left with a blank white page and nothing to act on. A boundary
 * confines the damage to the part that failed: one category card, one product
 * row, the dashboard preview — or, as the last line of defence, the menu itself.
 *
 * It catches errors thrown while rendering, in lifecycle methods and in
 * constructors below it. Event handlers and asynchronous code are not covered.
 *
 * @param {node|Function} fallback  - A node, or `(error, reset) => node`. It must
 *                                    not read anything that could throw again: a
 *                                    boundary cannot catch its own fallback.
 * @param {Array}         resetKeys - Values compared element-wise. When any of
 *                                    them changes while the fallback is showing,
 *                                    the boundary clears and renders its
 *                                    children again.
 * @param {node}          children
 */
export default class ErrorBoundary extends Component {
  constructor(props) {
    super(props)

    // A flag rather than `error !== null`: JavaScript can throw anything,
    // null and undefined included, and a boundary keyed on the value would
    // read `throw null` as "no error" and render the failing children again.
    this.state = { hasError: false, error: null }
    this.reset = this.reset.bind(this)
  }

  static getDerivedStateFromError(error) {
    return { hasError: true, error }
  }

  componentDidCatch(error, info) {
    console.error(
      '[karecik] Render error caught by ErrorBoundary:',
      error,
      info?.componentStack || '',
    )
  }

  componentDidUpdate(previousProps, previousState) {
    /* Only a boundary that was ALREADY showing its fallback before this update
       resets. The update that catches an error may carry the very keys that
       caused it — a fresh payload, say — and resetting on those would render
       the failing children a second time for nothing. */
    if (!previousState.hasError || !this.state.hasError) return

    if (resetKeysChanged(previousProps.resetKeys, this.props.resetKeys)) {
      this.reset()
    }
  }

  reset() {
    this.setState({ hasError: false, error: null })
  }

  render() {
    if (!this.state.hasError) return this.props.children

    const { fallback = null } = this.props
    return typeof fallback === 'function' ? fallback(this.state.error, this.reset) : fallback
  }
}
