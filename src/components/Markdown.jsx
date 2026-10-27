import { useEffect, useMemo, useRef } from "react"
import { copyImage } from "../lib/clipboard.js"
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
      block.append(copyButton())
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

function setLabel(button, icon, text) {
  const i = document.createElement("i")
  i.className = `fa-solid ${icon}`
  i.setAttribute("aria-hidden", "true")
  button.replaceChildren(i, ` ${text}`)
}

// Added to the diagram's DOM (not React's), so the Markdown wrapper handles its clicks.
function copyButton() {
  const button = document.createElement("button")
  button.type = "button"
  button.className = "ah-copy-btn ah-diagram__copy"
  button.title = "Copy the diagram as a PNG image"
  button.dataset.copyDiagram = ""
  setLabel(button, "fa-copy", "Copy image")
  return button
}

// The first opaque background up the tree, so the PNG keeps the diagram's contrast once pasted.
function backgroundOf(node) {
  for (; node; node = node.parentElement) {
    const color = getComputedStyle(node).backgroundColor
    if (color !== "transparent" && color !== "rgba(0, 0, 0, 0)") return color
  }
  return "#fff"
}

/** Draws a mermaid SVG onto a canvas at 2x (capped for huge diagrams) and encodes it as PNG. */
async function svgToPng(svg, background) {
  const box = svg.viewBox.baseVal
  const rect = svg.getBoundingClientRect()
  const width = box?.width || rect.width
  const height = box?.height || rect.height
  const scale = Math.min(2, 8192 / Math.max(width, height))
  const clone = svg.cloneNode(true)
  clone.setAttribute("width", width)
  clone.setAttribute("height", height)
  clone.style.removeProperty("max-width")
  // HTML labels take their font from the page, which an <img> doesn't have; mermaid sized the
  // boxes for that font, so carry it over.
  const labels = svg.querySelectorAll("foreignObject > *")
  clone.querySelectorAll("foreignObject > *").forEach((label, i) => {
    const style = getComputedStyle(labels[i])
    label.style.fontFamily = style.fontFamily
    label.style.fontSize = style.fontSize
    label.style.lineHeight = style.lineHeight
  })
  const img = new Image()
  img.src = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(new XMLSerializer().serializeToString(clone))}`
  await img.decode()
  const canvas = document.createElement("canvas")
  canvas.width = Math.ceil(width * scale)
  canvas.height = Math.ceil(height * scale)
  const ctx = canvas.getContext("2d")
  ctx.fillStyle = background
  ctx.fillRect(0, 0, canvas.width, canvas.height)
  ctx.drawImage(img, 0, 0, canvas.width, canvas.height)
  return new Promise((resolve, reject) => {
    canvas.toBlob((blob) => (blob ? resolve(blob) : reject(new Error("PNG encoding failed"))), "image/png")
  })
}

async function copyDiagram(button) {
  const block = button.parentElement
  const svg = block.querySelector("svg")
  if (!svg) return
  const ok = await copyImage(svgToPng(svg, backgroundOf(block)))
  setLabel(button, ok ? "fa-check" : "fa-xmark", ok ? "Copied" : "Copy failed")
  setTimeout(() => setLabel(button, "fa-copy", "Copy image"), 1600)
}

function onClick(event) {
  const button = event.target.closest?.("[data-copy-diagram]")
  if (button) copyDiagram(button)
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
    <div key={mode} ref={ref} className={`markdown-body ${className}`.trim()} onClick={onClick} dangerouslySetInnerHTML={{ __html: html }} />
  )
}
