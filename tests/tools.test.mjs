import { describe, expect, it } from "vitest"
import { shellCommand, toolIcon, toolLabel, toolSummary } from "../src/lib/tools.js"

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
  it("summarizes Codex, pi and OpenCode tool inputs", () => {
    expect(toolSummary("shell", { command: ["bash", "-lc", "sed -n '1,9p' a.go"] })).toBe("sed -n '1,9p' a.go")
    expect(toolSummary("exec_command", { cmd: "pwd", workdir: "/w" })).toBe("pwd")
    expect(toolSummary("apply_patch", "*** Begin Patch\n*** Update File: /w/src/lib/a.js\n*** Add File: b.md\n")).toBe("…/src/lib/a.js, b.md")
    expect(toolSummary("exec", 'const r = await tools.exec_command({cmd:"gh pr view 1 --json title",workdir:"/w"});')).toBe("gh pr view 1 --json title")
    expect(toolSummary("update_plan", { plan: [{}, {}] })).toBe("2 steps")
    expect(toolSummary("read", { filePath: "/w/a.go" })).toBe("/w/a.go")
    expect(toolSummary("skill", { name: "ridge-rca" })).toBe("ridge-rca")
    expect(shellCommand({ command: ["git", "status"] })).toBe("git status")
  })
  it("labels MCP tools by server and tool", () => expect(toolLabel("mcp__playwright__browser_click")).toBe("playwright · browser_click"))
  it("picks icons across harness names", () => {
    expect(toolIcon("Shell")).toBe(toolIcon("Bash"))
    expect(toolIcon("StrReplace")).toBe(toolIcon("Edit"))
    expect(toolIcon("mcp__a__b")).toBe("fa-plug")
    expect(toolIcon("exec_command")).toBe(toolIcon("Bash"))
    expect(toolIcon("apply_patch")).toBe(toolIcon("Edit"))
    expect(toolIcon("Unknown")).toBe("fa-wrench")
  })
})
