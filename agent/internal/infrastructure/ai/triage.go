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

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

const (
	defaultOpenRouterURL = "https://openrouter.ai/api/v1/chat/completions"
	defaultOpenAIURL     = "https://api.openai.com/v1/chat/completions"
	defaultAnthropicURL  = "https://api.anthropic.com/v1/messages"
	defaultOllamaURL     = "http://localhost:11434/api/chat"
)

// TriageClient realiza inferencia para diagnóstico pasivo de contenedores en falla.
type TriageClient struct {
	config domain.AIConfig
	client *http.Client
}

// NewTriageClient inicializa el cliente con credenciales del entorno.
func NewTriageClient() *TriageClient {
	cfg := domain.DefaultAIConfig()
	if orKey := os.Getenv("OPENROUTER_API_KEY"); orKey != "" {
		p := cfg.Providers[domain.ProviderOpenRouter]
		p.APIKey = orKey
		cfg.Providers[domain.ProviderOpenRouter] = p
		cfg.ActiveProvider = domain.ProviderOpenRouter
	}
	return NewTriageClientWithConfig(cfg)
}

// NewTriageClientWithKey inicializa el cliente con una clave explícita de OpenRouter (compatibilidad).
func NewTriageClientWithKey(apiKey string) *TriageClient {
	cfg := domain.DefaultAIConfig()
	p := cfg.Providers[domain.ProviderOpenRouter]
	p.APIKey = apiKey
	cfg.Providers[domain.ProviderOpenRouter] = p
	cfg.ActiveProvider = domain.ProviderOpenRouter
	return NewTriageClientWithConfig(cfg)
}

// NewTriageClientWithConfig inicializa el cliente con configuración de perfiles multi-proveedor.
func NewTriageClientWithConfig(cfg domain.AIConfig) *TriageClient {
	return &TriageClient{
		config: cfg,
		client: &http.Client{Timeout: 6 * time.Second},
	}
}

// SetConfig actualiza la configuración de IA en caliente.
func (c *TriageClient) SetConfig(cfg domain.AIConfig) {
	c.config = cfg
}

// SetAPIKey actualiza la clave del proveedor activo en caliente.
func (c *TriageClient) SetAPIKey(apiKey string) {
	p := c.config.Providers[c.config.ActiveProvider]
	p.APIKey = apiKey
	c.config.Providers[c.config.ActiveProvider] = p
}

// DiagnoseContainer analiza logs y estado para generar una causa raíz en 1 sola línea (wrapper de compatibilidad).
func (c *TriageClient) DiagnoseContainer(ctx context.Context, name, image, status, logs string) string {
	diag, _ := c.DiagnoseContainerWithUsage(ctx, name, image, status, logs)
	return diag
}

// DiagnoseContainerWithUsage analiza logs y estado retornando el diagnóstico junto con el consumo de tokens y costo estimado.
func (c *TriageClient) DiagnoseContainerWithUsage(ctx context.Context, name, image, status, logs string) (string, domain.TokenUsage) {
	provider := c.config.ActiveProvider
	pConfig, ok := c.config.Providers[provider]
	if !ok && provider != "" {
		pConfig = domain.ProviderConfig{}
	}

	meta := domain.GetProviderMeta(provider)
	if meta.RequiresKey && strings.TrimSpace(pConfig.APIKey) == "" {
		return fmt.Sprintf("Diagnóstico no configurado (%s no tiene API Key).", meta.Name), domain.TokenUsage{}
	}

	model := c.config.ActiveModel
	if model == "" {
		model = pConfig.DefaultModel
	}
	if model == "" {
		model = meta.DefaultModel
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

	switch provider {
	case domain.ProviderAnthropic:
		return c.callAnthropic(ctx, pConfig.APIKey, model, systemPrompt, userPrompt)
	case domain.ProviderOllama:
		ep := pConfig.Endpoint
		if ep == "" {
			ep = defaultOllamaURL
		}
		if !strings.HasSuffix(ep, "/api/chat") {
			ep = strings.TrimSuffix(ep, "/") + "/api/chat"
		}
		return c.callOllama(ctx, ep, model, systemPrompt, userPrompt)
	case domain.ProviderOpenAI:
		return c.callOpenAI(ctx, defaultOpenAIURL, pConfig.APIKey, model, systemPrompt, userPrompt, false)
	default:
		// OpenRouter o genérico compatible OpenAI
		ep := defaultOpenRouterURL
		if pConfig.Endpoint != "" {
			ep = pConfig.Endpoint
		}
		return c.callOpenAI(ctx, ep, pConfig.APIKey, model, systemPrompt, userPrompt, true)
	}
}

func (c *TriageClient) callAnthropic(ctx context.Context, apiKey, model, systemPrompt, userPrompt string) (string, domain.TokenUsage) {
	payload := map[string]interface{}{
		"model":      model,
		"max_tokens": 80,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return "Error serializando solicitud a Anthropic.", domain.TokenUsage{}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", defaultAnthropicURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "Error preparando petición a Anthropic.", domain.TokenUsage{}
	}

	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "Diagnóstico no disponible (timeout en Anthropic).", domain.TokenUsage{}
		}
		return "Diagnóstico no disponible (error de red Anthropic).", domain.TokenUsage{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Anthropic no disponible (HTTP %d).", resp.StatusCode), domain.TokenUsage{}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "Error leyendo respuesta de Anthropic.", domain.TokenUsage{}
	}

	var parsed struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &parsed); err != nil || len(parsed.Content) == 0 {
		return "Respuesta no concluyente de Anthropic.", domain.TokenUsage{}
	}

	usage := domain.TokenUsage{
		PromptTokens:     parsed.Usage.InputTokens,
		CompletionTokens: parsed.Usage.OutputTokens,
		TotalTokens:      parsed.Usage.InputTokens + parsed.Usage.OutputTokens,
		EstimatedCostUSD: domain.CalculateCost(domain.ProviderAnthropic, model, parsed.Usage.InputTokens, parsed.Usage.OutputTokens),
	}

	line := strings.TrimSpace(parsed.Content[0].Text)
	return strings.ReplaceAll(line, "\n", " "), usage
}

