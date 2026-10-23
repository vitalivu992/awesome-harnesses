import { useEffect, useState } from "react"

/**
 * Runs `load(signal)` whenever `deps` change (skipped while `enabled` is
 * false) and returns `{ data, error, loading }`. A newer load aborts the
 * older one, so a slow response never overwrites a newer selection.
 */
export function useFetch(load, deps, { enabled = true } = {}) {
  const [state, setState] = useState({ data: null, error: null, loading: enabled })

  useEffect(() => {
    if (!enabled) {
      setState({ data: null, error: null, loading: false })
      return undefined
    }
    const ctrl = new AbortController()
    setState((s) => ({ data: s.data, error: null, loading: true }))
    load(ctrl.signal).then(
      (data) => !ctrl.signal.aborted && setState({ data, error: null, loading: false }),
      (error) => !ctrl.signal.aborted && setState({ data: null, error, loading: false })
    )
    return () => ctrl.abort()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, ...deps])

  return state
}
