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
		http.Error(w, "El ID de la actividad es obligatorio", http.StatusBadRequest)
		return
	}

	ctx := h.authCtx(r)
	assignment, err := h.Client.GetAssignment(ctx, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Safe to make this assumption since AuthZ ensures students only have access to their own submissions
	var existingAnswer *core.Answer
	if len(assignment.Feedback) > 0 {
		existingAnswer = &assignment.Feedback[0]
	}

	h.Render(w, "feedback_new.html", h.templateData(r, map[string]any{
		"Assignment":     assignment,
		"ExistingAnswer": existingAnswer,
	}))
}

func (h *Handler) HandleCreateFeedback(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "El ID de la actividad es obligatorio", http.StatusBadRequest)
		return
	}

	ctx := h.authCtx(r)
	assignment, err := h.Client.GetAssignment(ctx, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	var errs []string
	answer := core.Answer{
		MemberContributions: make(map[string]core.Contribution),
	}

	for _, member := range assignment.Team {
		descKey := fmt.Sprintf("description_%s", member.ID)
		weightKey := fmt.Sprintf("weight_%s", member.ID)

		desc := strings.TrimSpace(r.FormValue(descKey))
		weightStr := strings.TrimSpace(r.FormValue(weightKey))

		if desc == "" {
			errs = append(errs, fmt.Sprintf("La descripción de %s es obligatoria", member.Name))
		}

		if weightStr == "" {
			errs = append(errs, fmt.Sprintf("El porcentaje de %s es obligatorio", member.Name))
			continue
		}

		weight, parseErr := strconv.Atoi(weightStr)
		if parseErr != nil {
			errs = append(errs, fmt.Sprintf("El porcentaje de %s debe ser un número", member.Name))
			continue
		}
		if weight < 0 {
			errs = append(errs, fmt.Sprintf("El porcentaje de %s no puede ser negativo", member.Name))
			continue
		}
		if weight > 100 {
			errs = append(errs, fmt.Sprintf("El porcentaje de %s no puede ser mayor que 100", member.Name))
			continue
		}

		answer.MemberContributions[member.ID] = core.Contribution{
			Description: desc,
			Weight:      uint8(weight),
		}
	}

	if len(errs) > 0 {
		h.Render(w, "feedback_new.html", h.templateData(r, map[string]any{
			"Assignment":     assignment,
			"ExistingAnswer": &answer,
			"Errors":         errs,
		}))
		return
	}

	_, err = h.Client.AddFeedback(ctx, id, answer)
	if err != nil {
		h.Render(w, "feedback_new.html", h.templateData(r, map[string]any{
			"Assignment":     assignment,
			"ExistingAnswer": &answer,
			"Errors":         []string{localizedError(err)},
		}))
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/assignments/%s", id), http.StatusSeeOther)
}
