package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ludanortmun/teamback/internal/core"
)

// OpenAIClient implements core.Summarizer using the OpenAI Chat Completions API.
type OpenAIClient struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewOpenAIClient creates a new OpenAI-compatible summarizer.
func NewOpenAIClient(apiKey, baseURL, model string) *OpenAIClient {
	return &OpenAIClient{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		client:  &http.Client{},
	}
}

const systemPrompt = `Eres un asistente de análisis educativo. Tu tarea es analizar las respuestas de retroalimentación entre pares de un equipo estudiantil y generar un resumen para el profesor.

Debes:
1. Identificar contradicciones entre las respuestas de los diferentes estudiantes (por ejemplo, si un estudiante dice que otro contribuyó mucho pero otro dice lo contrario).
2. Señalar distribuciones de carga de trabajo muy desiguales (si los porcentajes asignados por múltiples estudiantes sugieren que alguien trabajó significativamente más o menos).
3. Para cada estudiante del equipo, generar un breve párrafo resumiendo sus contribuciones según todas las respuestas recibidas.

Responde siempre en español. Sé conciso y directo. Usa los identificadores de estudiante proporcionados tal como aparecen.`

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Summarize generates a summary from assignment feedback, anonymizing student names.
func (c *OpenAIClient) Summarize(assignment core.Assignment, feedback []core.Answer) (string, error) {
	nameToAlias, aliasToName := buildAnonymizationMaps(assignment.Team)

	userPrompt := buildUserPrompt(assignment, feedback, nameToAlias)

	req := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("calling OpenAI API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	summary := chatResp.Choices[0].Message.Content

	// Reverse-map aliases back to real names
	summary = deAnonymize(summary, aliasToName)

	return summary, nil
}

// buildAnonymizationMaps creates bidirectional mappings between real names and aliases.
func buildAnonymizationMaps(team []core.User) (nameToAlias map[string]string, aliasToName map[string]string) {
	nameToAlias = make(map[string]string)
	aliasToName = make(map[string]string)

	for i, user := range team {
		alias := fmt.Sprintf("Estudiante %c", 'A'+rune(i))
		nameToAlias[user.ID] = alias
		aliasToName[alias] = user.Name
	}
	return
}

func buildUserPrompt(assignment core.Assignment, feedback []core.Answer, nameToAlias map[string]string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Actividad: \"%s\"\n", assignment.Title))
	sb.WriteString(fmt.Sprintf("Equipo: %s\n\n", strings.Join(aliasList(assignment.Team, nameToAlias), ", ")))

	for _, answer := range feedback {
		authorAlias := nameToAlias[answer.Author.ID]
		sb.WriteString(fmt.Sprintf("--- Respuesta de %s ---\n", authorAlias))
		for memberID, contrib := range answer.MemberContributions {
			memberAlias := nameToAlias[memberID]
			sb.WriteString(fmt.Sprintf("- %s: %d%% — %s\n", memberAlias, contrib.Weight, contrib.Description))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func aliasList(team []core.User, nameToAlias map[string]string) []string {
	aliases := make([]string, len(team))
	for i, user := range team {
		aliases[i] = nameToAlias[user.ID]
	}
	return aliases
}

// deAnonymize replaces all alias occurrences in the summary with real names.
func deAnonymize(text string, aliasToName map[string]string) string {
	for alias, name := range aliasToName {
		text = strings.ReplaceAll(text, alias, name)
	}
	return text
}
