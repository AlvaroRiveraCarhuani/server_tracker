package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTriageClient_WithoutAPIKey(t *testing.T) {
	client := &TriageClient{apiKey: ""}
	res := client.DiagnoseContainer(context.Background(), "c1", "img1", "exited", "err log")
	if !strings.Contains(res, "no presente") {
		t.Errorf("expected missing key message, got: %s", res)
	}
}

func TestTriageClient_SuccessfulDiagnosis(t *testing.T) {
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

	client := &TriageClient{
		apiKey:  "test-key-123",
		model:   "test-model",
		baseURL: mockServer.URL,
		client:  &http.Client{Timeout: 2 * time.Second},
	}

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

	client := &TriageClient{
		apiKey:  "test-key-123",
		model:   "test-model",
		baseURL: mockServer.URL,
		client:  &http.Client{Timeout: 2 * time.Second},
	}

	res := client.DiagnoseContainer(context.Background(), "solv_api", "api:v1", "stopped", "")
	if !strings.Contains(res, "429") {
		t.Errorf("expected 429 rate limit message, got: %s", res)
	}
}
