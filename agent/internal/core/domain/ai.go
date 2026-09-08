package domain

import "strings"

// AIProvider representa un proveedor de modelos de lenguaje soportado.
type AIProvider string

const (
	ProviderAnthropic  AIProvider = "anthropic"
	ProviderOpenAI     AIProvider = "openai"
	ProviderOpenRouter AIProvider = "openrouter"
	ProviderOllama     AIProvider = "ollama"
)

// ProviderMetadata contiene la información pública y modelos sugeridos de cada proveedor.
type ProviderMetadata struct {
	ID             AIProvider
	Name           string
	Description    string
	SuggestedModel []string
	DefaultModel   string
	RequiresKey    bool
}

// AvailableProviders lista los proveedores soportados por el motor AIOps.
var AvailableProviders = []ProviderMetadata{
	{
		ID:             ProviderAnthropic,
		Name:           "Anthropic (Claude)",
		Description:    "Inferencia de alta precisión y razonamiento estructurado",
		SuggestedModel: []string{"claude-3-5-haiku-latest", "claude-3-5-sonnet-latest"},
		DefaultModel:   "claude-3-5-haiku-latest",
		RequiresKey:    true,
	},
	{
		ID:             ProviderOpenAI,
		Name:           "OpenAI (ChatGPT)",
		Description:    "Modelos versátiles para análisis de telemetría y causa raíz",
		SuggestedModel: []string{"gpt-4o-mini", "gpt-4o"},
		DefaultModel:   "gpt-4o-mini",
		RequiresKey:    true,
	},
	{
		ID:             ProviderOpenRouter,
		Name:           "OpenRouter (Multi-Model)",
		Description:    "Acceso unificado a Gemini, DeepSeek, Llama y Claude",
		SuggestedModel: []string{"google/gemini-2.0-flash-001", "deepseek/deepseek-chat", "anthropic/claude-3.5-haiku"},
		DefaultModel:   "google/gemini-2.0-flash-001",
		RequiresKey:    true,
	},
	{
		ID:             ProviderOllama,
		Name:           "Ollama (Local On-Prem)",
		Description:    "Inferencia 100% local y privada sin salida a internet",
		SuggestedModel: []string{"llama3.2", "qwen2.5-coder", "mistral"},
		DefaultModel:   "llama3.2",
		RequiresKey:    false,
	},
}

// ProviderConfig almacena las credenciales y configuración específica de un proveedor.
type ProviderConfig struct {
	APIKey       string `json:"api_key,omitempty"`
	Endpoint     string `json:"endpoint,omitempty"`
	DefaultModel string `json:"default_model,omitempty"`
}

// MaskedKey devuelve la clave ofuscada mostrando solo los últimos 4 caracteres.
func (c ProviderConfig) MaskedKey() string {
	k := strings.TrimSpace(c.APIKey)
	if k == "" {
		return "[Sin Clave]"
	}
	if len(k) <= 4 {
		return "••••"
	}
	return "••••" + k[len(k)-4:]
}

// AIConfig es la configuración completa de IA almacenada bajo el blindaje D2 en la bóveda cifrada.
type AIConfig struct {
	ActiveProvider AIProvider                   `json:"active_provider"`
	ActiveModel    string                       `json:"active_model"`
	Providers      map[AIProvider]ProviderConfig `json:"providers"`
}

// DefaultAIConfig genera la configuración inicial con OpenRouter como default.
func DefaultAIConfig() AIConfig {
	return AIConfig{
		ActiveProvider: ProviderOpenRouter,
		ActiveModel:    "google/gemini-2.0-flash-001",
		Providers: map[AIProvider]ProviderConfig{
			ProviderAnthropic:  {DefaultModel: "claude-3-5-haiku-latest"},
			ProviderOpenAI:     {DefaultModel: "gpt-4o-mini"},
			ProviderOpenRouter: {DefaultModel: "google/gemini-2.0-flash-001"},
			ProviderOllama:     {Endpoint: "http://localhost:11434", DefaultModel: "llama3.2"},
		},
	}
}

// ModelOption representa una opción de modelo en el catálogo interactivo estilo OpenCode.
type ModelOption struct {
	ID                   string     `json:"id"`
	DisplayName          string     `json:"display_name"`
	Provider             AIProvider `json:"provider"`
	Description          string     `json:"description"`
	InputPricePerMillion float64    `json:"input_price_per_million"`  // USD por 1M tokens de entrada
	OutputPricePerMillion float64   `json:"output_price_per_million"` // USD por 1M tokens de salida
}

