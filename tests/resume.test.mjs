import { describe, expect, it } from "vitest"
import { resumeCommand, shellQuote } from "../src/lib/resume.js"

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

  it("skips the cd without a directory", () => {
    expect(resumeCommand({ harness: "cursor", id: "x" })).toBe("cursor-agent --resume x")
    expect(resumeCommand(null)).toBe("")
  })

  it("quotes for a POSIX shell", () => {
    expect(shellQuote("it's")).toBe(`'it'\\''s'`)
    expect(shellQuote("")).toBe("''")
  })
})
