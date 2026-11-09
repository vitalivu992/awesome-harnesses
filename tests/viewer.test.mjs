import { describe, expect, it } from "vitest"
import { RESOLUTIONS, bothSides, diffFileName, diffStats, isMarker, languageOf, lineCount, parseDiff, splitHighlighted, splitPath } from "../src/lib/viewer.js"
import { viewerKey } from "../src/lib/explorer.js"

const DIFF = `diff --git a/a.go b/a.go
index 1111111..2222222 100644
--- a/a.go
+++ b/a.go
@@ -1,3 +1,3 @@ package a
 package a
-var x = 1
+var x = 2
 // end
\\ No newline at end of file
`

describe("parseDiff", () => {
  it("numbers the lines of each side", () => {
    const files = parseDiff(DIFF)
    expect(files).toHaveLength(1)
    const [hunk] = files[0].hunks
    expect(hunk.lines.map((l) => [l.kind, l.old, l.new])).toEqual([
      ["ctx", 1, 1],
      ["del", 2, undefined],
      ["add", undefined, 2],
      ["ctx", 3, 3],
      ["meta", undefined, undefined],
    ])
    expect(diffStats(files)).toEqual({ add: 1, del: 1 })
    expect(diffFileName(files[0])).toBe("a.go")
  })

  it("reads a combined (conflict) diff", () => {
    const files = parseDiff("diff --cc c.txt\nindex 1,2..0\n@@@ -1,1 -1,1 +1,5 @@@\n++<<<<<<< HEAD\n +mine\n++=======\n+ theirs\n++>>>>>>> other\n")
    const lines = files[0].hunks[0].lines
    expect(lines.every((l) => l.kind === "add")).toBe(true)
    expect(lines[0].text).toBe("<<<<<<< HEAD")
    expect(isMarker(lines[0].text)).toBe(true)
    expect(isMarker(lines[1].text)).toBe(false)
    expect(diffFileName(files[0])).toBe("c.txt")
  })

  it("keeps several files apart (an untracked folder)", () => {
    const two = "diff --git a/x b/x\nnew file mode 100644\n@@ -0,0 +1 @@\n+x\ndiff --git a/y b/y\nnew file mode 100644\n@@ -0,0 +1 @@\n+y\n"
    expect(parseDiff(two).map(diffFileName)).toEqual(["x", "y"])
  })
})

describe("splitHighlighted", () => {
  it("closes and reopens spans across lines", () => {
    expect(splitHighlighted('<span class="c">/* a\nb */</span>\nx')).toEqual(['<span class="c">/* a</span>', '<span class="c">b */</span>', "x"])
  })
})

describe("helpers", () => {
  it("splits paths, counts lines and names languages", () => {
    expect(splitPath("src/lib/a.js")).toEqual({ name: "a.js", dir: "src/lib" })
    expect(splitPath("a.js")).toEqual({ name: "a.js", dir: "" })
    expect(lineCount("a\nb\n")).toBe(2)
    expect(lineCount("a\nb")).toBe(2)
    expect(lineCount("")).toBe(0)
    expect(languageOf("cmd/main.go")).toBe("go")
    expect(languageOf("Makefile")).toBe("makefile")
    expect(languageOf("notes")).toBe("")
  })

  it("offers two resolutions up front and two that keep both sides", () => {
    expect(RESOLUTIONS.filter((r) => !r.both).map((r) => r.label)).toEqual(["Take theirs", "Take mine"])
    expect(RESOLUTIONS.filter((r) => r.both).map((r) => r.take)).toEqual(["theirs-mine", "mine-theirs"])
    expect(bothSides("UU")).toBe(true)
    expect(bothSides("UD")).toBe(false)
  })

  it("keys what the viewer shows", () => {
    expect(viewerKey(null)).toBe("")
    expect(viewerKey({ kind: "file", path: "/p/a.go" })).toBe("file:/p/a.go")
    expect(viewerKey({ kind: "diff", staged: true, entry: { path: "a.go" } })).toBe("diff:staged:a.go")
    expect(viewerKey({ kind: "diff", staged: false, entry: { path: "a.go" } })).toBe("diff:changes:a.go")
  })
})
