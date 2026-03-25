/**
 * Copy text to clipboard with fallback for non-HTTPS contexts.
 * Returns a promise that resolves on success and rejects on failure.
 */
export function copyToClipboard(text) {
  if (navigator.clipboard && window.isSecureContext) {
    return navigator.clipboard.writeText(text)
  }
  // Fallback for HTTP
  return new Promise((resolve, reject) => {
    const textarea = document.createElement('textarea')
    textarea.value = text
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    try {
      document.execCommand('copy')
      resolve()
    } catch (err) {
      reject(err)
    }
    document.body.removeChild(textarea)
  })
}
