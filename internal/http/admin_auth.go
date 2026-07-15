package http

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	adminSessionCookie = "kd_admin_session"
	adminSessionTTL    = 8 * time.Hour
)

type adminAuthenticator struct {
	username string
	password string
	mu       sync.Mutex
	sessions map[string]time.Time
}

func newAdminAuthenticator(username, password string) *adminAuthenticator {
	return &adminAuthenticator{
		username: strings.TrimSpace(username),
		password: password,
		sessions: make(map[string]time.Time),
	}
}

func (a *adminAuthenticator) login(w http.ResponseWriter, r *http.Request, username, password string) error {
	if a.username == "" || a.password == "" {
		return errors.New("admin console is not configured")
	}
	if !constantTimeEqual(username, a.username) || !constantTimeEqual(password, a.password) {
		return errors.New("invalid username or password")
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return errors.New("create admin session")
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	expiresAt := time.Now().Add(adminSessionTTL)

	a.mu.Lock()
	a.removeExpiredLocked(time.Now())
	a.sessions[token] = expiresAt
	a.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookie,
		Value:    token,
		Path:     "/admin",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		Expires:  expiresAt,
		MaxAge:   int(adminSessionTTL.Seconds()),
	})
	return nil
}

func (a *adminAuthenticator) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(adminSessionCookie); err == nil {
		a.mu.Lock()
		delete(a.sessions, cookie.Value)
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookie,
		Path:     "/admin",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

// logoutAll invalidates every active admin session (including the caller's) and
// returns how many were cleared. Used by the settings "sign out everywhere" action.
func (a *adminAuthenticator) logoutAll() int {
	a.mu.Lock()
	n := len(a.sessions)
	a.sessions = make(map[string]time.Time)
	a.mu.Unlock()
	return n
}

// sessionCount returns the number of currently active (unexpired) admin sessions.
func (a *adminAuthenticator) sessionCount() int {
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	a.removeExpiredLocked(now)
	return len(a.sessions)
}

func (a *adminAuthenticator) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(adminSessionCookie)
		if err != nil || !a.validSession(cookie.Value) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "admin login required"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authenticated reports whether the request carries a valid admin session.
func (a *adminAuthenticator) authenticated(r *http.Request) bool {
	cookie, err := r.Cookie(adminSessionCookie)
	return err == nil && a.validSession(cookie.Value)
}

func (a *adminAuthenticator) validSession(token string) bool {
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()

	a.removeExpiredLocked(now)
	expiresAt, ok := a.sessions[token]
	return ok && expiresAt.After(now)
}

func (a *adminAuthenticator) removeExpiredLocked(now time.Time) {
	for token, expiresAt := range a.sessions {
		if !expiresAt.After(now) {
			delete(a.sessions, token)
		}
	}
}

func constantTimeEqual(actual, expected string) bool {
	if len(actual) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}
