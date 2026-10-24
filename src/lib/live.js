// A live session: the view polls the conversation while its transcript keeps changing.

/** The agent is still on its last turn: the transcript changed recently and that turn hasn't ended. */
export function isWorking(conversation) {
  return Boolean(conversation?.active && conversation.status === "incomplete")
}

/** How long to wait before asking for the conversation again (the server answers 304 when nothing changed). */
export function pollDelay(conversation) {
  return conversation?.active ? 2000 : 15000
}

/** How long before a list (projects, conversations) is reloaded: soon while something in it is running. */
export function listPollDelay(running) {
  return running ? 4000 : 15000
}

/** A listed conversation's live mark: "running", "finished" (active, last turn ended) or "". */
export function liveState(conversation) {
  if (!conversation?.active || !conversation.turns) return ""
  return conversation.status === "incomplete" ? "running" : "finished"
}

/** The scroll box is at (or within `slack` px of) its bottom. */
export function atBottom(el, slack = 80) {
  return el.scrollHeight - el.scrollTop - el.clientHeight <= slack
}
