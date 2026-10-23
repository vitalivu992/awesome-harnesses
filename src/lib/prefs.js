// UI preferences in localStorage (per browser). Reads and writes never throw:
// storage can be blocked, and the app works with the defaults.

const KEY = "ah_prefs"

export function readPrefs() {
  try {
    return JSON.parse(localStorage.getItem(KEY) || "{}") || {}
  } catch {
    return {}
  }
}

export function writePref(name, value) {
  try {
    localStorage.setItem(KEY, JSON.stringify({ ...readPrefs(), [name]: value }))
  } catch {
    // ignore: the preference just isn't remembered
  }
}
