import { describe, expect, it } from "vitest"
import { attachURL, folderName, neighbourId, shellTitle, terminalTheme } from "../src/lib/shells.js"

describe("folderName", () => {
  it("takes the last folder of POSIX and Windows paths", () => {
    expect(folderName("/home/me/app")).toBe("app")
    expect(folderName("/home/me/app/")).toBe("app")
    expect(folderName("C:\\Users\\me\\app")).toBe("app")
    expect(folderName("/")).toBe("/")
    expect(folderName("")).toBe("")
  })
})

describe("shellTitle", () => {
  it("names the shell, folder and command", () => {
    expect(shellTitle({ n: 2, cwd: "/x/app", command: "npm test" })).toBe("Shell 2 · /x/app · npm test")
    expect(shellTitle({ n: 1, cwd: "/x", command: "" })).toBe("Shell 1 · /x")
    expect(shellTitle({ n: 1, cwd: "/x", command: "vim", exited: true, exitCode: 3 })).toBe("Shell 1 · /x · exited (3)")
  })
})

describe("attachURL", () => {
  it("follows the page's scheme and host", () => {
    expect(attachURL("ab", { protocol: "http:", host: "127.0.0.1:8767" })).toBe("ws://127.0.0.1:8767/api/shells/ab/attach")
    expect(attachURL("ab", { protocol: "https:", host: "h" })).toBe("wss://h/api/shells/ab/attach")
  })
})

describe("neighbourId", () => {
  const shells = [{ id: "a" }, { id: "b" }, { id: "c" }]
  it("prefers the next tab, then the previous one", () => {
    expect(neighbourId(shells, "b")).toBe("c")
    expect(neighbourId(shells, "c")).toBe("b")
    expect(neighbourId([{ id: "a" }], "a")).toBe("")
  })
})

describe("terminalTheme", () => {
  it("has light and dark palettes", () => {
    expect(terminalTheme("dark").background).toBe("#0d1117")
    expect(terminalTheme("nope")).toBe(terminalTheme("light"))
  })
})
