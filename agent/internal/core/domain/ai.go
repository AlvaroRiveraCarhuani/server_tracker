package domain

import (
	"strings"
	"time"
)

// AIProvider representa un proveedor de modelos de lenguaje soportado.
type AIProvider string

const (
	ProviderAnthropic  AIProvider = "anthropic"
	ProviderOpenAI     AIProvider = "openai"
	ProviderOpenRouter AIProvider = "openrouter"
	ProviderOllama     AIProvider = "ollama"
	ProviderVLLM       AIProvider = "vllm"
	ProviderLMStudio   AIProvider = "lmstudio"
	ProviderCustom     AIProvider = "custom"
)

// DiagnosisSlot representa un slot ejecutor con modelo asignado en SOLV.
type DiagnosisSlot string

const (
	SlotFast DiagnosisSlot = "fast"
	SlotDeep DiagnosisSlot = "deep"
)

// ConnectorFamily clasifica la topologia de conexion del proveedor.
type ConnectorFamily string

const (
	FamilyCloudManaged ConnectorFamily = "cloud"
	FamilyLocalRuntime ConnectorFamily = "local"
	FamilyCustomOpenAI ConnectorFamily = "custom"
)

// ConnectorStatus representa el estado de conectividad e infraestructura.
type ConnectorStatus string

const (
	StatusConnected ConnectorStatus = "connected"
	StatusNoKey     ConnectorStatus = "no_key"
	StatusDetected  ConnectorStatus = "detected"
	StatusAbsent    ConnectorStatus = "absent"
	StatusTimeout   ConnectorStatus = "timeout"
)

// ModelRef representa una referencia canonica y ligera de un modelo asignado a un slot o selector.
type ModelRef struct {
	ID            string        `json:"id"`
	DisplayName   string        `json:"display_name"`
	ProviderID    AIProvider    `json:"provider_id"`
	ContextLength int64         `json:"context_length"`
	IsFree        bool          `json:"is_free"`
	PricingTier   PricingTier   `json:"pricing_tier"`
	LatencyP95    time.Duration `json:"latency_p95,omitempty"`
	EvalOK        bool          `json:"eval_ok"`
}

// SlotPolicy gestiona los mapeos de slots ejecutores FAST y DEEP con politica de precedencia.
type SlotPolicy struct {
	Assignments map[DiagnosisSlot]ModelRef `json:"assignments"`
}

// DefaultSlotPolicy genera la politica embebida de fabrica con openrouter/free como router universal.
func DefaultSlotPolicy() SlotPolicy {
	return SlotPolicy{
		Assignments: map[DiagnosisSlot]ModelRef{
			SlotFast: {
				ID:            "openrouter/free",
				DisplayName:   "Free Router (openrouter/free)",
				ProviderID:    ProviderOpenRouter,
				ContextLength: 131072,
				IsFree:        true,
				PricingTier:   PricingFree,
				EvalOK:        true,
			},
			SlotDeep: {
				ID:            "openrouter/free",
				DisplayName:   "Free Router (openrouter/free)",
				ProviderID:    ProviderOpenRouter,
				ContextLength: 131072,
				IsFree:        true,
				PricingTier:   PricingFree,
				EvalOK:        true,
			},
		},
	}
}

// GetAssignedModel resuelve el modelo para un slot, donde el override del usuario siempre gana al default.
func GetAssignedModel(slot DiagnosisSlot, override SlotPolicy) ModelRef {
	defaults := DefaultSlotPolicy()
	if override.Assignments != nil {
		if m, ok := override.Assignments[slot]; ok && m.ID != "" {
			return m
		}
	}
	return defaults.Assignments[slot]
}

// PricingTier clasifica el nivel de costo del modelo para AIOps.
type PricingTier string

const (
	PricingFree    PricingTier = "free"
	PricingLow     PricingTier = "low"
	PricingPremium PricingTier = "premium"
)

// MetricSampleState indica la confiabilidad estadística de la métrica observada.
type MetricSampleState string

const (
	SampleStateObserved  MetricSampleState = "observed"  // >= 5 requests reales
	SampleStateEstimated MetricSampleState = "estimated" // Heurística inicial del catálogo
	SampleStateUnknown   MetricSampleState = "unknown"   // Sin datos
)

