package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

func TestCatalogService_BootstrapSeeds(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "solv_catalog_test")
	if err != nil {
		t.Fatalf("error creando tempDir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cacheFile := filepath.Join(tempDir, "non_existent_cache.json")
	cs := NewCatalogService(cacheFile)

	models := cs.AllModels()
	if len(models) != len(domain.BootstrapSeedModels) {
		t.Fatalf("esperaba %d modelos seed iniciales, obtuvo %d", len(domain.BootstrapSeedModels), len(models))
	}

	fastM, fastA := cs.GetSlotFast()
	if fastM.ID == "" || fastA.FastSuitability <= 0 {
		t.Errorf("GetSlotFast devolvio modelo o assessment invalido: %+v", fastM)
	}

	freeM, freeA := cs.GetSlotFree()
	if freeM.ID != "openrouter/free" || !freeM.IsRouter {
		t.Errorf("GetSlotFree debio priorizar openrouter/free como router, obtuvo %+v", freeM)
	}
	if freeA.OverallScore <= 0 {
		t.Errorf("GetSlotFree debio calcular score positivo, obtuvo %f", freeA.OverallScore)
	}
}

func TestCatalogService_P95RollingWindowAndSampleState(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "solv_p95_test")
	if err != nil {
		t.Fatalf("error creando tempDir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cs := NewCatalogService(filepath.Join(tempDir, "cache.json"))
	modelID := "google/gemini-2.0-flash-001"

	// 1. Con menos de 5 muestras, el estado debe ser Estimated
	cs.RecordInference(modelID, 1000*time.Millisecond, true)
	cs.RecordInference(modelID, 1200*time.Millisecond, true)
	cs.RecordInference(modelID, 1100*time.Millisecond, true)

	a1 := cs.GetAssessment(modelID)
	if a1.ObservedMetrics.SampleState != domain.SampleStateEstimated {
		t.Errorf("con 3 muestras el estado debe ser Estimated, obtuvo %v", a1.ObservedMetrics.SampleState)
	}
	if a1.ObservedMetrics.RequestCount != 3 {
		t.Errorf("esperaba 3 requests, obtuvo %d", a1.ObservedMetrics.RequestCount)
	}

	// 2. Agregar 2 muestras mas para alcanzar umbral de significancia (>= 5)
	cs.RecordInference(modelID, 1500*time.Millisecond, true)
	cs.RecordInference(modelID, 2000*time.Millisecond, true)

	a2 := cs.GetAssessment(modelID)
	if a2.ObservedMetrics.SampleState != domain.SampleStateObserved {
		t.Errorf("con 5 muestras el estado debe ser Observed, obtuvo %v", a2.ObservedMetrics.SampleState)
	}
	if a2.ObservedMetrics.P95Latency <= 0 {
		t.Errorf("P95Latency debe ser mayor a 0, obtuvo %v", a2.ObservedMetrics.P95Latency)
	}
	if a2.ObservedMetrics.SuccessRate != 1.0 {
		t.Errorf("esperaba SuccessRate 1.0, obtuvo %f", a2.ObservedMetrics.SuccessRate)
	}
}

func TestCatalogProvider_OpenRouterAndOllamaMock(t *testing.T) {
	// Mock server para OpenRouter
	orServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":             "meta-llama/llama-3.3-70b-instruct:free",
					"name":           "Llama 3.3 70B Instruct (free)",
					"context_length": 131072,
					"architecture": map[string]string{
						"modality": "text->text",
					},
					"pricing": map[string]string{
						"prompt":     "0",
						"completion": "0",
					},
				},
				{
					"id":             "anthropic/claude-3.5-sonnet",
					"name":           "Claude 3.5 Sonnet",
					"context_length": 200000,
					"architecture": map[string]string{
						"modality": "text->text",
					},
					"pricing": map[string]string{
						"prompt":     "0.000003",
						"completion": "0.000015",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer orServer.Close()

	// Mock server para Ollama
	ollamaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"models": []map[string]interface{}{
				{
					"name":  "qwen2.5-coder:latest",
					"model": "qwen2.5-coder:latest",
					"details": map[string]string{
						"family": "qwen",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ollamaServer.Close()

	orProvider := NewOpenRouterCatalogProvider(orServer.Client(), orServer.URL)
	ollamaProvider := NewOllamaCatalogProvider(ollamaServer.Client(), ollamaServer.URL)

	tempDir, err := os.MkdirTemp("", "solv_sync_test")
	if err != nil {
		t.Fatalf("error creando tempDir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cs := NewCatalogService(filepath.Join(tempDir, "catalog.json"), orProvider, ollamaProvider)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = cs.SyncIfExpired(ctx, 0, nil) // TTL 0 fuerza refresco
	if err != nil {
		t.Fatalf("error en SyncIfExpired: %v", err)
	}

	models := cs.AllModels()
	if len(models) < 3 {
		t.Fatalf("esperaba al menos 3 modelos sincronizados (openrouter/free + 2 de OR + 1 de Ollama), obtuvo %d", len(models))
	}

	// Verificar router openrouter/free inyectado
	freeFound := false
	for _, m := range models {
		if m.ID == "openrouter/free" && m.IsRouter && m.Pricing.IsFree {
			freeFound = true
			break
		}
	}
	if !freeFound {
		t.Errorf("openrouter/free no fue encontrado como router gratuito en los modelos sincronizados")
	}

	// Verificar modelo local de Ollama
	ollamaFound := false
	for _, m := range models {
		if m.ProviderID == domain.ProviderOllama && m.DisplayName == "qwen2.5-coder:latest" {
			ollamaFound = true
			break
		}
	}
	if !ollamaFound {
		t.Errorf("modelo de Ollama qwen2.5-coder:latest no fue encontrado")
	}

	// Probar persistencia y recarga
	cs2 := NewCatalogService(filepath.Join(tempDir, "catalog.json"), orProvider, ollamaProvider)
	if len(cs2.AllModels()) != len(models) {
		t.Errorf("esperaba que cs2 cargara %d modelos de la cache, obtuvo %d", len(models), len(cs2.AllModels()))
	}
}
