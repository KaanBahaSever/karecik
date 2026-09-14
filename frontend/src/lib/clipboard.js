// Clipboard access for the customer menu.
//
// The Wi-Fi network name, the Wi-Fi password and the phone number are all
// copied through this one helper, so the fallback path exists once: two copies
// of it would be two chances for one of them to drift.
//
// No React here; it touches the DOM only for the fallback below.

/**
 * Copies one string to the clipboard and reports whether that worked.
 *
 * navigator.clipboard is unavailable on plain-HTTP hosts and on older in-app
 * browsers, which is exactly where a QR menu tends to be opened, so the hidden
 * textarea + execCommand path stays as a fallback.
 *
 * @param {unknown} value
 * @returns {Promise<boolean>}
 */
export async function copyToClipboard(value) {
  const text = String(value || '')
  if (!text) return false

  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
      return true
    }
  } catch {
    /* falls through to the textarea fallback below */
  }

  let area = null
  try {
    area = document.createElement('textarea')
    area.value = text
    area.setAttribute('readonly', '')
    area.style.position = 'fixed'
    area.style.top = '-1000px'
    area.style.opacity = '0'
    document.body.appendChild(area)
    area.select()
    return document.execCommand('copy')
  } catch {
    return false
  } finally {
    // Removed on the failure path too, so a throwing execCommand cannot leave
    // a stray textarea holding the password at the end of the page.
    if (area?.parentNode) area.parentNode.removeChild(area)
  }
}
