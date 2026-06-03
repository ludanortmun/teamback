package handler

import (
	"context"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/ludanortmun/teamback/internal/auth"
	"github.com/ludanortmun/teamback/internal/core"
	"github.com/ludanortmun/teamback/internal/middleware"
)

type Handler struct {
	Client    *core.Client
	Sessions  *auth.SessionStore
	Storage   core.Storage
	Templates map[string]*template.Template
}

func New(client *core.Client, sessions *auth.SessionStore, storage core.Storage, templatesDir string) (*Handler, error) {
	layoutFile := filepath.Join(templatesDir, "layout.html")
	layout, err := template.ParseFiles(layoutFile)
	if err != nil {
		return nil, err
	}

	pages, err := filepath.Glob(filepath.Join(templatesDir, "*.html"))
	if err != nil {
		return nil, err
	}

	templates := make(map[string]*template.Template)
	for _, page := range pages {
		name := filepath.Base(page)
		if name == "layout.html" {
			continue
		}
		cloned, err := layout.Clone()
		if err != nil {
			return nil, err
		}
		t, err := cloned.ParseFiles(page)
		if err != nil {
			return nil, err
		}
		templates[name] = t
	}

	return &Handler{Client: client, Sessions: sessions, Storage: storage, Templates: templates}, nil
}

// templateData builds a base data map including the current user (if authenticated).
func (h *Handler) templateData(r *http.Request, extra map[string]any) map[string]any {
	data := map[string]any{}
	userID := middleware.GetUserID(r.Context())
	if userID != "" {
		if user, err := h.Storage.ReadUser(userID); err == nil {
			data["User"] = user
			data["IsTeacher"] = user.Role == core.RoleTeacher
			data["IsStudent"] = user.Role == core.RoleStudent
		}
	}
	for k, v := range extra {
		data[k] = v
	}
	return data
}

// authCtx returns a context with the caller's user ID set for core.Client calls.
func (h *Handler) authCtx(r *http.Request) context.Context {
	userID := middleware.GetUserID(r.Context())
	return context.WithValue(r.Context(), core.CtxCallerKey, userID)
}

func (h *Handler) Render(w http.ResponseWriter, tmpl string, data any) {
	t, ok := h.Templates[tmpl]
	if !ok {
		http.Error(w, "No se encontró la plantilla", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "Error al renderizar la plantilla", http.StatusInternalServerError)
	}
}

func (h *Handler) HandleHome(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.Sessions.Get(r); ok {
		http.Redirect(w, r, "/assignments", http.StatusSeeOther)
		return
	}
	h.Render(w, "home.html", h.templateData(r, nil))
}

func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.Sessions.Get(r); ok {
		http.Redirect(w, r, "/assignments", http.StatusSeeOther)
		return
	}
	h.Render(w, "login.html", h.templateData(r, nil))
}
