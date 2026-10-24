import { describe, expect, it } from "vitest"
import { WORKING_WORDS, atBottom, isWorking, listPollDelay, liveState, nextWorkingWord, pollDelay } from "../src/lib/live.js"

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

describe("listPollDelay", () => {
  it("reloads a list soon while something in it runs", () => {
    expect(listPollDelay(true)).toBe(4000)
    expect(listPollDelay(false)).toBe(15000)
  })
})

describe("liveState", () => {
  it.each([
    [{ active: true, turns: 2, status: "incomplete" }, "running"],
    [{ active: true, turns: 2, status: "done" }, "finished"],
    [{ active: true, turns: 2, status: "error" }, "finished"],
    [{ active: true, turns: 0, status: "incomplete" }, ""],
    [{ active: false, turns: 2, status: "incomplete" }, ""],
    [null, ""],
  ])("%o → %o", (c, want) => expect(liveState(c)).toBe(want))
})

describe("atBottom", () => {
  it("allows some slack", () => {
    expect(atBottom({ scrollHeight: 1000, scrollTop: 400, clientHeight: 550 })).toBe(true)
    expect(atBottom({ scrollHeight: 1000, scrollTop: 100, clientHeight: 550 })).toBe(false)
  })
})

describe("nextWorkingWord", () => {
  it("picks a word other than the current one", () => {
    expect(nextWorkingWord("", () => 0)).toBe(WORKING_WORDS[0])
    expect(nextWorkingWord(WORKING_WORDS[0], () => 0)).toBe(WORKING_WORDS[1])
    expect(nextWorkingWord("", () => 0.9999)).toBe(WORKING_WORDS[WORKING_WORDS.length - 1])
    for (const w of WORKING_WORDS) expect(nextWorkingWord(w)).not.toBe(w)
  })
})
