import { useEffect, useState } from "react"
import { fetchConversation } from "../lib/api.js"
import { pollDelay } from "../lib/live.js"

/**
 * Loads a conversation like `useFetch`, then keeps asking for it (with its
 * ETag, so an unchanged one is a cheap 304) and swaps in each new version:
 * the view of a live session shows new messages as the agent writes them.
 * Polls fast while the session is active, slowly otherwise, and not at all
 * while the page is hidden. A failed poll keeps what is shown.
 */
export function useLiveConversation(harness, id) {
  const enabled = Boolean(harness && id)
  const [state, setState] = useState({ data: null, error: null, loading: enabled })

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
      if (data && document.hidden) return schedule(pollDelay(data))
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
        if (!data) return setState({ data: null, error, loading: false })
      } finally {
        busy = false
      }
      schedule(pollDelay(data))
    }
    const onVisible = () => !document.hidden && data && schedule(0)

    setState((s) => ({ data: s.data, error: null, loading: true }))
    tick()
    document.addEventListener("visibilitychange", onVisible)
    return () => {
      ctrl.abort()
      clearTimeout(timer)
      document.removeEventListener("visibilitychange", onVisible)
    }
  }, [enabled, harness, id])

  return state
}
