import { useEffect, useState } from "react"
import { nextWorkingWord } from "../lib/live.js"

/**
 * Windows 11's indeterminate progress bar, laid on the bottom edge of its
 * positioned parent (the view header): the agent is at work there.
 */
export function RunningBar() {
  return <span className="ah-progress" aria-hidden="true"></span>
}

/** "Pondering…", "Brewing…": a new word every `every` ms while the agent works. */
export function WorkingWord({ every = 3000 }) {
  const [word, setWord] = useState(() => nextWorkingWord(""))
  useEffect(() => {
    const timer = setInterval(() => setWord((w) => nextWorkingWord(w)), every)
    return () => clearInterval(timer)
  }, [every])
  return (
    <span className="ah-working-word" key={word}>
      {word}…
    </span>
  )
}

const LIVE = {
  running: { icon: "fa-tower-broadcast", label: "Working" },
  finished: { icon: "fa-circle-check", label: "Finished" },
}

/**
 * A list row's live mark (`lib/live.js` `liveState`): a pulsing broadcast icon while
 * the agent works, a check once it has just finished. `count` > 1 is shown beside it.
 */
export function LiveIcon({ state, count = 0, title = "" }) {
  const meta = LIVE[state]
  if (!meta) return null
  const label = title || meta.label
  return (
    <span className={`ah-live ah-live--${state}`} title={label} aria-label={label} role="img">
      <i className={`fa-solid ${meta.icon}`} aria-hidden="true"></i>
      {count > 1 ? <span className="ah-live__count">{count}</span> : null}
    </span>
  )
}
