import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react"
import { fetchRuns, startRun, stopRun } from "../lib/api.js"

const RunsContext = createContext(null)

/** `{ runs, start(req, {turns}), stop(id), dismiss(id), runFor(harness, session), sentAt(id) }`. */
export function useRuns() {
  return useContext(RunsContext)
}

// While a run is going its state is asked for this often (the new conversation's id, its end).
const POLL_MS = 1500

/**
 * The prompts sent from this server (GET /api/runs): restored on load, polled
 * while one is running. `start` remembers how many turns the conversation had,
 * so the view can show the prompt as sent until the transcript has it.
 */
export function RunsProvider({ children }) {
  const [runs, setRuns] = useState([])
  const [turns, setTurns] = useState({})
  const [dismissed, setDismissed] = useState(() => new Set())

  const refresh = useCallback(async () => {
    try {
      setRuns(await fetchRuns())
    } catch {
      // keep what is shown; the next poll asks again
    }
  }, [])

  useEffect(() => {
    refresh()
  }, [refresh])

  const anyRunning = runs.some((r) => r.running)
  useEffect(() => {
    if (!anyRunning) return undefined
    const timer = setInterval(() => !document.hidden && refresh(), POLL_MS)
    return () => clearInterval(timer)
  }, [anyRunning, refresh])

  const start = useCallback(async (req, { turns: before = 0 } = {}) => {
    const run = await startRun(req)
    setTurns((t) => ({ ...t, [run.id]: before }))
    setRuns((list) => [run, ...list.filter((r) => r.id !== run.id)])
    return run
  }, [])

  const stop = useCallback(
    async (id) => {
      await stopRun(id)
      await refresh()
    },
    [refresh]
  )

  const dismiss = useCallback((id) => setDismissed((s) => new Set(s).add(id)), [])

  const value = useMemo(() => {
    const shown = runs.filter((r) => !dismissed.has(r.id))
    return {
      runs: shown,
      start,
      stop,
      dismiss,
      refresh,
      // The newest run of a conversation (the list is newest first).
      runFor: (harness, session) => shown.find((r) => r.harness === harness && r.session && r.session === session) || null,
      sentAt: (id) => turns[id] ?? 0,
    }
  }, [runs, dismissed, turns, start, stop, dismiss, refresh])

  return <RunsContext.Provider value={value}>{children}</RunsContext.Provider>
}
