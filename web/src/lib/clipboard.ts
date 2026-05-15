// Copy text to the system clipboard.
//
// navigator.clipboard requires a secure context (HTTPS or localhost). Over
// plain HTTP it throws or rejects silently, which is why the admin couldn't
// copy invite codes when visiting http://<ip>:5555/. This wrapper tries the
// modern API first and falls back to the legacy execCommand trick that
// works in any context.
export async function copyText(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text)
      return true
    }
  } catch {
    // fall through to legacy path
  }

  // Legacy path: a transient textarea + execCommand('copy'). Works on HTTP.
  try {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.top = '0'
    ta.style.left = '0'
    ta.style.opacity = '0'
    ta.style.pointerEvents = 'none'
    ta.setAttribute('readonly', '')
    document.body.appendChild(ta)
    ta.select()
    ta.setSelectionRange(0, text.length)
    const ok = document.execCommand('copy')
    document.body.removeChild(ta)
    return ok
  } catch {
    return false
  }
}
