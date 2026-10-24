import { useEffect, useRef, useState } from "react"

/**
 * Runs `load(signal)` whenever `deps` change (skipped while `enabled` is
 * false) and returns `{ data, error, loading }`. A newer load aborts the
 * older one, so a slow response never overwrites a newer selection.
 *
 * `poll(data)` → ms (0 = stop) reloads quietly after each load: `loading`
 * stays false, a failed reload keeps what is shown, and nothing is asked
 * for while the page is hidden.
 */
export function useFetch(load, deps, { enabled = true, poll } = {}) {
  const [state, setState] = useState({ data: null, error: null, loading: enabled })
  const pollRef = useRef(poll)
  pollRef.current = poll

  useEffect(() => {
    if (!enabled) {
      setState({ data: null, error: null, loading: false })
      return undefined
    }
    const ctrl = new AbortController()
    let data = null
    let timer = 0
    const schedule = () => {
      clearTimeout(timer)
      const ms = pollRef.current?.(data)
      if (ms) timer = setTimeout(reload, ms)
    }
    function reload() {
      if (ctrl.signal.aborted) return
      if (document.hidden) return schedule()
      load(ctrl.signal).then(
        (d) => {
          if (ctrl.signal.aborted) return
          data = d
          setState({ data, error: null, loading: false })
          schedule()
        },
        () => !ctrl.signal.aborted && schedule()
      )
    }
    const onVisible = () => !document.hidden && data && pollRef.current && reload()

    setState((s) => ({ data: s.data, error: null, loading: true }))
    load(ctrl.signal).then(
      (d) => {
        if (ctrl.signal.aborted) return
        data = d
        setState({ data, error: null, loading: false })
        schedule()
      },
      (error) => !ctrl.signal.aborted && setState({ data: null, error, loading: false })
    )
    document.addEventListener("visibilitychange", onVisible)
    return () => {
      ctrl.abort()
      clearTimeout(timer)
      document.removeEventListener("visibilitychange", onVisible)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, ...deps])

  return state
}
