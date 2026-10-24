import { describe, expect, it } from "vitest"
import { applyCompletion, completionAt, highlightSegments, matchCommands, usedCommands } from "../src/lib/prompt.js"

const commands = [
  { name: "commit", insert: "/commit", kind: "command", description: "Create grouped commits" },
  { name: "fix-backlog", insert: "/fix-backlog", kind: "skill", description: "Clear the backlog" },
  { name: "ralph:help", insert: "/ralph:help", kind: "command", description: "Explain Ralph" },
  { name: "playwright", insert: "$playwright", kind: "skill", description: "Drive a browser" },
]

describe("completionAt", () => {
  it("finds a slash command only at the start of the prompt", () => {
    expect(completionAt("/fi", 3)).toEqual({ start: 0, end: 3, query: "/fi" })
    expect(completionAt("  /fix more", 5)).toEqual({ start: 2, end: 6, query: "/fi" })
    expect(completionAt("run /fi", 7)).toBeNull()
    expect(completionAt("plain", 3)).toBeNull()
  })

  it("finds $ mentions anywhere when the harness has them", () => {
    expect(completionAt("use $pla", 8, { mentions: true })).toEqual({ start: 4, end: 8, query: "$pla" })
    expect(completionAt("use $pla", 8)).toBeNull()
  })
})

describe("matchCommands", () => {
  it("lists prefix matches before matches inside the name or description", () => {
    expect(matchCommands(commands, "/").map((c) => c.insert)).toEqual(["/commit", "/fix-backlog", "/ralph:help"])
    expect(matchCommands(commands, "/c").map((c) => c.insert)).toEqual(["/commit", "/fix-backlog"])
    expect(matchCommands(commands, "/help").map((c) => c.insert)).toEqual(["/ralph:help"])
  })

  it("keeps / commands and $ mentions apart", () => {
    expect(matchCommands(commands, "$").map((c) => c.insert)).toEqual(["$playwright"])
    expect(matchCommands(commands, "/play")).toEqual([])
  })
})

describe("applyCompletion", () => {
  it("replaces the token and adds a space", () => {
    expect(applyCompletion("/fi", { start: 0, end: 3 }, "/fix-backlog")).toEqual({ text: "/fix-backlog ", caret: 13 })
    expect(applyCompletion("/fi now", { start: 0, end: 3 }, "/fix-backlog")).toEqual({ text: "/fix-backlog now", caret: 13 })
  })
})

describe("highlightSegments", () => {
  it("marks the known command the prompt starts with", () => {
    const segs = highlightSegments("/fix-backlog the bugs", commands)
    expect(segs.map((s) => [s.text, s.command?.name || null])).toEqual([
      ["/fix-backlog", "fix-backlog"],
      [" the bugs", null],
    ])
  })

  it("marks nothing for an unknown command, a longer name or a command later on", () => {
    for (const text of ["/fix-backlogs x", "/nope", "run /commit", ""]) {
      expect(highlightSegments(text, commands).every((s) => !s.command)).toBe(true)
    }
  })

  it("marks $ mentions anywhere, keeping the text whole", () => {
    const text = "  /commit then $playwright, $nope"
    const segs = highlightSegments(text, commands)
    expect(segs.map((s) => s.text).join("")).toBe(text)
    expect(usedCommands(text, commands).map((c) => c.insert)).toEqual(["/commit", "$playwright"])
  })
})
