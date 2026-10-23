// The login screen's backdrop: rows of chat lines under redaction bars.
// Rows alternate like the chat view (prompt on the right, answer on the left),
// and each line takes the colour of one of the harnesses.

export const HARNESS_COLORS = ["#d97757", "#8f9bb3", "#10a37f", "#e8b931", "#7c8cff", "#f06292"]

// A small seeded PRNG (mulberry32), so the layout is the same on every load.
function random(seed) {
  let a = seed >>> 0
  return () => {
    a = (a + 0x6d2b79f5) >>> 0
    let t = a
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

/**
 * `rows` rows of 1–3 lines: `{ side: "left" | "right", lines: [{ width, color, delay }] }`.
 * `width` is a percentage of the row; `delay` (ms) staggers the unlock wipe
 * along a diagonal (top-left first).
 */
export function redactionRows(rows = 30, seed = 7) {
  const rnd = random(seed)
  return Array.from({ length: rows }, (_, r) => {
    const side = r % 3 === 1 ? "right" : "left"
    const count = side === "right" ? 1 : 1 + Math.floor(rnd() * 3)
    let offset = 0
    const lines = Array.from({ length: count }, (_, i) => {
      const width = Math.round((side === "right" ? 14 : 8) + rnd() * 22)
      const line = {
        width,
        color: HARNESS_COLORS[Math.floor(rnd() * HARNESS_COLORS.length)],
        delay: Math.round(r * 26 + offset * 5 + i * 30),
      }
      offset += width
      return line
    })
    return { side, lines }
  })
}