// CatalogModels lista los modelos disponibles en el selector rápido.
var CatalogModels = []ModelOption{
	// Anthropic
	{
		ID:                   "claude-3-5-sonnet-latest",
		DisplayName:          "Claude 3.5 Sonnet",
		Provider:             ProviderAnthropic,
		Description:          "Recomendado: Máxima precisión analítica",
		InputPricePerMillion: 3.00,
		OutputPricePerMillion: 15.00,
	},
	{
		ID:                   "claude-3-5-haiku-latest",
		DisplayName:          "Claude 3.5 Haiku",
		Provider:             ProviderAnthropic,
		Description:          "Ultra rápido y económico para triajes ligeros",
		InputPricePerMillion: 0.80,
		OutputPricePerMillion: 4.00,
	},
	// OpenAI
	{
		ID:                   "gpt-4o-mini",
		DisplayName:          "GPT-4o Mini",
		Provider:             ProviderOpenAI,
		Description:          "Económico y eficiente para telemetría general",
		InputPricePerMillion: 0.15,
		OutputPricePerMillion: 0.60,
	},
	{
		ID:                   "gpt-4o",
		DisplayName:          "GPT-4o",
		Provider:             ProviderOpenAI,
		Description:          "Modelo insignia multimodal para fallos complejos",
		InputPricePerMillion: 2.50,
		OutputPricePerMillion: 10.00,
	},
	// OpenRouter
	{
		ID:                   "deepseek/deepseek-r1",
		DisplayName:          "DeepSeek R1",
		Provider:             ProviderOpenRouter,
		Description:          "Razonamiento profundo OpenRouter",
		InputPricePerMillion: 0.55,
		OutputPricePerMillion: 2.19,
	},
	{
		ID:                   "deepseek/deepseek-chat",
		DisplayName:          "DeepSeek V3",
		Provider:             ProviderOpenRouter,
		Description:          "Alta velocidad y costo mínimo",
		InputPricePerMillion: 0.14,
		OutputPricePerMillion: 0.28,
	},
	{
		ID:                   "google/gemini-2.0-flash-001",
		DisplayName:          "Gemini 2.0 Flash",
		Provider:             ProviderOpenRouter,
		Description:          "Latencia ultra baja y ventana masiva",
		InputPricePerMillion: 0.10,
		OutputPricePerMillion: 0.40,
	},
	// Ollama Local
	{
		ID:                   "llama3.2",
		DisplayName:          "Llama 3.2",
		Provider:             ProviderOllama,
		Description:          "Inferencia 100% local en host (Sin costo de red)",
		InputPricePerMillion: 0.0,
		OutputPricePerMillion: 0.0,
	},
	{
		ID:                   "qwen2.5-coder",
		DisplayName:          "Qwen 2.5 Coder",
		Provider:             ProviderOllama,
		Description:          "Especializado en logs y código local",
		InputPricePerMillion: 0.0,
		OutputPricePerMillion: 0.0,
	},
}

// TokenUsage almacena el consumo de tokens y costo estimado de una inferencia.
type TokenUsage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
}

// CalculateCost calcula el costo estimado en dólares a partir de los tokens usados.
func CalculateCost(provider AIProvider, model string, promptTokens, completionTokens int) float64 {
	if provider == ProviderOllama {
		return 0.0
	}
	// Buscar en catálogo
	for _, m := range CatalogModels {
		if m.ID == model || strings.HasSuffix(model, m.ID) {
			inputCost := (float64(promptTokens) / 1_000_000.0) * m.InputPricePerMillion
			outputCost := (float64(completionTokens) / 1_000_000.0) * m.OutputPricePerMillion
			return inputCost + outputCost
		}
	}
	// Fallback estándar si es un modelo custom
	inputCost := (float64(promptTokens) / 1_000_000.0) * 1.00
	outputCost := (float64(completionTokens) / 1_000_000.0) * 3.00
	return inputCost + outputCost
}

// GetProviderMeta busca los metadatos de un proveedor dado.
func GetProviderMeta(id AIProvider) ProviderMetadata {
	for _, p := range AvailableProviders {
		if p.ID == id {
			return p
		}
	}
	return ProviderMetadata{
		ID:           id,
		Name:         string(id),
		DefaultModel: "default",
	}
}
