package service

import (
	"fmt"
	"strings"
	"sync"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

// ModelPricing define los precios por millón de tokens para modelos curados.
type ModelPricing struct {
	InputPricePerMillion  float64
	OutputPricePerMillion float64
	IsFree                bool
}

// curatedPricingTable contiene únicamente defaults curados embebidos en el binario (S5).
var curatedPricingTable = map[string]ModelPricing{
	"openrouter/free":                        {IsFree: true},
	"meta-llama/llama-3.3-70b-instruct:free": {IsFree: true},
	"google/gemini-2.0-flash-exp:free":       {IsFree: true},
	"google/gemini-2.5-flash:free":           {IsFree: true},
	"anthropic/claude-3.5-sonnet":            {InputPricePerMillion: 3.00, OutputPricePerMillion: 15.00},
	"claude-3-5-sonnet-20241022":            {InputPricePerMillion: 3.00, OutputPricePerMillion: 15.00},
	"openai/gpt-4o":                          {InputPricePerMillion: 2.50, OutputPricePerMillion: 10.00},
	"gpt-4o":                                 {InputPricePerMillion: 2.50, OutputPricePerMillion: 10.00},
	"openai/gpt-4o-mini":                     {InputPricePerMillion: 0.15, OutputPricePerMillion: 0.60},
	"gpt-4o-mini":                            {InputPricePerMillion: 0.15, OutputPricePerMillion: 0.60},
	"deepseek/deepseek-chat":                 {InputPricePerMillion: 0.14, OutputPricePerMillion: 0.28},
}

// SlotMeter mantiene contadores en memoria para un provider+slot.
type SlotMeter struct {
	Requests         int
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	EstimatedCostUSD float64
	HasUnknownPrice  bool
}

// AIMeter gestiona los contadores de sesión de IA estrictamente en memoria (S4).
type AIMeter struct {
	mu    sync.RWMutex
	stats map[string]*SlotMeter // clave: provider:slot
}

// NewAIMeter inicializa un nuevo medidor de sesión.
func NewAIMeter() *AIMeter {
	return &AIMeter{
		stats: make(map[string]*SlotMeter),
	}
}

// Record registra una inferencia de IA con sus tokens y costo calculado.
func (m *AIMeter) Record(
	provider domain.AIProvider,
	slot domain.DiagnosisSlot,
	modelID string,
	usage domain.TokenUsage,
	promptText string,
	responseText string,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	slotName := string(slot)
	if slotName == "" {
		slotName = string(domain.SlotFast)
	}
	key := fmt.Sprintf("%s:%s", provider, slotName)
	meter, ok := m.stats[key]
	if !ok {
		meter = &SlotMeter{}
		m.stats[key] = meter
	}

	meter.Requests++

	pTokens := usage.PromptTokens
	cTokens := usage.CompletionTokens
	if pTokens == 0 && cTokens == 0 {
		// Estimación fallback len(texto)/4 (S5)
		if len(promptText) > 0 {
			pTokens = len(promptText) / 4
			if pTokens == 0 {
				pTokens = 1
			}
		}
		if len(responseText) > 0 {
			cTokens = len(responseText) / 4
			if cTokens == 0 {
				cTokens = 1
			}
		}
	}
	totalTokens := pTokens + cTokens
	if totalTokens == 0 && usage.TotalTokens > 0 {
		totalTokens = usage.TotalTokens
	}

	meter.PromptTokens += pTokens
	meter.CompletionTokens += cTokens
	meter.TotalTokens += totalTokens

	pricing, found := lookupPricing(provider, modelID)
	if !found {
		meter.HasUnknownPrice = true
	} else if !pricing.IsFree {
		cost := (float64(pTokens)/1_000_000.0)*pricing.InputPricePerMillion +
			(float64(cTokens)/1_000_000.0)*pricing.OutputPricePerMillion
		meter.EstimatedCostUSD += cost
	}
}

// TotalRequests retorna la cantidad agregada de solicitudes en la sesión.
func (m *AIMeter) TotalRequests() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := 0
	for _, sm := range m.stats {
		total += sm.Requests
	}
	return total
}

