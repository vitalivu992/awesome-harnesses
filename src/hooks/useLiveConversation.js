import { useEffect, useRef, useState } from "react"
import { fetchConversation } from "../lib/api.js"
import { pollDelay } from "../lib/live.js"

const RUNNING_POLL_MS = 1500

/**
 * Loads a conversation like `useFetch`, then keeps asking for it (with its
 * ETag, so an unchanged one is a cheap 304) and swaps in each new version:
 * the view of a live session shows new messages as the agent writes them.
 * Polls fast while the session is active, slowly otherwise, and not at all
 * while the page is hidden. A failed poll keeps what is shown.
 *
 * `running`: a prompt sent from here is running in it, so it is asked for
 * often, and a conversation not found yet (a new one whose transcript the
 * CLI hasn't written) is asked for again instead of failing.
 */
export function useLiveConversation(harness, id, { running = false } = {}) {
  const enabled = Boolean(harness && id)
  const [state, setState] = useState({ data: null, error: null, loading: enabled })
  const runningRef = useRef(running)
  runningRef.current = running
  const delay = (data) => (runningRef.current ? Math.min(pollDelay(data), RUNNING_POLL_MS) : pollDelay(data))
  // Asks at once (a prompt was just sent: the slow idle poll would only notice it later).
  const wake = useRef(() => {})

  useEffect(() => {
    if (!enabled) {
      setState({ data: null, error: null, loading: false })
      return undefined
    }
    const ctrl = new AbortController()
    let etag = ""
    let data = null
    let timer = 0
    let busy = false
    const schedule = (ms) => {
      clearTimeout(timer)
      timer = setTimeout(tick, ms)
    }
    async function tick() {
      if (busy || ctrl.signal.aborted) return
      if (data && document.hidden) return schedule(delay(data))
      busy = true
      try {
        const res = await fetchConversation(harness, id, { signal: ctrl.signal, etag })
        if (ctrl.signal.aborted) return
        etag = res.etag
        if (res.conversation) {
          data = res.conversation
          setState({ data, error: null, loading: false })
        }
      } catch (error) {
        if (ctrl.signal.aborted) return
        if (!data && error.status === 404 && runningRef.current) {
          setState({ data: null, error: null, loading: true })
        } else if (!data) {
          return setState({ data: null, error, loading: false })
        }
      } finally {
        busy = false
      }
      schedule(delay(data))
    }
    const onVisible = () => !document.hidden && data && schedule(0)
    wake.current = () => schedule(0)

    setState((s) => ({ data: s.data, error: null, loading: true }))
    tick()
    document.addEventListener("visibilitychange", onVisible)
    return () => {
      ctrl.abort()
      clearTimeout(timer)
      document.removeEventListener("visibilitychange", onVisible)
      wake.current = () => {}
    }
  }, [enabled, harness, id])

  useEffect(() => {
    if (running) wake.current()
  }, [running])

  return state
}
