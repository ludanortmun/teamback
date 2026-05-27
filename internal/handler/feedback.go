package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/ludanortmun/teamback/internal/core"
)

func (h *Handler) HandleNewFeedback(w http.ResponseWriter, r *http.Request) {
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

	h.Render(w, "feedback_new.html", h.templateData(r, map[string]any{
		"Assignment": assignment,
	}))
}

func (h *Handler) HandleCreateFeedback(w http.ResponseWriter, r *http.Request) {
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

	var errs []string
	contributions := make(map[string]core.Contribution)

	for _, member := range assignment.Team {
		descKey := fmt.Sprintf("description_%s", member.ID)
		weightKey := fmt.Sprintf("weight_%s", member.ID)

		desc := strings.TrimSpace(r.FormValue(descKey))
		weightStr := strings.TrimSpace(r.FormValue(weightKey))

		if desc == "" {
			errs = append(errs, fmt.Sprintf("Description for %s is required", member.Name))
		}

		if weightStr == "" {
			errs = append(errs, fmt.Sprintf("Weight for %s is required", member.Name))
			continue
		}

		weight, parseErr := strconv.Atoi(weightStr)
		if parseErr != nil {
			errs = append(errs, fmt.Sprintf("Weight for %s must be a number", member.Name))
			continue
		}
		if weight < 0 {
			errs = append(errs, fmt.Sprintf("Weight for %s cannot be negative", member.Name))
			continue
		}
		if weight > 100 {
			errs = append(errs, fmt.Sprintf("Weight for %s cannot exceed 100", member.Name))
			continue
		}

		contributions[member.ID] = core.Contribution{
			Description: desc,
			Weight:      uint8(weight),
		}
	}

	if len(errs) > 0 {
		h.Render(w, "feedback_new.html", h.templateData(r, map[string]any{
			"Assignment": assignment,
			"Errors":     errs,
		}))
		return
	}

	answer := core.Answer{
		MemberContributions: contributions,
	}

	_, err = h.Client.AddFeedback(ctx, id, answer)
	if err != nil {
		h.Render(w, "feedback_new.html", h.templateData(r, map[string]any{
			"Assignment": assignment,
			"Errors":     []string{err.Error()},
		}))
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/assignments/%s", id), http.StatusSeeOther)
}
