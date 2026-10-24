import { describe, expect, it } from "vitest"
import { agentLabel, cursorClient, duration, formatMs, greeting, modelLabel, shortDateTime } from "../src/lib/format.js"

describe("modelLabel", () => {
  it.each([
    ["claude-opus-5-5", "Opus 5.5"],
    ["claude-sonnet-5-5", "Sonnet 5.5"],
    ["claude-sonnet-4-5-20250929", "Sonnet 4.5"],
    ["claude-opus-4-20250514", "Opus 4"],
    ["claude-fable-5-1", "Fable 5.1"],
    ["claude-opus-5-5-high", "Opus 5.5 High"],
    ["glm-5.3-flash[1m]", "GLM 5.3 Flash"],
    ["claude-4.6-opus-high-thinking", "Opus 4.6 High Thinking"],
    ["composer-2.5-fast", "Composer 2.5 Fast"],
    ["gpt-5.3-codex", "GPT 5.3 Codex"],
    ["cline-pass/deepseek-v4-flash", "Deepseek V4 Flash"],
    ["gpt-5.6-luna", "GPT 5.6 Luna"],
    ["", ""],
  ])("%s → %s", (id, label) => expect(modelLabel(id)).toBe(label))
})

describe("agentLabel", () => {
  it("names the harness and model", () => {
    expect(agentLabel("claude", "claude-sonnet-5-5")).toBe("Claude Code - Sonnet 5.5")
    expect(agentLabel("cursor", "claude-opus-5-5")).toBe("Cursor - Opus 5.5")
  })
  it("falls back to the harness alone", () => expect(agentLabel("cursor", "")).toBe("Cursor"))
  it("names every harness", () => {
    expect(agentLabel("codex", "gpt-5.5")).toBe("Codex - GPT 5.5")
    expect(agentLabel("pi", "")).toBe("pi")
    expect(agentLabel("opencode", "zai/glm-5.3")).toBe("OpenCode - GLM 5.3")
    expect(agentLabel("kilo", "")).toBe("Kilo Code")
  })
})

describe("cursorClient", () => {
  it("tells CLI chats from app chats", () => {
    expect(cursorClient({ harness: "cursor", resumable: true }).id).toBe("cli")
    expect(cursorClient({ harness: "cursor", resumable: false }).id).toBe("app")
  })
  it("is null for other harnesses", () => expect(cursorClient({ harness: "claude", resumable: true })).toBeNull())
})

describe("times", () => {
  it("formats durations", () => {
    expect(formatMs(850)).toBe("850 ms")
    expect(formatMs(12_000)).toBe("12 s")
    expect(formatMs(200_000)).toBe("3m 20s")
    expect(formatMs(3_900_000)).toBe("1h 05m")
    expect(duration("2026-10-20T10:00:00Z", "2026-10-20T10:00:04Z")).toBe("4 s")
    expect(duration("", "2026-10-20T10:00:04Z")).toBe("")
  })
  it("shortens list dates by distance", () => {
    const now = new Date(2026, 9, 23, 18, 0)
    expect(shortDateTime(new Date(2026, 9, 23, 14, 5).toISOString(), now)).toBe("14:05")
    expect(shortDateTime(new Date(2026, 9, 2, 9, 7).toISOString(), now)).toBe("Oct 2 09:07")
    expect(shortDateTime(new Date(2025, 0, 5, 9, 7).toISOString(), now)).toBe("2025-01-05")
    expect(shortDateTime("nope", now)).toBe("")
  })
})

describe("greeting", () => {
  it.each([
    [4, "Hi"],
    [5, "Good morning"],
    [11, "Good morning"],
    [12, "Good afternoon"],
    [17, "Good afternoon"],
    [18, "Good evening"],
    [22, "Good evening"],
    [23, "Hi"],
  ])("%i h → %s", (hour, want) => expect(greeting(new Date(2026, 9, 25, hour, 30))).toBe(want))
})
