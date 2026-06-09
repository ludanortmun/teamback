package auth

import (
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
)

const (
	sessionCookieName = "session"
	sessionMaxAge     = 7 * 24 * time.Hour
	flashKey          = "flash"
	userIDKey         = "user_id"
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
	_ = s.setValues(w, map[string]string{userIDKey: userID})
}

func (s *SessionStore) Get(r *http.Request) (string, bool) {
	values, err := s.getValues(r)
	if err != nil {
		return "", false
	}
	userID, ok := values[userIDKey]
	return userID, ok && userID != ""
}

func (s *SessionStore) SetFlash(w http.ResponseWriter, r *http.Request, message string) error {
	values, err := s.getValues(r)
	if err != nil {
		return err
	}
	values[flashKey] = message
	return s.setValues(w, values)
}

func (s *SessionStore) PopFlash(w http.ResponseWriter, r *http.Request) (string, bool, error) {
	values, err := s.getValues(r)
	if err != nil {
		return "", false, err
	}
	message, ok := values[flashKey]
	if !ok || message == "" {
		return "", false, nil
	}
	delete(values, flashKey)
	if err := s.setValues(w, values); err != nil {
		return "", false, err
	}
	return message, true, nil
}

func (s *SessionStore) getValues(r *http.Request) (map[string]string, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	if err := s.sc.Decode(sessionCookieName, cookie.Value, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func (s *SessionStore) setValues(w http.ResponseWriter, values map[string]string) error {
	encoded, err := s.sc.Encode(sessionCookieName, values)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    encoded,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionMaxAge.Seconds()),
	})
	return nil
}

func (s *SessionStore) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   sessionCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}
