import { describe, expect, it } from "vitest"
import {
  canDiscard,
  entryLabel,
  entryPath,
  fileIcon,
  joinPath,
  sizeLabel,
  sortEntries,
  statusMeta,
} from "../src/lib/explorer.js"

describe("statusMeta", () => {
  it("maps the git status letters", () => {
    expect(statusMeta("M")).toEqual(["M", "modified", "Modified"])
    expect(statusMeta("A")).toEqual(["A", "added", "Added"])
    expect(statusMeta("D")).toEqual(["D", "deleted", "Deleted"])
    expect(statusMeta("?")).toEqual(["U", "untracked", "Untracked"])
    expect(statusMeta("!")).toEqual(["!", "conflict", "Conflict"])
  })

  it("falls back to modified for unknown letters", () => {
    expect(statusMeta("T")[0]).toBe("M")
    expect(statusMeta(" ")[0]).toBe("M")
  })
})

describe("entryLabel / entryPath / canDiscard", () => {
  it("shows the file name and the old name of a rename", () => {
    expect(entryLabel({ path: "src/new.js" })).toBe("new.js")
    expect(entryLabel({ path: "src/new.js", orig: "src/old.js" })).toBe("new.js (from old.js)")
  })

  it("keeps the trailing slash of an untracked folder for git", () => {
    expect(entryPath({ path: "node_modules/" })).toBe("node_modules/")
    expect(entryLabel({ path: "node_modules/" })).toBe("node_modules")
  })

  it("only tracked files can be reverted", () => {
    expect(canDiscard({ x: "M", y: "M" })).toBe(true)
    expect(canDiscard({ x: "?", y: "?" })).toBe(false)
    expect(canDiscard({ x: "!", y: "!" })).toBe(false)
  })
})

describe("sortEntries", () => {
  it("puts folders first, then names case-insensitive and numeric", () => {
    const sorted = sortEntries([
      { name: "z.txt", dir: false },
      { name: "Beta", dir: true },
      { name: "10.js", dir: false },
      { name: "alpha", dir: true },
      { name: "2.js", dir: false },
    ])
    expect(sorted.map((e) => e.name)).toEqual(["alpha", "Beta", "2.js", "10.js", "z.txt"])
  })
})

describe("fileIcon", () => {
  it("picks an icon by extension", () => {
    expect(fileIcon("main.go")).toBe("fa-file-code")
    expect(fileIcon("README.md")).toBe("fa-file-lines")
    expect(fileIcon("logo.png")).toBe("fa-file-image")
    expect(fileIcon("app.tar.gz")).toBe("fa-file-zipper")
    expect(fileIcon("noext")).toBe("fa-file")
  })
})

describe("joinPath / sizeLabel", () => {
  it("joins without duplicate slashes", () => {
    expect(joinPath("/a/", "b")).toBe("/a/b")
    expect(joinPath("", "b")).toBe("/b")
  })

  it("formats sizes", () => {
    expect(sizeLabel(0)).toBe("")
    expect(sizeLabel(42)).toBe("42 B")
    expect(sizeLabel(2048)).toBe("2 KB")
    expect(sizeLabel(5 * 1024 * 1024)).toBe("5 MB")
  })
})
