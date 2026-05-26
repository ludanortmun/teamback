package handler

import (
	"context"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/ludanortmun/teamback/internal/core"
	"github.com/ludanortmun/teamback/internal/middleware"
)

type Handler struct {
	Client    *core.Client
	Templates *template.Template
}

func New(client *core.Client, templatesDir string) (*Handler, error) {
	tmpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	if err != nil {
		return nil, err
	}
	return &Handler{Client: client, Templates: tmpl}, nil
}

// authCtx returns a context with the caller's user ID set for core.Client calls.
func (h *Handler) authCtx(r *http.Request) context.Context {
	userID := middleware.GetUserID(r.Context())
	return context.WithValue(r.Context(), core.CtxCallerKey, userID)
}

func (h *Handler) Render(w http.ResponseWriter, tmpl string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.Templates.ExecuteTemplate(w, tmpl, data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *Handler) HandleHome(w http.ResponseWriter, r *http.Request) {
	h.Render(w, "home.html", nil)
}

func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	h.Render(w, "login.html", nil)
}
