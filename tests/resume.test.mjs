import { describe, expect, it } from "vitest"
import { bypassFlag, noBypassReason, resumeCommand, shellQuote } from "../src/lib/resume.js"

describe("resumeCommand", () => {
  const claude = { harness: "claude", id: "9e7b-1", cwd: "/home/me/app" }
  const cursor = { harness: "cursor", id: "e95b-2", projectPath: "/home/me/My App" }

  it("cds into the session's directory and resumes it", () => {
    expect(resumeCommand(claude)).toBe("cd /home/me/app && claude --resume 9e7b-1")
    expect(resumeCommand(cursor)).toBe("cd '/home/me/My App' && cursor-agent --resume e95b-2")
  })

  it("adds the bypass-permissions flag of each CLI", () => {
    expect(resumeCommand(claude, { bypass: true })).toBe(
      "cd /home/me/app && claude --resume 9e7b-1 --dangerously-skip-permissions"
    )
    expect(resumeCommand(cursor, { bypass: true })).toBe("cd '/home/me/My App' && cursor-agent --resume e95b-2 --force")
  })

  it.each([
    ["codex", "codex resume s1", "codex resume s1 --dangerously-bypass-approvals-and-sandbox"],
    ["pi", "pi --session s1", "pi --session s1"],
    ["opencode", "opencode --session s1", "opencode --session s1 --auto"],
    ["kilo", "kilo --session s1", "kilo --session s1"],
  ])("resumes %s sessions", (harness, plain, bypassed) => {
    const conv = { harness, id: "s1", cwd: "/w" }
    expect(resumeCommand(conv)).toBe(`cd /w && ${plain}`)
    expect(resumeCommand(conv, { bypass: true })).toBe(`cd /w && ${bypassed}`)
  })

  it("explains harnesses without a bypass flag", () => {
    expect(bypassFlag("pi")).toBe("")
    expect(noBypassReason("pi")).toContain("doesn't ask")
    expect(bypassFlag("kilo")).toBe("")
    expect(noBypassReason("kilo")).toContain("kilo run")
    expect(noBypassReason("claude")).toBe("")
    expect(resumeCommand({ harness: "gemini", id: "x" })).toBe("")
  })

  it("skips the cd without a directory", () => {
    expect(resumeCommand({ harness: "cursor", id: "x" })).toBe("cursor-agent --resume x")
    expect(resumeCommand(null)).toBe("")
  })

  it("quotes for a POSIX shell", () => {
    expect(shellQuote("it's")).toBe(`'it'\\''s'`)
    expect(shellQuote("")).toBe("''")
  })
})
