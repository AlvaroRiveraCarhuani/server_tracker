package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultOpenRouterURL = "https://openrouter.ai/api/v1/chat/completions"
	defaultModel         = "google/gemma-4-26b-a4b-it:free"
)

// TriageClient realiza inferencia para diagnóstico pasivo de contenedores en falla.
type TriageClient struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewTriageClient inicializa el cliente con credenciales del entorno.
func NewTriageClient() *TriageClient {
	return NewTriageClientWithKey(os.Getenv("OPENROUTER_API_KEY"))
}

// NewTriageClientWithKey inicializa el cliente con una clave explícita.
func NewTriageClientWithKey(apiKey string) *TriageClient {
	model := os.Getenv("OPENROUTER_MODEL")
	if model == "" {
		model = defaultModel
	}

	return &TriageClient{
		apiKey:  apiKey,
		model:   model,
		baseURL: defaultOpenRouterURL,
		client:  &http.Client{Timeout: 6 * time.Second},
	}
}

// SetAPIKey actualiza la clave en caliente sin reiniciar el proceso.
func (c *TriageClient) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
}

// DiagnoseContainer analiza logs y estado para generar una causa raíz en 1 sola línea.
func (c *TriageClient) DiagnoseContainer(ctx context.Context, name, image, status, logs string) string {
	if c.apiKey == "" {
		return "Diagnóstico no configurado (OPENROUTER_API_KEY no presente)."
	}

	systemPrompt := "Eres el motor de diagnóstico AIOps para la TUI de SOLV Server Tracker. " +
		"Analiza el fallo y responde ÚNICAMENTE en exactamente UNA línea breve sin formato ni asteriscos: " +
		"[Causa probable -> Acción recomendada]"

	// Limitar logs a últimas 1200 runas para no gastar cuota innecesaria
	trimmedLogs := logs
	if len(trimmedLogs) > 1200 {
		trimmedLogs = trimmedLogs[len(trimmedLogs)-1200:]
	}
	if strings.TrimSpace(trimmedLogs) == "" {
		trimmedLogs = "Sin logs de error disponibles."
	}

	userPrompt := fmt.Sprintf("Contenedor: %s | Imagen: %s | Estado: %s\nLogs:\n%s", name, image, status, trimmedLogs)

	payload := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"max_tokens":  80,
		"temperature": 0.1,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return "Error serializando solicitud a modelo."
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "Error preparando petición HTTP."
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/alvaroriverac/server_tracker")
	req.Header.Set("X-Title", "SOLV Server Tracker TUI")

	resp, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "Diagnóstico no disponible (tiempo de espera agotado)."
		}
		return "Diagnóstico no disponible (error de red)."
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return "Cuota gratuita compartida saturada momentáneamente (HTTP 429)."
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Modelo no disponible (HTTP %d).", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "Error leyendo respuesta del modelo."
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &parsed); err != nil || len(parsed.Choices) == 0 {
		return "Respuesta no concluyente del modelo."
	}

	line := strings.TrimSpace(parsed.Choices[0].Message.Content)
	line = strings.ReplaceAll(line, "\n", " ")
	return line
}
