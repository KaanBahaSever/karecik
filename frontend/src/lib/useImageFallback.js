import { useCallback, useState } from 'react'

/** How long a load failure is remembered for other components showing the same URL. */
const REMEMBER_FAILURE_MS = 30 * 1000

/*
  URLs that recently failed to load, with the moment each one failed.

  Without this, the next component to show a broken file requested it again —
  the product's detail sheet right after its row thumbnail had failed, every row
  again after switching to search — and drew the image area for a moment until
  its own error arrived.

  A failure is remembered only for a short while, though. The very same error
  also happens to a perfectly good image on weak venue Wi-Fi or during a server
  hiccup, and remembering that for the whole page session would keep the picture
  hidden until a full reload, long after the connection came back. So a
  component mounted within REMEMBER_FAILURE_MS of a failure starts from
  "failed", and one mounted later simply tries again. A component already
  showing its fallback keeps it until its URL changes.
*/
const failedAt = new Map()

function failedRecently(src) {
  if (src === null) return false

  const at = failedAt.get(src)
  if (at === undefined) return false
  if (Date.now() - at < REMEMBER_FAILURE_MS) return true

  failedAt.delete(src)
  return false
}

/**
 * Decides whether an <img> should be drawn for `url`, and withdraws it once the
 * browser reports that the image cannot be loaded.
 *
 * A URL that does not load is an ordinary state, not an exotic one: an upload
 * whose file is gone, a hand-typed link to somebody else's server, a dropped
 * connection. Left alone the <img> keeps its box and draws it empty, so every
 * caller swaps in its own fallback — an emoji, a placeholder icon, or nothing.
 *
 * The failure belongs to ONE url and is forgotten the moment the url changes.
 * That reset happens during render rather than in an effect: an effect would
 * run after the new url had already been painted as "failed" for a frame.
 *
 * The state lives in the component that calls this hook, so that component
 * must be declared at module level. A component declared inside another
 * component's render body is a new type on every render, is remounted every
 * time, and would forget the failure — drawing the broken box again.
 *
 * @param {unknown} url - Anything; only a non-blank string counts as an image
 * @returns {{ src: string|null, failed: boolean, onError: Function }}
 *   `src`     the URL to draw, or null when there is none OR it failed to load
 *   `failed`  true only in the second case
 *   `onError` attach to the <img>
 */
export function useImageFallback(url) {
  const src = typeof url === 'string' && url.trim() !== '' ? url.trim() : null
  const [failure, setFailure] = useState(() => ({ src, failed: failedRecently(src) }))

  if (failure.src !== src) {
    setFailure({ src, failed: failedRecently(src) })
  }

  const onError = useCallback(() => {
    if (src !== null) failedAt.set(src, Date.now())
    setFailure((current) => (current.src === src ? { src, failed: true } : current))
  }, [src])

  const failed = src !== null && failure.src === src && failure.failed

  return { src: failed ? null : src, failed, onError }
}
