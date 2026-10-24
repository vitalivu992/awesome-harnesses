// A live session: the view polls the conversation while its transcript keeps changing.

/** The agent is still on its last turn: the transcript changed recently and that turn hasn't ended. */
export function isWorking(conversation) {
  return Boolean(conversation?.active && conversation.status === "incomplete")
}

/** How long to wait before asking for the conversation again (the server answers 304 when nothing changed). */
export function pollDelay(conversation) {
  return conversation?.active ? 2000 : 15000
}

/** The scroll box is at (or within `slack` px of) its bottom. */
export function atBottom(el, slack = 80) {
  return el.scrollHeight - el.scrollTop - el.clientHeight <= slack
}
