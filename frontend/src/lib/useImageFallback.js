import { useCallback, useState } from 'react'

/**
 * Decides whether an <img> should be drawn for `url`, and withdraws it once the
 * browser reports that the image cannot be loaded.
 *
 * A URL that does not load is an ordinary state, not an exotic one: an upload
 * whose file is gone, a hand-typed link to somebody else's server. Left alone
 * the <img> keeps its box and draws it empty, so every caller swaps in its own
 * fallback — an emoji, a placeholder icon, or nothing at all.
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
  const [failure, setFailure] = useState({ src, failed: false })

  if (failure.src !== src) {
    setFailure({ src, failed: false })
  }

  const onError = useCallback(() => {
    setFailure((current) => (current.src === src ? { src, failed: true } : current))
  }, [src])

  const failed = src !== null && failure.src === src && failure.failed

  return { src: failed ? null : src, failed, onError }
}
