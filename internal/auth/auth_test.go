package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	h := &Handler{Auth: func(user, password string) error {
		if user == "me" && password == "secret" {
			return nil
		}
		return ErrDenied
	}}
	mux := http.NewServeMux()
	h.Register(mux)
	mux.HandleFunc("GET /api/projects", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"projects":[]}`)) })
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`ok`)) })
	srv := httptest.NewServer(h.Guard(mux))
	t.Cleanup(srv.Close)
	return srv
}

func do(t *testing.T, method, url, body, cookie string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: CookieName, Value: cookie})
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res
}

func sessionCookie(res *http.Response) *http.Cookie {
	for _, c := range res.Cookies() {
		if c.Name == CookieName {
			return c
		}
	}
	return nil
}

func TestLoginGuardLogout(t *testing.T) {
	srv := newServer(t)

	if res := do(t, "GET", srv.URL+"/api/projects", "", ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("api without login: %d", res.StatusCode)
	}
	if res := do(t, "GET", srv.URL+"/health", "", ""); res.StatusCode != http.StatusOK {
		t.Fatalf("health stays public: %d", res.StatusCode)
	}
	if res := do(t, "GET", srv.URL+"/api/auth/session", "", ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("session without login: %d", res.StatusCode)
	}

	res := do(t, "POST", srv.URL+"/api/auth/login", `{"username":"me","password":"wrong"}`, "")
	if res.StatusCode != http.StatusUnauthorized || sessionCookie(res) != nil {
		t.Fatalf("wrong password: %d, cookie %v", res.StatusCode, sessionCookie(res))
	}

	res = do(t, "POST", srv.URL+"/api/auth/login", `{"username":"me","password":"secret"}`, "")
	c := sessionCookie(res)
	if res.StatusCode != http.StatusOK || c == nil {
		t.Fatalf("login: %d, cookie %v", res.StatusCode, c)
	}
	// A session cookie: no expiry, so the browser forgets it on exit.
	if !c.Expires.IsZero() || c.MaxAge != 0 || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie attributes: %+v", c)
	}
	if res := do(t, "GET", srv.URL+"/api/projects", "", c.Value); res.StatusCode != http.StatusOK {
		t.Fatalf("api after login: %d", res.StatusCode)
	}
	if res := do(t, "GET", srv.URL+"/api/projects", "", "forged"); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("forged cookie: %d", res.StatusCode)
	}

	res = do(t, "POST", srv.URL+"/api/auth/logout", `{}`, c.Value)
	if res.StatusCode != http.StatusNoContent || sessionCookie(res) == nil || sessionCookie(res).MaxAge >= 0 {
		t.Fatalf("logout: %d, cookie %v", res.StatusCode, sessionCookie(res))
	}
	if res := do(t, "GET", srv.URL+"/api/projects", "", c.Value); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("api after logout: %d", res.StatusCode)
	}
}

func TestLoginNeedsJSON(t *testing.T) {
	srv := newServer(t)
	req, _ := http.NewRequest("POST", srv.URL+"/api/auth/login", strings.NewReader("username=me&password=secret"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnsupportedMediaType || sessionCookie(res) != nil {
		t.Fatalf("form post: %d", res.StatusCode)
	}
}

func TestNoLogin(t *testing.T) {
	h := &Handler{NoLogin: "me", Auth: func(string, string) error { return ErrDenied }}
	mux := http.NewServeMux()
	h.Register(mux)
	mux.HandleFunc("GET /api/projects", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"projects":[]}`)) })
	srv := httptest.NewServer(h.Guard(mux))
	t.Cleanup(srv.Close)

	if res := do(t, "GET", srv.URL+"/api/projects", "", ""); res.StatusCode != http.StatusOK {
		t.Fatalf("api without login: %d", res.StatusCode)
	}
	if res := do(t, "GET", srv.URL+"/api/auth/session", "", ""); res.StatusCode != http.StatusOK {
		t.Fatalf("session without login: %d", res.StatusCode)
	}
}
