package auth

import (
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
)

const (
	sessionCookieName = "session"
	sessionMaxAge     = 7 * 24 * time.Hour
)

type SessionStore struct {
	sc *securecookie.SecureCookie
}

func NewSessionStore(secret string) *SessionStore {
	hashKey := []byte(secret)
	// Use first 16 bytes as block key for AES encryption if secret is long enough
	var blockKey []byte
	if len(hashKey) >= 32 {
		blockKey = hashKey[:16]
	}
	return &SessionStore{
		sc: securecookie.New(hashKey, blockKey),
	}
}

func (s *SessionStore) Set(w http.ResponseWriter, userID string) {
	encoded, err := s.sc.Encode(sessionCookieName, map[string]string{"user_id": userID})
	if err != nil {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    encoded,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionMaxAge.Seconds()),
	})
}

func (s *SessionStore) Get(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}
	values := map[string]string{}
	if err := s.sc.Decode(sessionCookieName, cookie.Value, &values); err != nil {
		return "", false
	}
	userID, ok := values["user_id"]
	return userID, ok && userID != ""
}

func (s *SessionStore) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   sessionCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}
