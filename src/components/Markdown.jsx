import { useEffect, useMemo, useRef } from "react"
import { renderMarkdown } from "../lib/markdown.js"
import { useTheme } from "../hooks/useTheme.js"
import "markdown-it-github-alerts/styles/github-base.css"
import "markdown-it-github-alerts/styles/github-colors-light.css"

let mermaidPromise = null
let mermaidSeq = 0

// mermaid is MBs with its diagram chunks: load it only for an answer that has one.
function loadMermaid() {
  mermaidPromise ??= import("mermaid").then(({ default: mermaid }) => mermaid)
  return mermaidPromise
}

async function renderMermaidBlocks(blocks, mode, isCancelled) {
  let mermaid
  try {
    mermaid = await loadMermaid()
  } catch {
    mermaidPromise = null
    return
  }
  // "strict": no click handlers/scripts in diagrams, labels are sanitized.
  mermaid.initialize({ startOnLoad: false, securityLevel: "strict", theme: mode === "dark" ? "dark" : "default" })
  for (const block of blocks) {
    if (isCancelled()) return
    const source = block.textContent
    try {
      const { svg } = await mermaid.render(`ah-mermaid-${++mermaidSeq}`, source)
      if (isCancelled()) return
      block.innerHTML = svg
    } catch (err) {
      if (isCancelled()) return
      const message = document.createElement("p")
      message.className = "ah-diagram__error"
      message.textContent = `Mermaid: ${String(err?.message || err).split("\n")[0]}`
      const pre = document.createElement("pre")
      pre.textContent = source
      block.replaceChildren(message, pre)
    }
  }
}

/** Renders markdown with Primer's `.markdown-body` styles (raw HTML is escaped). */
export function Markdown({ children, className = "", docId }) {
  const source = String(children ?? "")
  const html = useMemo(() => renderMarkdown(source, { docId }), [source, docId])
  const ref = useRef(null)
  const { mode } = useTheme()

  useEffect(() => {
    const blocks = ref.current?.querySelectorAll("[data-mermaid]")
    if (!blocks?.length) return undefined
    let cancelled = false
    renderMermaidBlocks([...blocks], mode, () => cancelled)
    return () => {
      cancelled = true
    }
  }, [html, mode])

  return (
    // Keyed by colour mode so a theme switch restores the mermaid placeholders to redraw.
    <div key={mode} ref={ref} className={`markdown-body ${className}`.trim()} dangerouslySetInnerHTML={{ __html: html }} />
  )
}
