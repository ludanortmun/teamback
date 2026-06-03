package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ludanortmun/teamback/internal/core"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Identity is used to link a given core.User profile to an external Identity (e.g. Google account).
// The main purpose of this is to decouple authentication from the core.User model.
type Identity struct {
	UserID     string
	ExternalID string
}

// IdentityStore allows interacting with the Identity database
type IdentityStore interface {
	// LinkIfNecessary performs an idempotent linkage between a core.User profile with an external user identity (e.g. Google account). It returns an Identity instance.
	LinkIfNecessary(user core.User, externalUser ExternalUserInfo) (Identity, error)
}

type OIDCHandler struct {
	oauthConfig *oauth2.Config
	sessions    *SessionStore
	storage     core.Storage
	identities  IdentityStore
}

func NewOIDCHandler(clientID, clientSecret, redirectURL string, sessions *SessionStore, str core.Storage, idStore IdentityStore) *OIDCHandler {
	return &OIDCHandler{
		oauthConfig: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
		sessions:   sessions,
		storage:    str,
		identities: idStore,
	}
}

func (h *OIDCHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := generateState()
	if err != nil {
		http.Error(w, "Error interno", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300,
	})

	url := h.oauthConfig.AuthCodeURL(state,
		oauth2.SetAuthURLParam("prompt", "select_account"))
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *OIDCHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value == "" {
		http.Error(w, "Falta el estado de autorización", http.StatusBadRequest)
		return
	}
	if r.URL.Query().Get("state") != stateCookie.Value {
		http.Error(w, "El estado de autorización no es válido", http.StatusBadRequest)
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Falta el código de autorización", http.StatusBadRequest)
		return
	}

	token, err := h.oauthConfig.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "No se pudo completar el inicio de sesión", http.StatusUnauthorized)
		return
	}

	userInfo, err := fetchGoogleUserInfo(r.Context(), token.AccessToken)
	if err != nil {
		http.Error(w, "No se pudo obtener la información del usuario", http.StatusInternalServerError)
		return
	}

	user, err := h.storage.ReadUserByEmail(strings.ToLower(userInfo.Email))
	if err != nil {
		if strings.Contains(err.Error(), "not registered") {
			http.Error(w, "Tu cuenta no está registrada. Contacta a tu profesor.", http.StatusForbidden)
			return
		}
		http.Error(w, "No se pudo iniciar sesión", http.StatusInternalServerError)
		return
	}

	identity, err := h.identities.LinkIfNecessary(user, *userInfo)
	if err != nil {
		http.Error(w, "No se pudo vincular el usuario", http.StatusInternalServerError)
		return
	}

	h.sessions.Set(w, identity.UserID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *OIDCHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	h.sessions.Clear(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

type ExternalUserInfo struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func fetchGoogleUserInfo(ctx context.Context, accessToken string) (*ExternalUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userinfo returned %d: %s", resp.StatusCode, body)
	}

	var info ExternalUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
