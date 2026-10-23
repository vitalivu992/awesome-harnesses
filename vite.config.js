import { defineConfig } from "vite"
import react from "@vitejs/plugin-react"

const apiTarget = process.env.API_TARGET || "http://127.0.0.1:8767"

export default defineConfig({
  plugins: [react()],
  build: {
    // web/dist is embedded into the Go binary via go:embed (web/embed.go).
    outDir: "web/dist",
    emptyOutDir: true,
    // mermaid's diagram chunks are big but lazy-loaded (only for answers with a diagram).
    chunkSizeWarningLimit: 1600,
  },
  // `pnpm dev` serves the app with HMR and proxies the Go API (`make dev`).
  server: {
    proxy: {
      "/api": apiTarget,
      "/health": apiTarget,
    },
  },
  test: {
    include: ["tests/**/*.test.{js,mjs,jsx}"],
    environment: "node",
  },
})
