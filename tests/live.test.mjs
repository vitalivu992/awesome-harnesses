import { describe, expect, it } from "vitest"
import { atBottom, isWorking, pollDelay } from "../src/lib/live.js"

describe("isWorking", () => {
  it.each([
    [{ active: true, status: "incomplete" }, true],
    [{ active: true, status: "done" }, false],
    [{ active: false, status: "incomplete" }, false],
    [null, false],
  ])("%o → %s", (c, want) => expect(isWorking(c)).toBe(want))
})

describe("pollDelay", () => {
  it("polls an active session fast and a quiet one slowly", () => {
    expect(pollDelay({ active: true })).toBe(2000)
    expect(pollDelay({ active: false })).toBe(15000)
  })
})

describe("atBottom", () => {
  it("allows some slack", () => {
    expect(atBottom({ scrollHeight: 1000, scrollTop: 400, clientHeight: 550 })).toBe(true)
    expect(atBottom({ scrollHeight: 1000, scrollTop: 100, clientHeight: 550 })).toBe(false)
  })
})
