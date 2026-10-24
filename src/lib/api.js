// The Go server's read-only API (internal/api) and its login (internal/auth).

// Fired when the API answers 401: the session is gone (logged out in another
// tab, server restarted), so the app goes back to the login screen.
export const LOGGED_OUT_EVENT = "ah:logged-out"

async function request(url, { signal, method = "GET", body } = {}) {
  const headers = { Accept: "application/json" }
  if (body !== undefined) headers["Content-Type"] = "application/json"
  const res = await fetch(url, { signal, method, headers, body: body === undefined ? undefined : JSON.stringify(body) })
  let data = null
  try {
    data = await res.json()
  } catch {
    data = null
  }
  if (!res.ok) {
    const err = new Error(data?.error || `${res.status} ${res.statusText}`)
    err.status = res.status
    throw err
  }
  return data
}

async function getJSON(url, opts) {
  try {
    return await request(url, opts)
  } catch (err) {
    if (err.status === 401) window.dispatchEvent(new Event(LOGGED_OUT_EVENT))
    throw err
  }
}

/** The logged-in user, or null. */
export async function fetchSession(opts) {
  try {
    return (await request("/api/auth/session", opts)).user
  } catch (err) {
    if (err.status === 401) return null
    throw err
  }
}

/** Checks the password with PAM on the server; resolves to the user, or throws. */
export async function login(username, password) {
  return (await request("/api/auth/login", { method: "POST", body: { username, password } })).user
}

export async function logout() {
  await request("/api/auth/logout", { method: "POST", body: {} })
}

export async function fetchProjects(opts) {
  return (await getJSON("/api/projects", opts)).projects || []
}

export async function fetchConversations(projectKey, opts) {
  const q = new URLSearchParams({ project: projectKey })
  return (await getJSON(`/api/conversations?${q}`, opts)).conversations || []
}

export async function fetchConversation(harness, id, opts) {
  return getJSON(`/api/conversations/${encodeURIComponent(harness)}/${encodeURIComponent(id)}`, opts)
}

// The terminal panel's shells (internal/shell); each terminal streams over a WebSocket.

export async function fetchShells(opts) {
  return (await getJSON("/api/shells", opts)).shells || []
}

/** Starts a login shell in `cwd` (the server's home when empty); `run` is typed at its first prompt. */
export async function createShell({ cwd = "", run = "" } = {}) {
  return getJSON("/api/shells", { method: "POST", body: { cwd, run } })
}

export async function killShell(id) {
  await getJSON(`/api/shells/${encodeURIComponent(id)}`, { method: "DELETE" })
}
