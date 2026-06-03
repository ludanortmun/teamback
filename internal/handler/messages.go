package handler

import (
	"github.com/ludanortmun/teamback/internal/core"
)

func localizedError(err error) string {
	switch err.Error() {
	case core.ErrInvalidContributions:
		return "La retroalimentación debe incluir una entrada por cada integrante del equipo."
	case core.ErrEmptyDescription:
		return "Todas las descripciones deben tener contenido."
	case core.ErrInvalidWeights:
		return "Los porcentajes deben sumar 100."
	case core.ErrUnauthorizedMsg:
		return "No tienes permisos para realizar esta acción."
	case core.ErrUserNotFound:
		return "No se encontró el usuario."
	default:
		return err.Error()
	}
}
