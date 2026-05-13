package handler

import (
	"database/sql"
	"html/template"
	"net/http"
	"path/filepath"
)

type Handler struct {
	DB        *sql.DB
	Templates *template.Template
}

func New(db *sql.DB, templatesDir string) (*Handler, error) {
	tmpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	if err != nil {
		return nil, err
	}
	return &Handler{DB: db, Templates: tmpl}, nil
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
