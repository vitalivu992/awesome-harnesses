// Markdown → HTML for agent answers, on markdown-it: GFM tables /
// strikethrough / autolinks (core), task lists, footnotes, GitHub alerts
// (> [!NOTE]) and highlight.js code blocks. ```mermaid blocks become
// `[data-mermaid]` placeholders holding the source; the Markdown component
// draws them in the browser (mermaid is lazy-loaded).
//
// Raw HTML in the source is escaped (`html: false`) and markdown-it refuses
// javascript:/vbscript:/data: links, so rendered answers are safe to inject.
import MarkdownIt from "markdown-it"
import footnote from "markdown-it-footnote"
import githubAlerts from "markdown-it-github-alerts"
import taskLists from "markdown-it-task-lists"
import hljs from "highlight.js/lib/common"

function fenceLang(info) {
  return String(info || "").trim().split(/\s+/)[0].toLowerCase()
}

function createMarkdown() {
  const md = new MarkdownIt({
    html: false,
    linkify: true,
    highlight(code, lang) {
      if (!lang || !hljs.getLanguage(lang)) return ""
      const value = hljs.highlight(code, { language: lang, ignoreIllegals: true }).value
      return `<pre><code class="hljs language-${md.utils.escapeHtml(lang)}">${value}</code></pre>`
    },
  })
  md.use(taskLists, { enabled: false }).use(footnote).use(githubAlerts)

  const defaultFence = md.renderer.rules.fence
  md.renderer.rules.fence = (tokens, idx, options, env, self) => {
    const token = tokens[idx]
    if (fenceLang(token.info) === "mermaid") {
      return `<div class="ah-diagram" data-mermaid>${md.utils.escapeHtml(token.content)}</div>\n`
    }
    return defaultFence(tokens, idx, options, env, self)
  }

  // External links open in a new tab; in-answer anchors (footnotes) stay put.
  md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
    const token = tokens[idx]
    if (!String(token.attrGet("href") || "").startsWith("#")) {
      token.attrSet("target", "_blank")
      token.attrSet("rel", "noopener noreferrer")
    }
    return self.renderToken(tokens, idx, options)
  }
  return md
}

let instance = null

/** Renders markdown to an HTML string; `docId` namespaces footnote ids per answer. */
export function renderMarkdown(source, { docId } = {}) {
  instance ??= createMarkdown()
  const env = docId ? { docId: String(docId) } : {}
  return instance.render(String(source ?? ""), env)
}
