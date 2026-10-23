// Package api serves the catalog as JSON:
//
//	GET /api/projects                         every project, newest activity first
//	GET /api/conversations?project=<key>      one project's conversation summaries
//	GET /api/conversations/{harness}/{id}     one conversation in full
package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"awesome-harness/internal/catalog"
)

// Handler serves the catalog.
type Handler struct {
	Catalog *catalog.Catalog
}

// Register adds the routes to mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/projects", h.projects)
	mux.HandleFunc("GET /api/conversations", h.conversations)
	mux.HandleFunc("GET /api/conversations/{harness}/{id}", h.conversation)
}

func (h *Handler) projects(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"projects": h.Catalog.Projects()})
}

func (h *Handler) conversations(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("project")
	if key == "" {
		writeError(w, http.StatusBadRequest, "project is required")
		return
	}
	sums, err := h.Catalog.Conversations(key)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": sums})
}

func (h *Handler) conversation(w http.ResponseWriter, r *http.Request) {
	c, err := h.Catalog.Conversation(r.PathValue("harness"), r.PathValue("id"))
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func writeCatalogError(w http.ResponseWriter, err error) {
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	log.Printf("catalog: %v", err)
	writeError(w, http.StatusInternalServerError, err.Error())
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("api: write response: %v", err)
	}
}
