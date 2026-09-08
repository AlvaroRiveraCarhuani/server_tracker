package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

func TestTriageClient_WithoutAPIKey(t *testing.T) {
	cfg := domain.DefaultAIConfig()
	client := NewTriageClientWithConfig(cfg)
	res := client.DiagnoseContainer(context.Background(), "c1", "img1", "exited", "err log")
	if !strings.Contains(res, "no tiene API Key") && !strings.Contains(res, "no configurado") {
		t.Errorf("expected missing key message, got: %s", res)
	}
}

func TestTriageClient_SuccessfulDiagnosis_OpenRouter(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key-123" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": "Agotamiento de memoria OOMKilled -> Incrementar límites de RAM",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	cfg := domain.DefaultAIConfig()
	cfg.ActiveProvider = domain.ProviderOpenRouter
	p := cfg.Providers[domain.ProviderOpenRouter]
	p.APIKey = "test-key-123"
	p.Endpoint = mockServer.URL
	cfg.Providers[domain.ProviderOpenRouter] = p

	client := NewTriageClientWithConfig(cfg)
	client.client = &http.Client{Timeout: 2 * time.Second}

	res := client.DiagnoseContainer(context.Background(), "solv_db", "postgres:16", "exited", "fatal: out of memory")
	if !strings.Contains(res, "OOMKilled") {
		t.Errorf("expected diagnosis containing OOMKilled, got: %s", res)
	}
}

func TestTriageClient_RateLimited429(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer mockServer.Close()

	cfg := domain.DefaultAIConfig()
	p := cfg.Providers[domain.ProviderOpenRouter]
	p.APIKey = "test-key-123"
	p.Endpoint = mockServer.URL
	cfg.Providers[domain.ProviderOpenRouter] = p

	client := NewTriageClientWithConfig(cfg)
	client.client = &http.Client{Timeout: 2 * time.Second}

	res := client.DiagnoseContainer(context.Background(), "solv_api", "api:v1", "stopped", "")
	if !strings.Contains(res, "429") {
		t.Errorf("expected 429 rate limit message, got: %s", res)
	}
}

func TestTriageClient_SuccessfulDiagnosis_Ollama(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"message": map[string]string{
				"content": "Fallo en migración de base de datos -> Ejecutar rollback",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	cfg := domain.DefaultAIConfig()
	cfg.ActiveProvider = domain.ProviderOllama
	cfg.ActiveModel = "llama3.2"
	p := cfg.Providers[domain.ProviderOllama]
	p.Endpoint = mockServer.URL
	cfg.Providers[domain.ProviderOllama] = p

	client := NewTriageClientWithConfig(cfg)
	client.client = &http.Client{Timeout: 2 * time.Second}

	res := client.DiagnoseContainer(context.Background(), "app", "node:20", "exited", "migration failed")
	if !strings.Contains(res, "rollback") {
		t.Errorf("expected rollback diagnosis, got: %s", res)
	}
}

