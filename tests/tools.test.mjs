import { describe, expect, it } from "vitest"
import { toolIcon, toolLabel, toolSummary } from "../src/lib/tools.js"

describe("tool calls", () => {
  it("summarizes the telling input", () => {
    expect(toolSummary("Bash", { command: "go test ./...\nmore", description: "Run tests" })).toBe("go test ./...")
    expect(toolSummary("Shell", { command: "pnpm build" })).toBe("pnpm build")
    expect(toolSummary("Read", { file_path: "/home/me/p/src/lib/a.js", offset: 40 })).toBe("…/src/lib/a.js (from line 40)")
    expect(toolSummary("Grep", { pattern: "RCA", path: "/w/src" })).toBe("RCA in /w/src")
    expect(toolSummary("StrReplace", { path: "/w/a.go" })).toBe("/w/a.go")
    expect(toolSummary("mcp__x__y", { query: "q" })).toBe("q")
    expect(toolSummary("Weird", null)).toBe("")
  })
  it("labels MCP tools by server and tool", () => expect(toolLabel("mcp__playwright__browser_click")).toBe("playwright · browser_click"))
  it("picks icons across harness names", () => {
    expect(toolIcon("Shell")).toBe(toolIcon("Bash"))
    expect(toolIcon("StrReplace")).toBe(toolIcon("Edit"))
    expect(toolIcon("mcp__a__b")).toBe("fa-plug")
    expect(toolIcon("Unknown")).toBe("fa-wrench")
  })
})
