// The Go server's read-only API (internal/api).

async function getJSON(url, { signal } = {}) {
  const res = await fetch(url, { signal, headers: { Accept: "application/json" } })
  let body = null
  try {
    body = await res.json()
  } catch {
    body = null
  }
  if (!res.ok) throw new Error(body?.error || `${res.status} ${res.statusText}`)
  return body
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
