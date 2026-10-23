import { StrictMode } from "react"
import { createRoot } from "react-dom/client"
import "@primer/css/dist/primer.css"
import "@fortawesome/fontawesome-free/css/all.min.css"
import "./styles/app.css"
import { App } from "./App.jsx"
import { initTheme } from "./lib/theme.js"

initTheme()

createRoot(document.getElementById("root")).render(
  <StrictMode>
    <App />
  </StrictMode>
)
