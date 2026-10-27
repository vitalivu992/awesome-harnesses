/** Copies text to the clipboard; true when it worked. */
export async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    // Older browsers / non-secure origins: fall back to a hidden textarea.
    const area = document.createElement("textarea")
    area.value = text
    area.setAttribute("readonly", "")
    area.style.position = "fixed"
    area.style.opacity = "0"
    document.body.appendChild(area)
    area.select()
    const ok = document.execCommand("copy")
    area.remove()
    return ok
  }
}

/** Copies a PNG (a Blob, or a promise of one so the write starts inside the click); true when it worked. */
export async function copyImage(png) {
  try {
    await navigator.clipboard.write([new ClipboardItem({ "image/png": png })])
    return true
  } catch {
    return false
  }
}
