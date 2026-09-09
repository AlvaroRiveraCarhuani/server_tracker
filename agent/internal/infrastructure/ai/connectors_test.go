package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

func TestConnectors_Parsing(t *testing.T) {
	// 1. Ollama Parsing
	ollamaBody := []byte(`{
		"models": [
			{"name": "llama3.2:latest", "model": "llama3.2:latest"},
			{"name": "qwen2.5-coder:7b", "model": "qwen2.5-coder:7b"}
		]
	}`)
	ollamaConn := NewOllamaConnector(nil)
	ollamaModels, err := ollamaConn.ParseModels(ollamaBody)
	if err != nil {
		t.Fatalf("unexpected error parsing Ollama models: %v", err)
	}
	if len(ollamaModels) != 2 {
		t.Fatalf("expected 2 Ollama models, got %d", len(ollamaModels))
	}
	if ollamaModels[0].ID != "llama3.2:latest" || !ollamaModels[0].IsFree {
		t.Errorf("unexpected Ollama model properties: %+v", ollamaModels[0])
	}

	// 2. OpenRouter Parsing
	openRouterBody := []byte(`{
		"data": [
			{
				"id": "google/gemini-2.0-flash-001",
				"name": "Gemini 2.0 Flash",
				"context_length": 1048576,
				"pricing": {"prompt": "0.0000001", "completion": "0.0000004"}
			}
		]
	}`)
	orConn := NewOpenRouterConnector(nil)
	orModels, err := orConn.ParseModels(openRouterBody)
	if err != nil {
		t.Fatalf("unexpected error parsing OpenRouter models: %v", err)
	}
	// Note: ParseModels ensures openrouter/free router is automatically added if missing
	if len(orModels) < 2 {
		t.Fatalf("expected at least 2 models (including free router), got %d", len(orModels))
	}
	if orModels[0].ID != "openrouter/free" {
		t.Errorf("expected first model to be openrouter/free router, got %s", orModels[0].ID)
	}

	// 3. vLLM / LM Studio / OpenAI standard format
	vllmBody := []byte(`{
		"data": [
			{"id": "meta-llama/Llama-3-8B-Instruct"}
		]
	}`)
	vllmConn := NewVLLMConnector(nil)
	vllmModels, err := vllmConn.ParseModels(vllmBody)
	if err != nil {
		t.Fatalf("unexpected error parsing vLLM models: %v", err)
	}
	if len(vllmModels) != 1 || vllmModels[0].ID != "meta-llama/Llama-3-8B-Instruct" {
		t.Errorf("unexpected vLLM model: %+v", vllmModels)
	}
}

func TestConnectors_HealthAndParallelProbing(t *testing.T) {
	// Mock server for local runtime
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/api/tags") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"models":[{"name":"llama3.2:latest"}]}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/v1/models") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"id":"qwen-local"}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	configs := map[domain.AIProvider]domain.ProviderConfig{
		domain.ProviderOllama:   {Endpoint: mockServer.URL},
		domain.ProviderVLLM:     {Endpoint: mockServer.URL},
		domain.ProviderLMStudio: {Endpoint: "http://127.0.0.1:59999"}, // unreachable port
	}

	connectors := []Connector{
		NewOllamaConnector(mockServer.Client()),
		NewVLLMConnector(mockServer.Client()),
		NewLMStudioConnector(mockServer.Client()),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	results := ProbeConnectors(ctx, connectors, configs, mockServer.Client())

	ollamaRes := results[domain.ProviderOllama]
	if ollamaRes.Status != domain.StatusDetected {
		t.Errorf("expected Ollama to be detected, got %v", ollamaRes.Status)
	}
	if len(ollamaRes.Models) != 1 {
		t.Errorf("expected 1 Ollama model discovered, got %d", len(ollamaRes.Models))
	}

	vllmRes := results[domain.ProviderVLLM]
	if vllmRes.Status != domain.StatusDetected {
		t.Errorf("expected vLLM to be detected, got %v", vllmRes.Status)
	}

	lmRes := results[domain.ProviderLMStudio]
	if lmRes.Status != domain.StatusAbsent && lmRes.Status != domain.StatusTimeout {
		t.Errorf("expected LM Studio to be absent or timeout, got %v", lmRes.Status)
	}
}
