import { useSyncExternalStore } from "react"
import { getThemeState, subscribeTheme } from "../lib/theme.js"

const SERVER_STATE = { pref: "system", mode: "light" }

/** `{ pref: "system"|"light"|"dark", mode: "light"|"dark" }`. */
export function useTheme() {
  return useSyncExternalStore(subscribeTheme, getThemeState, () => SERVER_STATE)
}
