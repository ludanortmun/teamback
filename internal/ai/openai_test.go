package ai

import (
	"testing"

	"github.com/ludanortmun/teamback/internal/core"
)

func TestBuildAnonymizationMaps(t *testing.T) {
	team := []core.User{
		{ID: "id-1", Name: "Alice García"},
		{ID: "id-2", Name: "Bob López"},
		{ID: "id-3", Name: "Charlie Pérez"},
	}

	nameToAlias, aliasToName := buildAnonymizationMaps(team)

	// Check that each user gets a unique alias
	if nameToAlias["id-1"] != "Estudiante A" {
		t.Errorf("expected 'Estudiante A', got %q", nameToAlias["id-1"])
	}
	if nameToAlias["id-2"] != "Estudiante B" {
		t.Errorf("expected 'Estudiante B', got %q", nameToAlias["id-2"])
	}
	if nameToAlias["id-3"] != "Estudiante C" {
		t.Errorf("expected 'Estudiante C', got %q", nameToAlias["id-3"])
	}

	// Check reverse mapping
	if aliasToName["Estudiante A"] != "Alice García" {
		t.Errorf("expected 'Alice García', got %q", aliasToName["Estudiante A"])
	}
	if aliasToName["Estudiante B"] != "Bob López" {
		t.Errorf("expected 'Bob López', got %q", aliasToName["Estudiante B"])
	}
}

func TestDeAnonymize(t *testing.T) {
	aliasToName := map[string]string{
		"Estudiante A": "María",
		"Estudiante B": "Juan",
	}

	input := "Estudiante A contribuyó más que Estudiante B en esta tarea."
	expected := "María contribuyó más que Juan en esta tarea."

	result := deAnonymize(input, aliasToName)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestParseAttentionFlag_True(t *testing.T) {
	input := "Este es el resumen del equipo.\n\n{\"atencion_requerida\": true}"
	summary, attention := parseAttentionFlag(input)

	if attention != true {
		t.Error("expected attention_required to be true")
	}
	if summary != "Este es el resumen del equipo." {
		t.Errorf("expected clean summary, got %q", summary)
	}
}

func TestParseAttentionFlag_False(t *testing.T) {
	input := "Todo bien con el equipo.\n{\"atencion_requerida\": false}"
	summary, attention := parseAttentionFlag(input)

	if attention != false {
		t.Error("expected attention_required to be false")
	}
	if summary != "Todo bien con el equipo." {
		t.Errorf("expected clean summary, got %q", summary)
	}
}

func TestParseAttentionFlag_NoJSON(t *testing.T) {
	input := "Un resumen sin JSON al final."
	summary, attention := parseAttentionFlag(input)

	if attention != false {
		t.Error("expected attention_required to be false when no JSON present")
	}
	if summary != input {
		t.Errorf("expected full content returned, got %q", summary)
	}
}

func TestBuildUserPrompt_NoPII(t *testing.T) {
	team := []core.User{
		{ID: "id-1", Name: "Alice García", Email: "alice@school.edu"},
		{ID: "id-2", Name: "Bob López", Email: "bob@school.edu"},
	}

	assignment := core.Assignment{
		ID:    "asgn-1",
		Title: "Proyecto Final",
		Team:  team,
	}

	feedback := []core.Answer{
		{
			Author: team[0],
			MemberContributions: map[string]core.Contribution{
				"id-1": {Description: "Hizo la presentación", Weight: 60},
				"id-2": {Description: "Escribió el informe", Weight: 40},
			},
		},
	}

	nameToAlias, _ := buildAnonymizationMaps(team)
	prompt := buildUserPrompt(assignment, feedback, nameToAlias)

	// Verify no PII in prompt
	if contains(prompt, "Alice") || contains(prompt, "García") {
		t.Error("prompt contains PII (Alice García)")
	}
	if contains(prompt, "Bob") || contains(prompt, "López") {
		t.Error("prompt contains PII (Bob López)")
	}
	if contains(prompt, "alice@school.edu") || contains(prompt, "bob@school.edu") {
		t.Error("prompt contains email addresses")
	}

	// Verify aliases are used
	if !contains(prompt, "Estudiante A") {
		t.Error("prompt should contain 'Estudiante A'")
	}
	if !contains(prompt, "Estudiante B") {
		t.Error("prompt should contain 'Estudiante B'")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
