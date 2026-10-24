import { useEffect, useState } from "react"
import { fetchCommands, fetchHarnesses, fetchModels } from "../lib/api.js"

// Asked once per page (harnesses, models) or per folder (commands); a failed load is asked again next time.
const cache = new Map()

function cached(key, load) {
  if (!cache.has(key)) {
    const p = load()
    cache.set(key, p)
    p.catch(() => cache.delete(key))
  }
  return cache.get(key)
}

function useLoad(key, load) {
  const [state, setState] = useState({ data: null, error: null, loading: Boolean(key) })
  useEffect(() => {
    if (!key) {
      setState({ data: null, error: null, loading: false })
      return undefined
    }
    let live = true
    setState((s) => ({ data: s.data, error: null, loading: true }))
    cached(key, load).then(
      (data) => live && setState({ data, error: null, loading: false }),
      (error) => live && setState({ data: null, error, loading: false })
    )
    return () => {
      live = false
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key])
  return state
}

/** The harnesses and their CLIs (GET /api/harnesses), by id. */
export function useHarnesses() {
  const state = useLoad("harnesses", () => fetchHarnesses())
  const byId = {}
  for (const h of state.data || []) byId[h.id] = h
  return { ...state, byId }
}

/** The slash commands and skills of a harness in a folder. */
export function useCommands(harness, cwd) {
  return useLoad(harness ? `commands:${harness}:${cwd || ""}` : "", () => fetchCommands(harness, cwd))
}

/** The models a harness offers; `version` > 0 asks the CLI again (refresh). */
export function useModels(harness, { enabled = true, version = 0 } = {}) {
  const key = harness && enabled ? `models:${harness}:${version}` : ""
  return useLoad(key, () => fetchModels(harness, { refresh: version > 0 }))
}
