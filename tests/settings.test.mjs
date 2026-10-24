import { describe, expect, it } from "vitest"
import { configuredModels, effortsFor, launchRequest, normalizeSettings } from "../src/lib/settings.js"

describe("normalizeSettings", () => {
  it("fills every harness and drops what isn't a setting", () => {
    const s = normalizeSettings({ defaultHarness: "nope", harnesses: { claude: { models: [{ id: "opus" }, null, { x: 1 }], fast: "yes" } } })
    expect(s.defaultHarness).toBe("claude")
    expect(s.harnesses.claude).toEqual({ models: [{ id: "opus", effort: "" }], effort: "", fast: false, bypass: false })
    expect(Object.keys(s.harnesses)).toEqual(["claude", "cursor", "codex", "pi", "opencode", "kilo"])
  })
})

describe("launchRequest", () => {
  const settings = normalizeSettings({
    harnesses: {
      claude: { models: [{ id: " opus ", effort: "high" }, { id: "", effort: "" }], effort: "low", fast: true, bypass: true },
    },
  })

  it("uses the model's effort, or the harness's for its default model", () => {
    expect(launchRequest(settings, { harness: "claude", model: "opus", prompt: "hi", cwd: "/w" })).toEqual({
      harness: "claude",
      session: "",
      cwd: "/w",
      prompt: "hi",
      model: "opus",
      effort: "high",
      fast: true,
      bypass: true,
    })
    expect(launchRequest(settings, { harness: "claude", prompt: "hi" }).effort).toBe("low")
    expect(launchRequest(settings, { harness: "codex", prompt: "hi" })).toMatchObject({ effort: "", fast: false, bypass: false })
  })

  it("offers only the models with an id", () => {
    expect(configuredModels(settings, "claude").map((m) => m.id)).toEqual([" opus "])
  })
})

describe("effortsFor", () => {
  const harness = { efforts: ["low", "medium", "high"] }
  it("prefers what the model supports", () => {
    expect(effortsFor(harness, { efforts: ["low"] })).toEqual(["low"])
    expect(effortsFor(harness, { efforts: [] })).toEqual([])
    expect(effortsFor(harness, { efforts: null })).toEqual(["low", "medium", "high"])
    expect(effortsFor(harness, undefined)).toEqual(["low", "medium", "high"])
  })
})
