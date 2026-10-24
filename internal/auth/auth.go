// Package auth puts a login in front of the API: a username and password
// checked by an Authenticator (System: PAM on Linux and macOS, LogonUser on
// Windows), then a session
// cookie with no expiry, so the browser drops it when it exits.
//
//	GET  /api/auth/session   200 {"user"} while logged in, else 401
//	POST /api/auth/login     {"username", "password"} → 200 {"user"} + cookie, or 401
//	POST /api/auth/logout    forgets the session, clears the cookie → 204
//
// Sessions live in memory: restarting the server logs everyone out.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"mime"
	"net/http"
	"strings"
	"sync"
)

// CookieName is the session cookie.
const CookieName = "ah_session"

// ErrDenied is what an Authenticator returns for a wrong username or password.
var ErrDenied = errors.New("wrong username or password")

// Authenticator checks a username and password. It returns ErrDenied (or
// another error) when the login must be refused.
type Authenticator func(user, password string) error

// Handler serves the auth routes and guards the rest of the API.
type Handler struct {
	Auth Authenticator
	// NoLogin, when set, turns the login off: every request counts as this
	// user's session (AUTH_DISABLED, for local testing and agents).
	NoLogin string
	// OnLogout, when set, runs after a logout (it ends the shells).
	OnLogout func()

	mu       sync.Mutex
	sessions map[[32]byte]string // user by sha256 of the cookie value
	loginMu  sync.Mutex          // one password check at a time (a failure takes ~2-3 s)
}

// Register adds the auth routes to mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/session", h.session)
	mux.HandleFunc("POST /api/auth/login", h.login)
	mux.HandleFunc("POST /api/auth/logout", h.logout)
}

// Guard answers 401 to every /api/* request without a valid session, except
// the auth routes. The app bundle and /health stay public: they hold no data.
func (h *Handler) Guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && !strings.HasPrefix(r.URL.Path, "/api/auth/") {
			if _, ok := h.user(r); !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "login required"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) session(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "login required"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"user": user})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	// A JSON body can't come from a plain cross-site form post.
	if !isJSON(r) {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "expected application/json"})
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	user := strings.TrimSpace(body.Username)
	if user == "" || body.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password are required"})
		return
	}

	h.loginMu.Lock()
	err := h.Auth(user, body.Password)
	h.loginMu.Unlock()
	if err != nil {
		if !errors.Is(err, ErrDenied) {
			log.Printf("auth: login %q from %s: %v", user, r.RemoteAddr, err)
		} else {
			log.Printf("auth: login %q from %s refused", user, r.RemoteAddr)
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": ErrDenied.Error()})
		return
	}

	token, err := newToken()
	if err != nil {
		log.Printf("auth: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not start a session"})
		return
	}
	h.mu.Lock()
	if h.sessions == nil {
		h.sessions = map[[32]byte]string{}
	}
	h.sessions[sha256.Sum256([]byte(token))] = user
	h.mu.Unlock()
	log.Printf("auth: %q logged in from %s", user, r.RemoteAddr)

	// No Expires / Max-Age: a session cookie, gone when the browser exits.
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})
	writeJSON(w, http.StatusOK, map[string]string{"user": user})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if !isJSON(r) {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "expected application/json"})
		return
	}
	if c, err := r.Cookie(CookieName); err == nil {
		h.mu.Lock()
		delete(h.sessions, sha256.Sum256([]byte(c.Value)))
		h.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
	if h.OnLogout != nil {
		go h.OnLogout()
	}
}

// user returns the logged-in user of r's session cookie.
func (h *Handler) user(r *http.Request) (string, bool) {
	if h.NoLogin != "" {
		return h.NoLogin, true
	}
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return "", false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	user, ok := h.sessions[sha256.Sum256([]byte(c.Value))]
	return user, ok
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func isJSON(r *http.Request) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mt == "application/json"
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("auth: write response: %v", err)
	}
}
