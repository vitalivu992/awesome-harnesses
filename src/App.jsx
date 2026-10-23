import { useCallback, useEffect, useState } from "react"
import { BrowserRouter, Route, Routes, useSearchParams } from "react-router"
import {
  LOGGED_OUT_EVENT,
  fetchConversation,
  fetchConversations,
  fetchProjects,
  fetchSession,
  logout,
} from "./lib/api.js"
import { readPrefs, writePref } from "./lib/prefs.js"
import { useFetch } from "./hooks/useFetch.js"
import { ProjectsPanel } from "./components/ProjectsPanel.jsx"
import { ConversationList } from "./components/ConversationList.jsx"
import { ConversationView } from "./components/ConversationView.jsx"
import { LoginScreen } from "./components/LoginScreen.jsx"

/** "claude:<id>" → ["claude", "<id>"]. */
export function splitConversationKey(key) {
  const s = String(key || "")
  const i = s.indexOf(":")
  return i > 0 ? [s.slice(0, i), s.slice(i + 1)] : ["", ""]
}

/**
 * The browser: projects | conversations | conversation. The selection lives
 * in the URL (`?project=<key>&c=<harness>:<id>`), so a conversation can be
 * linked to and the back button works.
 */
function Browser({ user, onLogout }) {
  const [params, setParams] = useSearchParams()
  const projectKey = params.get("project") || ""
  const convKey = params.get("c") || ""
  const [collapsed, setCollapsed] = useState(() => Boolean(readPrefs().projectsCollapsed))
  const [projectsReload, setProjectsReload] = useState(0)
  const [convsReload, setConvsReload] = useState(0)

  const projects = useFetch((signal) => fetchProjects({ signal }), [projectsReload])
  const conversations = useFetch((signal) => fetchConversations(projectKey, { signal }), [projectKey, convsReload], {
    enabled: Boolean(projectKey),
  })
  const [harness, id] = splitConversationKey(convKey)
  const conversation = useFetch((signal) => fetchConversation(harness, id, { signal }), [harness, id], {
    enabled: Boolean(harness && id),
  })

  // Open the most recently active project when none is selected.
  useEffect(() => {
    if (!projectKey && projects.data?.length) {
      setParams({ project: projects.data[0].key }, { replace: true })
    }
  }, [projectKey, projects.data, setParams])

  const project = projects.data?.find((p) => p.key === projectKey) || null

  useEffect(() => {
    const title = conversation.data?.title || project?.name
    document.title = title ? `${title} · awesome-harness` : "awesome-harness"
  }, [conversation.data?.title, project?.name])

  const selectProject = useCallback((key) => setParams({ project: key }), [setParams])
  const selectConversation = useCallback((key) => setParams({ project: projectKey, c: key }), [setParams, projectKey])
  const toggleCollapsed = () => {
    setCollapsed((v) => {
      writePref("projectsCollapsed", !v)
      return !v
    })
  }

  return (
    <div className={`ah-app${collapsed ? " is-collapsed" : ""}`}>
      <ProjectsPanel
        projects={projects.data}
        loading={projects.loading}
        error={projects.error}
        selectedKey={projectKey}
        onSelect={selectProject}
        collapsed={collapsed}
        onToggle={toggleCollapsed}
        onRefresh={() => setProjectsReload((n) => n + 1)}
        user={user}
        onLogout={onLogout}
      />
      <ConversationList
        key={projectKey}
        project={project}
        conversations={conversations.data}
        loading={conversations.loading}
        error={conversations.error}
        selectedId={convKey}
        onSelect={selectConversation}
        onRefresh={() => setConvsReload((n) => n + 1)}
      />
      <main className="ah-main">
        <ConversationView
          conversation={conversation.data}
          loading={conversation.loading}
          error={conversation.error}
        />
      </main>
    </div>
  )
}

/**
 * The login gate: the browser only mounts with a session. Right after a login
 * it mounts under the login screen, which plays its unlock sequence on top
 * while the projects load.
 */
function AuthGate() {
  // undefined: still asking the server; null: logged out.
  const [user, setUser] = useState(undefined)
  const [unlocking, setUnlocking] = useState(false)

  useEffect(() => {
    const ctrl = new AbortController()
    fetchSession({ signal: ctrl.signal }).then(
      (u) => setUser(u),
      () => !ctrl.signal.aborted && setUser(null)
    )
    const loggedOut = () => setUser(null)
    window.addEventListener(LOGGED_OUT_EVENT, loggedOut)
    return () => {
      ctrl.abort()
      window.removeEventListener(LOGGED_OUT_EVENT, loggedOut)
    }
  }, [])

  useEffect(() => {
    if (user === null) document.title = "Log in · awesome-harness"
  }, [user])

  const onLogin = useCallback((u) => {
    setUnlocking(true)
    setUser(u)
  }, [])
  const onUnlocked = useCallback(() => setUnlocking(false), [])
  const onLogout = useCallback(async () => {
    try {
      await logout()
    } finally {
      setUnlocking(false)
      setUser(null)
    }
  }, [])

  if (user === undefined) return null
  return (
    <>
      {user ? <Browser user={user} onLogout={onLogout} /> : null}
      {!user || unlocking ? <LoginScreen unlockedAs={user} onLogin={onLogin} onDone={onUnlocked} /> : null}
    </>
  )
}

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="*" element={<AuthGate />} />
      </Routes>
    </BrowserRouter>
  )
}