// RuntimeMetrics contiene la telemetría real observada por SOLV en inferencias previas.
type RuntimeMetrics struct {
	AvgLatency   time.Duration     `json:"avg_latency"`
	P95Latency   time.Duration     `json:"p95_latency"`
	SuccessRate  float64           `json:"success_rate"` // 0.0 a 1.0
	RequestCount int               `json:"request_count"`
	SampleState  MetricSampleState `json:"sample_state"`
	LastMeasured time.Time         `json:"last_measured"`
}

// ModelCapabilities define las capacidades técnicas del modelo para observabilidad.
type ModelCapabilities struct {
	StructuredOutput bool `json:"structured_output"` // Requisito núcleo
	Tools            bool `json:"tools"`            // Requerido solo para retrieval agéntico
	Reasoning        bool `json:"reasoning"`
}

// ModelPricing detalla el costo por millón de tokens y restricciones de cuota.
type ModelPricing struct {
	InputPerMillion  float64     `json:"input_per_million"`
	OutputPerMillion float64     `json:"output_per_million"`
	IsFree           bool        `json:"is_free"`
	Tier             PricingTier `json:"tier"`
	RateLimited      bool        `json:"rate_limited"`
}

// ModelSource define el origen del modelo en el catálogo.
type ModelSource string

const (
	SourceRemote   ModelSource = "remote"
	SourceLocal    ModelSource = "local"
	SourceSeedInit ModelSource = "seed"
)

// AIModel representa la metadata cruda de un modelo o router en el catálogo unificado.
type AIModel struct {
	ID            string            `json:"id"`
	DisplayName   string            `json:"display_name"`
	ProviderID    AIProvider        `json:"provider_id"`
	Creator       string            `json:"creator"`
	ContextLength int64             `json:"context_length"` // Tokens (ej: 131072)
	Pricing       ModelPricing      `json:"pricing"`
	Capabilities  ModelCapabilities `json:"capabilities"`
	IsRouter      bool              `json:"is_router"` // true para openrouter/free
	Source        ModelSource       `json:"source"`
	Verified      bool              `json:"verified"`
}

// DiagnosisProfile distingue entre perfiles de evaluación técnica de un incidente.
type DiagnosisProfile string

const (
	ProfileFast DiagnosisProfile = "fast"
	ProfileDeep DiagnosisProfile = "deep"
)

// ModelSelectionMode representa la política de selección activa en el agente.
type ModelSelectionMode string

const (
	SelectionAuto   ModelSelectionMode = "auto"   // SOLV decide Fast o Deep según severidad
	SelectionFast   ModelSelectionMode = "fast"   // SOLV elige el mejor modelo para Fast
	SelectionDeep   ModelSelectionMode = "deep"   // SOLV elige el mejor modelo para Deep
	SelectionManual ModelSelectionMode = "manual" // El operador fija un modelo manual
)

// ModelAssessment contiene la evaluación técnica calculada por SOLV para un modelo.
type ModelAssessment struct {
	FastSuitability float64        `json:"fast_suitability"` // 1.0 a 5.0
	DeepSuitability float64        `json:"deep_suitability"` // 1.0 a 5.0
	OverallScore    float64        `json:"overall_score"`
	Recommended     bool           `json:"recommended"`
	Reasons         []string       `json:"reasons"`
	ObservedMetrics RuntimeMetrics `json:"observed_metrics"`
}

// MinOperationalContext define el contexto mínimo viable para telemetría y logs (8K).
const MinOperationalContext = int64(8192)