func (c *TriageClient) callOpenAI(ctx context.Context, endpoint, apiKey, model, systemPrompt, userPrompt string, isOpenRouter bool) (string, domain.TokenUsage) {
	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"max_tokens":  80,
		"temperature": 0.1,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return "Error serializando solicitud a modelo.", domain.TokenUsage{}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "Error preparando petición HTTP.", domain.TokenUsage{}
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	if isOpenRouter {
		req.Header.Set("HTTP-Referer", "https://github.com/alvaroriverac/server_tracker")
		req.Header.Set("X-Title", "SOLV Server Tracker TUI")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "Diagnóstico no disponible (tiempo de espera agotado).", domain.TokenUsage{}
		}
		return "Diagnóstico no disponible (error de red).", domain.TokenUsage{}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return "Cuota saturada momentáneamente (HTTP 429).", domain.TokenUsage{}
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Modelo no disponible (HTTP %d).", resp.StatusCode), domain.TokenUsage{}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "Error leyendo respuesta del modelo.", domain.TokenUsage{}
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &parsed); err != nil || len(parsed.Choices) == 0 {
		return "Respuesta no concluyente del modelo.", domain.TokenUsage{}
	}

	provider := domain.ProviderOpenAI
	if isOpenRouter {
		provider = domain.ProviderOpenRouter
	}

	usage := domain.TokenUsage{
		PromptTokens:     parsed.Usage.PromptTokens,
		CompletionTokens: parsed.Usage.CompletionTokens,
		TotalTokens:      parsed.Usage.TotalTokens,
		EstimatedCostUSD: domain.CalculateCost(provider, model, parsed.Usage.PromptTokens, parsed.Usage.CompletionTokens),
	}

	line := strings.TrimSpace(parsed.Choices[0].Message.Content)
	return strings.ReplaceAll(line, "\n", " "), usage
}

func (c *TriageClient) callOllama(ctx context.Context, endpoint, model, systemPrompt, userPrompt string) (string, domain.TokenUsage) {
	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"stream": false,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return "Error serializando solicitud a Ollama.", domain.TokenUsage{}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "Error preparando petición a Ollama.", domain.TokenUsage{}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "Ollama no responde (timeout).", domain.TokenUsage{}
		}
		return "Ollama local no disponible (¿está corriendo ollama serve?).", domain.TokenUsage{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Ollama error (HTTP %d).", resp.StatusCode), domain.TokenUsage{}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "Error leyendo respuesta de Ollama.", domain.TokenUsage{}
	}

	var parsed struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		PromptEvalCount int `json:"prompt_eval_count"`
		EvalCount       int `json:"eval_count"`
	}

	if err := json.Unmarshal(body, &parsed); err != nil {
		return "Respuesta no concluyente de Ollama.", domain.TokenUsage{}
	}

	usage := domain.TokenUsage{
		PromptTokens:     parsed.PromptEvalCount,
		CompletionTokens: parsed.EvalCount,
		TotalTokens:      parsed.PromptEvalCount + parsed.EvalCount,
		EstimatedCostUSD: 0.0,
	}

	line := strings.TrimSpace(parsed.Message.Content)
	return strings.ReplaceAll(line, "\n", " "), usage
}

