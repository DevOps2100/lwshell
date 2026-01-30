package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

const (
	cookieName   = "ssh_manager_session"
	sessionTTL   = 24 * time.Hour
	cookieMaxAge = 86400 // 24h in seconds
)

var (
	sessions   = make(map[string]time.Time)
	sessionsMu sync.RWMutex
)

func newSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func createSession(w http.ResponseWriter) (string, error) {
	id, err := newSessionID()
	if err != nil {
		return "", err
	}
	sessionsMu.Lock()
	sessions[id] = time.Now().Add(sessionTTL)
	sessionsMu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    id,
		Path:     "/",
		MaxAge:   cookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return id, nil
}

func getSession(r *http.Request) (string, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return "", false
	}
	sessionsMu.RLock()
	exp, ok := sessions[c.Value]
	sessionsMu.RUnlock()
	if !ok || time.Now().After(exp) {
		return "", false
	}
	return c.Value, true
}

func destroySession(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(cookieName)
	if err == nil && c.Value != "" {
		sessionsMu.Lock()
		delete(sessions, c.Value)
		sessionsMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