// AssessModel evalúa la idoneidad contextual de un modelo para diagnóstico AIOps.
func AssessModel(m AIModel, rt RuntimeMetrics) ModelAssessment {
	var reasons []string

	fastScore := 3.0
	deepScore := 3.0

	// 1. Structured Output es requisito troncal para acciones de remediación
	if m.Capabilities.StructuredOutput {
		fastScore += 0.8
		deepScore += 0.8
		reasons = append(reasons, "Salida estructurada")
	} else {
		fastScore -= 1.5
		deepScore -= 1.0
	}

	// 2. Evaluación de Latencia (Observada o Estimada)
	lat := rt.AvgLatency
	if rt.SampleState == SampleStateObserved && lat > 0 {
		if lat < 2*time.Second {
			fastScore += 0.7
			reasons = append(reasons, "Latencia ultra baja (<2s)")
		} else if lat < 4*time.Second {
			fastScore += 0.3
		} else if lat > 8*time.Second {
			fastScore -= 1.0
		}
	} else {
		// Estimada por nombre/proveedor
		if strings.Contains(strings.ToLower(m.ID), "flash") || strings.Contains(strings.ToLower(m.ID), "haiku") || strings.Contains(strings.ToLower(m.ID), "mini") {
			fastScore += 0.5
			reasons = append(reasons, "Optimizador de baja latencia")
		}
	}

	// 3. Evaluación de Razonamiento
	if m.Capabilities.Reasoning {
		deepScore += 1.2
		reasons = append(reasons, "Razonamiento profundo")
	}

	// 4. Contexto operativo
	if m.ContextLength >= MinOperationalContext {
		if m.ContextLength >= 32768 {
			deepScore += 0.3
		}
	} else if m.ContextLength > 0 {
		fastScore -= 0.5
		deepScore -= 1.0
	}

	// 5. Costo y disponibilidad
	if m.Pricing.IsFree || m.Pricing.Tier == PricingFree {
		reasons = append(reasons, "Inferencia sin costo")
	} else if m.Pricing.Tier == PricingLow {
		reasons = append(reasons, "Costo ultra bajo")
	}

	if m.IsRouter {
		reasons = append(reasons, "Router dinámico universal")
		fastScore += 0.2
	}

	// Clamping 1.0 a 5.0
	clamp := func(v float64) float64 {
		if v < 1.0 {
			return 1.0
		}
		if v > 5.0 {
			return 5.0
		}
		return v
	}

	fastScore = clamp(fastScore)
	deepScore = clamp(deepScore)
	overall := clamp((fastScore + deepScore) / 2.0)
	recommended := fastScore >= 4.0 || deepScore >= 4.2

	return ModelAssessment{
		FastSuitability: fastScore,
		DeepSuitability: deepScore,
		OverallScore:    overall,
		Recommended:     recommended,
		Reasons:         reasons,
		ObservedMetrics: rt,
	}
}

// MatchesProfile determina formalmente si un modelo califica para un perfil de diagnóstico.
func MatchesProfile(m AIModel, a ModelAssessment, profile DiagnosisProfile) bool {
	switch profile {
	case ProfileFast:
		return a.FastSuitability >= 3.8 && m.Capabilities.StructuredOutput
	case ProfileDeep:
		return a.DeepSuitability >= 4.0
	default:
		return true
	}
}

