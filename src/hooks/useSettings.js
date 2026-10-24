import { useCallback, useEffect, useState } from "react"
import { SETTINGS_EVENT, readSettings, writeSettings } from "../lib/settings.js"

/** `[settings, save]`; every mounted reader sees a save (in this tab and the others). */
export function useSettings() {
  const [settings, setSettings] = useState(readSettings)

  useEffect(() => {
    const onChange = () => setSettings(readSettings())
    window.addEventListener(SETTINGS_EVENT, onChange)
    window.addEventListener("storage", onChange)
    return () => {
      window.removeEventListener(SETTINGS_EVENT, onChange)
      window.removeEventListener("storage", onChange)
    }
  }, [])

  const save = useCallback((next) => writeSettings(typeof next === "function" ? next(readSettings()) : next), [])
  return [settings, save]
}
