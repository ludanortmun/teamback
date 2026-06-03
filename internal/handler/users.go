package handler

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/ludanortmun/teamback/internal/core"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func (h *Handler) HandleNewUser(w http.ResponseWriter, r *http.Request) {
	h.Render(w, "user_new.html", h.templateData(r, nil))
}

func (h *Handler) HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))

	var errs []string
	if name == "" {
		errs = append(errs, "El nombre es obligatorio")
	}
	if email == "" {
		errs = append(errs, "El correo electrónico es obligatorio")
	} else if !emailRegex.MatchString(email) {
		errs = append(errs, "El formato del correo electrónico no es válido")
	}

	if len(errs) > 0 {
		h.Render(w, "user_new.html", h.templateData(r, map[string]any{
			"Errors": errs,
			"Name":   name,
			"Email":  email,
		}))
		return
	}

	user := core.User{
		Name:  name,
		Email: email,
		Role:  core.RoleStudent,
	}

	ctx := h.authCtx(r)
	_, err := h.Client.AddUser(ctx, user)
	if err != nil {
		h.Render(w, "user_new.html", h.templateData(r, map[string]any{
			"Errors": []string{localizedError(err)},
			"Name":   name,
			"Email":  email,
		}))
		return
	}

	http.Redirect(w, r, "/assignments", http.StatusSeeOther)
}