// BootstrapSeedModels provee exactamente 4 semillas de rescate para arranque inicial.
var BootstrapSeedModels = []AIModel{
	{
		ID:            "openrouter/free",
		DisplayName:   "Free Router (openrouter/free)",
		ProviderID:    ProviderOpenRouter,
		Creator:       "OpenRouter",
		ContextLength: 131072,
		Pricing: ModelPricing{
			IsFree:      true,
			Tier:        PricingFree,
			RateLimited: true,
		},
		Capabilities: ModelCapabilities{
			StructuredOutput: true,
			Tools:            true,
			Reasoning:        false,
		},
		IsRouter: true,
		Source:   SourceSeedInit,
		Verified: false,
	},
	{
		ID:            "google/gemini-2.0-flash-001",
		DisplayName:   "Gemini 2.0 Flash",
		ProviderID:    ProviderOpenRouter,
		Creator:       "Google",
		ContextLength: 1048576,
		Pricing: ModelPricing{
			InputPerMillion:  0.10,
			OutputPerMillion: 0.40,
			IsFree:           false,
			Tier:             PricingLow,
		},
		Capabilities: ModelCapabilities{
			StructuredOutput: true,
			Tools:            true,
			Reasoning:        false,
		},
		IsRouter: false,
		Source:   SourceSeedInit,
		Verified: false,
	},
	{
		ID:            "claude-3-5-sonnet-latest",
		DisplayName:   "Claude 3.5 Sonnet",
		ProviderID:    ProviderAnthropic,
		Creator:       "Anthropic",
		ContextLength: 200000,
		Pricing: ModelPricing{
			InputPerMillion:  3.00,
			OutputPerMillion: 15.00,
			IsFree:           false,
			Tier:             PricingPremium,
		},
		Capabilities: ModelCapabilities{
			StructuredOutput: true,
			Tools:            true,
			Reasoning:        true,
		},
		IsRouter: false,
		Source:   SourceSeedInit,
		Verified: false,
	},
	{
		ID:            "llama3.2",
		DisplayName:   "Llama 3.2 (Local)",
		ProviderID:    ProviderOllama,
		Creator:       "Meta",
		ContextLength: 131072,
		Pricing: ModelPricing{
			IsFree: true,
			Tier:   PricingFree,
		},
		Capabilities: ModelCapabilities{
			StructuredOutput: true,
			Tools:            false,
			Reasoning:        false,
		},
		IsRouter: false,
		Source:   SourceSeedInit,
		Verified: false,
	},
}

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
		ID:             ProviderOpenRouter,
		Name:           "OpenRouter",
		Description:    "Acceso unificado a múltiples modelos y routers gratuitos",
		SuggestedModel: []string{"openrouter/free", "google/gemini-2.0-flash-001", "deepseek/deepseek-chat"},
		DefaultModel:   "openrouter/free",
		RequiresKey:    true,
	},
	{
		ID:             ProviderOllama,
		Name:           "Ollama",
		Description:    "Inferencia local y privada on-premise sin salida a internet",
		SuggestedModel: []string{"llama3.2", "qwen2.5-coder", "mistral"},
		DefaultModel:   "llama3.2",
		RequiresKey:    false,
	},
	{
		ID:             ProviderOpenAI,
		Name:           "OpenAI",
		Description:    "Modelos versátiles para análisis de telemetría y causa raíz",
		SuggestedModel: []string{"gpt-4o-mini", "gpt-4o"},
		DefaultModel:   "gpt-4o-mini",
		RequiresKey:    true,
	},
	{
		ID:             ProviderAnthropic,
		Name:           "Anthropic",
		Description:    "Inferencia de alta precision y razonamiento estructurado",
		SuggestedModel: []string{"claude-3-5-haiku-latest", "claude-3-5-sonnet-latest"},
		DefaultModel:   "claude-3-5-haiku-latest",
		RequiresKey:    true,
	},
	{
		ID:             ProviderVLLM,
		Name:           "vLLM",
		Description:    "Runtime local de alto rendimiento y baja latencia",
		SuggestedModel: []string{"default"},
		DefaultModel:   "default",
		RequiresKey:    false,
	},
	{
		ID:             ProviderLMStudio,
		Name:           "LM Studio",
		Description:    "Servidor de inferencia local y GUI on-host",
		SuggestedModel: []string{"default"},
		DefaultModel:   "default",
		RequiresKey:    false,
	},
	{
		ID:             ProviderCustom,
		Name:           "Custom OpenAI",
		Description:    "Endpoint HTTP personalizado compatible con OpenAI",
		SuggestedModel: []string{"custom"},
		DefaultModel:   "custom",
		RequiresKey:    false,
	},
}

// ProviderConfig almacena las credenciales y configuracion especifica de un proveedor.
type ProviderConfig struct {
	APIKey       string `json:"api_key,omitempty"`
	Endpoint     string `json:"endpoint,omitempty"`
	DefaultModel string `json:"default_model,omitempty"`
}

// MaskedKey devuelve la clave ofuscada mostrando solo los ultimos 4 caracteres.
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

// AIConfig es la configuracion completa de IA almacenada bajo el blindaje D2 en la boveda cifrada.
type AIConfig struct {
	ActiveProvider AIProvider                    `json:"active_provider"`
	ActiveModel    string                        `json:"active_model"`
	SelectionMode  ModelSelectionMode            `json:"selection_mode,omitempty"`
	SlotPolicy     SlotPolicy                    `json:"slot_policy,omitempty"`
	Providers      map[AIProvider]ProviderConfig `json:"providers"`
}

// DefaultAIConfig genera la configuracion inicial con OpenRouter, Seleccion Auto y Slots por defecto.
func DefaultAIConfig() AIConfig {
	return AIConfig{
		ActiveProvider: ProviderOpenRouter,
		ActiveModel:    "openrouter/free",
		SelectionMode:  SelectionAuto,
		SlotPolicy:     DefaultSlotPolicy(),
		Providers: map[AIProvider]ProviderConfig{
			ProviderAnthropic:  {DefaultModel: "claude-3-5-haiku-latest"},
			ProviderOpenAI:     {DefaultModel: "gpt-4o-mini"},
			ProviderOpenRouter: {DefaultModel: "openrouter/free"},
			ProviderOllama:     {Endpoint: "http://localhost:11434", DefaultModel: "llama3.2"},
			ProviderVLLM:       {Endpoint: "http://localhost:8000", DefaultModel: "default"},
			ProviderLMStudio:   {Endpoint: "http://localhost:1234", DefaultModel: "default"},
			ProviderCustom:     {Endpoint: "http://localhost:8080/v1", DefaultModel: "custom"},
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
