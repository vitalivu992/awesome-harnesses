import { describe, expect, it } from "vitest"
import { renderMarkdown } from "../src/lib/markdown.js"

describe("renderMarkdown", () => {
  it("renders GFM tables, code and alerts", () => {
    const html = renderMarkdown("| a | b |\n|---|---|\n| 1 | 2 |\n\n```go\nfunc main() {}\n```\n\n> [!NOTE]\n> hi")
    expect(html).toContain("<table>")
    expect(html).toContain('class="hljs language-go"')
    expect(html).toContain("markdown-alert-note")
  })
  it("turns mermaid fences into placeholders", () => {
    expect(renderMarkdown("```mermaid\ngraph TD; A-->B\n```")).toContain('<div class="ah-diagram" data-mermaid>graph TD; A--&gt;B')
  })
  it("escapes raw HTML and opens external links in a new tab", () => {
    const html = renderMarkdown('<script>alert(1)</script> [x](https://example.com)')
    expect(html).not.toContain("<script>")
    expect(html).toContain('target="_blank"')
  })
})