// TotalTokens retorna la cantidad agregada de tokens en la sesión.
func (m *AIMeter) TotalTokens() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := 0
	for _, sm := range m.stats {
		total += sm.TotalTokens
	}
	return total
}

// TotalCost retorna el costo total estimado acumulado en la sesión.
func (m *AIMeter) TotalCost() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := 0.0
	for _, sm := range m.stats {
		total += sm.EstimatedCostUSD
	}
	return total
}

// HasUnknownPricingSession indica si en la sesión se usó algún modelo sin precio conocido.
func (m *AIMeter) HasUnknownPricingSession() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, sm := range m.stats {
		if sm.HasUnknownPrice {
			return true
		}
	}
	return false
}

// GetProviderStats consolida las estadísticas de todos los slots para un proveedor dado.
func (m *AIMeter) GetProviderStats(provider domain.AIProvider) SlotMeter {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var agg SlotMeter
	prefix := fmt.Sprintf("%s:", provider)
	for k, sm := range m.stats {
		if strings.HasPrefix(k, prefix) {
			agg.Requests += sm.Requests
			agg.PromptTokens += sm.PromptTokens
			agg.CompletionTokens += sm.CompletionTokens
			agg.TotalTokens += sm.TotalTokens
			agg.EstimatedCostUSD += sm.EstimatedCostUSD
			if sm.HasUnknownPrice {
				agg.HasUnknownPrice = true
			}
		}
	}
	return agg
}

// FormatTokens convierte un conteo de tokens a formato legible con sufijo 'k' si supera 1000.
func FormatTokens(tokens int) string {
	if tokens >= 1000 {
		return fmt.Sprintf("%.1fk", float64(tokens)/1000.0)
	}
	return fmt.Sprintf("%d", tokens)
}

// FormatStatusBar genera el segmento de sesión para la barra de estado inferior.
// Ejemplos:
//   sesión: 47 req · ~$0.12
//   sesión: 3 req · ~$0.00 (modelos free)
//   sesión: 12 req · 4.2k tok (si hay modelo desconocido sin precio)
func (m *AIMeter) FormatStatusBar() string {
	req := m.TotalRequests()
	cost := m.TotalCost()
	hasUnknown := m.HasUnknownPricingSession()

	if hasUnknown {
		tok := m.TotalTokens()
		return fmt.Sprintf("sesión: %d req · %s tok", req, FormatTokens(tok))
	}
	return fmt.Sprintf("sesión: %d req · ~$%.2f", req, cost)
}

// FormatProviderStats genera el texto de sesión para una fila de proveedor en V3.
// Ejemplos:
//   sesión: 12 req · 8.4k tok · ~$0.03
//   sesión: 5 req · 1.2k tok (sin precio conocido)
func (m *AIMeter) FormatProviderStats(provider domain.AIProvider) string {
	agg := m.GetProviderStats(provider)
	if agg.Requests == 0 {
		return ""
	}

	tokStr := FormatTokens(agg.TotalTokens)
	if agg.HasUnknownPrice {
		return fmt.Sprintf("sesión: %d req · %s tok", agg.Requests, tokStr)
	}
	return fmt.Sprintf("sesión: %d req · %s tok · ~$%.2f", agg.Requests, tokStr, agg.EstimatedCostUSD)
}

// lookupPricing busca precios en la tabla curada o determina gratuidad para local/free.
func lookupPricing(provider domain.AIProvider, modelID string) (ModelPricing, bool) {
	// Modelos locales y free siempre son costo $0 (S5)
	if provider == domain.ProviderOllama || provider == domain.ProviderVLLM || provider == domain.ProviderLMStudio {
		return ModelPricing{IsFree: true}, true
	}
	if strings.HasSuffix(modelID, ":free") || strings.Contains(modelID, "/free") {
		return ModelPricing{IsFree: true}, true
	}

	// Buscar exacto o por sufijo en tabla curada
	if p, ok := curatedPricingTable[modelID]; ok {
		return p, true
	}
	for k, p := range curatedPricingTable {
		if strings.HasSuffix(modelID, k) || strings.HasSuffix(k, modelID) {
			return p, true
		}
	}

	return ModelPricing{}, false
}
