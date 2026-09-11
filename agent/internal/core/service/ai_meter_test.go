package service

import (
	"strings"
	"testing"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

func TestAIMeter_InitialState(t *testing.T) {
	meter := NewAIMeter()
	if meter.TotalRequests() != 0 {
		t.Errorf("expected 0 requests, got %d", meter.TotalRequests())
	}
	if meter.TotalCost() != 0.0 {
		t.Errorf("expected 0 cost, got %f", meter.TotalCost())
	}
	sb := meter.FormatStatusBar()
	if sb != "sesión: 0 req · ~$0.00" {
		t.Errorf("expected 'sesión: 0 req · ~$0.00', got %q", sb)
	}
}

func TestAIMeter_FreeModels(t *testing.T) {
	meter := NewAIMeter()

	meter.Record(
		domain.ProviderOpenRouter,
		domain.SlotFast,
		"openrouter/free",
		domain.TokenUsage{PromptTokens: 500, CompletionTokens: 100, TotalTokens: 600},
		"",
		"",
	)

	if meter.TotalRequests() != 1 {
		t.Fatalf("expected 1 request, got %d", meter.TotalRequests())
	}
	if meter.TotalCost() != 0.0 {
		t.Errorf("free model must have 0.0 cost, got %f", meter.TotalCost())
	}
	sb := meter.FormatStatusBar()
	if sb != "sesión: 1 req · ~$0.00" {
		t.Errorf("expected 'sesión: 1 req · ~$0.00', got %q", sb)
	}

	provStats := meter.FormatProviderStats(domain.ProviderOpenRouter)
	if !strings.Contains(provStats, "sesión: 1 req · 600 tok · ~$0.00") {
		t.Errorf("expected 'sesión: 1 req · 600 tok · ~$0.00', got %q", provStats)
	}
}

func TestAIMeter_CuratedPaidModel(t *testing.T) {
	meter := NewAIMeter()

	meter.Record(
		domain.ProviderAnthropic,
		domain.SlotDeep,
		"claude-3-5-sonnet-20241022",
		domain.TokenUsage{PromptTokens: 10000, CompletionTokens: 2000, TotalTokens: 12000},
		"",
		"",
	)

	// 10k prompt @ $3/M = $0.03, 2k completion @ $15/M = $0.03 -> Total $0.06
	expectedCost := 0.06
	if meter.TotalCost() < expectedCost-0.001 || meter.TotalCost() > expectedCost+0.001 {
		t.Errorf("expected cost ~%f, got %f", expectedCost, meter.TotalCost())
	}

	sb := meter.FormatStatusBar()
	if !strings.Contains(sb, "sesión: 1 req · ~$0.06") {
		t.Errorf("expected 'sesión: 1 req · ~$0.06', got %q", sb)
	}

	provStats := meter.FormatProviderStats(domain.ProviderAnthropic)
	if !strings.Contains(provStats, "12.0k tok · ~$0.06") {
		t.Errorf("expected 12.0k tok in provider stats, got %q", provStats)
	}
}

func TestAIMeter_UnknownModel_OmitsDollarSign(t *testing.T) {
	meter := NewAIMeter()

	meter.Record(
		domain.ProviderCustom,
		domain.SlotFast,
		"my-custom-unpriced-llm",
		domain.TokenUsage{PromptTokens: 200, CompletionTokens: 50, TotalTokens: 250},
		"",
		"",
	)

	if !meter.HasUnknownPricingSession() {
		t.Errorf("expected HasUnknownPricingSession to be true")
	}

	sb := meter.FormatStatusBar()
	if strings.Contains(sb, "$") {
		t.Errorf("unknown model MUST NOT display '$', got %q", sb)
	}
	if !strings.Contains(sb, "sesión: 1 req · 250 tok") {
		t.Errorf("expected 'sesión: 1 req · 250 tok', got %q", sb)
	}

	provStats := meter.FormatProviderStats(domain.ProviderCustom)
	if strings.Contains(provStats, "$") {
		t.Errorf("provider stats for unknown model MUST NOT display '$', got %q", provStats)
	}
	if !strings.Contains(provStats, "sesión: 1 req · 250 tok") {
		t.Errorf("expected 'sesión: 1 req · 250 tok', got %q", provStats)
	}
}

func TestAIMeter_FallbackTokenEstimation(t *testing.T) {
	meter := NewAIMeter()

	prompt := "Este es un prompt de prueba que mide exactamente cuarenta chars" // 64 bytes
	resp := "Respuesta breve"                                                    // 15 bytes

	meter.Record(
		domain.ProviderOpenRouter,
		domain.SlotFast,
		"openrouter/free",
		domain.TokenUsage{}, // usage vacío
		prompt,
		resp,
	)

	if meter.TotalTokens() == 0 {
		t.Errorf("expected fallback token estimation, got 0")
	}
	// prompt: 63/4 = 15, resp: 15/4 = 3 -> total 18
	if meter.TotalTokens() != 18 {
		t.Errorf("expected 18 estimated tokens, got %d", meter.TotalTokens())
	}
}
