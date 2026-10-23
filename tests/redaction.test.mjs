import { describe, expect, it } from "vitest"
import { HARNESS_COLORS, redactionRows } from "../src/lib/redaction.js"

describe("redactionRows", () => {
  it("is the same layout on every call", () => {
    expect(redactionRows(12)).toEqual(redactionRows(12))
  })

  it("alternates answers (left) and prompts (right) with lines that fit the row", () => {
    const rows = redactionRows(30)
    expect(rows).toHaveLength(30)
    expect(rows[1].side).toBe("right")
    for (const row of rows) {
      expect(row.lines.length).toBeGreaterThan(0)
      expect(row.lines.reduce((sum, l) => sum + l.width, 0)).toBeLessThanOrEqual(100)
      for (const l of row.lines) expect(HARNESS_COLORS).toContain(l.color)
    }
  })

  it("peels the bars top to bottom", () => {
    const rows = redactionRows(30)
    expect(rows[0].lines[0].delay).toBeLessThan(rows[29].lines[0].delay)
  })
})
