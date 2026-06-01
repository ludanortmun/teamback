package handler

import (
	"net/http"
	"strings"

	"github.com/ludanortmun/teamback/internal/core"
)

func (h *Handler) HandleNewAssignment(w http.ResponseWriter, r *http.Request) {
	h.Render(w, "assignment_new.html", h.templateData(r, nil))
}

func (h *Handler) HandleCreateAssignment(w http.ResponseWriter, r *http.Request) {
	title := strings.TrimSpace(r.FormValue("title"))
	teamEmails := strings.TrimSpace(r.FormValue("team_emails"))

	var errs []string
	if title == "" {
		errs = append(errs, "Title is required")
	}
	if teamEmails == "" {
		errs = append(errs, "At least one team member email is required")
	}

	var emails []string
	seenEmails := map[string]bool{}
	if teamEmails != "" {
		for _, e := range strings.Split(teamEmails, ",") {
			trimmed := strings.TrimSpace(e)
			if trimmed != "" {
				if !emailRegex.MatchString(trimmed) {
					errs = append(errs, "Invalid email: "+trimmed)
					continue
				}
				if seenEmails[trimmed] {
					errs = append(errs, "Duplicate email: "+trimmed)
					continue
				}
				seenEmails[trimmed] = true
				emails = append(emails, trimmed)
			}
		}
		if len(emails) == 0 {
			errs = append(errs, "At least one valid team member email is required")
		}
	}

	if len(errs) > 0 {
		h.Render(w, "assignment_new.html", h.templateData(r, map[string]any{
			"Errors":     errs,
			"Title":      title,
			"TeamEmails": teamEmails,
		}))
		return
	}

	ctx := h.authCtx(r)

	var team []core.User
	for _, email := range emails {
		user, err := h.Client.GetUserByEmail(ctx, email)
		if err != nil {
			errs = append(errs, "User not found: "+email)
			continue
		}
		team = append(team, user)
	}

	if len(errs) > 0 {
		h.Render(w, "assignment_new.html", h.templateData(r, map[string]any{
			"Errors":     errs,
			"Title":      title,
			"TeamEmails": teamEmails,
		}))
		return
	}

	_, err := h.Client.CreateAssignment(ctx, title, team)
	if err != nil {
		h.Render(w, "assignment_new.html", h.templateData(r, map[string]any{
			"Errors":     []string{err.Error()},
			"Title":      title,
			"TeamEmails": teamEmails,
		}))
		return
	}

	http.Redirect(w, r, "/assignments", http.StatusSeeOther)
}

func (h *Handler) HandleListAssignments(w http.ResponseWriter, r *http.Request) {
	ctx := h.authCtx(r)
	assignments, err := h.Client.GetAssignments(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.Render(w, "assignments.html", h.templateData(r, map[string]any{
		"Assignments": assignments,
	}))
}

func (h *Handler) HandleViewAssignment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Assignment ID is required", http.StatusBadRequest)
		return
	}

	ctx := h.authCtx(r)
	assignment, err := h.Client.GetAssignment(ctx, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	h.Render(w, "assignment.html", h.templateData(r, map[string]any{
		"Assignment": assignment,
	}))
}
